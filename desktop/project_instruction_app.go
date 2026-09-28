package main

import (
	"context"
	"errors"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

type EditCloudProjectInstructionInput struct {
	ProjectID        string `json:"projectId"`
	ExpectedVersion  uint64 `json:"expectedVersion"`
	HumanDescription string `json:"humanDescription"`
	AgentInstruction string `json:"agentInstruction"`
}

type ProjectInstructionVersionMutationInput struct {
	ProjectID       string `json:"projectId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type CloudProjectInstructionView struct {
	ProjectScopeKey  string `json:"projectScopeKey"`
	ProjectID        string `json:"projectId"`
	HumanDescription string `json:"humanDescription"`
	AgentInstruction string `json:"agentInstruction"`
	ContentHash      string `json:"contentHash"`
	Version          uint64 `json:"version"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// GetCloudProjectInstruction is always online and never falls back to the
// ordinary Project cache because that cache must not retain Instruction bodies.
func (a *App) GetCloudProjectInstruction(projectID string) (CloudProjectInstructionView, error) {
	services, err := a.projectServicesForInstructionOperation()
	if err != nil {
		return CloudProjectInstructionView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.GetProjectInstruction(ctx, projectID)
	if err != nil {
		return CloudProjectInstructionView{}, projectProtocolWailsError(err)
	}
	return cloudProjectInstructionView(result), nil
}

func (a *App) EditCloudProjectInstruction(input EditCloudProjectInstructionInput) (CloudProjectInstructionView, error) {
	domainInput := project.PutProjectInstructionInput{
		ExpectedVersion: input.ExpectedVersion, HumanDescription: input.HumanDescription,
		AgentInstruction: input.AgentInstruction,
	}
	if err := project.ValidatePutProjectInstructionInput(domainInput); err != nil {
		return CloudProjectInstructionView{}, &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: "project instruction request is invalid"}
	}
	services, err := a.projectServicesForInstructionOperation()
	if err != nil {
		return CloudProjectInstructionView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	return runCloudProjectInstructionMutation(ctx, services, input.ProjectID, domainInput)
}

func (a *App) ClearCloudProjectInstruction(input ProjectInstructionVersionMutationInput) (CloudProjectInstructionView, error) {
	return a.EditCloudProjectInstruction(EditCloudProjectInstructionInput{ProjectID: input.ProjectID, ExpectedVersion: input.ExpectedVersion})
}

func runCloudProjectInstructionMutation(ctx context.Context, services *cloudProjectServices, projectID string, input project.PutProjectInstructionInput) (CloudProjectInstructionView, error) {
	payload := map[string]any{
		"expected_version": input.ExpectedVersion, "human_description": input.HumanDescription,
		"agent_instruction": input.AgentInstruction,
	}
	if err := waitForCloudProjectCommandReplay(ctx, services); err != nil {
		return CloudProjectInstructionView{}, projectCommandPrepareWailsError(err)
	}
	record, err := services.Journal.Prepare(ctx, services.Scope, project.PrepareProjectCommandInput{
		Operation: project.ProjectCommandInstructionPut, ProjectID: projectID, Payload: payload,
	})
	if err != nil {
		return CloudProjectInstructionView{}, projectCommandPrepareWailsError(err)
	}
	workBuddyProjectFaultPoint("journal_durable_before_http", record.IdempotencyKey)
	result, err := services.Gateway.PutProjectInstruction(ctx, projectID, input, record.IdempotencyKey)
	if err != nil {
		err = settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
		return CloudProjectInstructionView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("response_before_authoritative_readback", record.IdempotencyKey)
	readback, err := services.Gateway.GetProjectInstruction(ctx, projectID)
	if err != nil {
		return CloudProjectInstructionView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("readback_before_journal_eviction", record.IdempotencyKey)
	proof := project.ProjectCommandReadback{
		ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "instruction",
		ResourceID: readback.Instruction.ProjectID, ProjectID: readback.Instruction.ProjectID,
		Version: readback.Instruction.Version,
	}
	if err := services.Journal.CompleteAfterReadback(ctx, services.Scope, record.IdempotencyKey, result, proof); err != nil {
		return CloudProjectInstructionView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "project instruction readback could not be completed")
	}
	return cloudProjectInstructionView(readback), nil
}

// settleProjectCommandFailure owns the shared pending transition only. Domain
// validation, transport, and authoritative readback remain with each caller.
func settleProjectCommandFailure(ctx context.Context, services *cloudProjectServices, key string, commandErr error) error {
	if projectMutationResultUnknown(commandErr) {
		if err := services.Journal.MarkUnknown(ctx, services.Scope, key); err != nil {
			return errors.Join(commandErr, err)
		}
	} else if projectMutationConclusiveRejection(commandErr) {
		if err := services.Journal.DiscardRejected(ctx, services.Scope, key); err != nil {
			return errors.Join(commandErr, err)
		}
	}
	return commandErr
}

func projectMutationConclusiveRejection(err error) bool {
	var typed *cloud.Error
	if !errors.As(err, &typed) {
		return false
	}
	switch typed.Code {
	case cloud.ErrorCode("VALIDATION_FAILED"), cloud.ErrorCode("NOT_FOUND"), cloud.ErrorCode("CONFLICT"),
		cloud.ErrorCode("IDEMPOTENCY_CONFLICT"), cloud.ErrorCode("QUOTA_EXCEEDED"), cloud.ErrorCode("RATE_LIMITED"):
		return true
	default:
		return false
	}
}

func cloudProjectInstructionView(result project.InstructionResult) CloudProjectInstructionView {
	value := result.Instruction
	return CloudProjectInstructionView{
		ProjectScopeKey: result.ProjectScopeKey, ProjectID: value.ProjectID,
		HumanDescription: value.HumanDescription, AgentInstruction: value.AgentInstruction,
		ContentHash: value.ContentHash, Version: value.Version,
		CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
