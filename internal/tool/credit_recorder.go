package tool

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync/atomic"

	"kwu/internal/provider"
)

type creditRecorderContextKey struct{}

type creditRecorderSnapshot struct {
	nextID  uint64
	charges map[uint64]provider.ToolChargeMetadata
}

// CreditRecorder collects immutable, host-observed tool billing facts for one
// top-level turn. Writers publish copy-on-write snapshots with CAS so paid tools
// can report concurrently without introducing a turn-wide lock.
type CreditRecorder struct {
	snapshot atomic.Pointer[creditRecorderSnapshot]
}

// WithCreditRecorder installs a fresh per-turn recorder on ctx.
func WithCreditRecorder(ctx context.Context) (context.Context, *CreditRecorder) {
	if ctx == nil {
		ctx = context.Background()
	}
	recorder := &CreditRecorder{}
	recorder.snapshot.Store(&creditRecorderSnapshot{charges: map[uint64]provider.ToolChargeMetadata{}})
	return context.WithValue(ctx, creditRecorderContextKey{}, recorder), recorder
}

// RecordCredit appends one successfully settled paid-tool charge when the
// current turn has a recorder. Calls outside the agent turn lifecycle remain a
// no-op so standalone tool use and focused unit tests keep working.
func RecordCredit(ctx context.Context, charge provider.ToolChargeMetadata) error {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(creditRecorderContextKey{}).(*CreditRecorder)
	if recorder == nil {
		return nil
	}
	return recorder.record(charge)
}

func (r *CreditRecorder) record(charge provider.ToolChargeMetadata) error {
	charge.Tool = strings.TrimSpace(charge.Tool)
	charge.ServiceTaskID = strings.TrimSpace(charge.ServiceTaskID)
	charge.Value = strings.TrimSpace(charge.Value)
	if charge.Tool == "" || charge.Value == "" {
		return fmt.Errorf("tool credit: tool and value are required")
	}
	switch charge.Kind {
	case provider.ToolChargeMaaSQuota, provider.ToolChargeCredits:
	default:
		return fmt.Errorf("tool credit: unsupported charge kind %q", charge.Kind)
	}
	if !nonNegativeDecimal(charge.Value) {
		return fmt.Errorf("tool credit: value must be a non-negative decimal")
	}
	for {
		current := r.snapshot.Load()
		if current == nil {
			empty := &creditRecorderSnapshot{charges: map[uint64]provider.ToolChargeMetadata{}}
			if !r.snapshot.CompareAndSwap(nil, empty) {
				continue
			}
			current = empty
		}
		next := &creditRecorderSnapshot{
			nextID:  current.nextID + 1,
			charges: make(map[uint64]provider.ToolChargeMetadata, len(current.charges)+1),
		}
		for id, existing := range current.charges {
			next.charges[id] = existing
		}
		next.charges[next.nextID] = charge
		if r.snapshot.CompareAndSwap(current, next) {
			return nil
		}
	}
}

// Snapshot returns the recorded charges in their recorder sequence order.
func (r *CreditRecorder) Snapshot() []provider.ToolChargeMetadata {
	if r == nil {
		return nil
	}
	current := r.snapshot.Load()
	if current == nil || len(current.charges) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(current.charges))
	for id := range current.charges {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	charges := make([]provider.ToolChargeMetadata, 0, len(ids))
	for _, id := range ids {
		charges = append(charges, current.charges[id])
	}
	return charges
}

// Summary returns exact decimal subtotals without rounding. MaaS quota stays
// separate until the authoritative task settlement supplies its conversion
// parameters.
func (r *CreditRecorder) Summary() (maasQuota string, chargedCredits string) {
	charges := r.Snapshot()
	maasValues := make([]string, 0, len(charges))
	creditValues := make([]string, 0, len(charges))
	for _, charge := range charges {
		switch charge.Kind {
		case provider.ToolChargeMaaSQuota:
			maasValues = append(maasValues, charge.Value)
		case provider.ToolChargeCredits:
			creditValues = append(creditValues, charge.Value)
		}
	}
	return exactDecimalSum(maasValues), exactDecimalSum(creditValues)
}

func exactDecimalSum(values []string) string {
	if len(values) == 0 {
		return ""
	}
	total := new(big.Rat)
	scale := 0
	for _, value := range values {
		parsed, _ := new(big.Rat).SetString(value)
		total.Add(total, parsed)
		if point := strings.IndexByte(value, '.'); point >= 0 && len(value)-point-1 > scale {
			scale = len(value) - point - 1
		}
	}
	result := total.FloatString(scale)
	if strings.Contains(result, ".") {
		result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	}
	if result == "" {
		return "0"
	}
	return result
}

func nonNegativeDecimal(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/eE+") {
		return false
	}
	parsed, ok := new(big.Rat).SetString(value)
	return ok && parsed.Sign() >= 0
}
