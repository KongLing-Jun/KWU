package modelbilling

import (
	"context"
	"os"
	"strings"
	"testing"

	"kwu/internal/cloud"
)

func TestDecodeCreditConfigFixtureStrictly(t *testing.T) {
	f, err := os.Open("testdata/CLOUD-CREDIT-CONFIG-01/normalized-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bundle, err := DecodeCreditConfigFixture(f)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Authenticity != "MOCK_ONLY" || bundle.Cases[0].Config.PricingGroup != "default" {
		t.Fatalf("bundle = %+v", bundle)
	}
	for _, raw := range []string{
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"OTHER","cases":[]}`,
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"READY","cases":[]}`,
		`{"fixture_version":"b0-v2","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"MOCK_ONLY","cases":[]}`,
		`{"fixture_version":"b0-v1","dependency_id":"OTHER","authenticity":"MOCK_ONLY","cases":[]}`,
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"MOCK_ONLY","cases":[{"name":"x","config":{"credit_per_cny":"2000","pricing_group":"","fetched_at":"2026-07-11T00:00:00Z"}}]}`,
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"MOCK_ONLY","cases":[{"name":"x","config":{"credit_per_cny":"-1","pricing_group":"default","fetched_at":"2026-07-11T00:00:00Z"}}]}`,
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"MOCK_ONLY","cases":[{"name":"x","config":{"credit_per_cny":"2000","pricing_group":"default","fetched_at":"0001-01-01T00:00:00Z"}}]}`,
		`{"fixture_version":"b0-v1","dependency_id":"CLOUD-CREDIT-CONFIG-01","authenticity":"MOCK_ONLY","unknown":1,"cases":[]}`,
	} {
		if _, err := DecodeCreditConfigFixture(strings.NewReader(raw)); err == nil {
			t.Fatalf("DecodeCreditConfigFixture accepted %s", raw)
		}
	}
}

func TestFixtureCreditConfigProviderUsesBundleOutcome(t *testing.T) {
	f, err := os.Open("testdata/CLOUD-CREDIT-CONFIG-01/normalized-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bundle, err := DecodeCreditConfigFixture(f)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewFixtureCreditConfigProvider(bundle)
	if err != nil {
		t.Fatal(err)
	}
	scope := testBindingScope()
	config, err := provider.GetCreditConfig(context.Background(), scope)
	if err != nil || config.PricingGroup != "default" || provider.Authenticity() != "MOCK_ONLY" {
		t.Fatalf("GetCreditConfig = %+v, %v; authenticity=%q", config, err, provider.Authenticity())
	}
	config.PricingGroup = "changed"
	again, err := provider.GetCreditConfig(context.Background(), scope)
	if err != nil || again.PricingGroup != "default" {
		t.Fatalf("provider leaked config mutation: %+v, %v", again, err)
	}

	bundle.Cases[0].Config.PricingGroup = "changed"
	changedVersion := "changed"
	bundle.Cases[0].Config.Version = &changedVersion
	if again, err := provider.GetCreditConfig(context.Background(), scope); err != nil || again.PricingGroup != "default" || *again.Version != "cloud-credit-config-fixture-v1" {
		t.Fatalf("provider retained caller bundle mutation: %+v, %v", again, err)
	}
	if _, err := provider.GetCreditConfig(context.Background(), cloud.BindingScope{}); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("invalid scope error = %v", err)
	}

	failure := CreditConfigFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudCreditConfig, Authenticity: "MOCK_ONLY", Cases: []CreditConfigFixtureCase{bundle.Cases[1]}}
	failedProvider, err := NewFixtureCreditConfigProvider(failure)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failedProvider.GetCreditConfig(context.Background(), scope); ErrorCodeOf(err) != ErrorPricingUnavailable {
		t.Fatalf("explicit failure = %v", err)
	}
	failure.Cases[0].Error.Message = "changed"
	if _, err := failedProvider.GetCreditConfig(context.Background(), scope); err == nil || !strings.Contains(err.Error(), "fixture credit config unavailable") {
		t.Fatalf("provider retained caller error mutation: %v", err)
	}
}
