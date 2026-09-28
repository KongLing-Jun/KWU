package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync/atomic"
)

var deliveryIdentitySequence atomic.Uint64

// The namespace outlives every controller in this process. Persisted identities
// must not collide when another process resumes the same session.
var deliveryIdentityNamespace = newDeliveryIdentityNamespace()

func newDeliveryIdentityNamespace() string {
	var namespace [16]byte
	if _, err := rand.Read(namespace[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(namespace[:])
}

func nextDeliveryIdentity(prefix string) string {
	return prefix + deliveryIdentityNamespace + "-" + strconv.FormatUint(deliveryIdentitySequence.Add(1), 36)
}

// NewEventID returns an opaque, randomly namespaced identity for one source event.
func NewEventID() string { return nextDeliveryIdentity("e-") }

// NewTurnID returns an opaque, randomly namespaced identity for one logical turn.
func NewTurnID() string { return nextDeliveryIdentity("t-") }

type turnIDContextKey struct{}

// WithTurnID binds the controller-owned identity for one logical user turn.
// Nested agents inherit it so their root start event and the controller's
// terminal event cannot be retagged as a later turn by an asynchronous sink.
func WithTurnID(ctx context.Context, turnID string) context.Context {
	if ctx == nil || turnID == "" {
		return ctx
	}
	return context.WithValue(ctx, turnIDContextKey{}, turnID)
}

// TurnIDFromContext returns a controller-bound turn identity when present.
func TurnIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	turnID, _ := ctx.Value(turnIDContextKey{}).(string)
	return turnID
}

// NewCandidateID returns an opaque, randomly namespaced identity for one model call's
// root assistant candidate. It intentionally contains no response content.
func NewCandidateID() string { return nextDeliveryIdentity("c-") }

// NewPromptID returns an opaque, randomly namespaced identity for one prompt
// aggregate. Existing identities are retained when their entities are replayed.
func NewPromptID() string { return nextDeliveryIdentity("p-") }

// NewOperationID identifies one logical prompt lifecycle.
func NewOperationID() string { return nextDeliveryIdentity("o-") }

// NewOccurrenceID identifies one retryable failure occurrence.
func NewOccurrenceID() string { return nextDeliveryIdentity("occ-") }
