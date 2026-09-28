package project

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const validWorkItemJSON = `{"work_item_id":"work-1","project_id":"project-1","title":"First task","description":null,"status":"todo","rank":1000,"version":1,"created_at":"2026-08-05T10:00:00Z","updated_at":"2026-08-05T10:00:00Z"}`

func TestWorkItemWireAndGatewayRoutes(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	page, err := DecodeWorkItemPage([]byte(`{"items":[`+validWorkItemJSON+`],"next_cursor":null,"total_count":17,"undated_count":6}`), scope, "project-1")
	if err != nil || len(page.Items) != 1 || page.Items[0].Rank != 1000 || page.Items[0].Status != WorkItemTodo || page.TotalCount != 17 || page.UndatedCount != 6 {
		t.Fatalf("DecodeWorkItemPage = %#v, %v", page, err)
	}

	requestIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIndex++
		w.Header().Set("Content-Type", "application/json")
		switch requestIndex {
		case 1:
			query := r.URL.Query()
			if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-1/work-items" || query.Get("status") != "todo" || query.Get("search") != "Release" || query.Get("assignee_user_id") != "user-2" || query.Get("priority") != "high" || query.Get("label_id") != "label-1" || query.Get("scheduled_from") != "2026-08-01" || query.Get("scheduled_to") != "2026-09-11" {
				t.Fatalf("list request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			}
			fmt.Fprintf(w, `{"data":{"items":[%s],"next_cursor":null,"total_count":17,"undated_count":6},"error":null,"request_id":"req-list","server_time":"2026-08-05T10:00:01Z"}`, validWorkItemJSON)
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/project-1/work-items" || r.Header.Get("X-Idempotency-Key") != "key-create" {
				t.Fatalf("create request = %s %s", r.Method, r.URL.Path)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-create","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":1},"error":null,"request_id":"req-create","server_time":"2026-08-05T10:00:01Z"}`)
		case 3:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/work-items/work-1/move" || r.Header.Get("X-Idempotency-Key") != "key-move" {
				t.Fatalf("move request = %s %s", r.Method, r.URL.Path)
			}
			body := decodeGatewayBody(t, r)
			if body["target_status"] != "in_progress" || body["before_work_item_id"] != "work-2" {
				t.Fatalf("move body = %#v", body)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-move","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":2},"error":null,"request_id":"req-move","server_time":"2026-08-05T10:00:02Z"}`)
		case 4:
			if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/work-items/work-1" || r.Header.Get("X-Idempotency-Key") != "key-delete" {
				t.Fatalf("delete request = %s %s", r.Method, r.URL.Path)
			}
			body := decodeGatewayBody(t, r)
			if body["expected_version"] != float64(2) {
				t.Fatalf("delete body = %#v", body)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-delete","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":3},"error":null,"request_id":"req-delete","server_time":"2026-08-05T10:00:03Z"}`)
		default:
			t.Fatalf("unexpected request %d", requestIndex)
		}
	}))
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	if _, err := gateway.ListWorkItems(context.Background(), "project-1", ListWorkItemsInput{Status: WorkItemTodo, Search: " Release ", AssigneeUserID: "user-2", Priority: "high", LabelID: "label-1", ScheduledFrom: "2026-08-01", ScheduledTo: "2026-09-11"}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.CreateWorkItem(context.Background(), "project-1", CreateWorkItemInput{Title: "First task"}, "key-create"); err != nil {
		t.Fatal(err)
	}
	before := "work-2"
	if _, err := gateway.MoveWorkItem(context.Background(), "project-1", "work-1", MoveWorkItemInput{ExpectedVersion: 1, TargetStatus: WorkItemInProgress, BeforeWorkItemID: &before}, "key-move"); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.DeleteWorkItem(context.Background(), "project-1", "work-1", DeleteWorkItemInput{ExpectedVersion: 2}, "key-delete"); err != nil {
		t.Fatal(err)
	}
}

func TestWorkItemValidationRejectsUnsafeOrAmbiguousMoves(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	if _, err := DecodeWorkItem([]byte(`{"work_item_id":"work-1","project_id":"project-1","title":"First","description":null,"status":"todo","rank":9007199254740992,"version":1,"created_at":"2026-08-05T10:00:00Z","updated_at":"2026-08-05T10:00:00Z"}`), scope, "project-1", "work-1"); err == nil {
		t.Fatal("unsafe rank accepted")
	}
	if _, err := DecodeWorkItem([]byte(`{"work_item_id":"work-1","project_id":"project-1","title":"First","description":null,"status":"todo","rank":null,"version":1,"created_at":"2026-08-05T10:00:00Z","updated_at":"2026-08-05T10:00:00Z"}`), scope, "project-1", "work-1"); err == nil {
		t.Fatal("null rank accepted")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	before, after := "work-2", "work-3"
	if _, err := gateway.MoveWorkItem(context.Background(), "project-1", "work-1", MoveWorkItemInput{ExpectedVersion: 1, TargetStatus: WorkItemTodo, BeforeWorkItemID: &before, AfterWorkItemID: &after}, "key"); err == nil {
		t.Fatal("move with two anchors accepted")
	}
}

func TestWorkItemCalendarQueryUsesContractNamesAndEncoding(t *testing.T) {
	query, err := workItemListQuery(ListWorkItemsInput{
		PageSize:       100,
		Cursor:         "cursor/page+2",
		Status:         WorkItemStatus("all"),
		Search:         " Release & QA ",
		AssigneeUserID: "user-2",
		Priority:       "urgent",
		LabelID:        "label-1",
		ScheduledFrom:  "2026-07-27",
		ScheduledTo:    "2026-09-06",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "assignee_user_id=user-2&cursor=cursor%2Fpage%2B2&label_id=label-1&page_size=100&priority=urgent&scheduled_from=2026-07-27&scheduled_to=2026-09-06&search=Release+%26+QA&status=all"
	if got := query.Encode(); got != want {
		t.Fatalf("calendar query = %q, want %q", got, want)
	}
}
