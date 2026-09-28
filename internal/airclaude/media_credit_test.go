package airclaude

import "testing"

func TestMediaResponseQuotaAcceptsConsistentBillingEvidence(t *testing.T) {
	quota, ok, err := mediaResponseQuota([]byte(`{"billing":{"quota":35800},"usage":{"quota":"35800.0"}}`))
	if err != nil || !ok || quota != "35800" {
		t.Fatalf("mediaResponseQuota = %q, %v, %v", quota, ok, err)
	}
}

func TestMediaResponseQuotaRejectsConflictingBillingEvidence(t *testing.T) {
	if _, _, err := mediaResponseQuota([]byte(`{"billing":{"quota":1},"usage":{"quota":2}}`)); err == nil {
		t.Fatal("mediaResponseQuota accepted conflicting quota")
	}
}

func TestEstimateImageMaaSQuotaUsesTokenDetailsAndRoundsProviderQuota(t *testing.T) {
	payload := []byte(`{
		"usage":{
			"input_tokens":43,
			"input_tokens_details":{"image_tokens":0,"text_tokens":43},
			"output_tokens":186,
			"output_tokens_details":{"image_tokens":186,"text_tokens":0},
			"total_tokens":229
		}
	}`)
	quota, err := estimateImageMaaSQuota(payload, ImageQuotaPricing{
		InputPerMillion: "5", OutputPerMillion: "30", ImageInputPerMillion: "8", QuotaPerUSD: "500000",
	})
	if err != nil {
		t.Fatalf("estimateImageMaaSQuota: %v", err)
	}
	// USD = (43*5 + 186*30) / 1,000,000 = 0.005795;
	// quota = round(0.005795 * 500000) = round(2897.5) = 2898.
	if quota != "2898" {
		t.Fatalf("quota = %q, want 2898", quota)
	}
}

func TestEstimateImageMaaSQuotaRejectsConflictingTokenDetails(t *testing.T) {
	_, err := estimateImageMaaSQuota([]byte(`{
		"usage":{"input_tokens":2,"input_tokens_details":{"text_tokens":1,"image_tokens":0},"output_tokens":1}
	}`), ImageQuotaPricing{InputPerMillion: "5", OutputPerMillion: "30", QuotaPerUSD: "500000"})
	if err == nil {
		t.Fatal("estimateImageMaaSQuota accepted conflicting usage")
	}
}
