package modelbilling

import (
	"context"
	"database/sql"
)

// CallEvidencePreparationInput contains only task-owned facts plus the
// resolver-generated evidence identity. Model call sites never construct it.
type CallEvidencePreparationInput struct {
	Task            TaskContext
	InitialEvidence PricingEvidence
	EvidenceID      string
	ModelID         string
}

type CallEvidencePreparer interface {
	PrepareForCall(context.Context, CallEvidencePreparationInput) (PricingEvidence, error)
}

type CallEvidenceStore interface {
	LoadForCall(context.Context, TaskContext, string) (PricingEvidence, error)
	StoreForCall(context.Context, TaskContext, PricingEvidence) error
}

type CallEvidenceResolver interface {
	ResolveForCall(context.Context, TaskContext, string) (PricingEvidence, error)
}

// DefaultCallEvidenceResolver is the single evidence path for derived calls.
// It reuses the persisted initial evidence or prepares and stores an additive
// model-specific evidence row before the call intent can be persisted.
type DefaultCallEvidenceResolver struct {
	Store    CallEvidenceStore
	Preparer CallEvidencePreparer
	IDs      IDGenerator
}

func (r DefaultCallEvidenceResolver) ResolveForCall(ctx context.Context, task TaskContext, modelID string) (PricingEvidence, error) {
	if err := requireTrimmed("call model_id", modelID); err != nil {
		return PricingEvidence{}, err
	}
	if r.Store == nil {
		return PricingEvidence{}, newError(ErrorContractInvalid, "call evidence store is required")
	}
	initial, err := r.Store.LoadForCall(ctx, task, task.PricingEvidenceID)
	if err != nil {
		return PricingEvidence{}, err
	}
	if err := validateInitialCallEvidence(task, initial); err != nil {
		return PricingEvidence{}, err
	}
	if modelID == task.InitialModelID {
		return initial, nil
	}
	return r.resolveDerived(ctx, task, initial, modelID)
}

func (r DefaultCallEvidenceResolver) resolveDerived(ctx context.Context, task TaskContext, initial PricingEvidence, modelID string) (PricingEvidence, error) {
	if r.Preparer == nil || r.IDs == nil {
		return PricingEvidence{}, newError(ErrorContractInvalid, "derived call evidence preparer and id generator are required")
	}
	evidenceID, err := r.IDs.NewID()
	if err != nil {
		return PricingEvidence{}, err
	}
	evidence, err := r.Preparer.PrepareForCall(ctx, CallEvidencePreparationInput{
		Task: task, InitialEvidence: initial, EvidenceID: evidenceID, ModelID: modelID,
	})
	if err != nil {
		return PricingEvidence{}, err
	}
	if err := validateDerivedCallEvidence(task, initial, evidenceID, modelID, evidence); err != nil {
		return PricingEvidence{}, err
	}
	if err := r.Store.StoreForCall(ctx, task, evidence); err != nil {
		return PricingEvidence{}, err
	}
	return evidence, nil
}

func validateInitialCallEvidence(task TaskContext, evidence PricingEvidence) error {
	facts := modelTaskFromContext(task)
	if evidence.ModelID != task.InitialModelID {
		return newError(ErrorContractInvalid, "initial pricing evidence model does not match task context")
	}
	return evidence.ValidateForTask(facts)
}

func validateDerivedCallEvidence(task TaskContext, initial PricingEvidence, evidenceID, modelID string, evidence PricingEvidence) error {
	if err := evidence.Validate(task.BillingSource); err != nil {
		return err
	}
	if evidence.TaskID != task.TaskID || !evidence.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "derived pricing evidence does not belong to task scope")
	}
	if evidence.EvidenceID != evidenceID || evidence.ModelID != modelID {
		return newError(ErrorContractInvalid, "derived pricing evidence identity does not match resolver input")
	}
	if evidence.PricingGroup != initial.PricingGroup {
		return newError(ErrorContractInvalid, "derived pricing evidence changed the task pricing group")
	}
	return nil
}

func modelTaskFromContext(task TaskContext) ModelTask {
	return ModelTask{
		TaskID: task.TaskID, Scope: cloneTaskScope(task.Scope), BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	}
}

// SQLiteCallEvidenceStore is the additive evidence persistence boundary used
// by DefaultCallEvidenceResolver.
type SQLiteCallEvidenceStore struct{ db *sql.DB }

func NewSQLiteCallEvidenceStore(db *sql.DB) *SQLiteCallEvidenceStore {
	return &SQLiteCallEvidenceStore{db: db}
}

func (s *SQLiteCallEvidenceStore) LoadForCall(ctx context.Context, taskContext TaskContext, evidenceID string) (PricingEvidence, error) {
	if err := validateCallEvidenceDatabase(s); err != nil {
		return PricingEvidence{}, err
	}
	if err := requireTrimmed("pricing_evidence_id", evidenceID); err != nil {
		return PricingEvidence{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return PricingEvidence{}, err
	}
	defer tx.Rollback()
	task, err := loadCallEvidenceTask(ctx, tx, taskContext)
	if err != nil {
		return PricingEvidence{}, err
	}
	evidence, owner, err := loadPricingEvidence(ctx, tx, evidenceID)
	if err != nil {
		return PricingEvidence{}, taskStoreLoadError(err)
	}
	if err := validateLoadedCallEvidence(task, evidence, owner); err != nil {
		return PricingEvidence{}, err
	}
	if err := tx.Commit(); err != nil {
		return PricingEvidence{}, err
	}
	return evidence, nil
}

func validateCallEvidenceDatabase(store *SQLiteCallEvidenceStore) error {
	if store == nil || store.db == nil {
		return newError(ErrorContractInvalid, "call evidence database is required")
	}
	return nil
}

func validateLoadedCallEvidence(task ModelTask, evidence PricingEvidence, owner PricingEvidenceOwner) error {
	if owner.TaskID != task.TaskID || !owner.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "pricing evidence does not belong to call task scope")
	}
	if owner.BillingSource != task.BillingSource {
		return newError(ErrorBillingSourceMismatch, "pricing evidence billing source does not match call task")
	}
	return evidence.validateTaskOwnership(task)
}

func (s *SQLiteCallEvidenceStore) StoreForCall(ctx context.Context, taskContext TaskContext, evidence PricingEvidence) error {
	if s == nil || s.db == nil {
		return newError(ErrorContractInvalid, "call evidence database is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadCallEvidenceTask(ctx, tx, taskContext)
	if err != nil {
		return err
	}
	if err := evidence.validateTaskOwnership(task); err != nil {
		return err
	}
	if err := InsertPricingEvidence(ctx, tx, PricingEvidenceOwner{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
	}, evidence); err != nil {
		return err
	}
	return tx.Commit()
}

func loadCallEvidenceTask(ctx context.Context, tx *sql.Tx, taskContext TaskContext) (ModelTask, error) {
	task, err := loadTask(ctx, tx, taskContext.TaskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if err := validatePersistedTaskContext(taskContext, task); err != nil {
		return ModelTask{}, err
	}
	if task.State != TaskReady && task.State != TaskRunning {
		return ModelTask{}, newError(ErrorInvalidStateTransition, "call evidence requires ready or running task")
	}
	return task, nil
}
