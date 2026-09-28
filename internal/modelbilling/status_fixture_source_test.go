package modelbilling

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewFixtureStatusSourcePreservesRegisteredNormalizedFixture(t *testing.T) {
	source, err := NewFixtureStatusSource()
	if err != nil {
		t.Fatalf("NewFixtureStatusSource: %v", err)
	}

	status, err := source.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("FetchStatus: %v", err)
	}
	wantFetchedAt := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	if status.USDExchangeRate != "7.2000" || status.RechargePrice != "1.0000" ||
		!status.FetchedAt.Equal(wantFetchedAt) || status.SourceVersion == nil || *status.SourceVersion != "fixture-v1" {
		t.Fatalf("fixture status = %+v", status)
	}

	*status.SourceVersion = "changed"
	again, err := source.FetchStatus(context.Background())
	if err != nil || again.SourceVersion == nil || *again.SourceVersion != "fixture-v1" {
		t.Fatalf("fixture status was aliased: %+v, %v", again, err)
	}
}

func TestFixtureStatusSourceHonorsContextCancellation(t *testing.T) {
	source, err := NewFixtureStatusSource()
	if err != nil {
		t.Fatalf("NewFixtureStatusSource: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.FetchStatus(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchStatus error = %v", err)
	}
}
