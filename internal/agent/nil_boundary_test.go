package agent

import (
	"context"
	"encoding/json"
	"testing"

	"kwu/internal/event"
	"kwu/internal/tool"
)

type typedNilAgentSink struct{}

func (*typedNilAgentSink) Emit(event.Event) {}

type typedNilGate struct{}

func (*typedNilGate) Check(context.Context, string, json.RawMessage, bool) (bool, string, error) {
	return true, "", nil
}

func TestNewNormalizesTypedNilInterfaces(t *testing.T) {
	var sink *typedNilAgentSink
	var gate *typedNilGate

	a := New(nil, tool.NewRegistry(), NewSession(""), Options{Gate: gate}, sink)
	a.sink.Emit(event.Event{Kind: event.Text, Text: "typed nil sink should not panic"})
	if a.gate != nil {
		t.Fatal("typed nil gate should be normalized to nil")
	}
}

func TestSetGateNormalizesTypedNil(t *testing.T) {
	var gate *typedNilGate
	a := New(nil, tool.NewRegistry(), NewSession(""), Options{}, event.Discard)

	a.SetGate(gate)
	if a.gate != nil {
		t.Fatal("typed nil gate should be normalized to nil")
	}
}
