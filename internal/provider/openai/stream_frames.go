package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kwu/internal/provider"
)

type streamLineResult struct {
	stop     bool
	progress bool
	finish   bool
}

func (s *openAIStreamState) handleSSELine(ctx context.Context, name string, out chan<- provider.Chunk, line string) (streamLineResult, error) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "event:") {
		s.billingEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
		return streamLineResult{}, nil
	}
	data, ok := streamData(line)
	if !ok {
		return streamLineResult{}, nil
	}
	if data == "[DONE]" {
		s.sawDone = true
		return streamLineResult{stop: true, progress: true}, nil
	}
	if s.includeBilling {
		billingFrame, err := s.captureBillingFrame(data, s.billingEvent == "billing")
		s.billingEvent = ""
		if err != nil {
			return streamLineResult{}, fmt.Errorf("%s: decode billing event: %w", name, err)
		}
		if billingFrame {
			return streamLineResult{}, nil
		}
	}
	var sr streamResponse
	if err := json.Unmarshal([]byte(data), &sr); err != nil {
		return streamLineResult{}, fmt.Errorf("%s: decode stream: %w", name, err)
	}
	result := streamLineResult{
		progress: streamResponseHasProgress(sr),
		finish:   streamResponseHasFinishReason(sr),
	}
	return result, s.handleStreamResponse(ctx, name, out, sr)
}

func streamResponseHasProgress(sr streamResponse) bool {
	if sr.Usage != nil || streamResponseHasFinishReason(sr) {
		return true
	}
	for _, choice := range sr.Choices {
		if streamChoiceHasProgress(choice) {
			return true
		}
	}
	return false
}

func streamChoiceHasProgress(choice streamChoice) bool {
	delta := choice.Delta
	if delta.ReasoningContent != "" || delta.Reasoning != "" || delta.Content != "" {
		return true
	}
	for _, call := range delta.ToolCalls {
		if call.ID != "" || call.Function.Name != "" || call.Function.Arguments != "" {
			return true
		}
	}
	return false
}

func streamResponseHasFinishReason(sr streamResponse) bool {
	for _, choice := range sr.Choices {
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			return true
		}
	}
	return false
}

func (s *openAIStreamState) captureBillingFrame(data string, required bool) (bool, error) {
	finalJSON, found, err := billingOnlyJSON(json.RawMessage(data))
	if err != nil {
		return false, err
	}
	if !found {
		if required {
			return false, fmt.Errorf("billing event requires a root billing object")
		}
		return false, nil
	}
	if len(s.billingEvidence.FinalJSON) != 0 && string(s.billingEvidence.FinalJSON) != "{}" &&
		!bytes.Equal(s.billingEvidence.FinalJSON, finalJSON) {
		return false, fmt.Errorf("conflicting billing events")
	}
	s.billingEvidence.FinalJSON = finalJSON
	return true, nil
}

func streamData(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}
