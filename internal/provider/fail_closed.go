package provider

import (
	"context"
	"errors"
)

// ContractInvalidError marks a model entry that B2B deliberately leaves for a
// later integration slice. It is returned before a provider can issue network
// traffic.
type ContractInvalidError struct{ Message string }

func (e *ContractInvalidError) Error() string {
	if e == nil || e.Message == "" {
		return "contract_invalid"
	}
	return "contract_invalid: " + e.Message
}

func IsContractInvalid(err error) bool {
	var target *ContractInvalidError
	return errors.As(err, &target)
}

// FailClosedProvider preserves the configured provider object but refuses this
// dispatch entry. It is used for B2C roles until their explicit route hand-off
// exists; it never changes credential or user configuration.
type FailClosedProvider struct {
	Base    Provider
	Message string
}

func (p FailClosedProvider) Name() string {
	if p.Base == nil {
		return "blocked"
	}
	return p.Base.Name()
}

func (p FailClosedProvider) Stream(context.Context, Request) (<-chan Chunk, error) {
	return nil, &ContractInvalidError{Message: p.Message}
}

var _ Provider = FailClosedProvider{}
