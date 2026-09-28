package anthropic

import (
	"encoding/json"

	"kwu/internal/provider"
)

type streamUsage struct {
	input, output, cacheCreate, cacheRead int
	presence                              provider.UsagePresence
}

func (u *streamUsage) setInput(wire wireUsage) {
	u.input, u.cacheCreate, u.cacheRead, u.presence = wire.InputTokens, wire.CacheCreationInputTokens, wire.CacheReadInputTokens, wire.presence
	u.presence.NormalInput = wire.presence.Input
}

func (u *streamUsage) setOutput(wire wireUsage) {
	u.output, u.presence.Output = wire.OutputTokens, wire.presence.Output
}

func (u streamUsage) providerUsage(finishReason string) *provider.Usage {
	return &provider.Usage{
		PromptTokens:      u.input + u.cacheCreate + u.cacheRead,
		NormalInputTokens: u.input,
		CompletionTokens:  u.output,
		TotalTokens:       u.input + u.cacheCreate + u.cacheRead + u.output,
		CacheHitTokens:    u.cacheRead,
		CacheMissTokens:   u.input + u.cacheCreate,
		CacheReadTokens:   u.cacheRead,
		CacheWriteTokens:  u.cacheCreate,
		FinishReason:      finishReason,
		Presence:          u.presence,
	}
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
	u.presence = provider.UsagePresence{
		Input:      fields["input_tokens"] != nil,
		Output:     fields["output_tokens"] != nil,
		CacheRead:  fields["cache_read_input_tokens"] != nil,
		CacheWrite: fields["cache_creation_input_tokens"] != nil,
	}
	return nil
}
