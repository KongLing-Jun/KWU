package project

import (
	"strconv"
	"strings"
	"testing"

	"kwu/internal/cloud"
)

const validProjectJSON = "{\"project_id\":\"project-1\",\"owner_user_id\":\"user-1\",\"name\":\"Alpha\",\"description\":null,\"status\":\"active\",\"version\":3,\"created_at\":\"2026-08-05T09:00:00Z\",\"updated_at\":\"2026-08-05T10:00:00.123456789Z\"}"
const validProjectPageJSON = "{\"items\":[" + validProjectJSON + "],\"next_cursor\":\"eyJwYWdlIjoyfQ\"}"

func TestProjectWireValidation(t *testing.T) {
	scope, err := NewScope(
		cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.com/"},
		cloud.BindingScope{ProfileID: "profile-a", CloudUserID: "user-1", BindingGeneration: 7},
	)
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	if scope.Key != "bbc3ef8499ee609b470d1874e8ea6a792d8d11e27f5cbda1480d8a593162dc83" {
		t.Fatalf("scope key = %q", scope.Key)
	}

	t.Run("valid page becomes a scope stamped view", func(t *testing.T) {
		page, err := DecodeProjectPage([]byte(validProjectPageJSON), scope)
		if err != nil {
			t.Fatalf("DecodeProjectPage: %v", err)
		}
		view := page.View()
		if view.ProjectScopeKey != scope.Key || len(view.Items) != 1 {
			t.Fatalf("view = %#v", view)
		}
		project := view.Items[0]
		if project.ProjectID != "project-1" || project.Name != "Alpha" || project.Description != nil || project.Status != "active" || project.Version != 3 {
			t.Fatalf("project view = %#v", project)
		}
		if view.NextCursor == nil || *view.NextCursor != "eyJwYWdlIjoyfQ" {
			t.Fatalf("next cursor = %#v", view.NextCursor)
		}
	})

	t.Run("safe integer version boundary", func(t *testing.T) {
		maximum := strings.Replace(validProjectPageJSON, `"version":3`, `"version":`+strconv.FormatUint(MaxSafeJSONInteger, 10), 1)
		page, err := DecodeProjectPage([]byte(maximum), scope)
		if err != nil || page.Items[0].Version != MaxSafeJSONInteger {
			t.Fatalf("maximum safe version = %#v, %v", page, err)
		}
		over := strings.Replace(validProjectPageJSON, `"version":3`, `"version":9007199254740992`, 1)
		if _, err := DecodeProjectPage([]byte(over), scope); err == nil || !strings.Contains(err.Error(), "version") {
			t.Fatalf("unsafe version error = %v", err)
		}
	})

	t.Run("joined project with a different owner remains in the authenticated scope", func(t *testing.T) {
		payload := strings.Replace(validProjectPageJSON, "\"owner_user_id\":\"user-1\"", "\"owner_user_id\":\"user-2\"", 1)
		page, err := DecodeProjectPage([]byte(payload), scope)
		if err != nil || len(page.Items) != 1 || page.Items[0].OwnerUserID != "user-2" || page.View().ProjectScopeKey != scope.Key {
			t.Fatalf("joined project page = %#v, err=%v", page, err)
		}
	})

	t.Run("scope identity cannot be forged", func(t *testing.T) {
		forged := scope
		forged.Key = strings.Repeat("0", 64)
		if _, err := DecodeProjectPage([]byte(validProjectPageJSON), forged); err == nil || !strings.Contains(err.Error(), "scope") {
			t.Fatalf("forged scope error = %v", err)
		}
	})

	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "unknown page field", payload: strings.Replace(validProjectPageJSON, "{\"items\":", "{\"unexpected\":true,\"items\":", 1), want: "unknown field"},
		{name: "unknown project field", payload: strings.Replace(validProjectPageJSON, "{\"project_id\":", "{\"unexpected\":true,\"project_id\":", 1), want: "unknown field"},
		{name: "missing items", payload: strings.Replace(validProjectPageJSON, "\"items\":["+validProjectJSON+"],", "", 1), want: "items"},
		{name: "null items", payload: strings.Replace(validProjectPageJSON, "["+validProjectJSON+"]", "null", 1), want: "items"},
		{name: "missing next cursor", payload: strings.Replace(validProjectPageJSON, ",\"next_cursor\":\"eyJwYWdlIjoyfQ\"", "", 1), want: "next_cursor"},
		{name: "missing project description", payload: strings.Replace(validProjectPageJSON, "\"description\":null,", "", 1), want: "description"},
		{name: "trailing JSON", payload: validProjectPageJSON + "{}", want: "multiple JSON"},
		{name: "duplicate page key", payload: "{\"items\":[],\"items\":[],\"next_cursor\":null}", want: "duplicate"},
		{name: "duplicate project key", payload: strings.Replace(validProjectPageJSON, "\"project_id\":\"project-1\"", "\"project_id\":\"project-1\",\"project_id\":\"project-2\"", 1), want: "duplicate"},
		{name: "empty project ID", payload: strings.Replace(validProjectPageJSON, "\"project_id\":\"project-1\"", "\"project_id\":\"\"", 1), want: "project_id"},
		{name: "padded project ID", payload: strings.Replace(validProjectPageJSON, "\"project_id\":\"project-1\"", "\"project_id\":\" project-1\"", 1), want: "project_id"},
		{name: "duplicate project ID", payload: "{\"items\":[" + validProjectJSON + "," + validProjectJSON + "],\"next_cursor\":null}", want: "duplicate project_id"},
		{name: "zero version", payload: strings.Replace(validProjectPageJSON, "\"version\":3", "\"version\":0", 1), want: "version"},
		{name: "invalid status", payload: strings.Replace(validProjectPageJSON, "\"status\":\"active\"", "\"status\":\"deleted\"", 1), want: "status"},
		{name: "empty name", payload: strings.Replace(validProjectPageJSON, "\"name\":\"Alpha\"", "\"name\":\"\"", 1), want: "name"},
		{name: "zero timestamp", payload: strings.Replace(validProjectPageJSON, "2026-08-05T09:00:00Z", "0001-01-01T00:00:00Z", 1), want: "timestamp"},
		{name: "non UTC timestamp", payload: strings.Replace(validProjectPageJSON, "2026-08-05T09:00:00Z", "2026-08-05T17:00:00+08:00", 1), want: "UTC"},
		{name: "updated before created", payload: strings.Replace(validProjectPageJSON, "2026-08-05T10:00:00.123456789Z", "2026-08-05T08:59:59Z", 1), want: "chronology"},
		{name: "empty cursor", payload: strings.Replace(validProjectPageJSON, "\"eyJwYWdlIjoyfQ\"", "\"\"", 1), want: "cursor"},
		{name: "padded cursor", payload: strings.Replace(validProjectPageJSON, "\"eyJwYWdlIjoyfQ\"", "\" eyJwYWdlIjoyfQ\"", 1), want: "cursor"},
		{name: "oversized cursor", payload: strings.Replace(validProjectPageJSON, "eyJwYWdlIjoyfQ", strings.Repeat("a", 2049), 1), want: "cursor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeProjectPage([]byte(test.payload), scope); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeProjectPage error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestProjectWireExactFieldsAndTextRules(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	withWhitespace := strings.Replace(validProjectPageJSON, `"name":"Alpha"`, `"name":" Alpha "`, 1)
	page, err := DecodeProjectPage([]byte(withWhitespace), scope)
	if err != nil || page.Items[0].Name != " Alpha " {
		t.Fatalf("preserved whitespace page = %#v, %v", page, err)
	}
	for name, payload := range map[string][]byte{
		"case alias":   []byte(strings.Replace(validProjectPageJSON, `"name":"Alpha"`, `"name":"Alpha","Name":"overwritten"`, 1)),
		"blank name":   []byte(strings.Replace(validProjectPageJSON, `"name":"Alpha"`, `"name":"   "`, 1)),
		"nul text":     []byte(strings.Replace(validProjectPageJSON, `"name":"Alpha"`, `"name":"A\u0000B"`, 1)),
		"invalid utf8": append([]byte(nil), []byte(validProjectPageJSON)...),
	} {
		if name == "invalid utf8" {
			payload[len(payload)-2] = 0xff
		}
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeProjectPage(payload, scope); err == nil {
				t.Fatal("invalid Project wire payload was accepted")
			}
		})
	}
}

func TestProjectScopeKey(t *testing.T) {
	environment := cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.com/"}
	binding := cloud.BindingScope{ProfileID: "profile-a", CloudUserID: "user-1", BindingGeneration: 7}
	base, err := NewScope(environment, binding)
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	if len(base.Key) != 64 || base.Key != strings.ToLower(base.Key) || strings.Contains(base.Key, binding.CloudUserID) {
		t.Fatalf("scope key is not an opaque lowercase SHA-256: %q", base.Key)
	}
	changes := []struct {
		name        string
		environment cloud.EnvironmentIdentity
		binding     cloud.BindingScope
	}{
		{name: "environment ID", environment: cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("b", 64), CloudBaseURL: environment.CloudBaseURL}, binding: binding},
		{name: "cloud base URL", environment: cloud.EnvironmentIdentity{EnvironmentID: environment.EnvironmentID, CloudBaseURL: "https://other.example.com/"}, binding: binding},
		{name: "profile ID", environment: environment, binding: cloud.BindingScope{ProfileID: "profile-b", CloudUserID: binding.CloudUserID, BindingGeneration: binding.BindingGeneration}},
		{name: "cloud user ID", environment: environment, binding: cloud.BindingScope{ProfileID: binding.ProfileID, CloudUserID: "user-2", BindingGeneration: binding.BindingGeneration}},
		{name: "binding generation", environment: environment, binding: cloud.BindingScope{ProfileID: binding.ProfileID, CloudUserID: binding.CloudUserID, BindingGeneration: 8}},
	}
	for _, change := range changes {
		t.Run(change.name, func(t *testing.T) {
			changed, err := NewScope(change.environment, change.binding)
			if err != nil {
				t.Fatalf("NewScope: %v", err)
			}
			if changed.Key == base.Key {
				t.Fatalf("scope identity change retained key %q", base.Key)
			}
		})
	}
}
