package automation

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveCapabilitySnapshotFreezesSelectedCapabilities(t *testing.T) {
	capturedAt := time.Date(2026, 8, 1, 10, 30, 0, 0, time.UTC)
	catalog := CapabilityCatalog{
		Models:  []CapabilityOption{{ID: "openai/gpt-5", Name: "GPT-5", Available: true}},
		Skills:  []CapabilityOption{{ID: "review", Name: "Review", Available: true}},
		Experts: []CapabilityOption{{ID: "architect", Name: "Architect", Available: true}},
		Connectors: []CapabilityOption{{
			ID: "github", Name: "GitHub", Available: true,
			ToolAllowlist: []string{"mcp__github__search", "mcp__github__create_issue"},
			ReadOnlyTools: []string{"mcp__github__search"},
		}},
	}

	snapshot, err := ResolveCapabilitySnapshot(CapabilitySelection{
		ModelRef: "openai/gpt-5", SkillIDs: []string{"review"},
		ExpertID: "architect", ConnectorIDs: []string{"github"},
	}, catalog, capturedAt)
	if err != nil {
		t.Fatalf("ResolveCapabilitySnapshot: %v", err)
	}
	if snapshot.ModelRef != "openai/gpt-5" || snapshot.ExpertID != "architect" {
		t.Fatalf("snapshot identity = %+v", snapshot)
	}
	if !reflect.DeepEqual(snapshot.SkillIDs, []string{"review"}) || !reflect.DeepEqual(snapshot.ConnectorIDs, []string{"github"}) {
		t.Fatalf("snapshot references = %+v", snapshot)
	}
	wantTools := []string{"mcp__github__search", "mcp__github__create_issue"}
	if !reflect.DeepEqual(snapshot.ToolAllowlist, wantTools) {
		t.Fatalf("ToolAllowlist = %v, want %v", snapshot.ToolAllowlist, wantTools)
	}
	if snapshot.RiskSummary != "1 of 2 connector tools may modify data" {
		t.Fatalf("RiskSummary = %q", snapshot.RiskSummary)
	}
	if !snapshot.CapturedAt.Equal(capturedAt) {
		t.Fatalf("CapturedAt = %v, want %v", snapshot.CapturedAt, capturedAt)
	}

	// The snapshot must remain immutable when the live catalog changes later.
	catalog.Connectors[0].ToolAllowlist[0] = "mcp__github__different"
	if !reflect.DeepEqual(snapshot.ToolAllowlist, wantTools) {
		t.Fatalf("snapshot changed with live catalog: %v", snapshot.ToolAllowlist)
	}
}

func TestResolveCapabilitySnapshotRejectsUnavailableReference(t *testing.T) {
	catalog := CapabilityCatalog{
		Skills: []CapabilityOption{{ID: "disabled", Name: "Disabled", Available: false, UnavailableReason: "disabled in settings"}},
	}

	_, err := ResolveCapabilitySnapshot(CapabilitySelection{SkillIDs: []string{"disabled"}}, catalog, time.Now())
	if !errors.Is(err, ErrCapabilityUnavailable) {
		t.Fatalf("error = %v, want ErrCapabilityUnavailable", err)
	}
}

func TestResolveCapabilitySnapshotRejectsUnknownConnector(t *testing.T) {
	_, err := ResolveCapabilitySnapshot(
		CapabilitySelection{ConnectorIDs: []string{"missing"}},
		CapabilityCatalog{},
		time.Now(),
	)
	if !errors.Is(err, ErrCapabilityUnavailable) {
		t.Fatalf("error = %v, want ErrCapabilityUnavailable", err)
	}
}

func TestResolveCapabilitySnapshotDeduplicatesTools(t *testing.T) {
	catalog := CapabilityCatalog{Connectors: []CapabilityOption{
		{ID: "one", Available: true, ToolAllowlist: []string{"mcp__shared__read", "mcp__one__write"}, ReadOnlyTools: []string{"mcp__shared__read"}},
		{ID: "two", Available: true, ToolAllowlist: []string{"mcp__shared__read"}, ReadOnlyTools: []string{"mcp__shared__read"}},
	}}

	snapshot, err := ResolveCapabilitySnapshot(
		CapabilitySelection{ConnectorIDs: []string{"one", "two"}},
		catalog,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("ResolveCapabilitySnapshot: %v", err)
	}
	if !reflect.DeepEqual(snapshot.ToolAllowlist, []string{"mcp__shared__read", "mcp__one__write"}) {
		t.Fatalf("ToolAllowlist = %v", snapshot.ToolAllowlist)
	}
	if snapshot.RiskSummary != "1 of 2 connector tools may modify data" {
		t.Fatalf("RiskSummary = %q", snapshot.RiskSummary)
	}
}

func TestResolveCapabilitySnapshotWithContextFreezesVerifiableMetadata(t *testing.T) {
	capturedAt := time.Date(2026, 8, 1, 10, 30, 0, 0, time.UTC)
	expiresAt := capturedAt.Add(2 * time.Hour)
	catalog := CapabilityCatalog{
		Skills: []CapabilityOption{{
			ID: "review", Name: "Review", Available: true,
			Version: "2.1.0", ContentHash: "sha256:skill-review",
		}},
		Experts: []CapabilityOption{{
			ID: "architect", Name: "Architect", Available: true,
			Version: "2026-08-01", ContentHash: "sha256:expert-architect",
		}},
		Connectors: []CapabilityOption{{
			ID: "github", Name: "GitHub", Available: true,
			ServerID: "github-mcp", ToolAllowlist: []string{"mcp__github__search"},
			ReadOnlyTools: []string{"mcp__github__search"}, GrantID: "grant-1",
			GrantPrincipal: "project:reasonix", GrantSource: "project_acl", GrantExpiresAt: expiresAt,
		}},
	}

	snapshot, err := ResolveCapabilitySnapshotWithContext(
		CapabilitySelection{SkillIDs: []string{"review"}, ExpertID: "architect", ConnectorIDs: []string{"github"}},
		catalog,
		CapabilitySnapshotContext{Prompt: "sensitive prompt", Principal: "user:elias"},
		capturedAt,
	)
	if err != nil {
		t.Fatalf("ResolveCapabilitySnapshotWithContext: %v", err)
	}
	if snapshot.PromptHash == "" || strings.Contains(snapshot.PromptHash, "sensitive") {
		t.Fatalf("PromptHash = %q, want a non-plaintext digest", snapshot.PromptHash)
	}
	if snapshot.Principal != "user:elias" {
		t.Fatalf("Principal = %q", snapshot.Principal)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].Version != "2.1.0" || snapshot.Skills[0].ContentHash != "sha256:skill-review" {
		t.Fatalf("Skills = %+v", snapshot.Skills)
	}
	if snapshot.Expert == nil || snapshot.Expert.Version != "2026-08-01" || snapshot.Expert.ContentHash != "sha256:expert-architect" {
		t.Fatalf("Expert = %+v", snapshot.Expert)
	}
	if len(snapshot.ConnectorGrants) != 1 {
		t.Fatalf("ConnectorGrants = %+v", snapshot.ConnectorGrants)
	}
	grant := snapshot.ConnectorGrants[0]
	if grant.ConnectorID != "github" || grant.ServerID != "github-mcp" || grant.GrantID != "grant-1" ||
		grant.Principal != "project:reasonix" || grant.Source != "project_acl" || !grant.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("connector grant = %+v", grant)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "sensitive prompt") {
		t.Fatalf("snapshot leaked prompt: %s", encoded)
	}

	// Mutating the live catalog after capture must not alter the snapshot.
	catalog.Skills[0].Version = "changed"
	catalog.Connectors[0].ToolAllowlist[0] = "mcp__github__write"
	if snapshot.Skills[0].Version != "2.1.0" || !reflect.DeepEqual(grant.ToolAllowlist, []string{"mcp__github__search"}) {
		t.Fatalf("snapshot changed with catalog: %+v", snapshot)
	}
}

func TestResolveCapabilitySnapshotWithContextFailsClosedOnIncompleteMetadata(t *testing.T) {
	now := time.Date(2026, 8, 1, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		selection CapabilitySelection
		catalog   CapabilityCatalog
		context   CapabilitySnapshotContext
	}{
		{name: "principal missing", context: CapabilitySnapshotContext{Prompt: "prompt"}},
		{
			name: "skill version and hash missing", selection: CapabilitySelection{SkillIDs: []string{"review"}},
			catalog: CapabilityCatalog{Skills: []CapabilityOption{{ID: "review", Available: true}}},
			context: CapabilitySnapshotContext{Prompt: "prompt", Principal: "user:elias"},
		},
		{
			name: "connector grant expired", selection: CapabilitySelection{ConnectorIDs: []string{"github"}},
			catalog: CapabilityCatalog{Connectors: []CapabilityOption{{
				ID: "github", Available: true, ServerID: "github-mcp", GrantID: "grant-1",
				GrantPrincipal: "project:reasonix", GrantSource: "project_acl", GrantExpiresAt: now,
			}}},
			context: CapabilitySnapshotContext{Prompt: "prompt", Principal: "user:elias"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveCapabilitySnapshotWithContext(tt.selection, tt.catalog, tt.context, now)
			if !errors.Is(err, ErrCapabilityUnavailable) {
				t.Fatalf("error = %v, want ErrCapabilityUnavailable", err)
			}
		})
	}
}

func TestCapabilitySnapshotLegacyJSONRemainsCompatible(t *testing.T) {
	const legacy = `{"modelRef":"openai/gpt-5","skillIds":["review"],"expertId":"architect","connectorIds":["github"],"toolAllowlist":["mcp__github__search"]}`
	var snapshot CapabilitySnapshot
	if err := json.Unmarshal([]byte(legacy), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if snapshot.ModelRef != "openai/gpt-5" || !reflect.DeepEqual(snapshot.SkillIDs, []string{"review"}) {
		t.Fatalf("legacy snapshot = %+v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"skillIds":["review"]`) {
		t.Fatalf("legacy fields were not preserved: %s", encoded)
	}
}
