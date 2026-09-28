package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestProjectGatewayListUsesAuthenticatedScope(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if got := r.URL.Query().Get("page_size"); got != "20" {
			t.Fatalf("page_size = %q", got)
		}
		if got := r.URL.Query().Get("status"); got != "active" {
			t.Fatalf("status = %q", got)
		}
		if r.URL.Query().Has("ownership") {
			t.Fatalf("default project list unexpectedly set ownership: %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s,"error":null,"request_id":"req-project-list","server_time":"2026-08-05T10:00:01Z"}`, validProjectPageJSON)
	}))
	defer server.Close()

	transport, err := cloud.NewHTTPSessionClient(server.URL, server.Client(), cloud.DeviceIdentity{DeviceID: "device-project", ClientVersion: "test"}, func() string { return "req-desktop-project" })
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	executor := &gatewayTestExecutor{token: "access-secret"}
	resolver := &gatewayTestScopeResolver{scope: scope}
	gateway, err := NewProjectGateway(executor, transport, resolver)
	if err != nil {
		t.Fatalf("NewProjectGateway: %v", err)
	}

	page, err := gateway.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if view := page.View(); view.ProjectScopeKey != scope.Key || len(view.Items) != 1 || view.Items[0].ProjectID != "project-1" {
		t.Fatalf("project page view = %#v", view)
	}
	if calls := executor.Scopes(); len(calls) != 1 || calls[0] != scope.Binding {
		t.Fatalf("executor scopes = %#v", calls)
	}
}

func TestProjectGatewayListOwnedBindsOwnershipStatusAndCursor(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("ownership") != "owned" || query.Get("status") != "all" || query.Get("cursor") != "owned-cursor" || query.Get("page_size") != "20" {
			t.Fatalf("owned query = %v", query)
		}
		fmt.Fprintf(w, `{"data":%s,"error":null,"request_id":"req-project-list-owned","server_time":"2026-08-20T10:00:01Z"}`, validProjectPageJSON)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	page, err := gateway.ListOwnedProjects(context.Background(), ListProjectsInput{Status: ProjectStatus("all"), Cursor: "owned-cursor"})
	if err != nil {
		t.Fatalf("ListOwnedProjects: %v", err)
	}
	if view := page.View(); view.ProjectScopeKey != scope.Key || len(view.Items) != 1 {
		t.Fatalf("owned project page = %#v", view)
	}
}

func TestProjectGatewayListsCurrentActiveMembers(t *testing.T) {
	scope := gatewayTestScope(t, "user-member", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-2/members" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"project_id":"project-2","user_id":"user-owner","role":"owner","status":"active","display_name":"Project Owner","version":1,"created_at":"2026-08-20T10:00:00Z","updated_at":"2026-08-20T10:00:00Z"},{"project_id":"project-2","user_id":"user-member","role":"member","status":"active","display_name":"Current Member","version":2,"created_at":"2026-08-20T10:10:00Z","updated_at":"2026-08-20T10:20:00Z"}],"error":null,"request_id":"req-members","server_time":"2026-08-20T10:35:00Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	result, err := gateway.ListMembers(context.Background(), "project-2")
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if result.ProjectScopeKey != scope.Key || result.ProjectID != "project-2" || len(result.Items) != 2 {
		t.Fatalf("members = %#v", result)
	}
	if result.Items[0].Role != "owner" || result.Items[1].UserID != "user-member" || result.Items[1].Status != "active" {
		t.Fatalf("member relationships = %#v", result.Items)
	}
}

func TestValidateProjectMembersRequiresActiveCurrentMembership(t *testing.T) {
	timestamp := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	owner := ProjectMember{ProjectID: "project-2", UserID: "user-owner", Role: "owner", Status: "active", DisplayName: "Owner", Version: 1, CreatedAt: timestamp, UpdatedAt: timestamp}
	current := ProjectMember{ProjectID: "project-2", UserID: "user-member", Role: "member", Status: "active", DisplayName: "Member", Version: 1, CreatedAt: timestamp, UpdatedAt: timestamp}

	if _, err := validateProjectMembers([]ProjectMember{owner, current}, "project-2", "user-member"); err != nil {
		t.Fatalf("valid active members: %v", err)
	}
	inactive := current
	inactive.Status = "inactive"
	if _, err := validateProjectMembers([]ProjectMember{owner, inactive}, "project-2", "user-member"); err == nil {
		t.Fatal("inactive member response was accepted")
	}
	if _, err := validateProjectMembers([]ProjectMember{owner}, "project-2", "user-member"); err == nil {
		t.Fatal("response without the current member was accepted")
	}
}

func TestProjectGatewayJoinUsesAuthenticatedIdentityAndExactBody(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/join" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		body := decodeGatewayBody(t, r)
		if len(body) != 1 || body["project_id"] != "prj_shared" {
			t.Fatalf("join body = %#v, want only project_id", body)
		}
		fmt.Fprint(w, `{"data":{"project_id":"prj_shared","user_id":"user-1","role":"member","status":"active","display_name":"张三","version":1,"created_at":"2026-08-20T10:30:00Z","updated_at":"2026-08-20T10:30:00Z"},"error":null,"request_id":"req-join","server_time":"2026-08-20T10:30:00Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	result, err := gateway.JoinProject(context.Background(), "prj_shared")
	if err != nil {
		t.Fatalf("JoinProject: %v", err)
	}
	if result.ProjectScopeKey != scope.Key || result.Member.ProjectID != "prj_shared" || result.Member.UserID != "user-1" || result.Member.Status != "active" {
		t.Fatalf("join result = %#v", result)
	}
}

func TestProjectGatewayListsJoinedProjectsWithFilters(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/joined" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("page_size") != "20" || query.Get("status") != "active" || query.Get("search") != "共享" || query.Get("cursor") != "next-page" {
			t.Fatalf("joined query = %v", query)
		}
		if query.Has("ownership") {
			t.Fatalf("joined query unexpectedly set ownership: %v", query)
		}
		fmt.Fprint(w, `{"data":{"items":[{"project_id":"prj_owned","owner_user_id":"user-1","name":"自己的项目","description":null,"status":"active","version":1,"created_at":"2026-08-20T09:00:00Z","updated_at":"2026-08-20T09:30:00Z"},{"project_id":"prj_shared","owner_user_id":"user-owner","name":"共享项目","description":"项目说明","status":"active","version":1,"created_at":"2026-08-20T10:00:00Z","updated_at":"2026-08-20T10:30:00Z"}],"next_cursor":null},"error":null,"request_id":"req-joined","server_time":"2026-08-20T10:35:00Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	page, err := gateway.ListJoinedProjects(context.Background(), ListProjectsInput{PageSize: 20, Status: ProjectActive, Search: "共享", Cursor: "next-page"})
	if err != nil {
		t.Fatalf("ListJoinedProjects: %v", err)
	}
	view := page.View()
	if view.ProjectScopeKey != scope.Key || len(view.Items) != 1 || view.Items[0].ProjectID != "prj_shared" || view.Items[0].Name != "共享项目" {
		t.Fatalf("joined page = %#v", view)
	}
}

func TestProjectGatewayClaimsServerOwnedOnboardingProject(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/onboarding/claim" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body := decodeGatewayBody(t, r)
		if len(body) != 0 {
			t.Fatalf("claim body = %#v, want server-owned empty intent", body)
		}
		fmt.Fprint(w, `{"data":{"project":{"project_id":"project-guide","owner_user_id":"user-1","name":"项目指引","description":"第一次使用项目的操作指南","status":"active","version":1,"created_at":"2026-08-10T10:00:01Z","updated_at":"2026-08-10T10:00:01Z"},"guide_version":1,"state":"active","created":true},"error":null,"request_id":"req-onboarding","server_time":"2026-08-10T10:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	result, err := gateway.ClaimOnboardingProject(context.Background())
	if err != nil {
		t.Fatalf("ClaimOnboardingProject: %v", err)
	}
	if result.ProjectScopeKey != scope.Key || result.Project == nil || result.Project.ProjectID != "project-guide" || result.GuideVersion != 1 || result.State != ProjectOnboardingStateActive || !result.Created {
		t.Fatalf("onboarding result = %#v", result)
	}
}

func TestProjectGatewayProjectRoutesAndCommandResult(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	requestIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIndex++
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch requestIndex {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects" || r.Header.Get("X-Idempotency-Key") != "key-create" {
				t.Fatalf("create request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
			body := decodeGatewayBody(t, r)
			if body["name"] != "Beta" || body["human_description"] != "调研模板" || body["agent_instruction"] != "# 角色\n请调研" || body["skill_id"] != "@reasonix/market-research" || body["expert_id"] != "exp_market_analyst" || body["connector_id"] != "github" {
				t.Fatalf("create body = %#v", body)
			}
			if _, present := body["description"]; present {
				t.Fatalf("create omitted description encoded as %#v", body["description"])
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":1},"error":null,"request_id":"req-create","server_time":"2026-08-05T10:00:01Z"}`)
		case 2:
			if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-2" {
				t.Fatalf("get request = %s %s", r.Method, r.URL.Path)
			}
			fmt.Fprint(w, `{"data":{"project_id":"project-2","owner_user_id":"user-1","name":"Beta","description":null,"status":"active","version":1,"created_at":"2026-08-05T10:00:01Z","updated_at":"2026-08-05T10:00:01Z"},"error":null,"request_id":"req-get","server_time":"2026-08-05T10:00:02Z"}`)
		case 3:
			if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/projects/project-2" || r.Header.Get("X-Idempotency-Key") != "key-patch" {
				t.Fatalf("patch request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
			body := decodeGatewayBody(t, r)
			if body["expected_version"] != float64(1) {
				t.Fatalf("patch body = %#v", body)
			}
			if value, present := body["description"]; !present || value != nil {
				t.Fatalf("patch clear description = %#v, present=%t", value, present)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-patch","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":2},"error":null,"request_id":"req-patch","server_time":"2026-08-05T10:00:03Z"}`)
		case 4:
			if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/projects/project-2" || r.Header.Get("X-Idempotency-Key") != "key-delete" {
				t.Fatalf("delete request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
			body := decodeGatewayBody(t, r)
			if body["expected_version"] != float64(2) {
				t.Fatalf("delete body = %#v", body)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-delete","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":3},"error":null,"request_id":"req-delete","server_time":"2026-08-05T10:00:04Z"}`)
		default:
			t.Fatalf("unexpected request %d", requestIndex)
		}
	}))
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})

	humanDescription := "调研模板"
	agentInstruction := "# 角色\n请调研"
	skillID, expertID, connectorID := "@reasonix/market-research", "exp_market_analyst", "github"
	created, err := gateway.CreateProject(context.Background(), CreateProjectInput{
		Name: "Beta", HumanDescription: &humanDescription, AgentInstruction: &agentInstruction,
		SkillID: &skillID, ExpertID: &expertID, ConnectorID: &connectorID,
	}, "key-create")
	if err != nil || created.ResourceID != "project-2" || created.ProjectScopeKey != scope.Key || created.CommittedVersion != 1 {
		t.Fatalf("CreateProject = %#v, %v", created, err)
	}
	got, err := gateway.GetProject(context.Background(), "project-2")
	if err != nil || got.Project.ProjectID != "project-2" || got.Project.OwnerUserID != "user-1" || got.ProjectScopeKey != scope.Key {
		t.Fatalf("GetProject = %#v, %v", got, err)
	}
	patched, err := gateway.PatchProject(context.Background(), "project-2", PatchProjectInput{
		ExpectedVersion: 1,
		Description:     NullableStringUpdate{Set: true, Value: nil},
	}, "key-patch")
	if err != nil || patched.ResourceID != "project-2" || patched.CommittedVersion != 2 {
		t.Fatalf("PatchProject = %#v, %v", patched, err)
	}
	deleted, err := gateway.DeleteProject(context.Background(), "project-2", 2, "key-delete")
	if err != nil || deleted.ResourceID != "project-2" || deleted.CommittedVersion != 3 {
		t.Fatalf("DeleteProject = %#v, %v", deleted, err)
	}
}

func TestProjectGatewayRequestFailureDoesNotPoisonLaterRequest(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "forbidden", status: http.StatusForbidden},
		{name: "not found", status: http.StatusNotFound},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "invalid response", status: http.StatusOK, body: `{"data":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					if test.status != http.StatusOK {
						w.WriteHeader(test.status)
						fmt.Fprintf(w, `{"data":null,"error":{"code":"REQUEST_FAILED","message":"request failed"},"request_id":"req-failed","server_time":"2026-08-05T10:00:01Z"}`)
						return
					}
					fmt.Fprint(w, test.body)
					return
				}
				fmt.Fprint(w, `{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-success","server_time":"2026-08-05T10:00:02Z"}`)
			}))
			defer server.Close()
			gateway := gatewayForServer(t, server, gatewayTestScope(t, "user-1", 7), &gatewayTestExecutor{token: "access-secret"})
			if _, err := gateway.ListProjects(context.Background(), ListProjectsInput{}); err == nil {
				t.Fatal("first request unexpectedly succeeded")
			} else if strings.Contains(strings.ToLower(err.Error()), "contract is incompatible") {
				t.Fatalf("request error was reported as contract incompatibility: %v", err)
			}
			page, err := gateway.ListProjects(context.Background(), ListProjectsInput{})
			if err != nil || len(page.Items) != 0 {
				t.Fatalf("request after failure = %#v, %v", page, err)
			}
		})
	}
}

func TestProjectGatewayCreatePreservesExplicitNullAndNameWhitespace(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeGatewayBody(t, r)
		if body["name"] != " Beta " {
			t.Fatalf("name = %#v", body["name"])
		}
		if value, present := body["description"]; !present || value != nil {
			t.Fatalf("explicit-null description = %#v, present=%t", value, present)
		}
		fmt.Fprint(w, `{"data":{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":1},"error":null,"request_id":"req-create","server_time":"2026-08-05T10:00:01Z"}`)
	}))
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	if _, err := gateway.CreateProject(context.Background(), CreateProjectInput{
		Name: " Beta ", Description: NullableStringUpdate{Set: true, Value: nil},
	}, "key-create"); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
}

func TestProjectGatewayPostMessageEncodesMissingMentionsAsEmptyArray(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/project-2/messages" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Idempotency-Key") != "key-message" {
			t.Fatalf("idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
		}
		body := decodeGatewayBody(t, r)
		mentions, ok := body["mention_user_ids"].([]any)
		if !ok || len(mentions) != 0 {
			t.Fatalf("mention_user_ids = %#v, want empty JSON array", body["mention_user_ids"])
		}
		assets, ok := body["asset_ids"].([]any)
		if !ok || len(assets) != 1 || assets[0] != "asset-1" {
			t.Fatalf("asset_ids = %#v, want one selected project asset", body["asset_ids"])
		}
		fmt.Fprint(w, `{"data":{"operation_id":"op-message","resource_kind":"activity","resource_id":"activity-1","project_id":"project-2","committed_version":1},"error":null,"request_id":"req-message","server_time":"2026-08-05T10:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	result, err := gateway.PostMessage(context.Background(), "project-2", "key-message", PostMessageInput{
		ClientMessageID: "client-message-1",
		Text:            "进度同步",
		AssetIDs:        []string{"asset-1"},
	})
	if err != nil {
		t.Fatalf("PostMessage: %v", err)
	}
	if result.ResourceID != "activity-1" || result.ProjectScopeKey != scope.Key {
		t.Fatalf("PostMessage result = %#v", result)
	}
}

func TestProjectGatewayListsMessagesWithIsolatedCursor(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-2/messages" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("page_size") != "20" || r.URL.Query().Get("cursor") != "message-cursor" {
			t.Fatalf("query = %v", r.URL.Query())
		}
		if r.URL.Query().Has("filter") || r.URL.Query().Has("member_user_ids") {
			t.Fatalf("activity filters leaked into message query: %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"items":[{"activity_id":"activity-1","project_id":"project-2","kind":"user_message","schema_version":1,"actor_user_id":"user-1","author_display_name":"Owner","client_message_id":"client-message-1","message_text":"Please review","mention_user_ids":[],"assets":[],"unread":true,"other_members_unread":true,"unread_other_member_count":1,"other_member_read_states":[{"user_id":"user-2","display_name":"Reviewer","status":"unread"},{"user_id":"user-3","display_name":"Observer","status":"read"}],"created_at":"2026-08-20T12:00:00Z"}],"next_cursor":null,"read_cursor":"message-read-cursor"},"error":null,"request_id":"req-messages","server_time":"2026-08-20T12:00:00Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	page, err := gateway.ListMessages(context.Background(), "project-2", ListMessagesInput{PageSize: 20, Cursor: "message-cursor"})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Kind != "user_message" || page.Items[0].MessageText != "Please review" {
		t.Fatalf("messages = %#v", page.Items)
	}
	if page.ReadCursor == nil || *page.ReadCursor != "message-read-cursor" {
		t.Fatalf("read cursor = %#v", page.ReadCursor)
	}
	item := page.Items[0]
	if item.OtherMembersUnread == nil || !*item.OtherMembersUnread || item.UnreadOtherMemberCount == nil || *item.UnreadOtherMemberCount != 1 || item.OtherMemberReadStates == nil || len(*item.OtherMemberReadStates) != 2 || (*item.OtherMemberReadStates)[0].UserID != "user-2" || (*item.OtherMemberReadStates)[1].Status != "read" {
		t.Fatalf("message member read states = %#v", item)
	}
}

func TestProjectGatewayRejectsInconsistentMessageReadStates(t *testing.T) {
	count := int64(0)
	hasUnread := false
	states := []ProjectMessageMemberReadState{{UserID: "user-2", DisplayName: "Reviewer", Status: "unread"}}
	err := validateMessageReadStates(ProjectActivity{OtherMembersUnread: &hasUnread, UnreadOtherMemberCount: &count, OtherMemberReadStates: &states})
	if err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("validateMessageReadStates error = %v", err)
	}
}

func TestProjectGatewayMarksMessagesReadWithIsolatedCursor(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/project-2/message-read" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		body := decodeGatewayBody(t, r)
		if body["through_cursor"] != "message-read-cursor" || len(body) != 1 {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"through_activity_id":"activity-1","updated_at":"2026-08-21T12:34:56.789Z"},"error":null,"request_id":"req-message-read","server_time":"2026-08-21T12:34:56.790Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	scopeKey, result, err := gateway.MarkMessageRead(context.Background(), "project-2", "message-read-cursor")
	if err != nil {
		t.Fatalf("MarkMessageRead: %v", err)
	}
	if scopeKey != scope.Key || result.ThroughActivityID != "activity-1" || result.UpdatedAt.UTC().Format(time.RFC3339Nano) != "2026-08-21T12:34:56.789Z" {
		t.Fatalf("MarkMessageRead result = %q %#v", scopeKey, result)
	}
}

func TestProjectGatewayAggregateCreatePreservesDescriptionAndBindings(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	description := "项目公开说明"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/aggregates" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body := decodeGatewayBody(t, r)
		if body["description"] != description {
			t.Fatalf("description = %#v", body["description"])
		}
		bindings, ok := body["bindings"].([]any)
		if !ok || len(bindings) != 1 {
			t.Fatalf("bindings = %#v", body["bindings"])
		}
		binding, ok := bindings[0].(map[string]any)
		if !ok || binding["kind"] != "expert" || binding["registry_ref"] != "expert-reviewer" {
			t.Fatalf("binding = %#v", bindings[0])
		}
		fmt.Fprint(w, `{"data":{"project_id":"project-2","project_version":1,"instruction_version":1,"binding_ids":["binding-1"],"created_at":"2026-08-05T10:00:01Z"},"error":null,"request_id":"req-aggregate","server_time":"2026-08-05T10:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	result, err := gateway.CreateAggregate(context.Background(), AggregateCreateInput{
		Name:        "Research",
		Description: NullableStringUpdate{Set: true, Value: &description},
		Bindings:    []ProjectBindingInput{{Kind: "expert", RegistryRef: "expert-reviewer", Position: 0, Config: map[string]any{}}},
	}, "key-aggregate")
	if err != nil || result.ProjectID != "project-2" || result.ProjectScopeKey != scope.Key {
		t.Fatalf("CreateAggregate = %#v, %v", result, err)
	}
}

func TestProjectGatewayPatchBindingsPreservesOmittedAndNullFields(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	requestIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIndex++
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/projects/project-2/bindings" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		body := decodeGatewayBody(t, r)
		if _, present := body["connector_id"]; present {
			t.Fatalf("omitted connector_id encoded as %#v", body["connector_id"])
		}
		switch requestIndex {
		case 1:
			if len(body) != 3 || body["expected_version"] != float64(3) || body["skill_id"] != "@reasonix/market-research" || body["expert_id"] != "exp_market_analyst" {
				t.Fatalf("simultaneous patch body=%#v", body)
			}
			fmt.Fprint(w, `{"code":"OK","message":"success","data":{"project_id":"project-2","project_version":4,"items":[{"binding_id":"binding-skill","project_id":"project-2","kind":"skill","registry_ref":"@reasonix/market-research","position":0,"config":{},"public_disabled":false,"version":1,"created_at":"2026-08-20T12:00:00Z","updated_at":"2026-08-20T12:00:00Z"},{"binding_id":"binding-expert","project_id":"project-2","kind":"expert","registry_ref":"exp_market_analyst","position":0,"config":{},"public_disabled":false,"version":1,"created_at":"2026-08-20T12:00:00Z","updated_at":"2026-08-20T12:00:00Z"}]},"request_id":"req-bindings-1"}`)
		case 2:
			if len(body) != 3 || body["expected_version"] != float64(4) || body["skill_id"] != nil || body["expert_id"] != "exp_new_analyst" {
				t.Fatalf("clear-and-replace body=%#v", body)
			}
			fmt.Fprint(w, `{"code":"OK","message":"success","data":{"project_id":"project-2","project_version":5,"items":[{"binding_id":"binding-expert","project_id":"project-2","kind":"expert","registry_ref":"exp_new_analyst","position":0,"config":{},"public_disabled":false,"version":2,"created_at":"2026-08-20T12:00:00Z","updated_at":"2026-08-20T12:01:00Z"}]},"request_id":"req-bindings-2"}`)
		default:
			t.Fatalf("unexpected request %d", requestIndex)
		}
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	skillID, expertID := "@reasonix/market-research", "exp_market_analyst"
	bindings, err := gateway.PatchBindings(context.Background(), "project-2", PatchProjectBindingsInput{
		ExpectedVersion: 3,
		SkillID:         NullableStringUpdate{Set: true, Value: &skillID},
		ExpertID:        NullableStringUpdate{Set: true, Value: &expertID},
	}, "key-bindings")
	if err != nil || bindings.ProjectVersion != 4 || len(bindings.Items) != 2 {
		t.Fatalf("PatchBindings = %#v, %v", bindings, err)
	}
	expertID = "exp_new_analyst"
	bindings, err = gateway.PatchBindings(context.Background(), "project-2", PatchProjectBindingsInput{
		ExpectedVersion: 4,
		SkillID:         NullableStringUpdate{Set: true, Value: nil},
		ExpertID:        NullableStringUpdate{Set: true, Value: &expertID},
	}, "key-bindings-next")
	if err != nil || bindings.ProjectVersion != 5 || len(bindings.Items) != 1 || bindings.Items[0].RegistryRef != "exp_new_analyst" {
		t.Fatalf("PatchBindings clear-and-replace = %#v, %v", bindings, err)
	}
}

func TestProjectGatewayPutBindingsReplacesTheCompleteBindingList(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/project-2/bindings" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" || r.Header.Get("X-Idempotency-Key") != "key-bindings-put" {
			t.Fatalf("headers=%v", r.Header)
		}
		body := decodeGatewayBody(t, r)
		items, ok := body["bindings"].([]any)
		if body["expected_version"] != float64(4) || !ok || len(items) != 3 {
			t.Fatalf("body=%#v", body)
		}
		first, _ := items[0].(map[string]any)
		second, _ := items[1].(map[string]any)
		if first["kind"] != "skill" || first["registry_ref"] != "@reasonix/planning" || second["kind"] != "expert" || second["registry_ref"] != "exp_architect" {
			t.Fatalf("bindings=%#v", items)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"OK","message":"success","data":{"project_id":"project-2","project_version":5,"items":[{"binding_id":"binding-skill","project_id":"project-2","kind":"skill","registry_ref":"@reasonix/planning","position":0,"config":{},"public_disabled":false,"version":1},{"binding_id":"binding-expert-1","project_id":"project-2","kind":"expert","registry_ref":"exp_architect","position":0,"config":{},"public_disabled":false,"version":1},{"binding_id":"binding-expert-2","project_id":"project-2","kind":"expert","registry_ref":"exp_reviewer","position":1,"config":{},"public_disabled":false,"version":1}]},"request_id":"req-bindings-put"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	bindings, err := gateway.PutBindings(context.Background(), "project-2", 4, []ProjectBindingInput{
		{Kind: "skill", RegistryRef: "@reasonix/planning", Position: 0, Config: map[string]any{}},
		{Kind: "expert", RegistryRef: "exp_architect", Position: 0, Config: map[string]any{}},
		{Kind: "expert", RegistryRef: "exp_reviewer", Position: 1, Config: map[string]any{}},
	}, "key-bindings-put")
	if err != nil || bindings.ProjectVersion != 5 || len(bindings.Items) != 3 {
		t.Fatalf("PutBindings = %#v, %v", bindings, err)
	}
}

func TestProjectGatewayListBindingsUsesExistingBodylessRoute(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		wantVersion   uint64
		wantItemCount int
	}{
		{
			name:        "complete list without timestamps",
			response:    `{"code":"OK","message":"success","data":{"project_id":"project-2","project_version":4,"items":[{"binding_id":"bind-skill","project_id":"project-2","kind":"skill","registry_ref":"@reasonix/market-research","position":0,"config":{},"public_disabled":false,"version":1},{"binding_id":"bind-expert","project_id":"project-2","kind":"expert","registry_ref":"exp_market_analyst","position":0,"config":{},"public_disabled":false,"version":1}]},"request_id":"req-bindings"}`,
			wantVersion: 4, wantItemCount: 2,
		},
		{
			name:        "empty list",
			response:    `{"code":"OK","message":"success","data":{"project_id":"project-2","project_version":1,"items":[]},"request_id":"req-empty-bindings"}`,
			wantVersion: 1, wantItemCount: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope := gatewayTestScope(t, "user-1", 7)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-2/bindings" || r.URL.RawQuery != "" {
					t.Fatalf("request=%s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				if r.ContentLength != 0 {
					t.Fatalf("GET binding request content length = %d", r.ContentLength)
				}
				fmt.Fprint(w, test.response)
			}))
			defer server.Close()

			gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
			bindings, err := gateway.ListBindings(context.Background(), "project-2")
			if err != nil || bindings.ProjectVersion != test.wantVersion || len(bindings.Items) != test.wantItemCount {
				t.Fatalf("ListBindings = %#v, %v", bindings, err)
			}
			if len(bindings.Items) == 2 && (bindings.Items[0].Kind != "skill" || bindings.Items[1].Kind != "expert") {
				t.Fatalf("binding kinds = %#v", bindings.Items)
			}
		})
	}
}

func TestProjectGatewayStrictResponseAndScopeRace(t *testing.T) {
	t.Run("strict command result", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":1,"unexpected":true},"error":null,"request_id":"req-create","server_time":"2026-08-05T10:00:01Z"}`)
		}))
		defer server.Close()
		gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
		if _, err := gateway.CreateProject(context.Background(), CreateProjectInput{Name: "Beta"}, "key-create"); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("strict command result error = %v", err)
		}
	})

	t.Run("case-variant command field cannot overwrite", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		payload := []byte(`{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","Project_ID":"project-other","committed_version":1}`)
		if _, err := DecodeCommandResult(payload, scope, ""); err == nil {
			t.Fatal("case-variant command field was accepted")
		}
	})

	t.Run("command result safe integer boundary", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		maximum := []byte(`{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":9007199254740991}`)
		result, err := DecodeCommandResult(maximum, scope, "")
		if err != nil || result.CommittedVersion != MaxSafeJSONInteger {
			t.Fatalf("maximum safe command result = %#v, %v", result, err)
		}
		over := []byte(`{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":9007199254740992}`)
		if _, err := DecodeCommandResult(over, scope, ""); err == nil || !strings.Contains(err.Error(), "committed_version") {
			t.Fatalf("unsafe committed version error = %v", err)
		}
	})

	t.Run("late response fails after scope switch", func(t *testing.T) {
		scopeA := gatewayTestScope(t, "user-1", 7)
		scopeB := gatewayTestScope(t, "user-2", 8)
		resolver := &gatewayTestScopeResolver{scope: scopeA}
		transport := newBlockingGatewayTransport(1)
		gateway, err := NewProjectGateway(&gatewayTokenExecutor{}, transport, resolver)
		if err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() {
			_, callErr := gateway.ListProjects(context.Background(), ListProjectsInput{})
			result <- callErr
		}()
		transport.waitEntered(t)
		resolver.Set(scopeB)
		transport.release()
		if err := <-result; err == nil || !strings.Contains(err.Error(), string(cloud.ErrorProfileScopeMismatch)) {
			t.Fatalf("late response error = %v", err)
		}
	})

	t.Run("concurrent reads do not serialize scopes", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		transport := newBlockingGatewayTransport(2)
		gateway, err := NewProjectGateway(&gatewayTokenExecutor{}, transport, &gatewayTestScopeResolver{scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() {
				_, callErr := gateway.ListProjects(context.Background(), ListProjectsInput{})
				results <- callErr
			}()
		}
		transport.waitEntered(t)
		transport.release()
		for i := 0; i < 2; i++ {
			if err := <-results; err != nil {
				t.Fatalf("concurrent ListProjects: %v", err)
			}
		}
	})
}

func TestProjectGatewayListQueryAndMutationReplay(t *testing.T) {
	t.Run("expected version safe integer boundary", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			var body bytes.Buffer
			_, _ = body.ReadFrom(r.Body)
			if !strings.Contains(body.String(), `"expected_version":9007199254740990`) {
				t.Fatalf("maximum expected version body = %s", body.String())
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-patch","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":9007199254740991},"error":null,"request_id":"req-patch","server_time":"2026-08-05T10:00:01Z"}`)
		}))
		defer server.Close()
		gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
		name := "Boundary"
		result, err := gateway.PatchProject(context.Background(), "project-2", PatchProjectInput{ExpectedVersion: MaxExpectedVersion, Name: &name}, "stable-key")
		if err != nil || result.CommittedVersion != MaxSafeJSONInteger {
			t.Fatalf("maximum expected version result = %#v, %v", result, err)
		}
		if _, err := gateway.PatchProject(context.Background(), "project-2", PatchProjectInput{ExpectedVersion: MaxSafeJSONInteger, Name: &name}, "unsafe-key"); err == nil || !strings.Contains(err.Error(), "expected_version") {
			t.Fatalf("unsafe expected version error = %v", err)
		}
		if calls != 1 {
			t.Fatalf("unsafe expected version reached transport: calls=%d", calls)
		}
	})

	t.Run("list preserves server query semantics", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("page_size") != "37" || query.Get("cursor") != "opaque-cursor" || query.Get("status") != "archived" || query.Get("search") != "Bet" {
				t.Fatalf("list query = %v", query)
			}
			fmt.Fprintf(w, `{"data":%s,"error":null,"request_id":"req-list","server_time":"2026-08-05T10:00:01Z"}`, strings.Replace(validProjectPageJSON, `"status":"active"`, `"status":"archived"`, 1))
		}))
		defer server.Close()
		gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
		page, err := gateway.ListProjects(context.Background(), ListProjectsInput{PageSize: 37, Cursor: "opaque-cursor", Status: ProjectArchived, Search: "Bet"})
		if err != nil || len(page.Items) != 1 || page.Items[0].Status != ProjectArchived || page.NextCursor == nil {
			t.Fatalf("ListProjects = %#v, %v", page, err)
		}
	})

	t.Run("authenticated replay keeps mutation key and body", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		var requests [][]byte
		var keys []string
		var authorizations []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body bytes.Buffer
			if _, err := body.ReadFrom(r.Body); err != nil {
				t.Fatal(err)
			}
			requests = append(requests, bytes.Clone(body.Bytes()))
			keys = append(keys, r.Header.Get("X-Idempotency-Key"))
			authorizations = append(authorizations, r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"data":{"operation_id":"op-create","resource_kind":"project","resource_id":"project-2","project_id":"project-2","committed_version":1},"error":null,"request_id":"req-create","server_time":"2026-08-05T10:00:01Z"}`)
		}))
		defer server.Close()
		gateway := gatewayForServer(t, server, scope, &gatewayReplayExecutor{tokens: []string{"expired-token", "fresh-token"}})
		if _, err := gateway.CreateProject(context.Background(), CreateProjectInput{Name: "Beta"}, "stable-key"); err != nil {
			t.Fatal(err)
		}
		if len(requests) != 2 || !bytes.Equal(requests[0], requests[1]) {
			t.Fatalf("replay bodies = %q", requests)
		}
		if fmt.Sprint(keys) != "[stable-key stable-key]" || fmt.Sprint(authorizations) != "[Bearer expired-token Bearer fresh-token]" {
			t.Fatalf("keys=%v authorizations=%v", keys, authorizations)
		}
	})

	t.Run("command result must match requested project", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"operation_id":"op-patch","resource_kind":"project","resource_id":"project-other","project_id":"project-other","committed_version":2},"error":null,"request_id":"req-patch","server_time":"2026-08-05T10:00:01Z"}`)
		}))
		defer server.Close()
		gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
		name := "Updated"
		_, err := gateway.PatchProject(context.Background(), "project-2", PatchProjectInput{ExpectedVersion: 1, Name: &name}, "stable-key")
		if err == nil || !strings.Contains(err.Error(), "project ID mismatch") {
			t.Fatalf("mismatched command error = %v", err)
		}
	})
}

func gatewayForServer(t *testing.T, server *httptest.Server, scope Scope, executor cloud.AuthenticatedRequestExecutor) *ProjectGateway {
	t.Helper()
	transport, err := cloud.NewHTTPSessionClient(server.URL, server.Client(), cloud.DeviceIdentity{DeviceID: "device-project", ClientVersion: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewProjectGateway(executor, transport, &gatewayTestScopeResolver{scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}

func decodeGatewayBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

type gatewayTokenExecutor struct{}

func (*gatewayTokenExecutor) Do(ctx context.Context, scope cloud.BindingScope, request func(context.Context, string) error) error {
	return request(ctx, scope.CloudUserID)
}

type gatewayReplayExecutor struct {
	tokens []string
}

func (e *gatewayReplayExecutor) Do(ctx context.Context, _ cloud.BindingScope, request func(context.Context, string) error) error {
	for _, token := range e.tokens {
		if err := request(ctx, token); err != nil {
			return err
		}
	}
	return nil
}

type blockingGatewayTransport struct {
	wantEntered int
	entered     chan struct{}
	releaseCh   chan struct{}
	once        sync.Once
}

func newBlockingGatewayTransport(wantEntered int) *blockingGatewayTransport {
	return &blockingGatewayTransport{wantEntered: wantEntered, entered: make(chan struct{}, wantEntered), releaseCh: make(chan struct{})}
}

func (t *blockingGatewayTransport) DoAuthenticatedJSON(context.Context, string, string, any, string, http.Header, any) error {
	return fmt.Errorf("unexpected non-query request")
}

func (t *blockingGatewayTransport) DoAuthenticatedJSONQuery(ctx context.Context, method, path string, query url.Values, body any, token string, headers http.Header, destination any) error {
	t.entered <- struct{}{}
	select {
	case <-t.releaseCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	owner := token
	payload := json.RawMessage(strings.Replace(validProjectPageJSON, "\"owner_user_id\":\"user-1\"", "\"owner_user_id\":\""+owner+"\"", 1))
	return assignGatewayRaw(destination, payload)
}

func (t *blockingGatewayTransport) waitEntered(testingT *testing.T) {
	testingT.Helper()
	for i := 0; i < t.wantEntered; i++ {
		select {
		case <-t.entered:
		case <-testingT.Context().Done():
			testingT.Fatal("timed out waiting for concurrent gateway request")
		}
	}
}

func (t *blockingGatewayTransport) release() {
	t.once.Do(func() { close(t.releaseCh) })
}

func assignGatewayRaw(destination any, payload json.RawMessage) error {
	raw, ok := destination.(*json.RawMessage)
	if !ok {
		return fmt.Errorf("destination = %T", destination)
	}
	*raw = bytes.Clone(payload)
	return nil
}

type gatewayTestExecutor struct {
	mu     sync.Mutex
	token  string
	scopes []cloud.BindingScope
}

func (e *gatewayTestExecutor) Do(ctx context.Context, scope cloud.BindingScope, request func(context.Context, string) error) error {
	e.mu.Lock()
	e.scopes = append(e.scopes, scope)
	e.mu.Unlock()
	return request(ctx, e.token)
}

func (e *gatewayTestExecutor) Scopes() []cloud.BindingScope {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]cloud.BindingScope(nil), e.scopes...)
}

type gatewayTestScopeResolver struct {
	mu    sync.RWMutex
	scope Scope
}

func (r *gatewayTestScopeResolver) CurrentProjectScope(context.Context) (Scope, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scope, nil
}

func (r *gatewayTestScopeResolver) Set(scope Scope) {
	r.mu.Lock()
	r.scope = scope
	r.mu.Unlock()
}

func gatewayTestScope(t *testing.T, userID string, generation uint64) Scope {
	t.Helper()
	scope, err := NewScope(
		cloud.EnvironmentIdentity{EnvironmentID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CloudBaseURL: "https://cloud.example.com/"},
		cloud.BindingScope{ProfileID: "profile-a", CloudUserID: userID, BindingGeneration: generation},
	)
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	return scope
}
