package expert_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"kwu/internal/expert"
)

const expectedFinancePrompt = "请调取本季度进销存流水、科目余额表和员工报销记录，诊断利润下滑原因并输出季度经营诊断与决策方案，生成Excel给我。"

func TestStoreListCreatesMissingTableAndSeedsBuiltins(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("List() returned no seeded experts")
	}
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("List() did not seed architect expert: %+v", defs)
	}
	architect, _ := findExpertDefinition(defs, "architect")
	if architect.Source != expert.SourceOfficial {
		t.Fatalf("architect source = %q, want official", architect.Source)
	}
	if architect.CreatedAt <= 0 || architect.UpdatedAt <= 0 {
		t.Fatalf("architect timestamps = created %d updated %d, want persisted timestamps", architect.CreatedAt, architect.UpdatedAt)
	}
	if !expertTableExists(t, dbPath) {
		t.Fatalf("List() did not create experts table")
	}
}

func TestStoreListDoesNotRewriteUnchangedBuiltins(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())
	if _, err := store.List(context.Background()); err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	db := openTestDB(t, dbPath)
	const sentinel int64 = 123
	if _, err := db.Exec(`UPDATE experts SET updated_at = ? WHERE id = ?`, sentinel, "architect"); err != nil {
		t.Fatalf("set sentinel updated_at: %v", err)
	}
	if _, err := store.List(context.Background()); err != nil {
		t.Fatalf("second List() error = %v", err)
	}
	var updatedAt int64
	if err := db.QueryRow(`SELECT updated_at FROM experts WHERE id = ?`, "architect").Scan(&updatedAt); err != nil {
		t.Fatalf("query architect updated_at: %v", err)
	}
	if updatedAt != sentinel {
		t.Fatalf("unchanged builtin updated_at = %d, want %d", updatedAt, sentinel)
	}
}

func TestStoreRejectsBuiltinMutation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())
	if err := store.Save(context.Background(), expert.Definition{ID: "architect"}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Save(official) error = %v, want read-only", err)
	}
	if err := store.Delete(context.Background(), "code-delivery-team"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Delete(official team) error = %v, want read-only", err)
	}
}

func TestStoreListSeedsBuiltinFinanceExpertTeam(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	financeTeam, ok := findExpertDefinition(defs, "finance-expert-team")
	if !ok {
		t.Fatalf("List() did not seed finance expert team: %+v", defs)
	}
	if financeTeam.Mode != expert.ModeTeam {
		t.Fatalf("finance expert team mode = %q, want team", financeTeam.Mode)
	}
	if financeTeam.Name != "财务专家团" {
		t.Fatalf("finance expert team name = %q, want 财务专家团", financeTeam.Name)
	}
	if !containsString(financeTeam.Categories, "财务分析") {
		t.Fatalf("finance expert team categories = %v, want 财务分析", financeTeam.Categories)
	}
	gotPrompt := ""
	if len(financeTeam.Prompts) > 0 {
		gotPrompt = financeTeam.Prompts[0]
	}
	if gotPrompt != expectedFinancePrompt {
		t.Fatalf("finance expert team first prompt = %q, want %q", gotPrompt, expectedFinancePrompt)
	}
	for _, memberName := range []string{"会计专家", "审计专家", "税务专家", "经营决策专家"} {
		if !hasMember(financeTeam.Members, memberName) {
			t.Fatalf("finance expert team members = %+v, want %s", financeTeam.Members, memberName)
		}
	}
	if !definitionTextContains(financeTeam, "进销存流水") || !definitionTextContains(financeTeam, "科目余额表") || !definitionTextContains(financeTeam, "员工报销记录") {
		t.Fatalf("finance expert team definition missing source data coverage: %+v", financeTeam)
	}
	if !definitionTextContains(financeTeam, "发票合规风险") || !definitionTextContains(financeTeam, "研发加计扣除") {
		t.Fatalf("finance expert team definition missing audit/tax coverage: %+v", financeTeam)
	}
}

func TestStoreListMigratesOldBuiltinFinancePrompt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(
		t,
		dbPath,
		"finance-expert-team",
		"team",
		"财务专家团",
		"财务分析",
		"汇聚进销存、科目余额和报销数据。",
		`["财务分析"]`,
		`["进销存"]`,
		"bar-chart-3",
		"amber",
		"data-analysis-team",
		`[]`,
		`["请调取本季度进销存流水、科目余额表和员工报销记录，诊断利润下滑原因并输出季度经营诊断与决策方案。"]`,
		140,
	)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	financeTeam, ok := findExpertDefinition(defs, "finance-expert-team")
	if !ok {
		t.Fatalf("List() did not return finance expert team: %+v", defs)
	}
	if len(financeTeam.Prompts) == 0 || financeTeam.Prompts[0] != expectedFinancePrompt {
		t.Fatalf("finance expert team prompt after migration = %v, want first prompt %q", financeTeam.Prompts, expectedFinancePrompt)
	}
}

func TestStoreListMigratesLegacyBuiltinEnglishDetails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(
		t,
		dbPath,
		"testing",
		"expert",
		"Testing Expert",
		"engineering",
		"Designs focused test strategies and identifies missing regression coverage.",
		`["engineering"]`,
		`["engineering","auto"]`,
		"bot",
		"stone",
		"",
		`[]`,
		`["You are a testing expert. Focus on behavior, regression risk, edge cases that matter, and practical test coverage. Prefer small targeted tests over broad brittle suites, and explain the minimum evidence needed to trust a change."]`,
		210,
	)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	testingExpert, ok := findExpertDefinition(defs, "testing")
	if !ok {
		t.Fatalf("List() did not return testing expert: %+v", defs)
	}
	if testingExpert.Name != "测试策略专家" {
		t.Fatalf("testing expert name after migration = %q, want 测试策略专家", testingExpert.Name)
	}
	if testingExpert.Role != "技术工程" {
		t.Fatalf("testing expert role after migration = %q, want 技术工程", testingExpert.Role)
	}
	if !containsString(testingExpert.Categories, "技术工程") {
		t.Fatalf("testing expert categories after migration = %v, want 技术工程", testingExpert.Categories)
	}
	if len(testingExpert.Prompts) != 3 {
		t.Fatalf("testing expert prompts after migration = %v, want three preset questions", testingExpert.Prompts)
	}
	if !strings.Contains(testingExpert.SystemPrompt, "测试策略专家") {
		t.Fatalf("testing expert system prompt after migration = %q, want role instructions", testingExpert.SystemPrompt)
	}
}

func TestStoreListExistingEmptyTableSeedsBuiltins(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("List() with existing empty table returned no seeded experts")
	}
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("List() did not seed frontend static experts: %+v", defs)
	}
	if !hasExpert(defs, "data-analysis-team", expert.ModeTeam) {
		t.Fatalf("List() did not seed frontend static expert teams: %+v", defs)
	}
}

func TestStoreListMigratesExistingCustomRowAndRestoresBuiltins(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "custom-only", "expert", "Custom Expert", "Custom Role", "Custom description.", `["Custom"]`, `["solo"]`, "bot", "stone", "", `[]`, `["Handle this custom task."]`, 10)
	insertExpertRow(t, dbPath, "custom-many", "expert", "Many Prompts", "Custom Role", "Custom description.", `["Custom"]`, `["many"]`, "bot", "stone", "", `[]`, `["one","two","three","four","five","six"]`, 20)
	insertExpertRow(t, dbPath, "custom-empty-team", "team", "Empty Team", "Coordination", "Legacy team without members.", `["Custom"]`, `["team"]`, "users", "stone", "", `[]`, `["Coordinate this task."]`, 30)
	insertExpertRow(t, dbPath, "custom-partial-team", "team", "Partial Team", "Coordination", "Legacy team with an incomplete member.", `["Custom"]`, `["team"]`, "users", "stone", "", `[{"kind":"sub","name":"Existing Member"}]`, `["Coordinate this task."]`, 40)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	custom, ok := findExpertDefinition(defs, "custom-only")
	if !ok {
		t.Fatalf("List() did not keep custom row: %+v", defs)
	}
	if custom.Name != "Custom Expert" || custom.Source != expert.SourceUser {
		t.Fatalf("custom row after migration = %+v", custom)
	}
	if len(custom.Prompts) != 3 || custom.Prompts[0] != "Handle this custom task." {
		t.Fatalf("custom prompts after migration = %v, want preserved first prompt plus two defaults", custom.Prompts)
	}
	many, ok := findExpertDefinition(defs, "custom-many")
	if !ok || len(many.Prompts) != 5 || many.Prompts[4] != "five" {
		t.Fatalf("oversized custom prompts after migration = %+v, want first five", many)
	}
	emptyTeam, ok := findExpertDefinition(defs, "custom-empty-team")
	if !ok || len(emptyTeam.Members) != 1 {
		t.Fatalf("empty legacy team after migration = %+v, want one generated member", emptyTeam)
	}
	assertCompleteLeadMember(t, emptyTeam.Members[0])
	partialTeam, ok := findExpertDefinition(defs, "custom-partial-team")
	if !ok || len(partialTeam.Members) != 1 || partialTeam.Members[0].Name != "Existing Member" {
		t.Fatalf("partial legacy team after migration = %+v, want preserved member name", partialTeam)
	}
	assertCompleteLeadMember(t, partialTeam.Members[0])
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("List() did not restore official catalog: %+v", defs)
	}
}

func TestStoreListInvalidJSONReturnsError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "bad-json", "expert", "Bad JSON", "Role", "Description.", `not-json`, `["tag"]`, "bot", "stone", "", `[]`, `["Prompt"]`, 1)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	_, err := store.List(context.Background())
	if err == nil {
		t.Fatalf("List() error = nil, want invalid JSON error")
	}
	if !strings.Contains(err.Error(), "categories_json") {
		t.Fatalf("List() error = %q, want categories_json context", err.Error())
	}
}

func TestStoreSaveCreatesMissingTableSeedsBuiltinsAndAddsExpert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Save(context.Background(), expert.Definition{
		ID:          "custom-advisor",
		Mode:        expert.ModeExpert,
		Name:        "Custom Advisor",
		Role:        "Custom Role",
		Description: "Handles custom tasks.",
		Categories:  []string{"Custom"},
		Tags:        []string{"custom"},
		IconKey:     "bot",
		Tone:        "stone",
		Prompts:     threePrompts("Handle the custom task."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("Save() before List() did not seed builtins: %+v", defs)
	}
	if !hasExpert(defs, "custom-advisor", expert.ModeExpert) {
		t.Fatalf("Save() did not add custom expert: %+v", defs)
	}
}

func TestStoreReplaceOfficialPreservesUserDefinitions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())
	if err := store.Save(context.Background(), expert.Definition{
		ID: "custom-advisor", Mode: expert.ModeExpert, Name: "Custom Advisor", Role: "Custom",
		Description: "User-owned.", Prompts: threePrompts("Help."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	replacement := []expert.Definition{
		{
			ID: "exp_architect", Mode: expert.ModeExpert, Name: "Remote Architect", Role: "System",
			Description: "Remote.", SystemPrompt: "Design.", Prompts: threePrompts("Design."),
		},
		{
			ID: "exg_empty", Mode: expert.ModeTeam, Name: "Empty Remote Team", Role: "System Team",
			Description: "No configured members.", Prompts: threePrompts("Coordinate."),
		},
	}
	remoteStore := expert.NewStore(dbPath, nil)
	if err := remoteStore.ReplaceOfficial(context.Background(), replacement); err != nil {
		t.Fatalf("ReplaceOfficial() error = %v", err)
	}
	defs, err := remoteStore.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) != 3 ||
		!hasExpert(defs, "custom-advisor", expert.ModeExpert) ||
		!hasExpert(defs, "exp_architect", expert.ModeExpert) ||
		!hasExpert(defs, "exg_empty", expert.ModeTeam) {
		t.Fatalf("definitions after replacement = %+v", defs)
	}
	if hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("packaged official expert survived replacement: %+v", defs)
	}
	remote, _ := findExpertDefinition(defs, "exp_architect")
	if remote.Source != expert.SourceOfficial || remote.SortOrder != 10 {
		t.Fatalf("remote definition = %+v", remote)
	}
}

func TestStoreDeleteUserDefinitionsPreservesOfficialDefinitions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)
	if err := store.ReplaceOfficial(context.Background(), []expert.Definition{{
		ID: "exp_architect", Mode: expert.ModeExpert, Name: "Remote Architect", Role: "System",
		Description: "Remote.", SystemPrompt: "Design.", Prompts: threePrompts("Design."),
	}}); err != nil {
		t.Fatalf("ReplaceOfficial() error = %v", err)
	}
	if err := store.Save(context.Background(), expert.Definition{
		ID: "custom-advisor", Mode: expert.ModeExpert, Name: "Custom Advisor", Role: "Custom",
		Description: "User-owned.", Prompts: threePrompts("Help."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.DeleteUserDefinitions(context.Background()); err != nil {
		t.Fatalf("DeleteUserDefinitions() error = %v", err)
	}
	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) != 1 || !hasExpert(defs, "exp_architect", expert.ModeExpert) {
		t.Fatalf("definitions after user deletion = %+v", defs)
	}
	if hasExpert(defs, "custom-advisor", expert.ModeExpert) {
		t.Fatalf("user definition survived deletion: %+v", defs)
	}
}

func TestStoreSaveRejectsInvalidPromptCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)

	err := store.Save(context.Background(), expert.Definition{
		ID:          "default-prompt",
		Mode:        expert.ModeExpert,
		Name:        "默认专家",
		Role:        "测试",
		Description: "Verifies default prompt language.",
		Prompts:     []string{"   "},
	})
	if err == nil || !strings.Contains(err.Error(), "between 3 and 5 prompts") {
		t.Fatalf("Save() error = %v, want prompt count validation", err)
	}
}

func TestStoreSaveAllowsTeamWithoutConfiguredMembers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)
	if err := store.Save(context.Background(), expert.Definition{
		ID:          "empty-team",
		Mode:        expert.ModeTeam,
		Name:        "Empty Team",
		Role:        "Validation",
		Description: "Allows members to be configured later.",
		Prompts:     threePrompts("Coordinate this task."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestStoreSaveRejectsInvalidConfiguredTeamMembers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)
	valid := expert.Member{Name: "Lead", Role: "Coordinator", Task: "Plan", Output: "Plan"}
	tests := []struct {
		name    string
		members []expert.Member
		want    string
	}{
		{name: "missing name", members: []expert.Member{{Role: valid.Role, Task: valid.Task, Output: valid.Output}}, want: "name is required"},
		{name: "missing role", members: []expert.Member{{Name: valid.Name, Task: valid.Task, Output: valid.Output}}, want: "role is required"},
		{name: "missing task", members: []expert.Member{{Name: valid.Name, Role: valid.Role, Output: valid.Output}}, want: "task is required"},
		{name: "missing output", members: []expert.Member{{Name: valid.Name, Role: valid.Role, Task: valid.Task}}, want: "output is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Save(context.Background(), expert.Definition{
				ID:          "invalid-team",
				Mode:        expert.ModeTeam,
				Name:        "Invalid Team",
				Role:        "Validation",
				Description: "Validates the member contract.",
				Members:     tt.members,
				Prompts:     threePrompts("Coordinate this task."),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Save() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestStoreSaveUpdatesExistingTeam(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "delivery-team", "team", "Delivery Team", "Delivery", "Old description.", `["Delivery"]`, `["old"]`, "users", "teal", "", `[{"name":"Lead","role":"Lead","task":"Plan","output":"Plan","tone":"blue"}]`, `["Old prompt"]`, 20)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Save(context.Background(), expert.Definition{
		ID:          "delivery-team",
		Mode:        expert.ModeTeam,
		Name:        "Updated Delivery Team",
		Role:        "Delivery",
		Description: "Updated description.",
		Categories:  []string{"Delivery", "Project"},
		Tags:        []string{"updated"},
		IconKey:     "users",
		Tone:        "green",
		Members: []expert.Member{
			{Name: "Planner", Role: "Planning", Task: "Break down work.", Output: "Plan", Tone: "blue"},
			{Name: "Reviewer", Role: "Review", Task: "Check delivery.", Output: "Review", Tone: "green"},
		},
		Prompts: threePrompts("Updated prompt."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	got, ok := findExpertDefinition(defs, "delivery-team")
	if !ok {
		t.Fatalf("List() did not return updated team: %+v", defs)
	}
	if got.Name != "Updated Delivery Team" || got.Description != "Updated description." || got.Tone != "green" {
		t.Fatalf("updated team = %+v", got)
	}
	if len(got.Members) != 2 || got.Members[1].Name != "Reviewer" {
		t.Fatalf("updated members = %+v", got.Members)
	}
	if got.SortOrder != 20 {
		t.Fatalf("updated team sort_order = %d, want preserved 20", got.SortOrder)
	}
}

func TestStoreTeamMembersNormalizeFirstLeadAndRemainingSub(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)

	if err := store.Save(context.Background(), expert.Definition{
		ID:          "agent-team",
		Mode:        expert.ModeTeam,
		Name:        "Agent Team",
		Role:        "Delivery",
		Description: "Coordinates a lead agent with sub agents.",
		IconKey:     "users",
		Tone:        "teal",
		Members: []expert.Member{
			{Kind: "sub", Name: "Lead Agent", Role: "Coordinator", Task: "Plan the work.", Output: "Plan", Tone: "blue"},
			{Kind: "lead", Name: "Worker Agent", Role: "Execution", Task: "Do the work.", Output: "Result", Tone: "green"},
		},
		Prompts: threePrompts("Coordinate this task."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) != 1 || len(defs[0].Members) != 2 {
		t.Fatalf("List() = %+v, want one team with two members", defs)
	}
	if defs[0].Members[0].Kind != "lead" || defs[0].Members[1].Kind != "sub" {
		t.Fatalf("member kinds = %+v, want lead/sub", defs[0].Members)
	}
}

func TestStoreTeamMembersPreserveAvatarKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)

	if err := store.Save(context.Background(), expert.Definition{
		ID:          "avatar-team",
		Mode:        expert.ModeTeam,
		Name:        "Avatar Team",
		Role:        "Delivery",
		Description: "Coordinates agents with avatars.",
		IconKey:     "users",
		Tone:        "teal",
		AvatarKey:   "data:image/png;base64,team",
		Members: []expert.Member{
			{Kind: "lead", Name: "Lead Agent", Role: "Coordinator", Task: "Plan.", Output: "Plan", Tone: "blue", AvatarKey: "architect"},
			{Kind: "sub", Name: "Worker Agent", Role: "Execution", Task: "Work.", Output: "Result", Tone: "green", AvatarKey: "data:image/png;base64,worker"},
		},
		Prompts: threePrompts("Coordinate this task."),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("List() = %+v, want one team", defs)
	}
	if defs[0].AvatarKey != "data:image/png;base64,team" {
		t.Fatalf("team avatar key = %q, want uploaded avatar data URL", defs[0].AvatarKey)
	}
	if len(defs[0].Members) != 2 || defs[0].Members[0].AvatarKey != "architect" || defs[0].Members[1].AvatarKey != "data:image/png;base64,worker" {
		t.Fatalf("member avatar keys = %+v, want preserved", defs[0].Members)
	}
}

func TestStoreListBackfillsLegacyTeamMemberKinds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "legacy-team", "team", "Legacy Team", "Delivery", "Legacy description.", `["Delivery"]`, `["legacy"]`, "users", "teal", "", `[{"name":"Old Lead","role":"Lead","task":"Plan","output":"Plan","tone":"blue"},{"name":"Old Worker","role":"Worker","task":"Work","output":"Result","tone":"green"}]`, `["Prompt"]`, 10)
	store := expert.NewStore(dbPath, nil)

	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) != 1 || len(defs[0].Members) != 2 {
		t.Fatalf("List() = %+v, want one legacy team with two members", defs)
	}
	if defs[0].Members[0].Kind != "lead" || defs[0].Members[1].Kind != "sub" {
		t.Fatalf("legacy member kinds = %+v, want first lead and second sub", defs[0].Members)
	}
}

func TestStoreListFilteredByModeCategoryAndQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "tech-expert", "expert", "Tech Expert", "Engineer", "Builds technical plans.", `["技术工程"]`, `["code"]`, "bot", "blue", "", `[]`, `["Prompt"]`, 10)
	insertExpertRow(t, dbPath, "report-team", "team", "Report Team", "Reports", "Writes operating reports.", `["报告生成"]`, `["report"]`, "users", "teal", "", `[{"kind":"lead","name":"Lead","role":"Lead","task":"Plan","output":"Plan","tone":"blue"}]`, `["Prompt"]`, 20)
	insertExpertRow(t, dbPath, "tech-team", "team", "Tech Team", "Engineering", "Coordinates engineering delivery.", `["技术工程"]`, `["delivery"]`, "users", "green", "", `[{"kind":"lead","name":"Lead","role":"Lead","task":"Plan","output":"Plan","tone":"blue"}]`, `["Prompt"]`, 30)
	store := expert.NewStore(dbPath, nil)

	teams, err := store.ListFiltered(context.Background(), expert.ListFilter{Mode: expert.ModeTeam})
	if err != nil {
		t.Fatalf("ListFiltered(mode) error = %v", err)
	}
	if got := expertIDs(teams); strings.Join(got, ",") != "report-team,tech-team" {
		t.Fatalf("ListFiltered(mode) ids = %v, want report-team,tech-team", got)
	}

	technical, err := store.ListFiltered(context.Background(), expert.ListFilter{Category: "技术工程"})
	if err != nil {
		t.Fatalf("ListFiltered(category) error = %v", err)
	}
	if got := expertIDs(technical); strings.Join(got, ",") != "tech-expert,tech-team" {
		t.Fatalf("ListFiltered(category) ids = %v, want tech-expert,tech-team", got)
	}

	matched, err := store.ListFiltered(context.Background(), expert.ListFilter{Mode: expert.ModeTeam, Category: "技术工程", Query: "coordinates"})
	if err != nil {
		t.Fatalf("ListFiltered(combined) error = %v", err)
	}
	if got := expertIDs(matched); strings.Join(got, ",") != "tech-team" {
		t.Fatalf("ListFiltered(combined) ids = %v, want tech-team", got)
	}
}

func TestStoreDeleteRemovesExistingRowAndSeedsEmptyTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "custom-only", "expert", "Custom Expert", "Custom Role", "Custom description.", `["Custom"]`, `["solo"]`, "bot", "stone", "", `[]`, `["Handle this custom task."]`, 10)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Delete(context.Background(), "custom-only"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if hasExpert(defs, "custom-only", expert.ModeExpert) {
		t.Fatalf("List() kept deleted custom row: %+v", defs)
	}
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("List() did not seed defaults after table became empty: %+v", defs)
	}
}

func TestStoreResetRebuildsDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	createExpertsTable(t, dbPath)
	insertExpertRow(t, dbPath, "custom-only", "expert", "Custom Expert", "Custom Role", "Custom description.", `["Custom"]`, `["solo"]`, "bot", "stone", "", `[]`, `["Handle this custom task."]`, 10)
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if hasExpert(defs, "custom-only", expert.ModeExpert) {
		t.Fatalf("Reset() kept custom row: %+v", defs)
	}
	if !hasExpert(defs, "architect", expert.ModeExpert) {
		t.Fatalf("Reset() did not seed builtins: %+v", defs)
	}
	if !hasExpert(defs, "finance-expert-team", expert.ModeTeam) {
		t.Fatalf("Reset() did not seed finance expert team: %+v", defs)
	}
}

func TestStoreResetCreatesMissingTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if !expertTableExists(t, dbPath) {
		t.Fatalf("Reset() did not create experts table")
	}
	defs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("Reset() seeded no builtins")
	}
	if !hasExpert(defs, "finance-expert-team", expert.ModeTeam) {
		t.Fatalf("Reset() did not seed finance expert team: %+v", defs)
	}
}

func TestStoreResetPersistsFinanceExpertExcelSystemPrompt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	db := openTestDB(t, dbPath)
	var prompt string
	if err := db.QueryRow(`SELECT system_prompt FROM experts WHERE id = ?`, "finance-expert-team").Scan(&prompt); err != nil {
		t.Fatalf("query finance expert system prompt: %v", err)
	}
	for _, want := range []string{".xlsx", "Excel", "path"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("finance expert system prompt = %q, want %q", prompt, want)
		}
	}
}

func TestStoreCatalogAfterResetFinanceExpertRequiresExcelWorkbook(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	userExpertPath := filepath.Join(t.TempDir(), "experts.json")
	store := expert.NewStore(dbPath, expert.BuiltinDefinitions())

	if err := store.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	catalog, err := expert.LoadStoreCatalog(context.Background(), store, userExpertPath)
	if err != nil {
		t.Fatalf("LoadStoreCatalog() error = %v", err)
	}
	financeTeam, err := catalog.Resolve("finance-expert-team")
	if err != nil {
		t.Fatalf("Resolve finance-expert-team: %v", err)
	}
	for _, want := range []string{".xlsx", "Excel", "path"} {
		if !strings.Contains(financeTeam.Prompt, want) {
			t.Fatalf("finance expert system prompt = %q, want %q", financeTeam.Prompt, want)
		}
	}
}

func TestStoreSaveRejectsInvalidDefinition(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewStore(dbPath, nil)

	err := store.Save(context.Background(), expert.Definition{
		ID:   "bad",
		Mode: "invalid",
		Name: "Bad",
	})
	if err == nil {
		t.Fatalf("Save() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid expert mode") {
		t.Fatalf("Save() error = %q, want invalid mode context", err.Error())
	}
}

func hasExpert(defs []expert.Definition, id string, mode expert.Mode) bool {
	for _, def := range defs {
		if def.ID == id && def.Mode == mode {
			return true
		}
	}
	return false
}

func findExpertDefinition(defs []expert.Definition, id string) (expert.Definition, bool) {
	for _, def := range defs {
		if def.ID == id {
			return def, true
		}
	}
	return expert.Definition{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func threePrompts(first string) []string {
	return []string{first, "Second preset question.", "Third preset question."}
}

func assertCompleteLeadMember(t *testing.T, member expert.Member) {
	t.Helper()
	if member.Kind != expert.MemberKindLead || strings.TrimSpace(member.Name) == "" ||
		strings.TrimSpace(member.Role) == "" || strings.TrimSpace(member.Task) == "" ||
		strings.TrimSpace(member.Output) == "" {
		t.Fatalf("member = %+v, want a complete lead", member)
	}
}

func hasMember(members []expert.Member, name string) bool {
	for _, member := range members {
		if member.Name == name {
			return true
		}
	}
	return false
}

func definitionTextContains(def expert.Definition, needle string) bool {
	if strings.Contains(def.Description, needle) {
		return true
	}
	for _, tag := range def.Tags {
		if strings.Contains(tag, needle) {
			return true
		}
	}
	for _, prompt := range def.Prompts {
		if strings.Contains(prompt, needle) {
			return true
		}
	}
	for _, member := range def.Members {
		if strings.Contains(member.Task, needle) || strings.Contains(member.Output, needle) {
			return true
		}
	}
	return false
}

func expertIDs(defs []expert.Definition) []string {
	out := make([]string, 0, len(defs))
	for _, def := range defs {
		out = append(out, def.ID)
	}
	return out
}

func openTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createExpertsTable(t *testing.T, dbPath string) {
	t.Helper()
	db := openTestDB(t, dbPath)
	if _, err := db.Exec(`
CREATE TABLE experts (
	id TEXT PRIMARY KEY,
	mode TEXT NOT NULL,
	name TEXT NOT NULL,
	role TEXT NOT NULL,
	description TEXT NOT NULL,
	categories_json TEXT NOT NULL,
	tags_json TEXT NOT NULL,
	icon_key TEXT NOT NULL,
	tone TEXT NOT NULL,
	avatar_key TEXT,
	members_json TEXT NOT NULL,
	prompts_json TEXT NOT NULL,
	sort_order INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
)`); err != nil {
		t.Fatalf("create experts table: %v", err)
	}
}

func expertTableExists(t *testing.T, dbPath string) bool {
	t.Helper()
	db := openTestDB(t, dbPath)
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'experts'`).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	return count == 1
}

func insertExpertRow(t *testing.T, dbPath, id, mode, name, role, description, categories, tags, iconKey, tone, avatarKey, members, prompts string, sortOrder int) {
	t.Helper()
	db := openTestDB(t, dbPath)
	if _, err := db.Exec(`
INSERT INTO experts (
	id, mode, name, role, description, categories_json, tags_json,
	icon_key, tone, avatar_key, members_json, prompts_json, sort_order,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1)`,
		id, mode, name, role, description, categories, tags, iconKey, tone, avatarKey, members, prompts, sortOrder,
	); err != nil {
		t.Fatalf("insert expert row: %v", err)
	}
}

func TestPersistentStoreReusesAndReleasesDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "experts.sqlite")
	store := expert.NewPersistentStore(dbPath, expert.BuiltinDefinitions())
	if _, err := store.List(context.Background()); err != nil {
		t.Fatalf("initial list: %v", err)
	}
	if _, err := store.List(context.Background()); err != nil {
		t.Fatalf("second list: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("database remained open after Close: %v", err)
	}
}
