package anthropic

import (
	"encoding/json"
	"testing"
)

func TestWireUsagePreservesAnthropicFieldPresence(t *testing.T) {
	for _, tc := range []struct {
		name                                        string
		wire                                        string
		wantInput, wantOutput, wantCreate, wantRead bool
	}{
		{"zero fields present", `{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`, true, true, true, true},
		{"cache omitted", `{"input_tokens":4,"output_tokens":2}`, true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var usage wireUsage
			if err := json.Unmarshal([]byte(tc.wire), &usage); err != nil {
				t.Fatal(err)
			}
			if usage.presence.Input != tc.wantInput || usage.presence.Output != tc.wantOutput || usage.presence.CacheWrite != tc.wantCreate || usage.presence.CacheRead != tc.wantRead {
				t.Fatalf("presence = %+v", usage)
			}
		})
	}
}

func TestStreamUsageKeepsNormalAndCacheWriteSeparate(t *testing.T) {
	var input, output wireUsage
	if err := json.Unmarshal([]byte(`{"input_tokens":8,"cache_creation_input_tokens":2,"cache_read_input_tokens":3}`), &input); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"output_tokens":5}`), &output); err != nil {
		t.Fatal(err)
	}
	var usage streamUsage
	usage.setInput(input)
	usage.setOutput(output)
	got := usage.providerUsage("stop")
	if got.NormalInputTokens != 8 || got.CacheWriteTokens != 2 || got.CacheReadTokens != 3 || got.PromptTokens != 13 {
		t.Fatalf("usage = %+v", got)
	}
}
