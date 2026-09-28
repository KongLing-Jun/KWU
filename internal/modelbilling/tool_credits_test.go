package modelbilling

import "testing"

func TestAggregateTaskCreditsUsesMaaSFormulaAndRoundsAfterTaskSum(t *testing.T) {
	got, err := AggregateTaskCredits("10.0000", "35800", "0.2640", "500000", "7.3", "2000")
	if err != nil {
		t.Fatalf("AggregateTaskCredits: %v", err)
	}
	// 10 + 0.264 + (35800 / 500000 * 7.3 * 2000) = 1055.624
	if got != "1055.6240" {
		t.Fatalf("credits = %q, want 1055.6240", got)
	}
}

func TestAggregateTaskCreditsRoundsOnlyFinalTotal(t *testing.T) {
	got, err := AggregateTaskCredits("0", "1", "0.00004", "3", "1", "1")
	if err != nil {
		t.Fatalf("AggregateTaskCredits: %v", err)
	}
	if got != "0.3334" {
		t.Fatalf("credits = %q, want 0.3334", got)
	}
}
