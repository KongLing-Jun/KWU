package airclaude

import (
	"encoding/json"
	"testing"
	"time"

	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/provider"
)

func TestGatewayAdaptsBillingEvidenceWithoutForwardingProviderMetadata(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	gateway := newTestGateway(t, func() time.Time { return now })
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkDone,
		BillingEvidence: &provider.BillingEvidence{
			ProviderRequestIDs: []string{"request-1"},
			FinalJSON:          json.RawMessage(`{"billing":{"quota":15}}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.Chunk.BillingEvidence != nil || chunk.ProviderRequestID != "request-1" ||
		chunk.QuotaEvidence == nil || chunk.QuotaEvidence.Status != modelbilling.QuotaEvidenceComplete ||
		len(chunk.QuotaEvidenceRevisions) != 0 ||
		chunk.QuotaEvidence.Quota == nil || *chunk.QuotaEvidence.Quota != "15" ||
		chunk.QuotaEvidence.ObservedAt != now {
		t.Fatalf("adapted chunk = %+v", chunk)
	}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("adapted chunk validation: %v", err)
	}
}

func TestGatewayPreservesMissingRequestIDAsIncompleteEvidence(t *testing.T) {
	gateway := newTestGateway(t, func() time.Time {
		return time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	})
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type:            provider.ChunkDone,
		BillingEvidence: &provider.BillingEvidence{FinalJSON: json.RawMessage(`{"billing":{"quota":0}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.QuotaEvidence == nil || chunk.QuotaEvidence.Status != modelbilling.QuotaEvidenceMissing ||
		chunk.QuotaEvidence.Quota == nil || *chunk.QuotaEvidence.Quota != "0" {
		t.Fatalf("missing request evidence = %+v", chunk.QuotaEvidence)
	}
}

func TestGatewayAdaptsMultipleRequestIDsAsRevisionsOnly(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	gateway := newTestGateway(t, func() time.Time { return now })
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkDone,
		BillingEvidence: &provider.BillingEvidence{
			ProviderRequestIDs: []string{"request-1", "request-2"},
			FinalJSON:          json.RawMessage(`{"billing":{"quota":15}}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.ProviderRequestID != "" || chunk.QuotaEvidence != nil || len(chunk.QuotaEvidenceRevisions) != 2 {
		t.Fatalf("multi-request envelope = %+v", chunk)
	}
	for index, want := range []string{"request-1", "request-2"} {
		revision := chunk.QuotaEvidenceRevisions[index]
		if revision.Revision != uint64(index+1) || revision.OneAPIRequestID == nil ||
			*revision.OneAPIRequestID != want || revision.Status != modelbilling.QuotaEvidenceComplete ||
			revision.Quota == nil || *revision.Quota != "15" || revision.ObservedAt != now {
			t.Fatalf("revision %d = %+v", index, revision)
		}
	}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("multi-request chunk validation: %v", err)
	}
}

func TestGatewayDeduplicatesRepeatedRequestID(t *testing.T) {
	gateway := newTestGateway(t, func() time.Time {
		return time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	})
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkDone,
		BillingEvidence: &provider.BillingEvidence{
			ProviderRequestIDs: []string{"request-1", "request-1"},
			FinalJSON:          json.RawMessage(`{"billing":{"quota":15}}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.QuotaEvidence == nil || len(chunk.QuotaEvidenceRevisions) != 0 || chunk.ProviderRequestID != "request-1" {
		t.Fatalf("deduplicated chunk = %+v", chunk)
	}
}

func TestGatewayPreservesMissingBillingAsIncompleteEvidence(t *testing.T) {
	gateway := newTestGateway(t, func() time.Time {
		return time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	})
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkDone,
		BillingEvidence: &provider.BillingEvidence{
			ProviderRequestIDs: []string{"request-1"},
			FinalJSON:          json.RawMessage(`{}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.QuotaEvidence == nil || chunk.QuotaEvidence.Status != modelbilling.QuotaEvidenceMissing ||
		chunk.QuotaEvidence.Quota != nil || chunk.ProviderRequestID != "request-1" {
		t.Fatalf("missing billing evidence = %+v", chunk)
	}
}

func TestGatewayLeavesAbsentProviderEvidenceForExistingTerminalHandling(t *testing.T) {
	gateway := newTestGateway(t, time.Now)
	chunk, err := gateway.adaptChunk(provider.Chunk{Type: provider.ChunkDone})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.QuotaEvidence != nil || len(chunk.QuotaEvidenceRevisions) != 0 || chunk.ProviderRequestID != "" {
		t.Fatalf("unexpected synthesized evidence = %+v", chunk)
	}
}

func TestGatewayRejectsBillingEvidenceBeforeDone(t *testing.T) {
	gateway := newTestGateway(t, time.Now)
	_, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkText, Text: "answer",
		BillingEvidence: &provider.BillingEvidence{
			ProviderRequestIDs: []string{"request-1"},
			FinalJSON:          json.RawMessage(`{"billing":{"quota":15}}`),
		},
	})
	if err == nil {
		t.Fatal("billing evidence before done was accepted")
	}
}

func TestGatewayNormalizesProviderUsageForGatewayContract(t *testing.T) {
	gateway := newTestGateway(t, time.Now)
	chunk, err := gateway.adaptChunk(provider.Chunk{
		Type: provider.ChunkUsage,
		Usage: &provider.Usage{
			PromptTokens: 13, NormalInputTokens: 8, CacheReadTokens: 3, CacheWriteTokens: 2,
			CompletionTokens: 5, TotalTokens: 18,
			Presence: provider.UsagePresence{
				Input: true, NormalInput: true, CacheRead: true, CacheWrite: true,
				Output: true, ProviderTotal: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunk.Usage == nil || chunk.Usage.InputTokens == nil || *chunk.Usage.InputTokens != 13 ||
		chunk.Usage.NormalInputTokens == nil || *chunk.Usage.NormalInputTokens != 8 ||
		chunk.Usage.CacheReadTokens == nil || *chunk.Usage.CacheReadTokens != 3 ||
		chunk.Usage.CacheWriteTokens == nil || *chunk.Usage.CacheWriteTokens != 2 ||
		chunk.Usage.OutputTokens == nil || *chunk.Usage.OutputTokens != 5 ||
		chunk.Usage.ProviderTotalTokens == nil || *chunk.Usage.ProviderTotalTokens != 18 ||
		chunk.Usage.RequestCount == nil || *chunk.Usage.RequestCount != 1 {
		t.Fatalf("normalized usage = %+v", chunk.Usage)
	}
}

func newTestGateway(t *testing.T, now func() time.Time) *Gateway {
	t.Helper()
	gateway, err := NewGateway("sk-test", netclient.ProxySpec{Mode: netclient.ModeOff}, now)
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}
