package main

import (
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
)

func TestExpertBindingsDelegateToActiveController(t *testing.T) {
	catalog, err := expert.NewCatalog([]expert.Expert{{
		ID:          "testing",
		Name:        "测试策略专家",
		Description: "检查行为正确性和回归覆盖。",
		Category:    "技术工程",
		Prompt:      "关注行为、回归风险和实际验证证据。",
		Language:    "auto",
		Enabled:     true,
	}})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		selection, err := expert.ResolveSelection(catalog, sessionExpertID, requestExpertID, "")
		if err != nil {
			return "", selection, err
		}
		prompt := "base system prompt"
		if selection.Selected {
			prompt = expert.ApplyPrompt(prompt, selection.Expert)
		}
		return prompt, selection, nil
	}
	exec := agent.New(nil, nil, agent.NewSession("base system prompt"), agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor:      exec,
		SystemPrompt:  "base system prompt",
		Experts:       catalog,
		PromptBuilder: builder,
		SessionDir:    t.TempDir(),
		ExpertCatalogLoader: func() (*expert.Catalog, error) {
			return catalog, nil
		},
	})
	ctrl.EnsureSessionPath()
	app := &App{}
	app.setTestCtrl(ctrl, "test/model")

	experts := app.ListSessionExperts()
	if len(experts) != 1 || experts[0].ID != "testing" {
		t.Fatalf("ListSessionExperts = %+v, want testing expert", experts)
	}
	if got := app.CurrentExpert(); got.SelectedID != "" || got.Source != expert.SourceNone || !got.CanSwitch {
		t.Fatalf("CurrentExpert before selection = %+v, want no selected expert and canSwitch", got)
	}

	selected, err := app.SetExpert("testing")
	if err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	if selected.SelectedID != "testing" || selected.SelectedName != "测试策略专家" || selected.Source != expert.SourceRequest {
		t.Fatalf("SetExpert state = %+v, want selected testing expert from request", selected)
	}
	if got := app.CurrentExpert(); got.SelectedID != "testing" {
		t.Fatalf("CurrentExpert after SetExpert = %+v, want testing", got)
	}
	if sys := exec.Session().Snapshot()[0].Content; !strings.Contains(sys, "Expert role: 测试策略专家") {
		t.Fatalf("system prompt missing expert role after SetExpert:\n%s", sys)
	}

	cleared, err := app.ClearExpert()
	if err != nil {
		t.Fatalf("ClearExpert: %v", err)
	}
	if cleared.SelectedID != "" || cleared.Source != expert.SourceNone || !cleared.CanSwitch {
		t.Fatalf("ClearExpert state = %+v, want none", cleared)
	}
}

func TestExpertGroupBindingsDelegateEverySelectedExpertToTab(t *testing.T) {
	catalog, err := expert.NewCatalog([]expert.Expert{
		{ID: "testing", Name: "Testing", Description: "Quality", Category: "Quality", Prompt: "Test the result.", Enabled: true, Mode: expert.ModeExpert},
		{ID: "architecture", Name: "Architecture", Description: "Design", Category: "Architecture", Prompt: "Review the architecture.", Enabled: true, Mode: expert.ModeExpert},
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		selection, resolveErr := expert.ResolveSelection(catalog, sessionExpertID, requestExpertID, "")
		if resolveErr != nil {
			return "", selection, resolveErr
		}
		prompt := "base system prompt"
		if selection.Selected {
			prompt = expert.ApplyPrompt(prompt, selection.Expert)
		}
		return prompt, selection, nil
	}
	exec := agent.New(nil, nil, agent.NewSession("base system prompt"), agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: exec, SystemPrompt: "base system prompt", Experts: catalog, PromptBuilder: builder,
		ExpertCatalogLoader: func() (*expert.Catalog, error) { return catalog, nil }, SessionDir: t.TempDir(),
	})
	ctrl.EnsureSessionPath()
	app := &App{}
	app.setTestCtrl(ctrl, "test/model")

	state, err := app.SetExpertGroupForTab("", []string{"testing", "architecture"})
	if err != nil {
		t.Fatalf("SetExpertGroupForTab: %v", err)
	}
	if state.SelectedName != "项目专家团" {
		t.Fatalf("state = %+v, want project expert team", state)
	}
	team, err := app.ExpertTeamForTab("")
	if err != nil {
		t.Fatalf("ExpertTeamForTab: %v", err)
	}
	if team == nil || team.Lead.Name != "Testing" || team.ID != state.SelectedID {
		t.Fatalf("ExpertTeamForTab = %+v, want the active project expert team", team)
	}
}

func TestExpertInfoRoundTripsTeamMemberAvatars(t *testing.T) {
	input := ExpertInfo{
		ID:          "custom-team",
		Source:      expert.SourceUser,
		Mode:        "team",
		Name:        "Custom Team",
		Role:        "Delivery",
		Description: "Coordinates delivery.",
		Categories:  []string{"Delivery"},
		Tags:        []string{"team"},
		IconKey:     "users",
		Tone:        "teal",
		AvatarKey:   "data:image/png;base64,team",
		Members: []ExpertMemberInfo{
			{Kind: "lead", Name: "Lead Agent", Role: "Coordinator", Task: "Plan.", Output: "Plan", Tone: "blue", AvatarKey: "architect"},
			{Kind: "sub", Name: "Worker Agent", Role: "Execution", Task: "Work.", Output: "Result", Tone: "green", AvatarKey: "data:image/png;base64,worker"},
		},
		Prompts: []string{"Handle the task.", "Review the plan.", "List the risks."},
		Enabled: true,
	}

	def, err := expertDefinition(input)
	if err != nil {
		t.Fatalf("expertDefinition: %v", err)
	}
	if def.AvatarKey != input.AvatarKey || len(def.Members) != 2 || def.Members[1].AvatarKey != input.Members[1].AvatarKey {
		t.Fatalf("definition avatars = team %q members %+v, want preserved", def.AvatarKey, def.Members)
	}

	infos := expertInfos([]expert.Definition{def})
	if len(infos) != 1 || infos[0].Source != expert.SourceUser || infos[0].AvatarKey != input.AvatarKey || len(infos[0].Members) != 2 || infos[0].Members[0].AvatarKey != "architect" || infos[0].Members[1].AvatarKey != input.Members[1].AvatarKey {
		t.Fatalf("expertInfos avatars = %+v, want preserved", infos)
	}
}

func TestDesktopExpertMutationsRejectOfficialIDs(t *testing.T) {
	app := &App{}
	if _, err := app.CreateExpert(ExpertInfo{ID: "architect"}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("CreateExpert(official) error = %v, want read-only", err)
	}
	if _, err := app.UpdateExpert("architect", ExpertInfo{}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("UpdateExpert(official) error = %v, want read-only", err)
	}
	if err := app.DeleteExpert("code-delivery-team"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("DeleteExpert(official team) error = %v, want read-only", err)
	}
}
