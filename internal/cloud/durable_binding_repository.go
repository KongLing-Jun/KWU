package cloud

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"kwu/internal/userctx"
)

const (
	durableBindingSchema  = "reasonix.cloud.binding"
	durableBindingVersion = 2
)

type durableBindingDocument struct {
	Schema      string              `json:"schema"`
	Version     int                 `json:"version"`
	ProfileID   string              `json:"profile_id"`
	Environment EnvironmentIdentity `json:"environment"`
	Binding     ProfileBinding      `json:"binding"`
}

// DurableBindingRepository persists one binding document inside each 04
// profile state root. It does not accept a caller-provided filesystem path.
type DurableBindingRepository struct {
	paths       ProfilePathProvider
	environment EnvironmentIdentity
}

func NewDurableBindingRepository(paths ProfilePathProvider, environment EnvironmentIdentity) *DurableBindingRepository {
	return &DurableBindingRepository{paths: paths, environment: environment}
}

func (r *DurableBindingRepository) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	binding, found, err := readDurableBinding(path, actor, r.environment)
	if err != nil {
		if isCredentialStateRejected(err) {
			return rejectedBinding(actor, binding.BindingGeneration), err
		}
		return ProfileBinding{}, err
	}
	if !found {
		return unboundBinding(actor), nil
	}
	return binding, nil
}

func (r *DurableBindingRepository) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()

	binding, found, err := readDurableBinding(path, actor, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	if !found {
		binding = ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: cloudUserID, Status: BindingBound, BindingGeneration: 1}
		return binding, writeDurableBinding(path, r.environment, binding)
	}
	if binding.Status == BindingBound && binding.CloudUserID == cloudUserID {
		return binding, nil
	}
	return binding, profileScopeMismatch(actor, binding)
}

func (r *DurableBindingRepository) Rebind(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()

	binding, found, err := readDurableBinding(path, actor, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	if !found {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: cannot rebind an unbound profile"}
	}
	if binding.CloudUserID == cloudUserID && binding.Status == BindingBound {
		return binding, nil
	}
	if binding.BindingGeneration == ^uint64(0) {
		return ProfileBinding{}, fmt.Errorf("cloud: binding_generation overflow")
	}
	binding.CloudUserID = cloudUserID
	binding.Status = BindingBound
	binding.BindingGeneration++
	return binding, writeDurableBinding(path, r.environment, binding)
}

// CompareAndRebind performs the equality check and durable replacement under
// the shared per-path lock, including across repository instances.
func (r *DurableBindingRepository) CompareAndRebind(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	if err := validateCompareAndRebind(actor, expected, cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}

	current, found, err := readDurableBinding(path, actor, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	if !found || current != expected {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: binding changed before account switch"}
	}
	next, err := comparedRebinding(expected, cloudUserID)
	if err != nil {
		return ProfileBinding{}, err
	}
	return next, writeDurableBinding(path, r.environment, next)
}

func readDurableBinding(path string, actor userctx.ActorContext, environment EnvironmentIdentity) (ProfileBinding, bool, error) {
	if err := environment.Validate(); err != nil {
		return ProfileBinding{}, false, err
	}
	var document durableBindingDocument
	found, err := durableReadJSON(path, &document)
	if err != nil || !found {
		return ProfileBinding{}, found, classifyCredentialReadError(err)
	}
	binding, err := validateDurableBindingDocument(document, actor, environment)
	return binding, true, err
}

func validateDurableBindingDocument(document durableBindingDocument, actor userctx.ActorContext, environment EnvironmentIdentity) (ProfileBinding, error) {
	if document.Schema == durableBindingSchema && document.Version == 1 {
		return trustedRejectedBinding(document, actor), credentialStateError(ErrorCode("LEGACY_CREDENTIAL_STATE"), ErrLegacyCredentialState)
	}
	if document.Schema != durableBindingSchema || document.Version != durableBindingVersion {
		return ProfileBinding{}, credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), ErrCredentialStateInvalid)
	}
	if document.Environment != environment {
		return trustedRejectedBinding(document, actor), credentialStateError(ErrorCode("ENVIRONMENT_MISMATCH"), ErrEnvironmentMismatch)
	}
	return validateCurrentDurableBindingDocument(document, actor)
}

func validateCurrentDurableBindingDocument(document durableBindingDocument, actor userctx.ActorContext) (ProfileBinding, error) {
	if document.ProfileID != actor.ProfileID || document.Binding.ProfileID != document.ProfileID {
		return ProfileBinding{}, credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), ErrCredentialStateInvalid)
	}
	if err := document.Binding.Validate(); err != nil {
		return ProfileBinding{}, credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), errors.Join(ErrCredentialStateInvalid, err))
	}
	if document.Binding.ActorID != actor.ActorID || document.Binding.Status != BindingBound {
		return ProfileBinding{}, credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), ErrCredentialStateInvalid)
	}
	return document.Binding, nil
}

func writeDurableBinding(path string, environment EnvironmentIdentity, binding ProfileBinding) error {
	if err := environment.Validate(); err != nil {
		return err
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	document := durableBindingDocument{Schema: durableBindingSchema, Version: durableBindingVersion, ProfileID: binding.ProfileID, Environment: environment, Binding: binding}
	return durableWriteJSON(path, document)
}

func trustedRejectedBinding(document durableBindingDocument, actor userctx.ActorContext) ProfileBinding {
	if document.ProfileID != actor.ProfileID || document.Binding.ProfileID != actor.ProfileID ||
		document.Binding.ActorID != actor.ActorID || document.Binding.Validate() != nil {
		return ProfileBinding{}
	}
	return document.Binding
}

func rejectedBinding(actor userctx.ActorContext, generation uint64) ProfileBinding {
	actor = actor.Normalize()
	return ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingExpired, BindingGeneration: generation}
}

func (r *DurableBindingRepository) PrepareRejectedBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	rejected, err := readRejectedBindingForReplacement(path, actor, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	generation, err := replacementGeneration(rejected.BindingGeneration, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	return ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: cloudUserID, Status: BindingBound, BindingGeneration: generation}, nil
}

func (r *DurableBindingRepository) ReplaceRejectedBinding(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	if err := expected.Validate(); err != nil {
		return ProfileBinding{}, err
	}
	if expected.ActorID != actor.ActorID || expected.ProfileID != actor.ProfileID || expected.Status != BindingBound {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: rejected binding replacement belongs to another actor"}
	}
	path, err := durableCloudFile(r.paths, actor, "binding.json")
	if err != nil {
		return ProfileBinding{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	rejected, err := readRejectedBindingForReplacement(path, actor, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	generation, err := replacementGeneration(rejected.BindingGeneration, r.environment)
	if err != nil {
		return ProfileBinding{}, err
	}
	if expected.BindingGeneration != generation {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: rejected binding changed before authentication commit"}
	}
	return expected, writeDurableBinding(path, r.environment, expected)
}

func readRejectedBindingForReplacement(path string, actor userctx.ActorContext, environment EnvironmentIdentity) (ProfileBinding, error) {
	rejected, found, readErr := readDurableBinding(path, actor, environment)
	if isCredentialStateRejected(readErr) {
		return rejected, nil
	}
	if readErr != nil {
		return ProfileBinding{}, readErr
	}
	if !found {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: credential state is not rejected"}
	}
	return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: credential state is no longer rejected"}
}

func replacementGeneration(previous uint64, environment EnvironmentIdentity) (uint64, error) {
	if previous != 0 {
		if previous == ^uint64(0) {
			return 0, fmt.Errorf("cloud: binding_generation overflow")
		}
		return previous + 1, nil
	}
	digest, err := hex.DecodeString(environment.EnvironmentID)
	if err != nil || len(digest) < 8 {
		return 0, fmt.Errorf("cloud: invalid replacement environment identity")
	}
	generation := binary.BigEndian.Uint64(digest[:8])
	if generation == 0 {
		generation = 1
	}
	return generation, nil
}

var _ BindingStore = (*DurableBindingRepository)(nil)
