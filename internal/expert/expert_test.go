package expert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinCatalog(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog() error = %v", err)
	}
	list := c.List()
	if len(list) < 11 {
		t.Fatalf("len(List()) = %d, want at least legacy and directory builtins", len(list))
	}
	if _, err := c.Resolve("architecture-review"); err != nil {
		t.Fatalf("Resolve architecture-review: %v", err)
	}
	for _, id := range []string{
		"accounting-expert",
		"audit-expert",
		"tax-compliance-expert",
		"cost-control-expert",
		"business-analysis-advisor",
		"quarterly-business-decision-team",
	} {
		if _, err := c.Resolve(id); err != nil {
			t.Fatalf("Resolve %s: %v", id, err)
		}
	}
	team, err := c.Resolve("quarterly-business-decision-team")
	if err != nil {
		t.Fatalf("Resolve quarterly-business-decision-team: %v", err)
	}
	if !strings.Contains(team.Prompt, "Expert team runtime") || !strings.Contains(team.Prompt, "会计核算专家") {
		t.Fatalf("team prompt should include real orchestration and members, got %q", team.Prompt)
	}
}

func TestBuiltinCatalogIncludesExpertDirectoryTeams(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog() error = %v", err)
	}
	for _, id := range []string{
		"architect",
		"code-review",
		"security-review",
		"automation-flow",
		"data-report",
		"doc-report",
		"research",
		"ops-growth",
		"code-delivery-team",
		"office-automation-team",
		"data-analysis-team",
		"customer-ops-team",
		"project-delivery-team",
	} {
		if _, err := c.Resolve(id); err != nil {
			t.Fatalf("Resolve %s: %v", id, err)
		}
	}
	team, err := c.Resolve("code-delivery-team")
	if err != nil {
		t.Fatalf("Resolve code-delivery-team: %v", err)
	}
	if !strings.Contains(team.Prompt, "delegate concrete work to configured members with task") ||
		!strings.Contains(team.Prompt, "Independent tasks may run concurrently") ||
		!strings.Contains(team.Prompt, "dependent tasks must wait for their prerequisites") ||
		!strings.Contains(team.Prompt, "At most 4 member tasks") ||
		!strings.Contains(team.Prompt, "wait_task") ||
		!strings.Contains(team.Prompt, "stop_task") ||
		!strings.Contains(team.Prompt, "complete_step") {
		t.Fatalf("code-delivery-team prompt missing orchestration rules:\n%s", team.Prompt)
	}
	for _, forbidden := range []string{"read_only_task", "parallel_tasks", "A2A SendMessage", "A2A Task artifacts"} {
		if strings.Contains(team.Prompt, forbidden) {
			t.Fatalf("code-delivery-team prompt contains internal or unsupported rule %q:\n%s", forbidden, team.Prompt)
		}
	}
}

func TestBuiltinDefinitionsExposeOfficialSourceAndPresetQuestions(t *testing.T) {
	defs := BuiltinDefinitions()
	for _, def := range defs {
		if def.Source != SourceOfficial {
			t.Fatalf("builtin %q source = %q, want official", def.ID, def.Source)
		}
		if len(def.Prompts) < 3 || len(def.Prompts) > 5 {
			t.Fatalf("builtin %q prompts = %v, want 3..5 preset questions", def.ID, def.Prompts)
		}
	}
	var quarterly Definition
	found := false
	for _, def := range defs {
		if def.ID == "quarterly-business-decision-team" {
			quarterly = def
			found = true
			break
		}
	}
	if !found {
		t.Fatal("quarterly business decision team is missing")
	}
	if quarterly.Mode != ModeTeam {
		t.Fatalf("quarterly business decision mode = %q, want team", quarterly.Mode)
	}
	if strings.TrimSpace(quarterly.SystemPrompt) == "" {
		t.Fatal("quarterly business decision team lost its system instructions")
	}
	if len(quarterly.Members) < 4 {
		t.Fatalf("quarterly business decision members = %+v, want a real expert team", quarterly.Members)
	}
	for i, member := range quarterly.Members {
		wantKind := MemberKindSub
		if i == 0 {
			wantKind = MemberKindLead
		}
		if member.Kind != wantKind {
			t.Fatalf("quarterly member %d kind = %q, want %q", i, member.Kind, wantKind)
		}
		if strings.TrimSpace(member.Name) == "" || strings.TrimSpace(member.Role) == "" ||
			strings.TrimSpace(member.Task) == "" || strings.TrimSpace(member.Output) == "" {
			t.Fatalf("quarterly member %d is incomplete: %+v", i, member)
		}
	}
}

func TestCatalogTeamArtifactDeliveryContract(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog() error = %v", err)
	}
	team, err := c.Resolve("code-delivery-team")
	if err != nil {
		t.Fatalf("Resolve code-delivery-team: %v", err)
	}
	prompt := strings.ToLower(team.Prompt)
	for _, want := range []string{
		"for file deliverables, delegate creation",
		"finish all member work",
		"actual workspace file path",
		"final response",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("team artifact contract missing %q:\n%s", want, team.Prompt)
		}
	}
	if strings.Contains(prompt, "verify_artifact") || strings.Contains(prompt, "final workspace operation") {
		t.Fatalf("team artifact contract retains host verifier guidance:\n%s", team.Prompt)
	}
}

func TestCatalogRejectsDuplicateID(t *testing.T) {
	_, err := NewCatalog([]Expert{
		{ID: "testing", Name: "A", Prompt: "A", Enabled: true},
		{ID: "testing", Name: "B", Prompt: "B", Enabled: true},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate", err)
	}
}

func TestUserExpertStoreRoundTripAndCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	record, err := NewUserExpert("custom-local-reviewer", UserExpertInput{
		Name:          "Local Reviewer",
		Mode:          "expert",
		Description:   "Reviews local changes.",
		Category:      "engineering",
		Tags:          []string{"review", "review", "code"},
		SystemPrompt:  "Review the current code with local context.",
		DisplayPrompt: "Review this change.",
		Language:      "auto",
		Enabled:       true,
	}, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if err := SaveUserExperts(path, []UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts: %v", err)
	}
	if _, err := MarkExpertUsed(path, record.ID); err != nil {
		t.Fatalf("MarkExpertUsed: %v", err)
	}
	if _, err := SetExpertFavorite(path, "testing", true); err != nil {
		t.Fatalf("SetExpertFavorite: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID || len(records[0].Tags) != 2 {
		t.Fatalf("records = %+v, want one normalized user expert", records)
	}
	if len(records[0].Prompts) != 3 || records[0].Prompts[0] != "Review this change." {
		t.Fatalf("record prompts = %v, want migrated three-question list", records[0].Prompts)
	}
	if records[0].Info().Source != SourceUser {
		t.Fatalf("record source = %q, want user", records[0].Info().Source)
	}
	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	ex, err := catalog.Resolve(record.ID)
	if err != nil {
		t.Fatalf("Resolve user expert: %v", err)
	}
	if ex.Prompt != record.SystemPrompt {
		t.Fatalf("catalog prompt = %q, want %q", ex.Prompt, record.SystemPrompt)
	}
	if ex.Source != SourceUser || ex.Mode != ModeExpert || len(ex.Prompts) != 3 {
		t.Fatalf("catalog user expert metadata = %+v, want user/expert with three prompts", ex)
	}
	state, err := LoadExpertLibraryState(path)
	if err != nil {
		t.Fatalf("LoadExpertLibraryState: %v", err)
	}
	if len(state.RecentIDs) != 1 || state.RecentIDs[0] != record.ID {
		t.Fatalf("recent state = %+v, want created expert", state)
	}
	if len(state.FavoriteIDs) != 1 || state.FavoriteIDs[0] != "testing" {
		t.Fatalf("favorite state = %+v, want testing", state)
	}
	if state.UsageCounts[record.ID] != 1 {
		t.Fatalf("usage counts = %v, want one use for %q", state.UsageCounts, record.ID)
	}
	state, err = MarkExpertUsed(path, record.ID)
	if err != nil {
		t.Fatalf("MarkExpertUsed again: %v", err)
	}
	if state.UsageCounts[record.ID] != 2 || len(state.RecentIDs) != 1 {
		t.Fatalf("state after repeated use = %+v, want count 2 and one recent entry", state)
	}
	if err := SaveUserExperts(path, []UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts preserve state: %v", err)
	}
	state, err = LoadExpertLibraryState(path)
	if err != nil {
		t.Fatalf("LoadExpertLibraryState after save: %v", err)
	}
	if len(state.RecentIDs) != 1 || state.RecentIDs[0] != record.ID || len(state.FavoriteIDs) != 1 || state.FavoriteIDs[0] != "testing" || state.UsageCounts[record.ID] != 2 {
		t.Fatalf("state after SaveUserExperts = %+v, want preserved", state)
	}
}

func TestUserExpertStoreMigratesV1DisplayPromptToPrompts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	payload := `{
  "version": 1,
  "experts": [{
    "id": "custom-legacy-team",
    "name": "Legacy Team",
    "mode": "team",
    "description": "Legacy team profile.",
    "systemPrompt": "Coordinate the team.",
    "displayPrompt": "Keep this first question.",
    "enabled": true,
    "createdAt": 1,
    "updatedAt": 1
  }]
}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write v1 user expert store: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts(v1): %v", err)
	}
	if len(records) != 1 || len(records[0].Prompts) != 3 {
		t.Fatalf("migrated records = %+v, want one expert with three prompts", records)
	}
	if records[0].Prompts[0] != "Keep this first question." || records[0].DisplayPrompt != records[0].Prompts[0] {
		t.Fatalf("migrated prompts = %v display=%q, want preserved first question", records[0].Prompts, records[0].DisplayPrompt)
	}
	if len(records[0].Members) != 1 || records[0].Members[0].Kind != MemberKindLead ||
		strings.TrimSpace(records[0].Members[0].Name) == "" ||
		strings.TrimSpace(records[0].Members[0].Role) == "" ||
		strings.TrimSpace(records[0].Members[0].Task) == "" ||
		strings.TrimSpace(records[0].Members[0].Output) == "" {
		t.Fatalf("migrated members = %+v, want one complete lead", records[0].Members)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated user expert store: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"version": 2`) || !strings.Contains(text, `"prompts": [`) || !strings.Contains(text, `"members": [`) {
		t.Fatalf("migrated user expert store was not persisted:\n%s", text)
	}
}

func TestUserExpertStoreV1CompletesPartialTeamMember(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	payload := `{
  "version": 1,
  "experts": [{
    "id": "custom-partial-team",
    "name": "Partial Team",
    "mode": "team",
    "systemPrompt": "Coordinate the team.",
    "displayPrompt": "Start the task.",
    "members": [{"kind":"sub","name":"Existing Member"}],
    "enabled": true,
    "createdAt": 1,
    "updatedAt": 1
  }]
}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write partial v1 user expert store: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts(partial v1): %v", err)
	}
	if len(records) != 1 || len(records[0].Members) != 1 {
		t.Fatalf("migrated records = %+v, want one team member", records)
	}
	member := records[0].Members[0]
	if member.Name != "Existing Member" || member.Kind != MemberKindLead ||
		member.Role == "" || member.Task == "" || member.Output == "" {
		t.Fatalf("migrated partial member = %+v, want preserved name and completed contract", member)
	}
}

func TestUserExpertStoreV1TruncatesOversizedPrompts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	payload := `{
  "version": 1,
  "experts": [{
    "id": "custom-legacy-expert",
    "name": "Legacy Expert",
    "mode": "expert",
    "description": "Legacy expert profile.",
    "systemPrompt": "Handle the task.",
    "prompts": ["one", "two", "three", "four", "five", "six"],
    "enabled": true,
    "createdAt": 1,
    "updatedAt": 1
  }]
}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write oversized v1 user expert store: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts(oversized v1): %v", err)
	}
	if len(records) != 1 || len(records[0].Prompts) != 5 || records[0].Prompts[4] != "five" {
		t.Fatalf("migrated oversized prompts = %+v, want first five", records)
	}
}

func TestCompleteLegacyPromptsHandlesDuplicateDefaultCandidate(t *testing.T) {
	const name = "Legacy Expert"
	defaultSecond := migratedUserPrompts("expert", name, "")[1]
	prompts := completeLegacyPrompts("expert", name, []string{defaultSecond})
	if len(prompts) != 3 {
		t.Fatalf("completeLegacyPrompts() = %v, want exactly three prompts", prompts)
	}
	if prompts[0] != defaultSecond {
		t.Fatalf("first prompt = %q, want preserved %q", prompts[0], defaultSecond)
	}
	for i, prompt := range prompts {
		if strings.TrimSpace(prompt) == "" {
			t.Fatalf("prompt %d is blank: %v", i, prompts)
		}
		for j := 0; j < i; j++ {
			if prompts[j] == prompt {
				t.Fatalf("prompts contain duplicate %q: %v", prompt, prompts)
			}
		}
	}
}

func TestUserExpertPromptCountValidation(t *testing.T) {
	base := UserExpertInput{
		Name:         "Prompt Validator",
		Mode:         "expert",
		Description:  "Validates preset questions.",
		SystemPrompt: "Keep system instructions separate.",
		Enabled:      true,
	}
	for _, prompts := range [][]string{
		{"one"},
		{"one", "two"},
		{"one", "two", "three", "four", "five", "six"},
	} {
		input := base
		input.Prompts = prompts
		if _, err := NewUserExpert("custom-prompt-validator", input, time.UnixMilli(1000)); err == nil || !strings.Contains(err.Error(), "between 3 and 5 prompts") {
			t.Fatalf("NewUserExpert prompts=%v error=%v, want count validation", prompts, err)
		}
	}
	for _, count := range []int{3, 5} {
		input := base
		for i := 0; i < count; i++ {
			input.Prompts = append(input.Prompts, "question "+string(rune('A'+i)))
		}
		if _, err := NewUserExpert("custom-prompt-validator", input, time.UnixMilli(1000)); err != nil {
			t.Fatalf("NewUserExpert with %d prompts: %v", count, err)
		}
	}
	validInput := base
	validInput.Prompts = []string{"one", "two", "three"}
	existing, err := NewUserExpert("custom-prompt-validator", validInput, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert(valid): %v", err)
	}
	invalidUpdate := base
	invalidUpdate.Prompts = []string{"one", "two"}
	if _, err := UpdateUserExpert(existing, invalidUpdate, time.UnixMilli(2000)); err == nil || !strings.Contains(err.Error(), "between 3 and 5 prompts") {
		t.Fatalf("UpdateUserExpert invalid prompts error = %v", err)
	}
	if _, err := ImportUserExpertConfig(`{
	  "name": "Bad Import",
	  "systemPrompt": "System instructions.",
	  "prompts": ["only one"]
	}`); err == nil || !strings.Contains(err.Error(), "between 3 and 5 prompts") {
		t.Fatalf("ImportUserExpertConfig invalid prompts error = %v", err)
	}
}

func TestUserExpertStorePreservesDisabledUserExpert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	input, err := ImportUserExpertConfig(`{
  "name": "Paused Reviewer",
  "mode": "expert",
  "description": "Temporarily disabled.",
  "category": "engineering",
  "systemPrompt": "Review only when enabled.",
  "displayPrompt": "Review this only when enabled.",
  "language": "auto",
  "enabled": false
}`)
	if err != nil {
		t.Fatalf("ImportUserExpertConfig: %v", err)
	}
	record, err := NewUserExpert("custom-paused-reviewer", input, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if record.Enabled {
		t.Fatalf("record.Enabled = true, want imported disabled state preserved")
	}
	if err := SaveUserExperts(path, []UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts: %v", err)
	}
	if len(records) != 1 || records[0].Enabled {
		t.Fatalf("records = %+v, want one disabled user expert", records)
	}
	if records[0].Info().Enabled {
		t.Fatalf("Info().Enabled = true, want disabled")
	}
	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if _, err := catalog.Resolve(record.ID); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Resolve disabled user expert err = %v, want disabled", err)
	}
	for _, listed := range catalog.List() {
		if listed.ID == record.ID {
			t.Fatalf("List included disabled user expert %q", record.ID)
		}
	}
}

func TestImportUserExpertConfigDefaultsEnabled(t *testing.T) {
	input, err := ImportUserExpertConfig(`{
  "name": "Default Enabled",
  "systemPrompt": "Act as this expert."
}`)
	if err != nil {
		t.Fatalf("ImportUserExpertConfig: %v", err)
	}
	record, err := NewUserExpert("custom-default-enabled", input, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if !record.Enabled {
		t.Fatalf("record.Enabled = false, want omitted enabled field to default true")
	}
}

func TestUserExpertStoreRoundTripsTeamMembersAndAvatar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	record, err := NewUserExpert("custom-avatar-team", UserExpertInput{
		Name:          "Avatar Team",
		Mode:          "team",
		Description:   "Coordinates custom agents.",
		Category:      "engineering",
		Tags:          []string{"agents"},
		AvatarKey:     "data:image/png;base64,team",
		SystemPrompt:  "Coordinate this as a custom expert team.",
		DisplayPrompt: "Coordinate this task.",
		Members: []Member{
			{Kind: "sub", Name: "Lead Agent", Role: "Coordinator", Task: "Plan the work.", Output: "Plan", Tone: "blue", AvatarKey: "architect"},
			{Kind: "lead", Name: "Worker Agent", Role: "Execution", Task: "Do the work.", Output: "Result", Tone: "green", AvatarKey: "data:image/png;base64,worker"},
		},
		Enabled: true,
	}, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if err := SaveUserExperts(path, []UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts: %v", err)
	}

	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %+v, want one custom expert team", records)
	}
	if records[0].AvatarKey != "data:image/png;base64,team" {
		t.Fatalf("avatar key = %q, want uploaded avatar data URL", records[0].AvatarKey)
	}
	if len(records[0].Members) != 2 || records[0].Members[0].Kind != MemberKindLead || records[0].Members[1].Kind != MemberKindSub || records[0].Members[0].AvatarKey != "architect" || records[0].Members[1].AvatarKey != "data:image/png;base64,worker" {
		t.Fatalf("members = %+v, want preserved member avatars", records[0].Members)
	}
	info := records[0].Info()
	if info.AvatarKey != records[0].AvatarKey || len(info.Members) != 2 || info.Members[1].AvatarKey != "data:image/png;base64,worker" {
		t.Fatalf("Info() = %+v, want avatar and members", info)
	}
}

func TestResetUserExpertStoreClearsExpertsAndLibraryState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "experts.json")
	record, err := NewUserExpert("custom-local-reviewer", UserExpertInput{
		Name:         "Local Reviewer",
		Mode:         "expert",
		Description:  "Reviews local changes.",
		Category:     "engineering",
		SystemPrompt: "Review the current code with local context.",
		Enabled:      true,
	}, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if err := SaveUserExperts(path, []UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts: %v", err)
	}
	if _, err := MarkExpertUsed(path, record.ID); err != nil {
		t.Fatalf("MarkExpertUsed: %v", err)
	}
	if _, err := SetExpertFavorite(path, "testing", true); err != nil {
		t.Fatalf("SetExpertFavorite: %v", err)
	}

	if err := ResetUserExpertStore(path); err != nil {
		t.Fatalf("ResetUserExpertStore: %v", err)
	}
	records, err := LoadUserExperts(path)
	if err != nil {
		t.Fatalf("LoadUserExperts: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records after reset = %+v, want none", records)
	}
	state, err := LoadExpertLibraryState(path)
	if err != nil {
		t.Fatalf("LoadExpertLibraryState: %v", err)
	}
	if len(state.FavoriteIDs) != 0 || len(state.RecentIDs) != 0 {
		t.Fatalf("library state after reset = %+v, want empty", state)
	}
}

func TestImportUserExpertConfigSchema(t *testing.T) {
	input, err := ImportUserExpertConfig(`{
  "name": "Imported Expert",
  "mode": "team",
  "description": "Imported schema.",
  "category": "custom",
  "tags": ["imported"],
  "avatarKey": "data:image/png;base64,team",
  "members": [{"kind":"sub","name":"Lead Agent","role":"Coordinator","task":"Plan","output":"Plan","tone":"blue","avatarKey":"architect"}],
  "systemPrompt": "Act as the imported expert.",
  "displayPrompt": "Handle this as imported.",
  "language": "auto",
  "enabled": true
}`)
	if err != nil {
		t.Fatalf("ImportUserExpertConfig: %v", err)
	}
	if input.Name != "Imported Expert" || input.Mode != "team" || input.AvatarKey == "" || len(input.Members) != 1 || input.Members[0].Kind != MemberKindLead || input.Members[0].AvatarKey != "architect" || input.SystemPrompt == "" {
		t.Fatalf("input = %+v, want imported fields", input)
	}
	if len(input.Prompts) != 3 || input.Prompts[0] != input.DisplayPrompt || input.DisplayPrompt != "Handle this as imported." {
		t.Fatalf("imported prompts = %v display=%q, want normalized v1-compatible questions", input.Prompts, input.DisplayPrompt)
	}
	if _, err := ImportUserExpertConfig(`{"name":"Bad","systemPrompt":"x","tools":[]}`); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field err = %v, want rejected schema", err)
	}
}

func TestImportUserExpertConfigRejectsInvalidTeamMembers(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "missing name",
			payload: `{"name":"Bad Member","mode":"team","systemPrompt":"Coordinate.","members":[{"role":"Coordinator","task":"Plan","output":"Plan"}]}`,
			want:    "name is required",
		},
		{
			name:    "missing role",
			payload: `{"name":"Bad Member","mode":"team","systemPrompt":"Coordinate.","members":[{"name":"Lead","task":"Plan","output":"Plan"}]}`,
			want:    "role is required",
		},
		{
			name:    "missing task",
			payload: `{"name":"Bad Member","mode":"team","systemPrompt":"Coordinate.","members":[{"name":"Lead","role":"Coordinator","output":"Plan"}]}`,
			want:    "task is required",
		},
		{
			name:    "missing output",
			payload: `{"name":"Bad Member","mode":"team","systemPrompt":"Coordinate.","members":[{"name":"Lead","role":"Coordinator","task":"Plan"}]}`,
			want:    "output is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ImportUserExpertConfig(tt.payload); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ImportUserExpertConfig() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestUserExpertAllowsEmptyTeamMembers(t *testing.T) {
	imported, err := ImportUserExpertConfig(`{"name":"No Members","mode":"team","systemPrompt":"Coordinate.","displayPrompt":"Start."}`)
	if err != nil {
		t.Fatalf("ImportUserExpertConfig: %v", err)
	}
	if len(imported.Members) != 0 {
		t.Fatalf("imported members = %+v, want empty", imported.Members)
	}

	existing, err := NewUserExpert("custom-update-team", UserExpertInput{
		Name:         "Update Team",
		Mode:         "team",
		SystemPrompt: "Coordinate the team.",
		Members: []Member{{
			Name: "Lead", Role: "Coordinator", Task: "Plan", Output: "Plan",
		}},
	}, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	updated, err := UpdateUserExpert(existing, UserExpertInput{
		Name:         "Update Team",
		Mode:         "team",
		SystemPrompt: "Coordinate the updated team.",
	}, time.UnixMilli(2000))
	if err != nil {
		t.Fatalf("UpdateUserExpert: %v", err)
	}
	if len(updated.Members) != 0 {
		t.Fatalf("updated members = %+v, want empty", updated.Members)
	}
}

func TestValidateRejectsBadID(t *testing.T) {
	err := Expert{ID: "Bad ID", Name: "Bad", Prompt: "Prompt", Enabled: true}.Validate()
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("error = %v, want kebab-case", err)
	}
}

func TestValidateRejectsDuplicateTeamMemberIdentity(t *testing.T) {
	base := UserExpertInput{
		Name:         "Duplicate Team",
		Mode:         "team",
		SystemPrompt: "Coordinate.",
		Members: []Member{
			{Name: "Lead", Role: "Lead", Task: "Plan", Output: "Plan", ExpertID: "architecture-review"},
			{Name: "Worker", Role: "Work", Task: "Build", Output: "Result", ExpertID: "testing"},
		},
	}
	for _, tt := range []struct {
		name   string
		mutate func(*UserExpertInput)
		want   string
	}{
		{name: "name", mutate: func(input *UserExpertInput) { input.Members[1].Name = " lead " }, want: "name \"lead\" is duplicated"},
		{name: "expert id", mutate: func(input *UserExpertInput) { input.Members[1].ExpertID = "architecture-review" }, want: "expert id \"architecture-review\" is duplicated"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			input.Members = append([]Member(nil), base.Members...)
			tt.mutate(&input)
			_, err := NewUserExpert("duplicate-team", input, time.UnixMilli(1000))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewUserExpert error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveSelectionPriority(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sel, err := ResolveSelection(c, "testing", "product", "code-review")
	if err != nil {
		t.Fatal(err)
	}
	if sel.ID != "testing" || sel.Source != SourceSession {
		t.Fatalf("selection = %+v, want session testing", sel)
	}
	sel, err = ResolveSelection(c, "", "product", "code-review")
	if err != nil {
		t.Fatal(err)
	}
	if sel.ID != "product" || sel.Source != SourceRequest {
		t.Fatalf("selection = %+v, want request product", sel)
	}
}

func TestResolveSelectionExplicitNoneSkipsConfigDefault(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sel, err := ResolveSelection(c, ExplicitNoneID, "", "architecture-review")
	if err != nil {
		t.Fatal(err)
	}
	if sel.Selected || sel.Source != SourceNone {
		t.Fatalf("selection = %+v, want explicit none", sel)
	}
}

func TestApplyPrompt(t *testing.T) {
	got := ApplyPrompt("base", Expert{Name: "Testing", Prompt: "Focus on tests."})
	if !strings.Contains(got, "base\n\nExpert role: Testing\nFocus on tests.") {
		t.Fatalf("ApplyPrompt() = %q", got)
	}
}
