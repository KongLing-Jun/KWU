package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanReportsFunctionLineViolation(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 5))

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 3, NestingDepth: 10, Complexity: 20, PackageFiles: 10},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	v := findViolation(violations, "func_lines")
	if v == nil {
		t.Fatalf("missing func_lines violation in %#v", violations)
	}
	if v.File != "internal/example/example.go" || v.Func != "TooLong" {
		t.Fatalf("unexpected violation target: %#v", *v)
	}
	if v.Got <= v.Limit {
		t.Fatalf("violation got=%d limit=%d, want got > limit", v.Got, v.Limit)
	}
}

func TestScanAllowsFunctionWithinLimit(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("Small", 2))

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 10, NestingDepth: 10, Complexity: 20, PackageFiles: 10},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("scan() violations = %#v, want none", violations)
	}
}

func TestScanReportsPackageFileCountViolation(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/a.go", "package example\nfunc A() {}\n")
	writeGoFile(t, root, "internal/example/b.go", "package example\nfunc B() {}\n")

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 10, NestingDepth: 10, Complexity: 20, PackageFiles: 1},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	v := findViolation(violations, "package_files")
	if v == nil {
		t.Fatalf("missing package_files violation in %#v", violations)
	}
	if v.File != "internal/example" || v.Got != 2 || v.Limit != 1 {
		t.Fatalf("unexpected package violation: %#v", *v)
	}
}

func TestBaselineAllowsSameViolationRejectsWorseAndShrinksImproved(t *testing.T) {
	base := Baseline{Version: baselineVersion, Entries: map[string]BaselineEntry{
		"func_lines|internal/example/example.go|TooLong": {
			Kind:  "func_lines",
			File:  "internal/example/example.go",
			Func:  "TooLong",
			Line:  3,
			Got:   100,
			Limit: 80,
		},
	}}

	same := []Violation{{Kind: "func_lines", File: "internal/example/example.go", Func: "TooLong", Line: 3, Got: 100, Limit: 80}}
	result := compareWithBaseline(same, base)
	if !result.OK() || result.Changed {
		t.Fatalf("same violation result = %#v, want OK with no baseline change", result)
	}

	worse := []Violation{{Kind: "func_lines", File: "internal/example/example.go", Func: "TooLong", Line: 3, Got: 101, Limit: 80}}
	result = compareWithBaseline(worse, base)
	if result.OK() {
		t.Fatalf("worse violation result = %#v, want failure", result)
	}

	improved := []Violation{{Kind: "func_lines", File: "internal/example/example.go", Func: "TooLong", Line: 3, Got: 90, Limit: 80}}
	result = compareWithBaseline(improved, base)
	if !result.OK() || !result.Changed {
		t.Fatalf("improved violation result = %#v, want OK with shrink", result)
	}
	entry := result.Baseline.Entries["func_lines|internal/example/example.go|TooLong"]
	if entry.Got != 90 {
		t.Fatalf("shrunk baseline got = %d, want 90", entry.Got)
	}
}

func TestBaselineKeysSeparateMethodsWithSameName(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/example.go", strings.Join([]string{
		"package example",
		"type A struct{}",
		"type B struct{}",
		longMethodSource("A", "Execute", 5),
		longMethodSource("B", "Execute", 5),
	}, "\n"))

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 3, NestingDepth: 10, Complexity: 20, PackageFiles: 10},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got := len(violations); got != 2 {
		t.Fatalf("violations = %d, want 2: %#v", got, violations)
	}

	baseline := baselineFromViolations(violations)
	if got := len(baseline.Entries); got != 2 {
		t.Fatalf("baseline entries = %d, want 2: %#v", got, baseline.Entries)
	}
	if _, ok := baseline.Entries["func_lines|internal/example/example.go|A.Execute"]; !ok {
		t.Fatalf("missing A.Execute key: %#v", baseline.Entries)
	}
	if _, ok := baseline.Entries["func_lines|internal/example/example.go|B.Execute"]; !ok {
		t.Fatalf("missing B.Execute key: %#v", baseline.Entries)
	}
}

func TestCLIDefaultImprovementLeavesBaselineUnchanged(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 85))

	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 82))
	var stdout strings.Builder
	if code := run([]string{"--root", root, "--baseline", baselinePath}, &stdout, nil); code != 0 {
		t.Fatalf("improved baseline check exit = %d, want 0", code)
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stdout.String(), "--ratchet") {
		t.Fatalf("improved baseline output = %q, want explicit --ratchet hint", stdout.String())
	}
}

func TestCLIRatchetWritesImprovedBaselineAndPrunesMissingEntries(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
	writeGoFile(t, root, "internal/example/b.go", longFunctionSource("TooLongB", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)

	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 82))
	if err := os.Remove(filepath.Join(root, "internal", "example", "b.go")); err != nil {
		t.Fatalf("remove b.go: %v", err)
	}
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--ratchet"}, nil, nil); code != 0 {
		t.Fatalf("ratchet exit = %d, want 0", code)
	}
	if after := readFile(t, baselinePath); string(after) == string(before) {
		t.Fatal("ratchet left baseline bytes unchanged, want tighter baseline")
	}

	baseline := readBaseline(t, baselinePath)
	if got := len(baseline.Entries); got != 1 {
		t.Fatalf("ratcheted baseline entries = %d, want 1: %#v", got, baseline.Entries)
	}
	entry := baseline.Entries["func_lines|internal/example/a.go|TooLongA"]
	if entry.Got != 84 {
		t.Fatalf("ratcheted baseline got = %d, want 84", entry.Got)
	}
	if _, ok := baseline.Entries["func_lines|internal/example/b.go|TooLongB"]; ok {
		t.Fatalf("ratcheted baseline retained missing b.go entry: %#v", baseline.Entries)
	}
}

func TestCLIRatchetRejectsNonDefaultRootsWithoutWritingBaseline(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
	writeGoFile(t, root, "cmd/example/b.go", longFunctionSource("TooLongB", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	if baseline := readBaseline(t, baselinePath); len(baseline.Entries) != 2 {
		t.Fatalf("baseline entries = %d, want internal and cmd entries: %#v", len(baseline.Entries), baseline.Entries)
	}
	before := readFile(t, baselinePath)

	var stderr strings.Builder
	code := run([]string{"--root", root, "--baseline", baselinePath, "--roots", "internal", "--ratchet"}, nil, &stderr)
	if code == 0 {
		t.Error("ratchet with non-default roots exit = 0, want failure")
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stderr.String(), "--ratchet") {
		t.Errorf("ratchet with non-default roots stderr = %q, want --ratchet usage error", stderr.String())
	}
}

func TestCLINonDefaultRootsIsPartialAndLeavesBaselineUnchanged(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
	writeGoFile(t, root, "cmd/example/b.go", longFunctionSource("TooLongB", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)

	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 82))
	var stdout strings.Builder
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--roots", "internal"}, &stdout, nil); code != 0 {
		t.Fatalf("non-default roots check exit = %d, want 0", code)
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stdout.String(), "partial check") {
		t.Fatalf("non-default roots output = %q, want partial check", stdout.String())
	}
}

func TestCLIRatchetRejectsPartialScanWithoutWritingBaseline(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
	writeGoFile(t, root, "internal/example/b.go", longFunctionSource("TooLongB", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--roots", "internal", "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)

	var stderr strings.Builder
	code := run([]string{"--root", root, "--baseline", baselinePath, "--ratchet", "internal/example/a.go"}, nil, &stderr)
	if code == 0 {
		t.Fatal("partial ratchet exit = 0, want failure")
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stderr.String(), "--ratchet") {
		t.Fatalf("partial ratchet stderr = %q, want --ratchet usage error", stderr.String())
	}
}

func TestCLIRatchetRejectsFreezeWithoutWritingBaseline(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)

	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 86))
	var stderr strings.Builder
	code := run([]string{"--root", root, "--baseline", baselinePath, "--ratchet", "--freeze"}, nil, &stderr)
	if code == 0 {
		t.Error("ratchet with freeze exit = 0, want failure")
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stderr.String(), "--ratchet") || !strings.Contains(stderr.String(), "--freeze") {
		t.Errorf("ratchet with freeze stderr = %q, want both conflicting flags", stderr.String())
	}
}

func TestCLINewOrWorsenedViolationLeavesBaselineUnchanged(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(t *testing.T, root string)
		wantStderr string
	}{
		{
			name: "new",
			mutate: func(t *testing.T, root string) {
				writeGoFile(t, root, "internal/example/b.go", longFunctionSource("TooLongB", 85))
			},
			wantStderr: "new violation",
		},
		{
			name: "worsened",
			mutate: func(t *testing.T, root string) {
				writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 86))
			},
			wantStderr: "worsened baseline violation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
			writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
			if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
				t.Fatalf("freeze exit = %d, want 0", code)
			}
			before := readFile(t, baselinePath)

			tt.mutate(t, root)
			var stderr strings.Builder
			code := run([]string{"--root", root, "--baseline", baselinePath, "--ratchet"}, nil, &stderr)
			if code == 0 {
				t.Fatalf("%s violation exit = 0, want failure", tt.name)
			}
			assertFileBytesEqual(t, baselinePath, before)
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("%s violation stderr = %q, want %q", tt.name, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestCLIJSONDefaultImprovementLeavesBaselineUnchanged(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 85))
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	before := readFile(t, baselinePath)

	writeGoFile(t, root, "internal/example/example.go", longFunctionSource("TooLong", 82))
	var stdout strings.Builder
	if code := run([]string{"--root", root, "--baseline", baselinePath, "--json"}, &stdout, nil); code != 0 {
		t.Fatalf("JSON improved baseline check exit = %d, want 0", code)
	}
	assertFileBytesEqual(t, baselinePath, before)
	if !strings.Contains(stdout.String(), `"violations"`) {
		t.Fatalf("JSON output = %q, want violations field", stdout.String())
	}
}

func TestPartialFileCheckDoesNotRewriteGlobalBaseline(t *testing.T) {
	root := t.TempDir()
	baselinePath := filepath.Join(root, "scripts", "code_guard_baseline.json")
	writeGoFile(t, root, "internal/example/a.go", longFunctionSource("TooLongA", 85))
	writeGoFile(t, root, "internal/example/b.go", longFunctionSource("TooLongB", 85))

	if code := run([]string{"--root", root, "--baseline", baselinePath, "--roots", "internal", "--freeze"}, nil, nil); code != 0 {
		t.Fatalf("freeze exit = %d, want 0", code)
	}
	if code := run([]string{"--root", root, "--baseline", baselinePath, "internal/example/a.go"}, nil, nil); code != 0 {
		t.Fatalf("partial check exit = %d, want 0", code)
	}

	baseline := readBaseline(t, baselinePath)
	if got := len(baseline.Entries); got != 2 {
		t.Fatalf("baseline entries after partial scan = %d, want 2: %#v", got, baseline.Entries)
	}
}

func TestLineCountingIgnoresCommentMarkersInsideStrings(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/example.go", strings.Join([]string{
		"package example",
		"func WithStringMarkers() int {",
		`	s := "/*"`,
		"	total := len(s)",
		"	total++",
		"	total++",
		"	return total",
		"}",
	}, "\n"))

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 3, NestingDepth: 10, Complexity: 20, PackageFiles: 10},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if v := findViolation(violations, "func_lines"); v == nil {
		t.Fatalf("missing func_lines violation in %#v", violations)
	}
}

func TestLongLinesAreCounted(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/example/example.go", "package example\n\nfunc LongLine() string {\n\ts := \""+strings.Repeat("x", 70*1024)+"\"\n\treturn s\n}\n")

	violations, err := scan(ScanOptions{
		Root:   root,
		Roots:  []string{"internal"},
		Limits: Limits{FileLines: 100, FuncLines: 1, NestingDepth: 10, Complexity: 20, PackageFiles: 10},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if v := findViolation(violations, "func_lines"); v == nil {
		t.Fatalf("missing func_lines violation in %#v", violations)
	}
}

func writeGoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readBaseline(t *testing.T, path string) Baseline {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("decode baseline: %v", err)
	}
	return baseline
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func assertFileBytesEqual(t *testing.T, path string, want []byte) {
	t.Helper()
	if got := readFile(t, path); string(got) != string(want) {
		t.Fatalf("%s bytes changed unexpectedly", path)
	}
}

func longFunctionSource(name string, bodyLines int) string {
	var b strings.Builder
	b.WriteString("package example\n\n")
	b.WriteString("func " + name + "() int {\n")
	b.WriteString("\ttotal := 0\n")
	for i := 0; i < bodyLines; i++ {
		b.WriteString("\ttotal++\n")
	}
	b.WriteString("\treturn total\n")
	b.WriteString("}\n")
	return b.String()
}

func longMethodSource(receiver, name string, bodyLines int) string {
	var b strings.Builder
	b.WriteString("func (" + strings.ToLower(receiver) + " " + receiver + ") " + name + "() int {\n")
	b.WriteString("\ttotal := 0\n")
	for i := 0; i < bodyLines; i++ {
		b.WriteString("\ttotal++\n")
	}
	b.WriteString("\treturn total\n")
	b.WriteString("}\n")
	return b.String()
}

func findViolation(violations []Violation, kind string) *Violation {
	for i := range violations {
		if violations[i].Kind == kind {
			return &violations[i]
		}
	}
	return nil
}
