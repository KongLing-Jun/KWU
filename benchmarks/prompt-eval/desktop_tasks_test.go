package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFixturesAreDeterministicAndComplete(t *testing.T) {
	firstManifest, err := desktopManifestHash()
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := desktopManifestHash()
	if err != nil {
		t.Fatal(err)
	}
	if firstManifest != secondManifest {
		t.Fatalf("manifest changed: %s != %s", firstManifest, secondManifest)
	}
	seen := map[string]bool{}
	for _, task := range desktopTasks() {
		if seen[task.ID] || task.Prompt == "" || !task.Scene.IsSelectable() || len(task.ExpectedOutputs) == 0 {
			t.Fatalf("invalid desktop task: %+v", task)
		}
		seen[task.ID] = true
		first, err := desktopFixtureHash(task)
		if err != nil {
			t.Fatalf("%s first fixture: %v", task.ID, err)
		}
		second, err := desktopFixtureHash(task)
		if err != nil {
			t.Fatalf("%s second fixture: %v", task.ID, err)
		}
		if first != second {
			t.Fatalf("%s fixture changed: %s != %s", task.ID, first, second)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("desktop task count = %d, want 6", len(seen))
	}
}

func TestDesktopOfficeFixturesContainReadableOfficeParts(t *testing.T) {
	project := mustDesktopTask(t, "desktop-office-project-report")
	root := t.TempDir()
	if err := project.Seed(root); err != nil {
		t.Fatal(err)
	}
	assertZipEntryContains(t, filepath.Join(root, "project-brief.docx"), "word/document.xml", "Project Orion")

	budget := mustDesktopTask(t, "desktop-office-budget-analysis")
	root = t.TempDir()
	if err := budget.Seed(root); err != nil {
		t.Fatal(err)
	}
	assertZipEntryContains(t, filepath.Join(root, "budget.xlsx"), "xl/worksheets/sheet1.xml", "135000")
	assertZipEntryContains(t, filepath.Join(root, "budget.xlsx"), "xl/sharedStrings.xml", "Product")
}

func TestDesktopOfficeProjectGraderPositiveAndInputPreservation(t *testing.T) {
	task := mustDesktopTask(t, "desktop-office-project-report")
	root := seedDesktopTaskForTest(t, task)
	mustWriteTestFile(t, root, "leadership-status.md", `# Project Orion
Executive sponsor: Liu Mei. Current launch target: 2026-09-30. The prior 2026-09-22 date was superseded.
Approved budget remains CNY 480,000.
Migration rehearsal — Chen Wei — 2026-09-12 — At risk.
Security review — Priya Shah — 2026-09-18 — On track.
Support training — Omar Haddad — 2026-09-24 — Not started.
Risk: vendor export may affect rehearsal, but no delay is confirmed.
`)
	mustWriteTestFile(t, root, "action-items.csv", "action,owner,due_date,status\nConfirm vendor export,Chen Wei,2026-09-10,open\nCirculate security findings,Priya Shah,2026-09-15,open\nDraft training roster,Omar Haddad,2026-09-20,open\n")
	assertDesktopChecks(t, append(task.Grade(root), gradePreservedInputs(task, root)...), true)

	mustWriteTestFile(t, root, "meeting-notes.md", "modified")
	if checksPassed(gradePreservedInputs(task, root)) {
		t.Fatal("modified Office input should fail preservation")
	}
}

func TestDesktopOfficeBudgetGraderRejectsWrongTotal(t *testing.T) {
	task := mustDesktopTask(t, "desktop-office-budget-analysis")
	root := seedDesktopTaskForTest(t, task)
	writeDesktopBudgetSolution(t, root, "402000")
	assertDesktopChecks(t, task.Grade(root), true)
	writeDesktopBudgetSolution(t, root, "401000")
	if checksPassed(task.Grade(root)) {
		t.Fatal("wrong actual total should fail")
	}
}

func TestDesktopCodingRetryGraderChecksSemanticsAndAddedTests(t *testing.T) {
	task := mustDesktopTask(t, "desktop-coding-retry-policy")
	root := seedDesktopTaskForTest(t, task)
	mustWriteTestFile(t, root, "retry_policy.py", `def retry_delay(attempt, base=2, cap=30):
    if isinstance(attempt, bool) or not isinstance(attempt, int) or attempt < 0:
        raise ValueError("attempt")
    if isinstance(base, bool) or not isinstance(base, (int, float)) or base <= 0:
        raise ValueError("base")
    if isinstance(cap, bool) or not isinstance(cap, (int, float)) or cap <= 0:
        raise ValueError("cap")
    return min(cap, base * 2**attempt)

def describe_retry(seconds):
    return f"retry in {seconds}s"
`)
	mustWriteTestFile(t, root, "test_retry_policy.py", `import unittest
import retry_policy
class Tests(unittest.TestCase):
    def test_zero(self): self.assertEqual(retry_policy.retry_delay(0), 2)
    def test_growth(self): self.assertEqual(retry_policy.retry_delay(2), 8)
    def test_cap(self): self.assertEqual(retry_policy.retry_delay(9), 30)
    def test_invalid(self):
        with self.assertRaises(ValueError): retry_policy.retry_delay(True)
`)
	assertDesktopChecks(t, task.Grade(root), true)
	mustWriteTestFile(t, root, "retry_policy.py", strings.ReplaceAll(mustReadTestFile(t, root, "retry_policy.py"), "base * 2**attempt", "base * attempt"))
	if checksPassed(task.Grade(root)) {
		t.Fatal("linear retry implementation should fail hidden semantics")
	}
}

func TestDesktopCodingRetryGraderAcceptsValueAndTypeErrorsOnly(t *testing.T) {
	task := mustDesktopTask(t, "desktop-coding-retry-policy")
	root := seedDesktopTaskForTest(t, task)
	source := `def retry_delay(attempt, base=2, cap=30):
    if isinstance(attempt, bool) or not isinstance(attempt, int) or attempt < 0:
        raise TypeError("attempt")
    if isinstance(base, bool) or not isinstance(base, (int, float)) or base <= 0:
        raise TypeError("base")
    if isinstance(cap, bool) or not isinstance(cap, (int, float)) or cap <= 0:
        raise TypeError("cap")
    return min(cap, base * 2**attempt)

def describe_retry(seconds):
    return f"retry in {seconds}s"
`
	tests := `import unittest
import retry_policy
class Tests(unittest.TestCase):
    def test_zero(self): self.assertEqual(retry_policy.retry_delay(0), 2)
    def test_growth(self): self.assertEqual(retry_policy.retry_delay(2), 8)
    def test_cap(self): self.assertEqual(retry_policy.retry_delay(9), 30)
    def test_invalid(self):
        with self.assertRaises(TypeError): retry_policy.retry_delay(True)
`
	mustWriteTestFile(t, root, "retry_policy.py", source)
	mustWriteTestFile(t, root, "test_retry_policy.py", tests)
	assertDesktopChecks(t, task.Grade(root), true)

	mustWriteTestFile(t, root, "retry_policy.py", strings.ReplaceAll(source, "TypeError", "RuntimeError"))
	mustWriteTestFile(t, root, "test_retry_policy.py", strings.ReplaceAll(tests, "TypeError", "RuntimeError"))
	if checksPassed(task.Grade(root)) {
		t.Fatal("unrelated exception type should fail hidden rejection semantics")
	}
}

func TestDesktopCodingConfigGraderChecksDetachedDeepMerge(t *testing.T) {
	task := mustDesktopTask(t, "desktop-coding-nested-config")
	root := seedDesktopTaskForTest(t, task)
	mustWriteTestFile(t, root, "config_merge.py", `import copy
from collections.abc import Mapping

def merge_config(base, override):
    result = copy.deepcopy(base)
    for key, value in override.items():
        if key in result and isinstance(result[key], Mapping) and isinstance(value, Mapping):
            result[key] = merge_config(result[key], value)
        else:
            result[key] = copy.deepcopy(value)
    return result
`)
	mustWriteTestFile(t, root, "test_config_merge.py", `import unittest
from config_merge import merge_config
class Tests(unittest.TestCase):
    def test_scalar(self): self.assertEqual(merge_config({"a": 1}, {"a": 2}), {"a": 2})
    def test_nested(self): self.assertEqual(merge_config({"a": {"x": 1}}, {"a": {"y": 2}}), {"a": {"x": 1, "y": 2}})
    def test_list(self): self.assertEqual(merge_config({"a": [1]}, {"a": [2]}), {"a": [2]})
    def test_detached(self):
        base={"a":{"x":[]}}; got=merge_config(base,{}); got["a"]["x"].append(1); self.assertEqual(base,{"a":{"x":[]}})
`)
	assertDesktopChecks(t, task.Grade(root), true)
}

func TestDesktopDesignEmptyGraderRequiresVisibleFocus(t *testing.T) {
	task := mustDesktopTask(t, "desktop-design-empty-state")
	root := seedDesktopTaskForTest(t, task)
	mustWriteTestFile(t, root, "empty-state.html", `<!doctype html><html><head><link rel="stylesheet" href="empty-state.css"></head><body><main><h1>No locations connected</h1><p>Connect a warehouse location to start tracking inventory.</p><button>Connect location</button><a href="#">View setup guide</a></main></body></html>`)
	mustWriteTestFile(t, root, "empty-state.css", `:root{color-scheme:light dark}button:focus-visible,a:focus-visible{outline:3px solid #1d4ed8}@media(max-width:480px){main{padding:16px}}@media(prefers-reduced-motion:reduce){*{transition:none}}`)
	assertDesktopChecks(t, task.Grade(root), true)
	mustWriteTestFile(t, root, "empty-state.css", `:root{color-scheme:light dark}button:focus-visible,a:focus-visible{outline:none}@media(max-width:480px){main{padding:16px}}@media(prefers-reduced-motion:reduce){*{transition:none}}`)
	if checksPassed(task.Grade(root)) {
		t.Fatal("focus attribute without visible treatment should fail")
	}
}

func TestDesktopDashboardGraderRejectsInvalidSVG(t *testing.T) {
	task := mustDesktopTask(t, "desktop-design-operations-dashboard")
	root := seedDesktopTaskForTest(t, task)
	writeDesktopDashboardSolution(t, root)
	assertDesktopChecks(t, task.Grade(root), true)
	mustWriteTestFile(t, root, "dashboard-visual.svg", `<svg><broken></svg>`)
	if checksPassed(task.Grade(root)) {
		t.Fatal("invalid SVG should fail")
	}
}

func TestDesktopDashboardAcceptsResponsiveViewBoxAndRejectsMissingCanvas(t *testing.T) {
	task := mustDesktopTask(t, "desktop-design-operations-dashboard")
	if !strings.Contains(task.Prompt, "HTML must reference dashboard-visual.svg") {
		t.Fatalf("dashboard task hides the SVG integration contract: %s", task.Prompt)
	}
	root := seedDesktopTaskForTest(t, task)
	writeDesktopDashboardSolution(t, root)
	svg := mustReadTestFile(t, root, "dashboard-visual.svg")
	svg = strings.Replace(svg, `width="1200" height="760"`, `viewBox="0 0 640 360"`, 1)
	mustWriteTestFile(t, root, "dashboard-visual.svg", svg)
	assertDesktopChecks(t, task.Grade(root), true)

	mustWriteTestFile(t, root, "dashboard-visual.svg", strings.Replace(svg, `viewBox="0 0 640 360"`, "", 1))
	if checksPassed(task.Grade(root)) {
		t.Fatal("SVG without positive dimensions or viewBox should fail")
	}
}

func TestDesktopObservationIsNeverInferredFromArtifacts(t *testing.T) {
	task := mustDesktopTask(t, "desktop-office-project-report")
	complete, passed := gradeDesktopObservation(task, nil)
	if complete || passed {
		t.Fatal("missing observation must remain incomplete")
	}
	truth, falsity := true, false
	complete, passed = gradeDesktopObservation(task, &desktopObservation{
		RunCompleted: &truth, ScopePreserved: &truth, FalseClaim: &falsity, TimedOut: &falsity,
	})
	if !complete || !passed {
		t.Fatal("complete passing Office observation should pass")
	}
	design := mustDesktopTask(t, "desktop-design-empty-state")
	complete, passed = gradeDesktopObservation(design, &desktopObservation{
		RunCompleted: &truth, ScopePreserved: &truth, FalseClaim: &falsity, TimedOut: &falsity,
	})
	if complete || passed {
		t.Fatal("Design observation without visual fields must remain incomplete")
	}
}

func TestDesktopObservationJSONRejectsInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	if err := os.WriteFile(path, []byte(`{"run_completed":"yes"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopObservation(path); err == nil {
		t.Fatal("invalid observation JSON should fail")
	}
	valid, _ := json.Marshal(desktopObservation{})
	if err := os.WriteFile(path, valid, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDesktopObservation(path); err != nil {
		t.Fatal(err)
	}
}

func mustDesktopTask(t *testing.T, id string) desktopTask {
	t.Helper()
	task, err := desktopTaskByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func seedDesktopTaskForTest(t *testing.T, task desktopTask) string {
	t.Helper()
	root := t.TempDir()
	if err := task.Seed(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWriteTestFile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := writeTextFile(root, name, body); err != nil {
		t.Fatal(err)
	}
}

func mustReadTestFile(t *testing.T, root, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertDesktopChecks(t *testing.T, checks []desktopCheck, want bool) {
	t.Helper()
	if got := checksPassed(checks); got != want {
		t.Fatalf("checks passed=%t want=%t: %+v", got, want, checks)
	}
}

func assertZipEntryContains(t *testing.T, path, name, want string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		body, err := func() ([]byte, error) {
			stream, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer stream.Close()
			return io.ReadAll(stream)
		}()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), want) {
			t.Fatalf("%s missing %q", name, want)
		}
		return
	}
	t.Fatalf("missing ZIP entry %s", name)
}

func writeDesktopBudgetSolution(t *testing.T, root, actualTotal string) {
	t.Helper()
	mustWriteTestFile(t, root, "budget-analysis.md", "Variance = actual - planned. Total planned 390000; total actual "+actualTotal+"; total variance 12000. Product planned 120000 actual 135000 variance 15000. Sales planned 180000 actual 172000 variance -8000. Support planned 90000 actual 95000 variance 5000.")
	mustWriteTestFile(t, root, "decision-summary.md", "Stay within 405000. Product contractor acceleration is confirmed. Sales campaign moved to Q4. Support cause is not confirmed; validate by 2026-09-08.")
}

func writeDesktopDashboardSolution(t *testing.T, root string) {
	t.Helper()
	mustWriteTestFile(t, root, "operations-dashboard.html", `<!doctype html><html><head><link rel="stylesheet" href="dashboard.css"></head><body><main><object data="dashboard-visual.svg"></object><button>Review incidents</button></main></body></html>`)
	mustWriteTestFile(t, root, "dashboard.css", `button:focus-visible{outline:3px solid #2563eb}@media(max-width:600px){main{padding:12px}}@media(prefers-reduced-motion:reduce){*{transition:none}}`)
	mustWriteTestFile(t, root, "dashboard-visual.svg", `<svg width="1200" height="760" xmlns="http://www.w3.org/2000/svg"><title>Northstar operations dashboard</title><desc>Service health, trend, and recent incidents</desc><rect width="220" height="100"/><rect x="240" width="220" height="100"/><rect x="480" width="220" height="100"/><rect x="720" width="220" height="100"/><text>On-time delivery 96.8%</text><text y="20">Open incidents 3</text><text y="40">Orders today 1248</text><text y="60">Average delay 18 min</text><text y="200">Trend</text><line x1="10" y1="220" x2="100" y2="200"/><line x1="100" y1="200" x2="200" y2="210"/><text y="300">INC-104 Carrier scan delay</text><text y="330">INC-108 Warehouse sync degraded</text><text y="360">INC-111 Customs hold</text></svg>`)
}
