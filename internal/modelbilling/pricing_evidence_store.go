package modelbilling

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kwu/internal/cloud"
)

type PricingEvidenceOwner struct {
	TaskID        string
	Scope         TaskScope
	BillingSource BillingSource
}

func InsertPricingEvidence(ctx context.Context, tx *sql.Tx, owner PricingEvidenceOwner, evidence PricingEvidence) error {
	if tx == nil {
		return newError(ErrorContractInvalid, "pricing evidence transaction is required")
	}
	if err := validatePricingEvidenceOwner(ctx, tx, owner, evidence); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pricing_evidence (
		evidence_id, snapshot_id, task_id, model_id, pricing_group,
		pricing_version, pricing_hash, state, captured_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.EvidenceID, evidence.SnapshotID, evidence.TaskID, evidence.ModelID,
		evidence.PricingGroup, nullableString(evidence.PricingVersion), nullableString(evidence.PricingHash),
		evidence.State, evidence.CapturedAt.UTC().Format(time.RFC3339Nano))
	if err == nil {
		return nil
	}
	return resolvePricingEvidenceInsertError(ctx, tx, owner, evidence, err)
}

func validatePricingEvidenceOwner(ctx context.Context, tx *sql.Tx, owner PricingEvidenceOwner, evidence PricingEvidence) error {
	if owner.TaskID != evidence.TaskID || !owner.Scope.Equal(evidence.Scope) {
		return newError(ErrorScopeMismatch, "pricing evidence owner does not match task scope")
	}
	if err := evidence.Validate(owner.BillingSource); err != nil {
		return err
	}
	persistedOwner, err := loadPricingEvidenceOwner(ctx, tx, owner.TaskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return newError(ErrorContractInvalid, "model task must be inserted before pricing evidence")
		}
		return fmt.Errorf("modelbilling: load pricing evidence task owner: %w", err)
	}
	if persistedOwner.BillingSource != owner.BillingSource {
		return newError(ErrorBillingSourceMismatch, "pricing evidence billing source does not match persisted task")
	}
	if !persistedOwner.Scope.Equal(owner.Scope) {
		return newError(ErrorScopeMismatch, "pricing evidence scope does not match persisted task")
	}
	return nil
}

func resolvePricingEvidenceInsertError(ctx context.Context, tx *sql.Tx, owner PricingEvidenceOwner, evidence PricingEvidence, insertErr error) error {
	existing, existingOwner, loadErr := loadPricingEvidence(ctx, tx, evidence.EvidenceID)
	if loadErr != nil {
		return fmt.Errorf("modelbilling: insert pricing evidence: %w", insertErr)
	}
	if existingOwner.BillingSource != owner.BillingSource {
		return newError(ErrorBillingSourceMismatch, "pricing evidence id belongs to another billing source")
	}
	if !existingOwner.Scope.Equal(owner.Scope) {
		return newError(ErrorScopeMismatch, "pricing evidence id belongs to another task scope")
	}
	if pricingEvidenceEqual(existing, evidence) {
		return nil
	}
	return newError(ErrorIdempotencyConflict, "pricing evidence id reused with different content")
}

func loadPricingEvidenceOwner(ctx context.Context, tx *sql.Tx, taskID string) (PricingEvidenceOwner, error) {
	var owner PricingEvidenceOwner
	var cloudUserID sql.NullString
	var bindingGeneration sql.NullInt64
	var billingSource string
	err := tx.QueryRowContext(ctx, `SELECT
		task_id, actor_id, profile_id, cloud_user_id, binding_generation, billing_source
		FROM model_tasks WHERE task_id = ?`, taskID).Scan(
		&owner.TaskID, &owner.Scope.ActorID, &owner.Scope.ProfileID,
		&cloudUserID, &bindingGeneration, &billingSource)
	if err != nil {
		return PricingEvidenceOwner{}, err
	}
	owner.BillingSource = BillingSource(billingSource)
	if cloudUserID.Valid && bindingGeneration.Valid {
		owner.Scope.CloudBinding = &cloud.BindingScope{
			ProfileID: owner.Scope.ProfileID, CloudUserID: cloudUserID.String,
			BindingGeneration: uint64(bindingGeneration.Int64),
		}
	}
	return owner, nil
}

func loadPricingEvidence(ctx context.Context, tx *sql.Tx, evidenceID string) (PricingEvidence, PricingEvidenceOwner, error) {
	var evidence PricingEvidence
	var owner PricingEvidenceOwner
	var version, hash sql.NullString
	var cloudUserID sql.NullString
	var bindingGeneration sql.NullInt64
	var billingSource string
	var state string
	var capturedAt string
	err := tx.QueryRowContext(ctx, `SELECT
		pe.evidence_id, pe.snapshot_id, pe.task_id, pe.model_id, pe.pricing_group,
		pe.pricing_version, pe.pricing_hash, pe.state, pe.captured_at,
		mt.task_id, mt.actor_id, mt.profile_id, mt.cloud_user_id, mt.binding_generation, mt.billing_source
		FROM pricing_evidence pe
		JOIN model_tasks mt ON mt.task_id = pe.task_id
		WHERE pe.evidence_id = ?`, evidenceID).Scan(
		&evidence.EvidenceID, &evidence.SnapshotID, &evidence.TaskID, &evidence.ModelID,
		&evidence.PricingGroup, &version, &hash, &state, &capturedAt,
		&owner.TaskID, &owner.Scope.ActorID, &owner.Scope.ProfileID,
		&cloudUserID, &bindingGeneration, &billingSource)
	if err != nil {
		return PricingEvidence{}, PricingEvidenceOwner{}, err
	}
	owner.BillingSource = BillingSource(billingSource)
	if cloudUserID.Valid && bindingGeneration.Valid {
		owner.Scope.CloudBinding = &cloud.BindingScope{
			ProfileID: owner.Scope.ProfileID, CloudUserID: cloudUserID.String,
			BindingGeneration: uint64(bindingGeneration.Int64),
		}
	}
	evidence.Scope = owner.Scope
	evidence.State = PricingState(state)
	if version.Valid {
		evidence.PricingVersion = &version.String
	}
	if hash.Valid {
		evidence.PricingHash = &hash.String
	}
	evidence.CapturedAt, err = time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return PricingEvidence{}, PricingEvidenceOwner{}, persistedFieldError("pricing evidence captured_at", err)
	}
	return evidence, owner, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func pricingEvidenceEqual(left, right PricingEvidence) bool {
	return left.EvidenceID == right.EvidenceID && left.SnapshotID == right.SnapshotID &&
		left.TaskID == right.TaskID && left.Scope.Equal(right.Scope) && left.ModelID == right.ModelID &&
		left.PricingGroup == right.PricingGroup && optionalStringEqual(left.PricingVersion, right.PricingVersion) &&
		optionalStringEqual(left.PricingHash, right.PricingHash) && left.State == right.State &&
		left.CapturedAt.Equal(right.CapturedAt)
}

func optionalStringEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func (e PricingEvidence) Validate(source BillingSource) error {
	for name, value := range map[string]string{
		"evidence_id": e.EvidenceID, "snapshot_id": e.SnapshotID, "task_id": e.TaskID,
		"model_id": e.ModelID, "pricing_group": e.PricingGroup,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if err := e.Scope.Validate(source); err != nil {
		return err
	}
	if !validPricingState(e.State) || e.CapturedAt.IsZero() {
		return newError(ErrorContractInvalid, "pricing evidence state and captured_at are required")
	}
	for name, value := range map[string]*string{"pricing_version": e.PricingVersion, "pricing_hash": e.PricingHash} {
		if value != nil {
			if err := requireTrimmed(name, *value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e PricingEvidence) ValidateForTask(task ModelTask) error {
	if err := e.validateTaskOwnership(task); err != nil {
		return err
	}
	if e.EvidenceID != task.PricingEvidenceID {
		return newError(ErrorContractInvalid, "task pricing evidence id does not match")
	}
	return nil
}

func (e PricingEvidence) ValidateForCall(task ModelTask, call ModelCall) error {
	if err := e.validateTaskOwnership(task); err != nil {
		return err
	}
	if call.TaskID != task.TaskID || !call.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "call does not belong to pricing evidence task")
	}
	if call.BillingSource != task.BillingSource {
		return newError(ErrorBillingSourceMismatch, "call must inherit task billing source")
	}
	if call.PricingEvidenceID != e.EvidenceID || call.ModelID != e.ModelID {
		return newError(ErrorContractInvalid, "call pricing evidence must match task and model")
	}
	return nil
}

func (e PricingEvidence) validateTaskOwnership(task ModelTask) error {
	if err := e.Validate(task.BillingSource); err != nil {
		return err
	}
	if e.TaskID != task.TaskID || !e.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "pricing evidence does not belong to task scope")
	}
	return nil
}
