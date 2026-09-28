// Package openai implements the OpenAI-compatible /chat/completions provider.
// It self-registers under the "openai" kind, so DeepSeek, MiMo, MiniMax-M3, and
// any other OpenAI-compatible endpoint are just config instances rather than
// code. Each instance picks the wire shape from its base URL:
//   - api.deepseek.com → emits thinking.type=enabled (DeepSeek-flavor CoT) plus
//     reasoning_effort as a depth hint.
//   - api.minimaxi.com → emits thinking.type=adaptive|disabled (M3's binary
//     knob) instead of reasoning_effort, since M3 has no level scale.
//   - everything else (MiMo and other OpenAI-compatible gateways) uses the
//     vanilla reasoning_effort scale (low/medium/high).
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kwu/internal/netclient"
	"kwu/internal/provider"
)

// The stream watchdog keeps transport activity, semantic model progress, the
// physical request wall clock, and post-finish cleanup separate. Heartbeats may
// keep a healthy connection alive, but cannot keep a request with no model
// progress alive forever. Values live on each client so tests can shorten them
// without racing other streams.
const (
	defaultStreamIdleTimeout     = 120 * time.Second
	defaultStreamProgressTimeout = 5 * time.Minute
	defaultStreamRequestTimeout  = 10 * time.Minute
	defaultStreamFinishGrace     = 5 * time.Second
	maxNoProgressStreamReplays   = 1
)

var (
	errStreamNoProgress     = errors.New("stream made no semantic progress")
	errStreamRequestTimeout = errors.New("stream request wall-clock timeout")
)

func init() {
	provider.Register("openai", New)
}

// New builds an OpenAI-compatible provider from a resolved config.
func New(cfg provider.Config) (provider.Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("openai: base_url is required for provider %q", cfg.Name)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("openai: model is required for provider %q", cfg.Name)
	}
	name := cfg.Name
	if name == "" {
		name = "openai"
	}
	keyEnv, _ := cfg.Extra["api_key_env"].(string) // for actionable auth errors
	keySource, _ := cfg.Extra["api_key_source"].(string)
	keyOwner, _ := cfg.Extra["credential_owner"].(string)
	effort, _ := cfg.Extra["effort"].(string)
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "auto" {
		effort = ""
	}
	protocol, _ := cfg.Extra["reasoning_protocol"].(string)
	protocol = normalizeReasoningProtocol(protocol)
	vision, _ := cfg.Extra["vision"].(bool)
	includeBilling, _ := cfg.Extra["include_billing"].(bool)
	visionDetail, _ := cfg.Extra["vision_detail"].(string)
	visionDetail = strings.ToLower(strings.TrimSpace(visionDetail))
	if visionDetail != "low" && visionDetail != "high" {
		visionDetail = "" // auto — omit the field
	}
	deepseek := protocol == "deepseek" || (protocol == "" && IsDeepSeek(cfg.BaseURL))
	minimax := protocol == "" && IsMiniMax(cfg.BaseURL)
	switch {
	case protocol == "none":
		effort = ""
	case deepseek:
		switch effort {
		case "", "off": // "off" is a retired level (disabled thinking); fall back to the default depth
			effort = "high"
		case "high", "max":
		default:
			return nil, fmt.Errorf("openai: provider %q uses DeepSeek thinking; effort must be high or max", name)
		}
	case minimax:
		// M3's knob is binary. The config effort layer normalises user input
		// to "adaptive", "disabled", or "" (== auto). We keep "high"/"max"
		// (legacy DeepSeek) and "low"/"medium" (Anthropic) out — config-level
		// NormalizeEffort remaps them to "adaptive" already, so anything
		// reaching here is expected to be one of: "", "adaptive", "disabled".
		effort = strings.ToLower(strings.TrimSpace(effort))
		switch effort {
		case "": // auto — leave empty so the wire emits thinking.type=adaptive
		case "adaptive", "disabled":
		default:
			return nil, fmt.Errorf("openai: provider %q uses MiniMax thinking; effort must be adaptive or disabled", name)
		}
	case effort != "":
		// Non-DeepSeek backends use OpenAI's reasoning_effort scale (low/medium/
		// high); "max" is a DeepSeek-ism MiMo et al. reject with 400, so clamp it
		// to the OpenAI ceiling and reject other values at boot, not at request time.
		switch effort {
		case "max":
			effort = "high"
		case "low", "medium", "high":
		default:
			return nil, fmt.Errorf("openai: provider %q: effort must be low, medium, or high", name)
		}
	}
	httpClient, err := newHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("openai: network: %w", err)
	}
	return &client{
		name:           name,
		apiKey:         cfg.APIKey,
		keyEnv:         keyEnv,
		keySource:      keySource,
		keyOwner:       keyOwner,
		baseURL:        strings.TrimRight(cfg.BaseURL, "/"),
		model:          cfg.Model,
		deepseek:       deepseek,
		minimax:        minimax,
		vision:         vision,
		includeBilling: includeBilling,
		visionDetail:   visionDetail,
		effort:         effort,
		http:           httpClient,
	}, nil
}

func newHTTPClient(cfg provider.Config) (*http.Client, error) {
	spec, _ := cfg.Extra["proxy_spec"].(netclient.ProxySpec)
	return netclient.NewHTTPClient(spec, netclient.TransportOptions{
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second, // models can think for a while before the first token
	})
}

type client struct {
	includeBilling  bool
	name            string
	apiKey          string
	keyEnv          string // api_key_env name, surfaced in auth errors
	keySource       string // source of keyEnv, surfaced in auth errors
	keyOwner        string // credential owner, surfaced in auth errors
	baseURL         string
	model           string
	http            *http.Client
	deepseek        bool
	minimax         bool          // true for api.minimaxi.com — emits MiniMax-M3's thinking knob instead of reasoning_effort
	vision          bool          // model accepts image input — embed attached images as image_url parts
	visionDetail    string        // image_url detail hint (low|high); "" = auto/omit
	effort          string        // reasoning_effort for OpenAI; thinking.type for MiniMax; "" = auto/provider default
	idleTimeout     time.Duration // SSE transport-idle watchdog; defaults unless a test overrides
	progressTimeout time.Duration // semantic no-progress watchdog; heartbeat lines do not refresh it
	requestTimeout  time.Duration // absolute wall-clock limit for one physical model request
	finishGrace     time.Duration // grace after finish_reason when a gateway omits [DONE]
	authed          atomic.Bool   // a request has succeeded — gate transient-401 retry
}

func (c *client) Name() string { return c.name }

func (c *client) sendOpts() provider.SendOptions {
	return provider.SendOptions{
		Provider:   c.name,
		KeyEnv:     c.keyEnv,
		KeySource:  c.keySource,
		KeyOwner:   c.keyOwner,
		KeyPresent: c.apiKey != "",
		RetryAuth:  c.authed.Load(),
	}
}

func normalizeReasoningProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "deepseek", "openai", "none":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

// bufPool reuses byte buffers for JSON-marshalled request bodies. Each turn
// allocates a buffer, marshals the request, and sends it — pooling avoids the
// GC churn from repeated alloc/free of ~10-100KB buffers. The pool is
// provider-level (not global) so OpenAI and Anthropic don't compete.
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func (c *client) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if err := json.NewEncoder(buf).Encode(c.buildRequest(req)); err != nil {
		bufPool.Put(buf)
		return nil, fmt.Errorf("%s: marshal request: %w", c.name, err)
	}
	body := make([]byte, buf.Len())
	copy(body, buf.Bytes())
	bufPool.Put(buf)

	newReq := func(ctx context.Context) (*http.Request, error) {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		if c.includeBilling {
			httpReq.Header.Set("X-Include-Billing", "true")
		}
		httpReq.Header.Set("Accept", "text/event-stream")
		return httpReq, nil
	}
	requestStarted := time.Now()
	resp, err := provider.SendWithRetry(ctx, c.http, c.sendOpts(), newReq)
	if err != nil {
		return nil, err
	}
	c.authed.Store(true)

	out := make(chan provider.Chunk)
	go c.streamWithReconnect(ctx, resp, newReq, out, requestStarted)
	return out, nil
}

// maxStreamReconnects bounds how many times a mid-stream connection drop is
// replayed from scratch before the error is surfaced — each replay re-runs the
// whole request (cheap under prompt caching, but not free).
const maxStreamReconnects = 3

// streamWithReconnect drives readStream and, when the connection is cut before
// any model output has been forwarded, replays the request rather than failing
// the turn. Once a token (reasoning/text/tool-call) has been emitted, a replay
// would duplicate output, so the error is surfaced instead.
func (c *client) streamWithReconnect(ctx context.Context, resp *http.Response, newReq func(context.Context) (*http.Request, error), out chan<- provider.Chunk, initialStarted ...time.Time) {
	defer close(out)
	requestStarted := time.Now()
	if len(initialStarted) > 0 && !initialStarted[0].IsZero() {
		requestStarted = initialStarted[0]
	}
	connResetReplays := 0
	noProgressReplays := 0
	for {
		emitted, err := c.readStream(ctx, resp, out, requestStarted)
		if err == nil {
			return
		}
		replay, terminalErr := streamReplayDecision(err, emitted, &connResetReplays, &noProgressReplays)
		if !replay {
			sendTerminalChunk(out, provider.Chunk{Type: provider.ChunkError, Err: terminalErr})
			return
		}
		requestStarted = time.Now()
		next, rerr := provider.SendWithRetry(ctx, c.http, c.sendOpts(), newReq)
		if rerr != nil {
			sendTerminalChunk(out, provider.Chunk{Type: provider.ChunkError, Err: rerr})
			return
		}
		resp = next
	}
}

func isBoundedStreamTimeout(err error) bool {
	return errors.Is(err, errStreamNoProgress) || errors.Is(err, errStreamRequestTimeout)
}

func streamReplayDecision(err error, emitted bool, connResetReplays, noProgressReplays *int) (bool, error) {
	if emitted && (isBoundedStreamTimeout(err) || provider.IsConnReset(err)) {
		return false, &provider.StreamInterruptedError{Err: err}
	}
	if isBoundedStreamTimeout(err) {
		return allowStreamReplay(err, noProgressReplays, maxNoProgressStreamReplays)
	}
	if provider.IsConnReset(err) {
		return allowStreamReplay(err, connResetReplays, maxStreamReconnects)
	}
	return false, err
}

func allowStreamReplay(err error, replays *int, limit int) (bool, error) {
	if *replays >= limit {
		return false, err
	}
	*replays++
	return true, nil
}

func sendTerminalChunk(out chan<- provider.Chunk, chunk provider.Chunk) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sendChunk(ctx, out, chunk)
}

func sendChunk(ctx context.Context, out chan<- provider.Chunk, chunk provider.Chunk) bool {
	select {
	case out <- chunk:
		return true
	default:
	}
	select {
	case <-ctx.Done():
		return false
	case out <- chunk:
		return true
	}
}

func (c *client) buildRequest(req provider.Request) chatRequest {
	// Repair tool-call pairing before sending: an interrupted/resumed history can
	// carry an assistant tool_calls turn whose results never landed, which DeepSeek
	// rejects with a 400 ("must be followed by tool messages …").
	src := provider.SanitizeToolPairing(req.Messages)
	msgs := make([]chatMessage, len(src))
	for i, m := range src {
		cm := chatMessage{
			Role:       string(m.Role),
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		// DeepSeek thinking mode 400s a tool_calls turn whose reasoning_content was
		// dropped on a cache-miss replay ("reasoning_content … must be passed back"),
		// so round it back — but only on the turn that carries the tool calls.
		if c.deepseek && m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
			cm.ReasoningContent = m.ReasoningContent
		}
		for _, tc := range m.ToolCalls {
			wire := chatToolCall{ID: tc.ID, Type: "function"}
			wire.Function.Name = tc.Name
			wire.Function.Arguments = tc.Arguments
			cm.ToolCalls = append(cm.ToolCalls, wire)
		}
		switch {
		case c.vision && m.Role == provider.RoleUser && len(m.Images) > 0:
			cm.Content = imageContentParts(m.Content, m.Images, c.visionDetail)
		case m.Role != provider.RoleAssistant || len(cm.ToolCalls) == 0 || m.Content != "":
			cm.Content = m.Content
		}
		msgs[i] = cm
	}

	var tools []chatTool
	for _, t := range req.Tools {
		tools = append(tools, chatTool{
			Type:     "function",
			Function: chatFunction{Name: t.Name, Description: t.Description, Parameters: t.Parameters},
		})
	}

	out := chatRequest{
		Model:           c.model,
		Messages:        msgs,
		Tools:           tools,
		Stream:          true,
		StreamOptions:   &streamOptions{IncludeUsage: true},
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
		ReasoningEffort: c.effort,
	}
	switch {
	case c.deepseek:
		// DeepSeek's CoT is controlled by `thinking` (always on) plus
		// `reasoning_effort` for depth. We never disable thinking for DeepSeek.
		out.Thinking = &thinkingMode{Type: "enabled"}
	case c.minimax:
		// M3 uses a single `thinking.type` field with two valid values:
		// "adaptive" (default, thinking on) and "disabled" (off). Reasoning
		// depth is not a knob on M3, so reasoning_effort is omitted entirely.
		t := c.effort
		if t == "" {
			t = "adaptive" // /effort auto == the M3 model default
		}
		out.Thinking = &thinkingMode{Type: t}
		out.ReasoningEffort = ""
	}
	return out
}

// readStream parses one SSE response into chunks: text deltas stream live,
// tool-call fragments accumulate by index and emit complete on [DONE], and a
// ChunkToolCallStart fires the moment a call's name is known. It returns whether
// any model output was forwarded (so the caller can decide a replay is safe) and
// the first fatal error — a nil error means the stream reached [DONE].
func (c *client) readStream(ctx context.Context, resp *http.Response, out chan<- provider.Chunk, requestStarted ...time.Time) (emitted bool, _ error) {
	defer resp.Body.Close()

	started := time.Now()
	if len(requestStarted) > 0 && !requestStarted[0].IsZero() {
		started = requestStarted[0]
	}
	watchdog := startStreamWatchdog(ctx, resp.Body, streamWatchdogConfig{
		transportIdle:  c.streamIdleTimeout(),
		progressIdle:   c.streamProgressTimeout(),
		requestLimit:   c.streamRequestTimeout(),
		finishGrace:    c.streamFinishGrace(),
		requestStarted: started,
	})
	defer watchdog.stop()

	state := newOpenAIStreamState(c.includeBilling, resp.Header)
	scanner := newStreamScanner(resp.Body)
	for scanner.Scan() {
		watchdog.transportActive()
		line, err := state.handleSSELine(ctx, c.name, out, scanner.Text())
		if err != nil {
			return state.emitted, err
		}
		if line.progress {
			watchdog.semanticProgress()
		}
		if line.finish {
			watchdog.finishSeen()
		}
		if line.stop {
			break
		}
	}
	if err := state.finish(ctx, c.name, out, scanner, watchdog.reason(), watchdog.config); err != nil {
		return state.emitted, err
	}
	return state.emitted, nil
}

func (c *client) streamIdleTimeout() time.Duration {
	if c.idleTimeout > 0 {
		return c.idleTimeout
	}
	return defaultStreamIdleTimeout
}

func (c *client) streamProgressTimeout() time.Duration {
	if c.progressTimeout > 0 {
		return c.progressTimeout
	}
	return defaultStreamProgressTimeout
}

func (c *client) streamRequestTimeout() time.Duration {
	if c.requestTimeout > 0 {
		return c.requestTimeout
	}
	return defaultStreamRequestTimeout
}

func (c *client) streamFinishGrace() time.Duration {
	if c.finishGrace > 0 {
		return c.finishGrace
	}
	return defaultStreamFinishGrace
}

func newStreamScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

type openAIStreamState struct {
	emitted          bool
	acc              map[int]*provider.ToolCall
	started          map[int]bool
	order            []int
	lastFinishReason string
	sawDone          bool
	reasoning        reasoningPrefixSplitter
	includeBilling   bool
	billingEvent     string
	billingEvidence  *provider.BillingEvidence
}

func newOpenAIStreamState(includeBilling bool, headers http.Header) *openAIStreamState {
	state := &openAIStreamState{
		acc:            map[int]*provider.ToolCall{},
		started:        map[int]bool{},
		includeBilling: includeBilling,
	}
	if includeBilling {
		state.billingEvidence = emptyBillingEvidence(headers)
	}
	return state
}

func (s *openAIStreamState) handleStreamResponse(ctx context.Context, name string, out chan<- provider.Chunk, sr streamResponse) error {
	if sr.Error != nil {
		code := strings.TrimSpace(sr.Error.Code)
		if code == "" {
			code = strings.TrimSpace(sr.Error.Type)
		}
		return &provider.APIError{Provider: name, Code: code, Message: strings.TrimSpace(sr.Error.Message)}
	}
	s.rememberFinishReason(sr)
	if sr.Usage != nil {
		if err := s.emitUsage(ctx, out, sr.Usage); err != nil {
			return err
		}
	}
	if len(sr.Choices) == 0 {
		return nil
	}
	return s.emitDelta(ctx, out, sr.Choices[0].Delta)
}

func (s *openAIStreamState) rememberFinishReason(sr streamResponse) {
	if len(sr.Choices) == 0 || sr.Choices[0].FinishReason == nil {
		return
	}
	if reason := *sr.Choices[0].FinishReason; reason != "" {
		s.lastFinishReason = reason
	}
}

func (s *openAIStreamState) emitUsage(ctx context.Context, out chan<- provider.Chunk, usage *wireUsage) error {
	u := normaliseUsage(usage)
	u.FinishReason = s.lastFinishReason
	return s.emitModelChunk(ctx, out, provider.Chunk{Type: provider.ChunkUsage, Usage: u})
}

func (s *openAIStreamState) emitDelta(ctx context.Context, out chan<- provider.Chunk, delta streamDelta) error {
	if err := s.emitReasoningDelta(ctx, out, delta); err != nil {
		return err
	}
	if err := s.emitContentDelta(ctx, out, delta.Content); err != nil {
		return err
	}
	return s.collectToolCalls(ctx, out, delta.ToolCalls)
}

func (s *openAIStreamState) emitReasoningDelta(ctx context.Context, out chan<- provider.Chunk, delta streamDelta) error {
	reasoningDelta := delta.ReasoningContent
	if reasoningDelta == "" {
		reasoningDelta = delta.Reasoning
	}
	if reasoningDelta == "" {
		return nil
	}
	return s.emitModelChunk(ctx, out, provider.Chunk{Type: provider.ChunkReasoning, Text: reasoningDelta})
}

func (s *openAIStreamState) emitContentDelta(ctx context.Context, out chan<- provider.Chunk, content string) error {
	if content == "" {
		return nil
	}
	reasoning, text := s.reasoning.push(content)
	if reasoning != "" {
		if err := s.emitModelChunk(ctx, out, provider.Chunk{Type: provider.ChunkReasoning, Text: reasoning}); err != nil {
			return err
		}
	}
	if text != "" {
		return s.emitModelChunk(ctx, out, provider.Chunk{Type: provider.ChunkText, Text: text})
	}
	return nil
}

func (s *openAIStreamState) collectToolCalls(ctx context.Context, out chan<- provider.Chunk, calls []chatToolCall) error {
	for _, tc := range calls {
		cur := s.toolCall(tc.Index)
		updateToolCall(cur, tc)
		if err := s.maybeEmitToolCallStart(ctx, out, tc.Index, cur); err != nil {
			return err
		}
	}
	return nil
}

func (s *openAIStreamState) toolCall(index int) *provider.ToolCall {
	if cur, ok := s.acc[index]; ok {
		return cur
	}
	cur := &provider.ToolCall{}
	s.acc[index] = cur
	s.order = append(s.order, index)
	return cur
}

func updateToolCall(cur *provider.ToolCall, tc chatToolCall) {
	if tc.ID != "" {
		cur.ID = tc.ID
	}
	if tc.Function.Name != "" {
		cur.Name = tc.Function.Name
	}
	cur.Arguments += tc.Function.Arguments
}

func (s *openAIStreamState) maybeEmitToolCallStart(ctx context.Context, out chan<- provider.Chunk, index int, cur *provider.ToolCall) error {
	if s.started[index] || cur.Name == "" {
		return nil
	}
	s.started[index] = true
	start := &provider.ToolCall{ID: cur.ID, Name: cur.Name}
	return s.emitModelChunk(ctx, out, provider.Chunk{Type: provider.ChunkToolCallStart, ToolCall: start})
}

func (s *openAIStreamState) emitModelChunk(ctx context.Context, out chan<- provider.Chunk, chunk provider.Chunk) error {
	s.emitted = true
	if !sendChunk(ctx, out, chunk) {
		return ctx.Err()
	}
	return nil
}

func (s *openAIStreamState) finish(ctx context.Context, name string, out chan<- provider.Chunk, scanner *bufio.Scanner, reason streamWatchdogReason, config streamWatchdogConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := streamWatchdogError(name, reason, config, s.lastFinishReason); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil && reason != streamWatchdogFinishGrace {
		return fmt.Errorf("%s: read stream: %w", name, err)
	}
	if !s.sawDone && s.lastFinishReason == "" {
		return fmt.Errorf("%s: stream ended before completion: %w", name, io.ErrUnexpectedEOF)
	}
	if err := s.flushReasoning(ctx, out); err != nil {
		return err
	}
	if err := s.emitToolCalls(ctx, out); err != nil {
		return err
	}
	return s.emitDone(ctx, out)
}

func streamWatchdogError(name string, reason streamWatchdogReason, config streamWatchdogConfig, finishReason string) error {
	switch reason {
	case streamWatchdogTransportIdle:
		return fmt.Errorf("%s: %w — no data for %s, connection likely dropped", name, provider.ErrStreamStalled, config.transportIdle)
	case streamWatchdogNoProgress:
		return fmt.Errorf("%s: %w for %s", name, errStreamNoProgress, config.progressIdle)
	case streamWatchdogRequestLimit:
		return fmt.Errorf("%s: %w after %s", name, errStreamRequestTimeout, config.requestLimit)
	case streamWatchdogFinishGrace:
		if finishReason == "" {
			return fmt.Errorf("%s: finish grace expired without finish_reason: %w", name, io.ErrUnexpectedEOF)
		}
	}
	return nil
}

func (s *openAIStreamState) flushReasoning(ctx context.Context, out chan<- provider.Chunk) error {
	reasoning, text := s.reasoning.flush()
	if reasoning != "" && !sendChunk(ctx, out, provider.Chunk{Type: provider.ChunkReasoning, Text: reasoning}) {
		return ctx.Err()
	}
	if text != "" && !sendChunk(ctx, out, provider.Chunk{Type: provider.ChunkText, Text: text}) {
		return ctx.Err()
	}
	return nil
}

func (s *openAIStreamState) emitToolCalls(ctx context.Context, out chan<- provider.Chunk) error {
	sort.Ints(s.order)
	for _, idx := range s.order {
		tc := s.acc[idx]
		if tc.ID == "" {
			tc.ID = fmt.Sprintf("call_%d", idx)
		}
		if !sendChunk(ctx, out, provider.Chunk{Type: provider.ChunkToolCall, ToolCall: tc}) {
			return ctx.Err()
		}
	}
	return nil
}

func (s *openAIStreamState) emitDone(ctx context.Context, out chan<- provider.Chunk) error {
	chunk := provider.Chunk{Type: provider.ChunkDone}
	if s.includeBilling {
		chunk.BillingEvidence = cloneBillingEvidence(s.billingEvidence)
	}
	if !sendChunk(ctx, out, chunk) {
		return ctx.Err()
	}
	return nil
}

// ParseBillingEvidence extracts the safe billing subset from a non-streaming
// OpenAI-compatible JSON response. Calling it is the explicit capability gate;
// ordinary provider responses remain unchanged.
func ParseBillingEvidence(headers http.Header, responseJSON json.RawMessage) (*provider.BillingEvidence, error) {
	finalJSON, _, err := billingOnlyJSON(responseJSON)
	if err != nil {
		return nil, err
	}
	evidence := emptyBillingEvidence(headers)
	evidence.FinalJSON = finalJSON
	return evidence, nil
}

func emptyBillingEvidence(headers http.Header) *provider.BillingEvidence {
	requestIDs := append([]string(nil), headers.Values("X-Oneapi-Request-Id")...)
	return &provider.BillingEvidence{
		ProviderRequestIDs: requestIDs,
		FinalJSON:          json.RawMessage(`{}`),
	}
}

func cloneBillingEvidence(source *provider.BillingEvidence) *provider.BillingEvidence {
	if source == nil {
		return nil
	}
	return &provider.BillingEvidence{
		ProviderRequestIDs: append([]string(nil), source.ProviderRequestIDs...),
		FinalJSON:          append(json.RawMessage(nil), source.FinalJSON...),
	}
}

func billingOnlyJSON(responseJSON json.RawMessage) (json.RawMessage, bool, error) {
	if len(responseJSON) == 0 || !json.Valid(responseJSON) {
		return nil, false, fmt.Errorf("response billing must be valid JSON")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(responseJSON, &root); err != nil || root == nil {
		return nil, false, fmt.Errorf("response billing must be a JSON object")
	}
	billing, found := root["billing"]
	if !found {
		return json.RawMessage(`{}`), false, nil
	}
	finalJSON, err := json.Marshal(map[string]json.RawMessage{"billing": billing})
	if err != nil {
		return nil, false, fmt.Errorf("encode response billing: %w", err)
	}
	return finalJSON, true, nil
}

// normaliseUsage folds the two cache-hit shapes the OpenAI-compatible ecosystem
// uses into a single Usage: DeepSeek puts prompt_cache_{hit,miss}_tokens at the
// top of usage; OpenAI and MiMo put it nested under prompt_tokens_details.
// Whichever side reports non-zero wins; miss is derived when only hit is given.
// Reasoning tokens land in completion_tokens_details on thinking-mode models.
func normaliseUsage(u *wireUsage) *provider.Usage {
	hit := u.PromptCacheHitTokens
	miss := u.PromptCacheMissTokens
	if hit == 0 && u.PromptTokensDetails != nil {
		hit = u.PromptTokensDetails.CachedTokens
	}
	if miss == 0 && hit > 0 && u.PromptTokens > hit {
		miss = u.PromptTokens - hit
	}
	reasoning := 0
	if u.CompletionTokensDetails != nil {
		reasoning = u.CompletionTokensDetails.ReasoningTokens
	}
	presence := provider.UsagePresence{Input: u.promptTokensPresent, Output: u.completionTokensPresent, ProviderTotal: u.totalTokensPresent}
	if u.promptCacheHitTokensPresent || u.promptTokensDetailsCachedPresent {
		presence.CachedInput, presence.CacheRead = true, true
	}
	if u.promptCacheMissTokensPresent {
		presence.NormalInput = true
	}
	return &provider.Usage{
		PromptTokens:      u.PromptTokens,
		NormalInputTokens: miss,
		CompletionTokens:  u.CompletionTokens,
		TotalTokens:       u.TotalTokens,
		CacheHitTokens:    hit,
		CacheMissTokens:   miss,
		CacheReadTokens:   hit,
		ReasoningTokens:   reasoning,
		Presence:          presence,
	}
}

// --- OpenAI-compatible wire protocol ---

type chatRequest struct {
	Model           string         `json:"model"`
	Messages        []chatMessage  `json:"messages"`
	Tools           []chatTool     `json:"tools,omitempty"`
	Stream          bool           `json:"stream"`
	StreamOptions   *streamOptions `json:"stream_options,omitempty"`
	Temperature     float64        `json:"temperature,omitempty"`
	MaxTokens       int            `json:"max_tokens,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Thinking        *thinkingMode  `json:"thinking,omitempty"`
}

type thinkingMode struct {
	Type string `json:"type"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role string `json:"role"`
	// content is always present (never omitted): DeepSeek's strict deserializer
	// rejects a message missing the field. A pure tool_calls assistant turn
	// serializes as null (nil here); a string for every other text message
	// (empty included — null is rejected by some backends for a tool message);
	// and a []chatContentPart array for a vision user turn carrying images.
	Content          any            `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

func imageContentParts(text string, images []string, detail string) []chatContentPart {
	parts := make([]chatContentPart, 0, len(images)+1)
	if text != "" {
		parts = append(parts, chatContentPart{Type: "text", Text: text})
	}
	for _, url := range images {
		parts = append(parts, chatContentPart{Type: "image_url", ImageURL: &chatImageURL{URL: url, Detail: detail}})
	}
	return parts
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type streamResponse struct {
	Choices []streamChoice `json:"choices"`
	Usage   *wireUsage     `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Type    string `json:"type"`
	} `json:"error"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Content          string         `json:"content"`
	ReasoningContent string         `json:"reasoning_content"`
	Reasoning        string         `json:"reasoning"`
	ToolCalls        []chatToolCall `json:"tool_calls"`
}

// wireUsage covers both DeepSeek's top-level cache fields and the
// OpenAI/MiMo nested details — normaliseUsage chooses whichever side
// reports values.
type wireUsage struct {
	PromptTokens          int `json:"prompt_tokens"`
	CompletionTokens      int `json:"completion_tokens"`
	TotalTokens           int `json:"total_tokens"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	PromptTokensDetails   *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	promptTokensPresent              bool
	completionTokensPresent          bool
	totalTokensPresent               bool
	promptCacheHitTokensPresent      bool
	promptCacheMissTokensPresent     bool
	promptTokensDetailsCachedPresent bool
}

func (u *wireUsage) UnmarshalJSON(data []byte) error {
	type plain wireUsage
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*u = wireUsage(decoded)
	_, u.promptTokensPresent = fields["prompt_tokens"]
	_, u.completionTokensPresent = fields["completion_tokens"]
	_, u.totalTokensPresent = fields["total_tokens"]
	_, u.promptCacheHitTokensPresent = fields["prompt_cache_hit_tokens"]
	_, u.promptCacheMissTokensPresent = fields["prompt_cache_miss_tokens"]
	if raw, ok := fields["prompt_tokens_details"]; ok {
		var details map[string]json.RawMessage
		if json.Unmarshal(raw, &details) == nil {
			_, u.promptTokensDetailsCachedPresent = details["cached_tokens"]
		}
	}
	return nil
}
