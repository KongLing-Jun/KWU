package provider

import (
	"context"
	"testing"
)

type failClosedTestProvider struct{ calls int }

func (*failClosedTestProvider) Name() string { return "configured" }
func (p *failClosedTestProvider) Stream(context.Context, Request) (<-chan Chunk, error) {
	p.calls++
	return nil, nil
}

func TestFailClosedProviderReturnsContractInvalidBeforeBaseDispatch(t *testing.T) {
	base := &failClosedTestProvider{}
	_, err := (FailClosedProvider{Base: base, Message: "B2C route is not connected"}).Stream(context.Background(), Request{})
	if !IsContractInvalid(err) || base.calls != 0 {
		t.Fatalf("err/calls = %v/%d", err, base.calls)
	}
}
