package openai

import (
	"encoding/json"
	"testing"
)

func TestNormaliseUsagePreservesOpenAIWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		wire                                         string
		wantInput, wantOutput, wantTotal, wantCached bool
	}{
		{"zero fields present", `{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"prompt_tokens_details":{"cached_tokens":0}}`, true, true, true, true},
		{"cache omitted", `{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}`, true, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wire wireUsage
			if err := json.Unmarshal([]byte(tc.wire), &wire); err != nil {
				t.Fatal(err)
			}
			got := normaliseUsage(&wire).Presence
			if got.Input != tc.wantInput || got.Output != tc.wantOutput || got.ProviderTotal != tc.wantTotal || got.CachedInput != tc.wantCached {
				t.Fatalf("presence = %+v", got)
			}
		})
	}
}

func TestNormaliseUsageKeepsConfirmedNormalAndCacheReadValues(t *testing.T) {
	var wire wireUsage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":13,"completion_tokens":5,"total_tokens":18,"prompt_cache_hit_tokens":3,"prompt_cache_miss_tokens":10}`), &wire); err != nil {
		t.Fatal(err)
	}
	got := normaliseUsage(&wire)
	if got.NormalInputTokens != 10 || got.CacheReadTokens != 3 || got.CacheWriteTokens != 0 || !got.Presence.NormalInput || !got.Presence.CacheRead || got.Presence.CacheWrite {
		t.Fatalf("usage = %+v", got)
	}
}
