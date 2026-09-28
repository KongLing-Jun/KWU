package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"

	"kwu/desktop/internal/diagnostics"
)

// crash_app.go is the crash/feedback/performance reporting surface. Reports are
// sent only on an explicit user click in the frontend UI — never automatically.

var crashEndpoint = "https://crash.kwu.example/v1/report"

func reportEndpointAvailable(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host != "crash.kwu.example" && !strings.HasSuffix(host, ".example")
}

const maxCrashDetailBytes = 16 << 10
const maxCrashStackBytes = 8 << 10
const maxCrashFieldBytes = 4 << 10
const maxCrashBreadcrumbs = 30

var (
	userPathSegment = regexp.MustCompile(`(?i)([A-Z]:\\Users\\|/(?:home|Users)/)[^/\\:\s"']+`)
)

func scrubUserPaths(s string) string {
	return userPathSegment.ReplaceAllString(s, "${1}_")
}

func scrubSensitiveText(s string) string {
	return diagnostics.ScrubSensitiveText(s)
}

type crashBreadcrumb struct {
	T   int64  `json:"t,omitempty"`
	Cat string `json:"cat,omitempty"`
	Msg string `json:"msg,omitempty"`
}

type crashReport struct {
	Kind           string            `json:"kind"`
	Version        string            `json:"version"`
	OS             string            `json:"os"`
	Arch           string            `json:"arch"`
	Message        string            `json:"message"`
	Device         deviceInfo        `json:"device"`
	SchemaVersion  int               `json:"schemaVersion,omitempty"`
	Source         string            `json:"source,omitempty"`
	Label          string            `json:"label,omitempty"`
	ErrorType      string            `json:"errorType,omitempty"`
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	Stack          string            `json:"stack,omitempty"`
	ComponentStack string            `json:"componentStack,omitempty"`
	TopFrame       string            `json:"topFrame,omitempty"`
	BuildCommit    string            `json:"buildCommit,omitempty"`
	Channel        string            `json:"channel,omitempty"`
	Language       string            `json:"language,omitempty"`
	View           string            `json:"view,omitempty"`
	Breadcrumbs    []crashBreadcrumb `json:"breadcrumbs,omitempty"`
	OccurredAt     string            `json:"occurredAt,omitempty"`
}

type frontendCrashPayload struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Kind           string            `json:"kind"`
	Source         string            `json:"source"`
	Label          string            `json:"label"`
	Message        string            `json:"message"`
	ErrorType      string            `json:"errorType"`
	ErrorMessage   string            `json:"errorMessage"`
	Stack          string            `json:"stack"`
	ComponentStack string            `json:"componentStack"`
	TopFrame       string            `json:"topFrame"`
	BuildCommit    string            `json:"buildCommit"`
	Channel        string            `json:"channel"`
	Language       string            `json:"language"`
	View           string            `json:"view"`
	Breadcrumbs    []crashBreadcrumb `json:"breadcrumbs"`
	OccurredAt     string            `json:"occurredAt"`
}

func normalizeReportKind(kind string) (string, bool) {
	switch strings.TrimSpace(kind) {
	case "crash", "exception", "feedback", "performance", "bot":
		return strings.TrimSpace(kind), true
	default:
		return "", false
	}
}

func clipCrashField(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func sanitizeCrashField(s string, max int) string {
	return clipCrashField(scrubUserPaths(strings.TrimSpace(s)), max)
}

func sanitizeCrashText(s string, max int) string {
	return clipCrashField(strings.TrimSpace(scrubSensitiveText(s)), max)
}

func sanitizeBreadcrumbs(in []crashBreadcrumb) []crashBreadcrumb {
	if len(in) > maxCrashBreadcrumbs {
		in = in[len(in)-maxCrashBreadcrumbs:]
	}
	out := make([]crashBreadcrumb, 0, len(in))
	for _, b := range in {
		cat := sanitizeCrashField(b.Cat, 64)
		msg := sanitizeCrashText(b.Msg, 240)
		if cat == "" && msg == "" {
			continue
		}
		out = append(out, crashBreadcrumb{T: b.T, Cat: cat, Msg: msg})
	}
	return out
}

func baseCrashReport(kind string) crashReport {
	return crashReport{
		Kind:    kind,
		Version: version,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Device:  collectDeviceInfo(),
		Channel: channel,
	}
}

func topFrameFromStack(stack string) string {
	for _, line := range strings.Split(stack, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ".go:") || strings.Contains(line, ".ts:") || strings.Contains(line, ".tsx:") || strings.Contains(line, ".js:") || strings.Contains(line, ".jsx:") {
			if strings.Contains(line, "/runtime/") || strings.Contains(line, `\runtime\`) || strings.Contains(line, "crash_pending.go") {
				continue
			}
			return sanitizeCrashText(line, 300)
		}
	}
	return ""
}

func nativeResourceContext() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	mb := func(n uint64) string {
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
	return strings.Join([]string{
		"go heap alloc: " + mb(m.Alloc),
		"go heap sys: " + mb(m.HeapSys),
		"go total sys: " + mb(m.Sys),
		fmt.Sprintf("goroutines: %d", runtime.NumGoroutine()),
		fmt.Sprintf("gc cycles: %d", m.NumGC),
	}, "\n")
}

func appendNativeResourceContext(kind, message string) string {
	if kind != "performance" {
		return message
	}
	return sanitizeCrashText(message+"\n\n--- native runtime context ---\n"+nativeResourceContext(), maxCrashDetailBytes)
}

func crashReportFromDetail(kind, detail string) (crashReport, error) {
	rawKind := kind
	kind, ok := normalizeReportKind(kind)
	if !ok {
		return crashReport{}, fmt.Errorf("unknown report kind %q", rawKind)
	}
	if strings.TrimSpace(detail) == "" {
		return crashReport{}, fmt.Errorf("empty report")
	}
	r := baseCrashReport(kind)

	var payload frontendCrashPayload
	if json.Unmarshal([]byte(detail), &payload) == nil && payload.SchemaVersion == 2 {
		if payloadKind, ok := normalizeReportKind(payload.Kind); ok {
			r.Kind = payloadKind
		}
		r.SchemaVersion = payload.SchemaVersion
		r.Source = sanitizeCrashField(payload.Source, 32)
		r.Label = sanitizeCrashField(payload.Label, 64)
		r.ErrorType = sanitizeCrashField(payload.ErrorType, 128)
		r.ErrorMessage = sanitizeCrashText(payload.ErrorMessage, maxCrashFieldBytes)
		r.Stack = sanitizeCrashText(payload.Stack, maxCrashStackBytes)
		r.ComponentStack = sanitizeCrashText(payload.ComponentStack, maxCrashStackBytes)
		r.TopFrame = sanitizeCrashText(payload.TopFrame, 300)
		r.BuildCommit = sanitizeCrashField(payload.BuildCommit, 64)
		r.Channel = sanitizeCrashField(payload.Channel, 32)
		r.Language = sanitizeCrashField(payload.Language, 64)
		r.View = sanitizeCrashText(payload.View, 200)
		r.Breadcrumbs = sanitizeBreadcrumbs(payload.Breadcrumbs)
		r.OccurredAt = sanitizeCrashField(payload.OccurredAt, 64)
		r.Message = sanitizeCrashText(payload.Message, maxCrashDetailBytes)
		if r.TopFrame == "" {
			r.TopFrame = topFrameFromStack(r.Stack)
		}
		if r.Message == "" {
			r.Message = sanitizeCrashText(fmt.Sprintf("[%s]\n\n%s", r.Label, r.ErrorMessage), maxCrashDetailBytes)
		}
		if r.Source == "" {
			r.Source = "frontend"
		}
		r.Message = appendNativeResourceContext(r.Kind, r.Message)
		return r, nil
	}

	r.SchemaVersion = 1
	r.Source = "legacy"
	r.Label = kind
	r.Message = sanitizeCrashText(detail, maxCrashDetailBytes)
	r.Message = appendNativeResourceContext(r.Kind, r.Message)
	return r, nil
}

func (a *App) ReportCrash(kind, detail string) error {
	r, err := crashReportFromDetail(kind, detail)
	if err != nil {
		return err
	}
	if !reportEndpointAvailable(crashEndpoint) {
		return fmt.Errorf("reporting service is not configured")
	}
	c, err := httpClient()
	if err != nil {
		return err
	}
	return postCrashReport(a.reqCtx(), c, crashEndpoint, r)
}

func postCrashReport(ctx context.Context, c *http.Client, endpoint string, r crashReport) error {
	if !reportEndpointAvailable(endpoint) {
		return fmt.Errorf("reporting service is not configured")
	}
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("crash endpoint returned %s", resp.Status)
	}
	return nil
}
