package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

func (g *ProjectGateway) ListMessages(ctx context.Context, projectID string, input ListMessagesInput) (ActivityPage, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ActivityPage{}, err
	}
	query, err := messageListQuery(input)
	if err != nil {
		return ActivityPage{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ActivityPage{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, projectPath(projectID)+"/messages", query, nil, token, nil, &payload)
	})
	if err != nil {
		return ActivityPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ActivityPage{}, err
	}
	return decodeMessagePage(payload, scope, projectID)
}

func (g *ProjectGateway) MarkMessageRead(ctx context.Context, projectID, throughCursor string) (string, ActivityReadResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", ActivityReadResult{}, err
	}
	if throughCursor == "" || len([]byte(throughCursor)) > 4096 {
		return "", ActivityReadResult{}, fmt.Errorf("projects-v2: invalid message read cursor")
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ActivityReadResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPut, projectPath(projectID)+"/message-read", map[string]any{"through_cursor": throughCursor}, token, nil, &payload)
	})
	if err != nil {
		return "", ActivityReadResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ActivityReadResult{}, err
	}
	var out ActivityReadResult
	if err := decodeV2Payload(payload, &out, "through_activity_id", "updated_at"); err != nil {
		return "", out, err
	}
	return scope.Key, out, nil
}

func decodeMessagePage(payload json.RawMessage, scope Scope, projectID string) (ActivityPage, error) {
	var page ActivityPage
	if err := decodeV2Payload(payload, &page, "items", "next_cursor", "read_cursor"); err != nil {
		return page, err
	}
	if page.Items == nil {
		page.Items = []ProjectActivity{}
	}
	for _, item := range page.Items {
		if item.ProjectID != projectID || item.Kind != "user_message" {
			return ActivityPage{}, fmt.Errorf("projects-v2: invalid message list response")
		}
		if err := validateMessageReadStates(item); err != nil {
			return ActivityPage{}, err
		}
	}
	page.scopeKey, page.projectID = scope.Key, projectID
	return page, nil
}

func validateMessageReadStates(item ProjectActivity) error {
	if item.OtherMembersUnread == nil || item.UnreadOtherMemberCount == nil || item.OtherMemberReadStates == nil {
		return fmt.Errorf("projects-v2: incomplete message member read states")
	}
	states := *item.OtherMemberReadStates
	if !slices.IsSortedFunc(states, func(a, b ProjectMessageMemberReadState) int { return stringCompare(a.UserID, b.UserID) }) {
		return fmt.Errorf("projects-v2: unordered message member read states")
	}
	if !validMessageMemberReadStates(states) {
		return fmt.Errorf("projects-v2: invalid message member read state")
	}
	unread := countUnreadMessageMembers(states)
	if unread != *item.UnreadOtherMemberCount || *item.OtherMembersUnread != (unread > 0) {
		return fmt.Errorf("projects-v2: inconsistent message member read summary")
	}
	return nil
}

func validMessageMemberReadStates(states []ProjectMessageMemberReadState) bool {
	for i, state := range states {
		if state.UserID == "" || state.DisplayName == "" || (state.Status != "read" && state.Status != "unread") {
			return false
		}
		if i > 0 && states[i-1].UserID == state.UserID {
			return false
		}
	}
	return true
}

func countUnreadMessageMembers(states []ProjectMessageMemberReadState) int64 {
	var unread int64
	for _, state := range states {
		if state.Status == "unread" {
			unread++
		}
	}
	return unread
}

func stringCompare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func messageListQuery(input ListMessagesInput) (url.Values, error) {
	if input.PageSize == 0 {
		input.PageSize = 20
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		return nil, fmt.Errorf("projects-v2: invalid message page size")
	}
	query := url.Values{"page_size": {strconv.Itoa(input.PageSize)}}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	return query, nil
}
