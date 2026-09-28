package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

// replayPendingCloudProjectCommands resolves the durable unknown-result window
// after a Desktop restart. It reuses the exact journal-owned idempotency key
// and canonical payload, then removes the record only after an authoritative
// readback proves the committed resource identity and version.
func replayPendingCloudProjectCommands(ctx context.Context, services *cloudProjectServices) error {
	if services == nil || services.Gateway == nil || services.Journal == nil {
		return fmt.Errorf("projects-v1: replay services are unavailable")
	}
	pending, err := services.Journal.Pending(ctx, services.Scope)
	if err != nil {
		return err
	}
	var replayErrors []error
	for _, record := range pending {
		if err := replayPendingCloudProjectCommand(ctx, services, record); err != nil {
			replayErrors = append(replayErrors, fmt.Errorf("%s: %w", record.Operation, err))
		}
	}
	return errors.Join(replayErrors...)
}

// waitForCloudProjectCommandReplay keeps a newly prepared foreground command
// out of the startup replay snapshot. Recovery is allowed to fail closed and
// release the waiter; the foreground command still owns its own journal entry.
func waitForCloudProjectCommandReplay(ctx context.Context, services *cloudProjectServices) error {
	if services == nil || services.commandReplayDone == nil {
		return nil
	}
	select {
	case <-services.commandReplayDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func replayPendingCloudProjectCommand(ctx context.Context, services *cloudProjectServices, record project.ProjectCommandRecord) error {
	fields, err := decodeProjectReplayFields(record.CanonicalPayload)
	if err != nil {
		return err
	}
	var result project.CommandResult
	switch record.Operation {
	case project.ProjectCommandCreate:
		name, decodeErr := requiredReplayString(fields, "name")
		if decodeErr != nil {
			return decodeErr
		}
		description, decodeErr := nullableReplayString(fields, "description")
		if decodeErr != nil {
			return decodeErr
		}
		humanDescription, decodeErr := optionalReplayString(fields, "human_description")
		if decodeErr != nil {
			return decodeErr
		}
		agentInstruction, decodeErr := optionalReplayString(fields, "agent_instruction")
		if decodeErr != nil {
			return decodeErr
		}
		skillID, decodeErr := optionalReplayString(fields, "skill_id")
		if decodeErr != nil {
			return decodeErr
		}
		expertID, decodeErr := optionalReplayString(fields, "expert_id")
		if decodeErr != nil {
			return decodeErr
		}
		connectorID, decodeErr := optionalReplayString(fields, "connector_id")
		if decodeErr != nil {
			return decodeErr
		}
		bindings, decodeErr := optionalReplayProjectBindings(fields)
		if decodeErr != nil {
			return decodeErr
		}
		if len(bindings) > 0 {
			human, instruction := "", ""
			if humanDescription != nil {
				human = *humanDescription
			}
			if agentInstruction != nil {
				instruction = *agentInstruction
			}
			created, createErr := services.Gateway.CreateAggregate(ctx, project.AggregateCreateInput{
				Name: name, Description: description, HumanDescription: human, AgentInstruction: instruction, Bindings: bindings,
			}, record.IdempotencyKey)
			err = createErr
			if createErr == nil {
				result = project.CommandResult{
					ProjectScopeKey: created.ProjectScopeKey, OperationID: "op_local_" + record.IdempotencyKey[:32],
					ResourceKind: "project", ResourceID: created.ProjectID, ProjectID: created.ProjectID, CommittedVersion: created.ProjectVersion,
				}
			}
		} else {
			result, err = services.Gateway.CreateProject(ctx, project.CreateProjectInput{
				Name: name, Description: description, HumanDescription: humanDescription, AgentInstruction: agentInstruction,
				SkillID: skillID, ExpertID: expertID, ConnectorID: connectorID,
			}, record.IdempotencyKey)
		}
		if err == nil && (humanDescription != nil || agentInstruction != nil) {
			if verifyErr := verifyInitialCloudProjectInstruction(ctx, services, result.ProjectID, humanDescription, agentInstruction); verifyErr != nil {
				return verifyErr
			}
		}
	case project.ProjectCommandPatch:
		input, decodeErr := decodeReplayProjectPatch(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.PatchProject(ctx, record.ProjectID, input, record.IdempotencyKey)
	case project.ProjectCommandInstructionPut:
		input, decodeErr := decodeReplayInstruction(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.PutProjectInstruction(ctx, record.ProjectID, input, record.IdempotencyKey)
	case project.ProjectCommandWorkItemCreate:
		input, decodeErr := decodeReplayWorkItemCreate(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.CreateWorkItem(ctx, record.ProjectID, input, record.IdempotencyKey)
	case project.ProjectCommandWorkItemPatch:
		input, decodeErr := decodeReplayWorkItemPatch(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.PatchWorkItem(ctx, record.ProjectID, record.ResourceID, input, record.IdempotencyKey)
	case project.ProjectCommandWorkItemMove:
		input, decodeErr := decodeReplayWorkItemMove(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.MoveWorkItem(ctx, record.ProjectID, record.ResourceID, input, record.IdempotencyKey)
	case project.ProjectCommandWorkItemDelete:
		version, decodeErr := requiredReplayVersion(fields)
		if decodeErr != nil {
			return decodeErr
		}
		result, err = services.Gateway.DeleteWorkItem(ctx, record.ProjectID, record.ResourceID, project.DeleteWorkItemInput{ExpectedVersion: version}, record.IdempotencyKey)
	default:
		return fmt.Errorf("projects-v1: unsupported replay operation")
	}
	if err != nil {
		return settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
	}
	if record.Operation == project.ProjectCommandWorkItemDelete {
		if err := proveCloudProjectWorkItemAbsent(ctx, services, record.ProjectID, record.ResourceID); err != nil {
			return err
		}
		proof := project.ProjectCommandDeletionReadback{ProjectScopeKey: services.Scope.Key, ResourceKind: "work_item", ResourceID: record.ResourceID, ProjectID: record.ProjectID, Absent: true}
		return services.Journal.CompleteAfterDeletion(ctx, services.Scope, record.IdempotencyKey, result, proof)
	}
	proof, err := authoritativeProjectReplayReadback(ctx, services, record.Operation, result)
	if err != nil {
		return err
	}
	return services.Journal.CompleteAfterReadback(ctx, services.Scope, record.IdempotencyKey, result, proof)
}

func authoritativeProjectReplayReadback(ctx context.Context, services *cloudProjectServices, operation project.ProjectCommandOperation, result project.CommandResult) (project.ProjectCommandReadback, error) {
	switch operation {
	case project.ProjectCommandCreate, project.ProjectCommandPatch:
		readback, err := services.Gateway.GetProject(ctx, result.ProjectID)
		if err != nil {
			return project.ProjectCommandReadback{}, err
		}
		return project.ProjectCommandReadback{ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "project", ResourceID: readback.Project.ProjectID, ProjectID: readback.Project.ProjectID, Version: readback.Project.Version}, nil
	case project.ProjectCommandInstructionPut:
		readback, err := services.Gateway.GetProjectInstruction(ctx, result.ProjectID)
		if err != nil {
			return project.ProjectCommandReadback{}, err
		}
		return project.ProjectCommandReadback{ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "instruction", ResourceID: readback.Instruction.ProjectID, ProjectID: readback.Instruction.ProjectID, Version: readback.Instruction.Version}, nil
	case project.ProjectCommandWorkItemCreate, project.ProjectCommandWorkItemPatch, project.ProjectCommandWorkItemMove:
		readback, err := services.Gateway.GetWorkItem(ctx, result.ProjectID, result.ResourceID)
		if err != nil {
			return project.ProjectCommandReadback{}, err
		}
		return project.ProjectCommandReadback{ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "work_item", ResourceID: readback.WorkItem.WorkItemID, ProjectID: readback.WorkItem.ProjectID, Version: readback.WorkItem.Version}, nil
	default:
		return project.ProjectCommandReadback{}, fmt.Errorf("projects-v1: unsupported replay readback")
	}
}

func decodeProjectReplayFields(payload []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, fmt.Errorf("projects-v1: decode replay payload")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("projects-v1: replay payload has trailing data")
	}
	return fields, nil
}

func requiredReplayString(fields map[string]json.RawMessage, name string) (string, error) {
	raw, ok := fields[name]
	if !ok || bytes.Equal(raw, []byte("null")) {
		return "", fmt.Errorf("projects-v1: replay payload is missing %s", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("projects-v1: replay payload has invalid %s", name)
	}
	return value, nil
}

func requiredReplayVersion(fields map[string]json.RawMessage) (uint64, error) {
	raw, ok := fields["expected_version"]
	if !ok {
		return 0, fmt.Errorf("projects-v1: replay payload is missing expected_version")
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("projects-v1: replay payload has invalid expected_version")
	}
	return value, nil
}

func optionalReplayString(fields map[string]json.RawMessage, name string) (*string, error) {
	raw, ok := fields[name]
	if !ok || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("projects-v1: replay payload has invalid %s", name)
	}
	return &value, nil
}

func nullableReplayString(fields map[string]json.RawMessage, name string) (project.NullableStringUpdate, error) {
	raw, ok := fields[name]
	if !ok {
		return project.NullableStringUpdate{}, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return project.NullableStringUpdate{Set: true}, nil
	}
	value, err := optionalReplayString(fields, name)
	return project.NullableStringUpdate{Set: true, Value: value}, err
}

func optionalReplayProjectBindings(fields map[string]json.RawMessage) ([]project.ProjectBindingInput, error) {
	raw, ok := fields["bindings"]
	if !ok {
		return nil, nil
	}
	var bindings []project.ProjectBindingInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bindings); err != nil || len(bindings) == 0 {
		return nil, fmt.Errorf("projects-v1: replay payload has invalid bindings")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("projects-v1: replay bindings have trailing data")
	}
	return bindings, nil
}

func decodeReplayProjectPatch(fields map[string]json.RawMessage) (project.PatchProjectInput, error) {
	version, err := requiredReplayVersion(fields)
	if err != nil {
		return project.PatchProjectInput{}, err
	}
	name, err := optionalReplayString(fields, "name")
	if err != nil {
		return project.PatchProjectInput{}, err
	}
	description, err := nullableReplayString(fields, "description")
	if err != nil {
		return project.PatchProjectInput{}, err
	}
	statusText, err := optionalReplayString(fields, "status")
	if err != nil {
		return project.PatchProjectInput{}, err
	}
	var status *project.ProjectStatus
	if statusText != nil {
		value := project.ProjectStatus(*statusText)
		status = &value
	}
	return project.PatchProjectInput{ExpectedVersion: version, Name: name, Description: description, Status: status}, nil
}

func decodeReplayInstruction(fields map[string]json.RawMessage) (project.PutProjectInstructionInput, error) {
	version, err := requiredReplayVersion(fields)
	if err != nil {
		return project.PutProjectInstructionInput{}, err
	}
	human, err := requiredReplayString(fields, "human_description")
	if err != nil {
		return project.PutProjectInstructionInput{}, err
	}
	agent, err := requiredReplayString(fields, "agent_instruction")
	if err != nil {
		return project.PutProjectInstructionInput{}, err
	}
	return project.PutProjectInstructionInput{ExpectedVersion: version, HumanDescription: human, AgentInstruction: agent}, nil
}

func decodeReplayWorkItemCreate(fields map[string]json.RawMessage) (project.CreateWorkItemInput, error) {
	title, err := requiredReplayString(fields, "title")
	if err != nil {
		return project.CreateWorkItemInput{}, err
	}
	description, err := nullableReplayString(fields, "description")
	if err != nil {
		return project.CreateWorkItemInput{}, err
	}
	status, err := optionalReplayString(fields, "status")
	if err != nil {
		return project.CreateWorkItemInput{}, err
	}
	var statusValue project.WorkItemStatus
	if status != nil {
		statusValue = project.WorkItemStatus(*status)
	}
	return project.CreateWorkItemInput{Title: title, Description: description, Status: statusValue}, nil
}

func decodeReplayWorkItemPatch(fields map[string]json.RawMessage) (project.PatchWorkItemInput, error) {
	version, err := requiredReplayVersion(fields)
	if err != nil {
		return project.PatchWorkItemInput{}, err
	}
	title, err := optionalReplayString(fields, "title")
	if err != nil {
		return project.PatchWorkItemInput{}, err
	}
	description, err := nullableReplayString(fields, "description")
	return project.PatchWorkItemInput{ExpectedVersion: version, Title: title, Description: description}, err
}

func decodeReplayWorkItemMove(fields map[string]json.RawMessage) (project.MoveWorkItemInput, error) {
	version, err := requiredReplayVersion(fields)
	if err != nil {
		return project.MoveWorkItemInput{}, err
	}
	status, err := requiredReplayString(fields, "target_status")
	if err != nil {
		return project.MoveWorkItemInput{}, err
	}
	before, err := optionalReplayString(fields, "before_work_item_id")
	if err != nil {
		return project.MoveWorkItemInput{}, err
	}
	after, err := optionalReplayString(fields, "after_work_item_id")
	if err != nil {
		return project.MoveWorkItemInput{}, err
	}
	return project.MoveWorkItemInput{ExpectedVersion: version, TargetStatus: project.WorkItemStatus(status), BeforeWorkItemID: before, AfterWorkItemID: after}, nil
}

func startCloudProjectCommandReplay(services *cloudProjectServices) {
	if services == nil {
		return
	}
	go func() {
		if services.commandReplayDone != nil {
			defer close(services.commandReplayDone)
		}
		ctx, cancel := services.operationContext(context.Background())
		defer cancel()
		if err := replayPendingCloudProjectCommands(ctx, services); err != nil && !errors.Is(err, context.Canceled) {
			code := "deferred"
			var typed *cloud.Error
			if errors.As(err, &typed) {
				code = string(typed.Code)
			}
			slog.Warn("project command replay deferred", "code", code)
		}
	}()
}
