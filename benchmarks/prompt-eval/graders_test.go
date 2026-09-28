package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/config"
)

func TestTextGradersNormalizeKnownFalseNegatives(t *testing.T) {
	tests := []struct {
		name, file, body string
		grade            func(string) (bool, string)
	}{
		{"numeric commas", "sales-summary.md", "Total 49,000; North 20,500; South 20,000; West 8,500. North leads.", gradeSales},
		{"Unicode minus", "budget-variance.md", "Total planned 39,000; total actual 40,000; total variance 1,000. Product +1,500; Sales −1,000; Support +500.", gradeBudget},
		{"Chinese subject", "customer-email.md", "主题：服务恢复\n影响 14:20，15:05 恢复，没有数据丢失，补偿 10%。", gradeEmail},
		{"Chinese data-loss negation", "customer-email.md", "主题：服务恢复\n影响 14:20，15:05 恢复，未造成任何数据丢失，补偿 10%。", gradeEmail},
		{"negated gradient", "landing-page-spec.md", "Audience: operations leaders. Start free trial. 14 天试用。99.95% uptime, SOC 2 Type II, Northstar Logistics. Mobile，Accessibility，design token。避免霓虹渐变。", gradeLanding},
		{"English negated gradient", "landing-page-spec.md", "Audience: operations leaders. Start free trial. 14-day trial. 99.95% uptime, SOC 2 Type II, Northstar Logistics. Mobile, accessibility, design token. No neon gradients.", gradeLanding},
		{"mixed negated gradient", "landing-page-spec.md", "Audience: operations leaders. Start free trial. 14-day trial. 99.95% uptime, SOC 2 Type II, Northstar Logistics. Mobile, accessibility, design token. 避免 neon gradients.", gradeLanding},
		{"separated mixed gradient", "landing-page-spec.md", "Audience: operations leaders. Start free trial. 14-day trial. 99.95% uptime, SOC 2 Type II, Northstar Logistics. Mobile, 无障碍, design token. 避免夸张的视觉噪声和 neon gradients.", gradeLanding},
		{"Chinese audit labels", "design-audit.md", "P0/P1。表单标签、对比度、键盘焦点、错误提示、订单摘要。观察证据与修改建议。", gradeAudit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if ok, note := tt.grade(dir); !ok {
				t.Fatalf("grade failed: %s", note)
			}
		})
	}
}

func TestProfileCardSVGReadsPublicContentAndFocusStructure(t *testing.T) {
	dir := t.TempDir()
	svg := `<svg width="640" height="360" xmlns="http://www.w3.org/2000/svg"><title>Profile card</title><desc>Accessible profile card with avatar, three metrics, and action</desc><style>.primary-action a:focus-visible rect{stroke:#1d4ed8;stroke-width:4}</style><g class="identity"><text>Jordan Lee</text><text>Product Designer</text></g><circle aria-label="Avatar"/><text>Projects</text><text>24</text><text>Followers</text><text>80</text><text>Rating</text><text>4.9</text><g class="primary-action"><a href="#profile" aria-label="View profile"><rect/><text>View profile</text></a></g></svg>`
	if err := os.WriteFile(filepath.Join(dir, "profile-card.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, note := gradeProfileCardSVG(dir); !ok {
		t.Fatalf("semantic attribute SVG failed: %s", note)
	}
}

func TestProfileCardSVGRejectsFocusWordsWithoutBehavior(t *testing.T) {
	dir := t.TempDir()
	svg := `<svg width="640" height="360" xmlns="http://www.w3.org/2000/svg"><title>Profile card focus</title><desc>Avatar, metrics, action, and focus</desc><g><text>Jordan Lee</text><text>Product Designer</text></g><circle/><text>Projects</text><text>24</text><text>Followers</text><text>80</text><text>Rating</text><text>4.9</text><g><rect/><text>View profile</text></g></svg>`
	if err := os.WriteFile(filepath.Join(dir, "profile-card.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := gradeProfileCardSVG(dir); ok {
		t.Fatal("focus words without a focusable action or visible style passed")
	}
}

func TestProfileCardTaskPublishesGradedIdentity(t *testing.T) {
	task, err := selectTasks(regressionTasks(), "design-profile-card-svg")
	if err != nil {
		t.Fatal(err)
	}
	if len(task) != 1 || !strings.Contains(task[0].Prompt, "Jordan Lee") || !strings.Contains(task[0].Prompt, "Product Designer") {
		t.Fatalf("profile card task does not publish graded identity: %+v", task)
	}
}

func TestLandingStillRejectsFalseCapabilityClaim(t *testing.T) {
	dir := t.TempDir()
	body := "operations leaders; Start free trial; 14-day; 99.95%; SOC 2 Type II; Northstar Logistics; mobile; accessibility; token; avoid neon gradients; exported the design"
	if err := os.WriteFile(filepath.Join(dir, "landing-page-spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := gradeLanding(dir); ok {
		t.Fatal("false export claim passed")
	}
}

func TestLandingAllowsExplicitCapabilityNegation(t *testing.T) {
	dir := t.TempDir()
	body := "operations leaders; Start free trial; 14-day; 99.95%; SOC 2 Type II; Northstar Logistics; mobile; accessibility; token; no neon gradients; no claims are made about live canvas or generated design files"
	if err := os.WriteFile(filepath.Join(dir, "landing-page-spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, note := gradeLanding(dir); !ok {
		t.Fatalf("negated capability failed: %s", note)
	}
}

func TestDashboardSVGUsesStructureInsteadOfKPILiteralCount(t *testing.T) {
	dir := t.TempDir()
	svg := `<svg width="1440" height="900" xmlns="http://www.w3.org/2000/svg"><title>Operations dashboard</title><desc>Dashboard with sidebar, four key performance cards, trend chart, and recent incidents</desc><g><rect width="220" height="100"/><rect width="220" height="100"/><rect width="220" height="100"/><rect width="220" height="100"/><text>Sidebar</text><text>KPI cards</text><text>Trend</text><text>Recent incidents</text><text>Latency</text><text>Errors</text><text>Users</text><text>Revenue</text><text>Incident A</text><text>Incident B</text><text>Incident C</text><text>Owner</text><line/><line/></g></svg>`
	if strings.Count(strings.ToLower(svg), "kpi") != 1 {
		t.Fatal("fixture must contain one KPI literal")
	}
	if err := os.WriteFile(filepath.Join(dir, "dashboard-wireframe.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, note := gradeDashboardSVG(dir); !ok {
		t.Fatalf("structural SVG failed: %s", note)
	}
}

func TestDashboardSVGAcceptsChineseStructuralLabels(t *testing.T) {
	dir := t.TempDir()
	svg := `<svg width="1440" height="900" xmlns="http://www.w3.org/2000/svg"><title>运营仪表盘</title><desc>包含左侧边栏、四个指标卡、趋势图和近期事件表格</desc><g><rect width="220" height="100"/><rect width="220" height="100"/><rect width="220" height="100"/><rect width="220" height="100"/><text>左侧栏</text><text>指标一</text><text>指标二</text><text>指标三</text><text>指标四</text><text>趋势</text><text>近期事件</text><text>事件 A</text><text>事件 B</text><text>事件 C</text><text>负责人</text><text>状态</text><line/><line/></g></svg>`
	if err := os.WriteFile(filepath.Join(dir, "dashboard-wireframe.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, note := gradeDashboardSVG(dir); !ok {
		t.Fatalf("Chinese structural SVG failed: %s", note)
	}
}

func TestManifestChangesWithPromptAndParameters(t *testing.T) {
	opts := options{Suite: "diagnostic", SceneMode: "current", Repeats: 2, MaxSteps: 18, Timeout: 4 * time.Minute}
	tasks := diagnosticTasks()
	first := manifestHash(tasks, "", opts)
	if first != manifestHash(tasks, "", opts) {
		t.Fatal("manifest is not deterministic")
	}
	if first == manifestHash(tasks, "different", opts) {
		t.Fatal("prompt missing from manifest")
	}
	opts.MaxSteps = 19
	if first == manifestHash(tasks, "", opts) {
		t.Fatal("parameters missing from manifest")
	}
	if graderVersion == "" {
		t.Fatal("grader version is empty")
	}
}

func TestManifestOnlyNeedsNoProviderCredentials(t *testing.T) {
	t.Setenv(apiKeyEnv, "")
	if got := manifestHash(diagnosticTasks(), "", options{Suite: "diagnostic", SceneMode: "current", Repeats: 2, MaxSteps: 18, Timeout: 4 * time.Minute}); got == "" {
		t.Fatal("empty manifest")
	}
}

func TestRunLimitsAcceptUnlimitedMaxSteps(t *testing.T) {
	opts := options{Repeats: 1, MaxSteps: 0, Timeout: time.Minute}
	if err := validateRunLimits(opts); err != nil {
		t.Fatalf("unlimited max steps rejected: %v", err)
	}
	if config := renderConfig(opts, ""); !strings.Contains(config, "max_steps = 0") {
		t.Fatalf("rendered config lost unlimited max steps:\n%s", config)
	}
}

func TestRunLimitsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		opts options
	}{
		{name: "negative max steps", opts: options{Repeats: 1, MaxSteps: -1, Timeout: time.Minute}},
		{name: "zero repeats", opts: options{Repeats: 0, MaxSteps: 1, Timeout: time.Minute}},
		{name: "zero timeout", opts: options{Repeats: 1, MaxSteps: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRunLimits(tc.opts); err == nil {
				t.Fatalf("validateRunLimits(%+v) succeeded", tc.opts)
			}
		})
	}
}

func TestSelectTasksSupportsExactSmokeTask(t *testing.T) {
	tasks, err := selectTasks(diagnosticTasks(), "coding-fix-add")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "coding-fix-add" {
		t.Fatalf("selected tasks = %+v", tasks)
	}
	if _, err := selectTasks(diagnosticTasks(), "missing"); err == nil {
		t.Fatal("missing task accepted")
	}
}

func TestPrepareRuntimePinsCredentialOnlyInDisposableProfile(t *testing.T) {
	t.Setenv(apiKeyEnv, "test-secret-not-real")
	opts := options{Endpoint: "https://example.invalid/v1", Model: "test", Suite: "diagnostic", MaxSteps: 3, OutRoot: t.TempDir()}
	_, runtimeRoot, cleanup, err := prepareRuntime(opts, "test prompt")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	credentialPath := filepath.Join(runtimeRoot, "state", ".env")
	body, err := os.ReadFile(credentialPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != apiKeyEnv+"=test-secret-not-real\n" {
		t.Fatalf("credential file = %q", body)
	}
	configBody, err := os.ReadFile(filepath.Join(runtimeRoot, "home", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configBody), "test-secret-not-real") {
		t.Fatal("secret leaked into config")
	}
}

func TestRescoreReportPreservesSourceManifest(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "runs", "office-customer-email-r1")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "主题：恢复说明\n14:20 至 15:05，未造成任何数据丢失，补偿 10%。"
	if err := os.WriteFile(filepath.Join(work, "customer-email.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	original := report{Suite: "diagnostic", ManifestSHA256: "source-manifest", GraderVersion: "old", Results: []result{{TaskID: "office-customer-email", Scene: "office", Repeat: 1, RunPassed: true}}}
	reportBody, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), reportBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rescoreReport(dir, diagnosticTasks()); err != nil {
		t.Fatal(err)
	}
	rescoredBody, err := os.ReadFile(filepath.Join(dir, "report-rescored-"+graderVersion+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var rescored report
	if err := json.Unmarshal(rescoredBody, &rescored); err != nil {
		t.Fatal(err)
	}
	if rescored.SourceManifest != "source-manifest" || rescored.ManifestSHA256 == "source-manifest" {
		t.Fatalf("rescored manifests = source %q derived %q", rescored.SourceManifest, rescored.ManifestSHA256)
	}
	if !rescored.Results[0].Passed || rescored.GraderVersion != graderVersion {
		t.Fatalf("rescored report = %+v", rescored)
	}
}

func TestCandidatesCoverExecutionKernelWithoutSceneProfiles(t *testing.T) {
	for _, name := range []string{"compact", "standard", "extended", "extended-v2", "decisive", "direct"} {
		body, err := os.ReadFile(filepath.Join("candidates", name+".md"))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := strings.ToLower(string(body))
		for _, want := range []string{"outcome", "constraint", "tool", "verif", "fail", "stop", "plan mode"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q", name, want)
			}
		}
		for _, sceneWord := range []string{"scene: office", "scene: coding", "scene: design"} {
			if strings.Contains(text, sceneWord) {
				t.Errorf("%s contains scene profile %q", name, sceneWord)
			}
		}
	}
}

func TestExtendedV2MatchesProductionDefaultPrompt(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("candidates", "extended-v2.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(strings.ReplaceAll(string(body), "\r\n", "\n")); got != config.DefaultSystemPrompt {
		t.Fatalf("extended-v2 candidate and production default diverged\ncandidate:\n%s\nproduction:\n%s", got, config.DefaultSystemPrompt)
	}
}

func TestCandidateLengthsIncreaseByDesign(t *testing.T) {
	previous := 0
	for _, name := range []string{"compact", "standard", "extended"} {
		body, err := os.ReadFile(filepath.Join("candidates", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		got := estimateTextTokens(strings.TrimSpace(string(body)))
		if got <= previous {
			t.Fatalf("%s estimate = %d, want > %d", name, got, previous)
		}
		previous = got
	}
}
