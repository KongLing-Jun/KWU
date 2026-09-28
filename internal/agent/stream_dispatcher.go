package agent

import (
	"context"
	"errors"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
)

const mainStreamDispatcherCloseTimeout = 12 * time.Second

// ErrMainStreamDispatcherCloseTimeout means a call-scoped dispatcher did not
// close after cancellation within its bounded terminal-persistence window.
// Callers must treat it as an indeterminate terminal state, not a normal
// cancellation.
var ErrMainStreamDispatcherCloseTimeout = errors.New("main stream dispatcher did not close after cancellation")

var errStreamDispatcherNilChannel = errors.New("stream dispatcher returned a nil chunk channel")

type streamDispatcherTerminalError struct{ cause error }

func (e *streamDispatcherTerminalError) Error() string { return e.cause.Error() }
func (e *streamDispatcherTerminalError) Unwrap() error { return e.cause }

// resolveAgentRunTurnID selects the identity used for all events emitted by one Agent.Run call.
func resolveAgentRunTurnID(scopeID, contextID string) string {
	if scopeID != "" {
		return scopeID
	}
	if contextID != "" {
		return contextID
	}
	return event.NewTurnID()
}

// IsStreamDispatcherTerminalError reports failures that must escape mechanical
// fallbacks because call terminal persistence is incomplete or indeterminate.
func IsStreamDispatcherTerminalError(err error) bool {
	if errors.Is(err, ErrMainStreamDispatcherCloseTimeout) {
		return true
	}
	var terminal *streamDispatcherTerminalError
	return errors.As(err, &terminal)
}

func modelCallFatalError(err error) error {
	if IsStreamDispatcherTerminalError(err) {
		return err
	}
	return nil
}

func modelCallOrContextError(modelErr, contextErr error) error {
	if modelErr != nil {
		return modelErr
	}
	return contextErr
}

func toolOutcomesFatalError(outcomes []toolOutcome) error {
	fatal := make([]error, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.fatalErr != nil {
			fatal = append(fatal, outcome.fatalErr)
		}
	}
	return errors.Join(fatal...)
}

// StreamDispatcher is the call-scoped main-executor dispatch seam.
type StreamDispatcher interface {
	Stream(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, error)
}

// StreamDispatcherWithTerminalResult is an optional extension for dispatchers
// that persist a terminal call state after cancellation. The result channel
// must receive that persistence error (or nil) before the chunk channel closes.
// It keeps the result scoped to one Stream invocation rather than storing it on
// an Agent or provider instance.
type StreamDispatcherWithTerminalResult interface {
	StreamDispatcher
	StreamWithTerminalResult(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, <-chan error, error)
}

type streamDispatcherContextKey struct{}

type streamDispatcherOverride struct {
	target     *Agent
	dispatcher StreamDispatcher
}

// WithStreamDispatcher overrides an Agent's direct dispatcher for one logical
// call boundary. Binding the override to one Agent prevents nested planner,
// subagent, guardian, or compaction calls from inheriting the caller's role.
// The override carries no billing facts; boot supplies a role/model-bound
// dispatcher and the model-billing layer derives everything else from the
// current TaskContext.
func WithStreamDispatcher(ctx context.Context, target *Agent, dispatcher StreamDispatcher) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, streamDispatcherContextKey{}, streamDispatcherOverride{target: target, dispatcher: dispatcher})
}

func (a *Agent) streamDispatcher(ctx context.Context) StreamDispatcher {
	if ctx != nil {
		if override, ok := ctx.Value(streamDispatcherContextKey{}).(streamDispatcherOverride); ok && override.target == a && override.dispatcher != nil {
			return override.dispatcher
		}
	}
	return a.streamer
}

func (a *Agent) dispatchStream(ctx context.Context, request provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	if streamer := a.streamDispatcher(ctx); streamer != nil {
		if dispatcher, ok := streamer.(StreamDispatcherWithTerminalResult); ok {
			chunks, terminalResult, err := dispatcher.StreamWithTerminalResult(ctx, a.prov, request)
			return validateDispatcherStream(chunks, terminalResult, err)
		}
		chunks, err := streamer.Stream(ctx, a.prov, request)
		return validateDispatcherStream(chunks, nil, err)
	}
	chunks, err := a.prov.Stream(ctx, request)
	return chunks, nil, err
}

func (a *Agent) dispatchDerivedStream(ctx context.Context, request provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	if a.derivedStreamer != nil {
		if dispatcher, ok := a.derivedStreamer.(StreamDispatcherWithTerminalResult); ok {
			chunks, terminalResult, err := dispatcher.StreamWithTerminalResult(ctx, a.prov, request)
			return validateDispatcherStream(chunks, terminalResult, err)
		}
		chunks, err := a.derivedStreamer.Stream(ctx, a.prov, request)
		return validateDispatcherStream(chunks, nil, err)
	}
	chunks, err := a.derivedProvider().Stream(ctx, request)
	return chunks, nil, err
}

func validateDispatcherStream(chunks <-chan provider.Chunk, terminalResult <-chan error, err error) (<-chan provider.Chunk, <-chan error, error) {
	if err != nil {
		return chunks, terminalResult, err
	}
	if chunks == nil {
		return nil, nil, errStreamDispatcherNilChannel
	}
	return chunks, terminalResult, nil
}

// awaitMainStreamDispatcherClose drains an installed dispatcher until it
// closes. Dispatcher terminal work can include a five-second call write then a
// five-second recovery write. The twelve-second budget is strictly greater
// than that sequential ten-second maximum, leaving two seconds for scheduling
// and terminal-result/channel closure before the lifecycle runner receives cancellation.
func (a *Agent) awaitMainStreamDispatcherClose(ctx context.Context, ch <-chan provider.Chunk, terminalResult <-chan error) error {
	if a.streamDispatcher(ctx) == nil {
		return nil
	}
	return awaitStreamDispatcherClose(ch, terminalResult)
}

func (a *Agent) awaitDerivedStreamDispatcherClose(ch <-chan provider.Chunk, terminalResult <-chan error) error {
	if a.derivedStreamer == nil {
		return nil
	}
	return awaitStreamDispatcherClose(ch, terminalResult)
}

func awaitStreamDispatcherClose(ch <-chan provider.Chunk, terminalResult <-chan error) error {
	if err := waitForStreamClose(ch, mainStreamDispatcherCloseTimeout); err != nil {
		return err
	}
	return streamDispatcherTerminalResult(terminalResult)
}

func streamDispatcherTerminalResult(terminalResult <-chan error) error {
	if terminalResult == nil {
		return nil
	}
	// The optional dispatcher contract sends this result before closing ch, so
	// this receive cannot extend the cancellation wait or block a legacy path.
	if err, ok := <-terminalResult; ok && err != nil {
		return &streamDispatcherTerminalError{cause: err}
	}
	return nil
}

func streamDispatcherChunkError(terminalResult <-chan error, streamErr error) error {
	if terminalErr := streamDispatcherTerminalResult(terminalResult); terminalErr != nil {
		return terminalErr
	}
	return streamErr
}

// streamCancellationResult keeps the dispatcher terminal-persistence wait out
// of Agent.stream's normal chunk loop. A cancelled main-executor call must not
// let the lifecycle runner write task state until the dispatcher has closed.
func (a *Agent) streamCancellationResult(ctx context.Context, ch <-chan provider.Chunk, terminalResult <-chan error, text, signature string, calls []provider.ToolCall, usage *provider.Usage, partialToolStarted bool, finishReasoning func() (string, string)) (string, string, string, []provider.ToolCall, *provider.Usage, bool, bool, error) {
	if err := a.awaitMainStreamDispatcherClose(ctx, ch, terminalResult); err != nil {
		return text, "", signature, calls, usage, false, partialToolStarted, err
	}
	stored, _ := finishReasoning()
	return text, stored, signature, calls, usage, false, partialToolStarted, ctx.Err()
}

func (a *Agent) streamClosedResult(ctx context.Context, ch <-chan provider.Chunk, terminalResult <-chan error, text, signature string, calls []provider.ToolCall, usage *provider.Usage, partialToolStarted bool, candidateID string, finishReasoning func() (string, string)) (string, string, string, []provider.ToolCall, *provider.Usage, bool, bool, error) {
	if err := ctx.Err(); err != nil {
		if terminalErr := a.awaitMainStreamDispatcherClose(ctx, ch, terminalResult); terminalErr != nil {
			return text, "", signature, calls, usage, false, partialToolStarted, terminalErr
		}
		stored, _ := finishReasoning()
		return text, stored, signature, calls, usage, false, partialToolStarted, err
	}
	if terminalErr := streamDispatcherTerminalResult(terminalResult); terminalErr != nil {
		return text, "", signature, calls, usage, false, partialToolStarted, terminalErr
	}
	stored, display := finishReasoning()
	if text != "" || display != "" {
		a.sink.Emit(event.Event{Kind: event.Message, EventID: event.NewEventID(), CandidateID: candidateID, Text: StripGoalMarkers(text), Reasoning: display})
	}
	return text, stored, signature, calls, usage, false, false, nil
}

func (a *Agent) streamErrorResult(ctx context.Context, ch <-chan provider.Chunk, terminalResult <-chan error, text, signature string, calls []provider.ToolCall, usage *provider.Usage, partialToolStarted bool, streamErr error, finishReasoning func() (string, string)) (string, string, string, []provider.ToolCall, *provider.Usage, bool, bool, error) {
	if ctx.Err() != nil {
		if err := a.awaitMainStreamDispatcherClose(ctx, ch, terminalResult); err != nil {
			return text, "", signature, calls, usage, false, partialToolStarted, err
		}
	} else {
		streamErr = streamDispatcherChunkError(terminalResult, streamErr)
	}
	if provider.IsStreamInterrupted(streamErr) || provider.IsConnReset(streamErr) {
		stored, _ := finishReasoning()
		return text, stored, signature, calls, usage, true, partialToolStarted, streamErr
	}
	return "", "", "", nil, nil, false, false, streamErr
}

func waitForStreamClose(ch <-chan provider.Chunk, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return nil
			}
		case <-timer.C:
			return ErrMainStreamDispatcherCloseTimeout
		}
	}
}

func (a *Agent) derivedProvider() provider.Provider {
	if a.derivedProv != nil {
		return a.derivedProv
	}
	return a.prov
}
