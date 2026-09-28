package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/scene"
)

const desktopGraderVersion = "desktop-prompt-eval-v1"

type desktopTask struct {
	ID              string
	Scene           scene.Scene
	Prompt          string
	ExpectedOutputs []string
	Preserve        []string
	Seed            func(string) error
	Grade           func(string) []desktopCheck
}

type desktopCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Note   string `json:"note,omitempty"`
}

type desktopObservation struct {
	Build             string `json:"build,omitempty"`
	Repeat            int    `json:"repeat,omitempty"`
	RunCompleted      *bool  `json:"run_completed,omitempty"`
	ScopePreserved    *bool  `json:"scope_preserved,omitempty"`
	FalseClaim        *bool  `json:"false_claim,omitempty"`
	TimedOut          *bool  `json:"timed_out,omitempty"`
	VisualOpened      *bool  `json:"visual_opened,omitempty"`
	VisualPassed      *bool  `json:"visual_passed,omitempty"`
	ModelCalls        int    `json:"model_calls,omitempty"`
	ToolCalls         int    `json:"tool_calls,omitempty"`
	ApprovalCount     int    `json:"approval_count,omitempty"`
	ModelDurationMS   int64  `json:"model_duration_ms,omitempty"`
	ApprovalWaitMS    int64  `json:"approval_wait_ms,omitempty"`
	RuntimeFindingRef string `json:"runtime_finding_ref,omitempty"`
	Notes             string `json:"notes,omitempty"`
}

type desktopPrepareReport struct {
	TaskID         string      `json:"task_id"`
	Scene          scene.Scene `json:"scene"`
	Workspace      string      `json:"workspace"`
	Prompt         string      `json:"prompt"`
	FixtureSHA256  string      `json:"fixture_sha256"`
	ManifestSHA256 string      `json:"manifest_sha256"`
	GraderVersion  string      `json:"grader_version"`
}

type desktopGradeReport struct {
	TaskID              string              `json:"task_id"`
	Scene               scene.Scene         `json:"scene"`
	Workspace           string              `json:"workspace"`
	FixtureSHA256       string              `json:"fixture_sha256"`
	ManifestSHA256      string              `json:"manifest_sha256"`
	GraderVersion       string              `json:"grader_version"`
	ArtifactPassed      bool                `json:"artifact_passed"`
	ObservationComplete bool                `json:"observation_complete"`
	EndToEndPassed      bool                `json:"end_to_end_passed"`
	Checks              []desktopCheck      `json:"checks"`
	Observation         *desktopObservation `json:"observation,omitempty"`
}

func runDesktopEval(opts options) error {
	if (opts.DesktopPrepare == "") == (opts.DesktopGrade == "") {
		return errors.New("pass exactly one of -desktop-prepare or -desktop-grade")
	}
	task, err := desktopTaskByID(opts.TaskID)
	if err != nil {
		return err
	}
	manifest, err := desktopManifestHash()
	if err != nil {
		return err
	}
	fixture, err := desktopFixtureHash(task)
	if err != nil {
		return err
	}

	if opts.DesktopPrepare != "" {
		root, err := filepath.Abs(opts.DesktopPrepare)
		if err != nil {
			return err
		}
		if err := requireEmptyDirectory(root); err != nil {
			return err
		}
		if err := task.Seed(root); err != nil {
			return fmt.Errorf("seed %s: %w", task.ID, err)
		}
		actual, err := hashDirectory(root)
		if err != nil {
			return err
		}
		if actual != fixture {
			return fmt.Errorf("fixture hash mismatch after prepare: got %s want %s", actual, fixture)
		}
		return writeJSON(os.Stdout, desktopPrepareReport{
			TaskID: task.ID, Scene: task.Scene, Workspace: root, Prompt: task.Prompt,
			FixtureSHA256: fixture, ManifestSHA256: manifest, GraderVersion: desktopGraderVersion,
		})
	}

	root, err := filepath.Abs(opts.DesktopGrade)
	if err != nil {
		return err
	}
	checks := task.Grade(root)
	checks = append(checks, gradePreservedInputs(task, root)...)
	artifactPassed := checksPassed(checks)
	observation, err := readDesktopObservation(opts.DesktopObservation)
	if err != nil {
		return err
	}
	complete, observationPassed := gradeDesktopObservation(task, observation)
	report := desktopGradeReport{
		TaskID: task.ID, Scene: task.Scene, Workspace: root, FixtureSHA256: fixture,
		ManifestSHA256: manifest, GraderVersion: desktopGraderVersion,
		ArtifactPassed: artifactPassed, ObservationComplete: complete,
		EndToEndPassed: artifactPassed && complete && observationPassed,
		Checks:         checks, Observation: observation,
	}
	return writeJSON(os.Stdout, report)
}

func desktopTaskByID(id string) (desktopTask, error) {
	id = strings.TrimSpace(id)
	for _, task := range desktopTasks() {
		if task.ID == id {
			return task, nil
		}
	}
	ids := make([]string, 0, len(desktopTasks()))
	for _, task := range desktopTasks() {
		ids = append(ids, task.ID)
	}
	return desktopTask{}, fmt.Errorf("desktop task %q not found; choose one of %s", id, strings.Join(ids, ", "))
}

func desktopTasks() []desktopTask {
	return []desktopTask{
		{
			ID: "desktop-office-project-report", Scene: scene.Office,
			Prompt:          "Work from project-brief.docx, milestones.csv, and meeting-notes.md. Create leadership-status.md and action-items.csv. The leadership report must give the current launch target, approved budget, executive sponsor, milestone status, decisions, risks, and any superseded date without treating it as current. The CSV must use the columns action,owner,due_date,status and include every explicit action. Resolve conflicts from the sources, do not invent causes or progress, preserve the input files, and verify both deliverables before finishing.",
			ExpectedOutputs: []string{"leadership-status.md", "action-items.csv"},
			Preserve:        []string{"project-brief.docx", "milestones.csv", "meeting-notes.md"},
			Seed:            seedDesktopOfficeProject, Grade: gradeDesktopOfficeProject,
		},
		{
			ID: "desktop-office-budget-analysis", Scene: scene.Office,
			Prompt:          "Read budget.xlsx and budget-context.md. Using variance = actual - planned, create budget-analysis.md and decision-summary.md. Show total and per-team planned, actual, and variance values; distinguish confirmed explanations from unknown causes; and keep recommendations within the approved CNY 405,000 ceiling. Preserve the inputs and verify the arithmetic and both deliverables before finishing.",
			ExpectedOutputs: []string{"budget-analysis.md", "decision-summary.md"},
			Preserve:        []string{"budget.xlsx", "budget-context.md"},
			Seed:            seedDesktopOfficeBudget, Grade: gradeDesktopOfficeBudget,
		},
		{
			ID: "desktop-coding-retry-policy", Scene: scene.Coding,
			Prompt:          "Fix retry_delay in retry_policy.py so attempt is a non-negative integer and the result is min(cap, base * 2**attempt); base and cap must be positive numbers and booleans are invalid. Preserve describe_retry and the public API. Add focused tests for normal, capped, and invalid cases, run the test suite, and stop after the requested behavior is verified. Do not change README.md.",
			ExpectedOutputs: []string{"retry_policy.py", "test_retry_policy.py"}, Preserve: []string{"README.md"},
			Seed: seedDesktopCodingRetry, Grade: gradeDesktopCodingRetry,
		},
		{
			ID: "desktop-coding-nested-config", Scene: scene.Coding,
			Prompt:          "Fix merge_config in config_merge.py. Nested mappings must merge recursively without mutating or sharing mutable nested state with either input; lists and scalar override values replace base values. Preserve unrelated keys and the behavior of settings.py. Add regression tests covering deep nesting, list replacement, input immutability, and detached output, then run the full local test suite. Do not change README.md or settings.py.",
			ExpectedOutputs: []string{"config_merge.py", "test_config_merge.py"}, Preserve: []string{"README.md", "settings.py"},
			Seed: seedDesktopCodingConfig, Grade: gradeDesktopCodingConfig,
		},
		{
			ID: "desktop-design-empty-state", Scene: scene.Design,
			Prompt:          "Read empty-state-brief.md and product-copy.md. Create empty-state.html and empty-state.css as a polished, responsive inventory-console empty state using the exact supplied copy. Include the required primary and secondary actions, semantic structure, a visible keyboard focus treatment, a mobile layout, reduced-motion handling, and no gradients. Preserve the source files. Verify the saved files and make sure the result can be opened locally before finishing; do not claim a cloud export or canvas.",
			ExpectedOutputs: []string{"empty-state.html", "empty-state.css"},
			Preserve:        []string{"empty-state-brief.md", "product-copy.md"},
			Seed:            seedDesktopDesignEmpty, Grade: gradeDesktopDesignEmpty,
		},
		{
			ID: "desktop-design-operations-dashboard", Scene: scene.Design,
			Prompt:          "Read dashboard-brief.md and data-snapshot.csv. Create operations-dashboard.html, dashboard.css, and dashboard-visual.svg as a locally openable operations dashboard. The SVG must be valid and include an accessible title and description, four KPI cards, a labeled trend region, and three recent-incident rows. The HTML must reference dashboard-visual.svg and expose a keyboard-accessible primary action with a clearly visible focus treatment. Support narrow screens and reduced motion, preserve the inputs, verify every artifact, and do not claim a cloud export or canvas.",
			ExpectedOutputs: []string{"operations-dashboard.html", "dashboard.css", "dashboard-visual.svg"},
			Preserve:        []string{"dashboard-brief.md", "data-snapshot.csv"},
			Seed:            seedDesktopDesignDashboard, Grade: gradeDesktopDesignDashboard,
		},
	}
}

func seedDesktopOfficeProject(root string) error {
	if err := writeDeterministicDOCX(filepath.Join(root, "project-brief.docx"), []string{
		"Project Orion leadership brief", "Executive sponsor: Liu Mei", "Current public launch target: 2026-09-30",
		"Approved budget: CNY 480,000. No increase has been approved.",
	}); err != nil {
		return err
	}
	if err := writeTextFile(root, "milestones.csv", "milestone,owner,due_date,status\nMigration rehearsal,Chen Wei,2026-09-12,At risk\nSecurity review,Priya Shah,2026-09-18,On track\nSupport training,Omar Haddad,2026-09-24,Not started\n"); err != nil {
		return err
	}
	return writeTextFile(root, "meeting-notes.md", `# Orion delivery review

- Decision: the earlier draft launch date of 2026-09-22 is superseded by the approved 2026-09-30 target.
- Decision: keep the approved budget unchanged.
- Risk: the vendor export is expected on 2026-09-10 and may delay the migration rehearsal; no delay is confirmed yet.
- Action: Chen Wei — confirm receipt and integrity of the vendor export by 2026-09-10 — open.
- Action: Priya Shah — circulate interim security findings by 2026-09-15 — open.
- Action: Omar Haddad — draft the support-training roster by 2026-09-20 — open.
`)
}

func seedDesktopOfficeBudget(root string) error {
	rows := [][]string{{"Team", "Planned", "Actual"}, {"Product", "120000", "135000"}, {"Sales", "180000", "172000"}, {"Support", "90000", "95000"}}
	if err := writeDeterministicXLSX(filepath.Join(root, "budget.xlsx"), "Budget", rows); err != nil {
		return err
	}
	return writeTextFile(root, "budget-context.md", `# Q3 budget context

- Variance is actual minus planned; positive means over plan.
- Product: the CNY 15,000 overrun is from approved contractor acceleration.
- Sales: the CNY 8,000 underspend is from a campaign moved to Q4.
- Support: the reason for the CNY 5,000 overrun is not confirmed.
- Leadership ceiling: total actual spend must remain at or below CNY 405,000.
- Requested follow-up: Support validates the cause by 2026-09-08.
`)
}

func seedDesktopCodingRetry(root string) error {
	if err := writeTextFile(root, "retry_policy.py", `def retry_delay(attempt, base=2, cap=30):
    return min(cap, base * attempt)

def describe_retry(seconds):
    return f"retry in {seconds}s"
`); err != nil {
		return err
	}
	if err := writeTextFile(root, "test_retry_policy.py", `import unittest
import retry_policy

class RetryPolicyTests(unittest.TestCase):
    def test_first_attempt(self):
        self.assertEqual(retry_policy.retry_delay(0), 2)

    def test_description(self):
        self.assertEqual(retry_policy.describe_retry(4), "retry in 4s")

if __name__ == "__main__":
    unittest.main()
`); err != nil {
		return err
	}
	return writeTextFile(root, "README.md", "# Retry policy\n\nPublic API: retry_delay and describe_retry.\n")
}

func seedDesktopCodingConfig(root string) error {
	if err := writeTextFile(root, "config_merge.py", `def merge_config(base, override):
    result = dict(base)
    result.update(override)
    return result
`); err != nil {
		return err
	}
	if err := writeTextFile(root, "settings.py", `from config_merge import merge_config

DEFAULTS = {"db": {"host": "localhost", "port": 5432}, "features": ["audit"], "keep": True}

def effective_settings(override):
    return merge_config(DEFAULTS, override)
`); err != nil {
		return err
	}
	if err := writeTextFile(root, "test_config_merge.py", `import unittest
from config_merge import merge_config
from settings import effective_settings

class ConfigMergeTests(unittest.TestCase):
    def test_shallow_override(self):
        self.assertEqual(merge_config({"a": 1}, {"a": 2}), {"a": 2})

    def test_settings_keeps_unrelated_key(self):
        self.assertTrue(effective_settings({"db": {"port": 6432}})["keep"])

if __name__ == "__main__":
    unittest.main()
`); err != nil {
		return err
	}
	return writeTextFile(root, "README.md", "# Settings merge\n\nPreserve effective_settings and the public merge_config API.\n")
}

func seedDesktopDesignEmpty(root string) error {
	if err := writeTextFile(root, "empty-state-brief.md", `# Inventory console empty state

Audience: warehouse coordinators setting up a new workspace.
Goal: connect the first warehouse location without making the screen feel like an error.
Primary action: Connect location
Secondary action: View setup guide
Direction: calm, operational, light and dark friendly; no gradients.
Responsive: usable at 320px and 1440px. Respect reduced motion.
Accessibility: semantic heading, descriptive text, keyboard navigation, and clearly visible focus.
`); err != nil {
		return err
	}
	return writeTextFile(root, "product-copy.md", "Heading: No locations connected\nBody: Connect a warehouse location to start tracking inventory.\n")
}

func seedDesktopDesignDashboard(root string) error {
	if err := writeTextFile(root, "dashboard-brief.md", `# Northstar operations dashboard

Audience: logistics operations leaders.
Goal: identify service health and open incidents in under 30 seconds.
Required KPI labels: On-time delivery, Open incidents, Orders today, Average delay.
Required primary action: Review incidents.
Direction: calm, precise, trustworthy; no gradients.
Responsive: narrow and desktop layouts. Respect reduced motion and keyboard use.
`); err != nil {
		return err
	}
	return writeTextFile(root, "data-snapshot.csv", "metric,value\nOn-time delivery,96.8%\nOpen incidents,3\nOrders today,1248\nAverage delay,18 min\nIncident INC-104,Carrier scan delay\nIncident INC-108,Warehouse sync degraded\nIncident INC-111,Customs hold\n")
}

func requireEmptyDirectory(root string) error {
	entries, err := os.ReadDir(root)
	if err == nil {
		if len(entries) != 0 {
			return fmt.Errorf("desktop workspace must be empty: %s", root)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(root, 0o755)
}

func writeTextFile(root, name, body string) error {
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.ReplaceAll(body, "\r\n", "\n")), 0o644)
}

func writeDeterministicDOCX(path string, paragraphs []string) error {
	var body strings.Builder
	for _, paragraph := range paragraphs {
		fmt.Fprintf(&body, "<w:p><w:r><w:t>%s</w:t></w:r></w:p>", xmlEscape(paragraph))
	}
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`,
	}
	return writeDeterministicZip(path, entries)
}

func writeDeterministicXLSX(path, sheet string, rows [][]string) error {
	shared := make([]string, 0)
	index := map[string]int{}
	var sheetXML strings.Builder
	for rowIndex, row := range rows {
		fmt.Fprintf(&sheetXML, "<row r=\"%d\">", rowIndex+1)
		for columnIndex, value := range row {
			column := string(rune('A' + columnIndex))
			if _, err := fmt.Sscan(value, new(float64)); err == nil && rowIndex > 0 && columnIndex > 0 {
				fmt.Fprintf(&sheetXML, "<c r=\"%s%d\"><v>%s</v></c>", column, rowIndex+1, value)
				continue
			}
			position, ok := index[value]
			if !ok {
				position = len(shared)
				index[value] = position
				shared = append(shared, value)
			}
			fmt.Fprintf(&sheetXML, "<c r=\"%s%d\" t=\"s\"><v>%d</v></c>", column, rowIndex+1, position)
		}
		sheetXML.WriteString("</row>")
	}
	var sharedXML strings.Builder
	for _, value := range shared {
		fmt.Fprintf(&sharedXML, "<si><t>%s</t></si>", xmlEscape(value))
	}
	entries := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + xmlEscape(sheet) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` + sheetXML.String() + `</sheetData></worksheet>`,
		"xl/sharedStrings.xml":       fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">%s</sst>`, len(shared), len(shared), sharedXML.String()),
	}
	return writeDeterministicZip(path, entries)
}

func writeDeterministicZip(path string, entries map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(file)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			return err
		}
		if _, err := io.WriteString(writer, entries[name]); err != nil {
			_ = zw.Close()
			_ = file.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func xmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(value)
}

func desktopFixtureHash(task desktopTask) (string, error) {
	root, err := os.MkdirTemp("", "desktop-prompt-fixture-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)
	if err := task.Seed(root); err != nil {
		return "", err
	}
	return hashDirectory(root)
}

func hashDirectory(root string) (string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, relative := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", relative, len(body))
		_, _ = hash.Write(body)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func desktopManifestHash() (string, error) {
	type manifestTask struct {
		ID, Scene, Prompt, Fixture string
		ExpectedOutputs, Preserve  []string
	}
	tasks := desktopTasks()
	manifest := make([]manifestTask, 0, len(tasks))
	for _, task := range tasks {
		fixture, err := desktopFixtureHash(task)
		if err != nil {
			return "", err
		}
		manifest = append(manifest, manifestTask{
			ID: task.ID, Scene: string(task.Scene), Prompt: task.Prompt, Fixture: fixture,
			ExpectedOutputs: task.ExpectedOutputs, Preserve: task.Preserve,
		})
	}
	body, _ := json.Marshal(struct {
		Version          string
		TaskSourceHash   string
		GraderSourceHash string
		Tasks            []manifestTask
	}{desktopGraderVersion, textHash(desktopTaskSource), textHash(desktopGraderSource), manifest})
	return textHash(string(body)), nil
}

func gradePreservedInputs(task desktopTask, root string) []desktopCheck {
	expectedRoot, err := os.MkdirTemp("", "desktop-prompt-preserve-")
	if err != nil {
		return []desktopCheck{{Name: "preserved-inputs", Passed: false, Note: err.Error()}}
	}
	defer os.RemoveAll(expectedRoot)
	if err := task.Seed(expectedRoot); err != nil {
		return []desktopCheck{{Name: "preserved-inputs", Passed: false, Note: err.Error()}}
	}
	checks := make([]desktopCheck, 0, len(task.Preserve))
	for _, name := range task.Preserve {
		want, wantErr := os.ReadFile(filepath.Join(expectedRoot, name))
		got, gotErr := os.ReadFile(filepath.Join(root, name))
		passed := wantErr == nil && gotErr == nil && string(want) == string(got)
		note := "unchanged"
		if !passed {
			note = "input missing or modified"
		}
		checks = append(checks, desktopCheck{Name: "preserve:" + name, Passed: passed, Note: note})
	}
	return checks
}

func readDesktopObservation(path string) (*desktopObservation, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read desktop observation: %w", err)
	}
	var observation desktopObservation
	if err := json.Unmarshal(body, &observation); err != nil {
		return nil, fmt.Errorf("decode desktop observation: %w", err)
	}
	return &observation, nil
}

func gradeDesktopObservation(task desktopTask, observation *desktopObservation) (bool, bool) {
	if observation == nil || observation.RunCompleted == nil || observation.ScopePreserved == nil ||
		observation.FalseClaim == nil || observation.TimedOut == nil {
		return false, false
	}
	complete := true
	passed := *observation.RunCompleted && *observation.ScopePreserved && !*observation.FalseClaim && !*observation.TimedOut
	if task.Scene == scene.Design {
		if observation.VisualOpened == nil || observation.VisualPassed == nil {
			complete = false
			passed = false
		} else {
			passed = passed && *observation.VisualOpened && *observation.VisualPassed
		}
	}
	return complete, passed
}

func checksPassed(checks []desktopCheck) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func readCSVRecords(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errors.New("CSV has no data rows")
	}
	headers := rows[0]
	result := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != len(headers) {
			return nil, errors.New("CSV row width does not match header")
		}
		record := make(map[string]string, len(headers))
		for index, header := range headers {
			record[normalizeText(header)] = strings.TrimSpace(row[index])
		}
		result = append(result, record)
	}
	return result, nil
}
