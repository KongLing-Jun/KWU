package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const baselineVersion = 1
const defaultRoots = "internal,cmd,scripts,tools"

type Limits struct {
	FileLines    int `json:"file_lines"`
	FuncLines    int `json:"func_lines"`
	NestingDepth int `json:"nesting_depth"`
	Complexity   int `json:"complexity"`
	PackageFiles int `json:"package_files"`
}

type ScanOptions struct {
	Root   string
	Roots  []string
	Files  []string
	Limits Limits
}

type Violation struct {
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Func    string `json:"func,omitempty"`
	Line    int    `json:"line,omitempty"`
	Got     int    `json:"got"`
	Limit   int    `json:"limit"`
	Message string `json:"message"`
}

type BaselineEntry struct {
	Kind  string `json:"kind"`
	File  string `json:"file"`
	Func  string `json:"func,omitempty"`
	Line  int    `json:"line,omitempty"`
	Got   int    `json:"got"`
	Limit int    `json:"limit"`
}

type Baseline struct {
	Version int                      `json:"version"`
	Entries map[string]BaselineEntry `json:"entries"`
}

type CompareResult struct {
	Baseline Baseline
	New      []Violation
	Worse    []Violation
	Changed  bool
}

type packageStat struct {
	files int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	args = stripArgSeparator(args)

	var root string
	var baselinePath string
	var rootsRaw string
	var freeze bool
	var ratchet bool
	var strict bool
	var jsonOut bool

	fs := flag.NewFlagSet("code_guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&root, "root", ".", "repository root")
	fs.StringVar(&baselinePath, "baseline", filepath.FromSlash("scripts/code_guard_baseline.json"), "baseline JSON path")
	fs.StringVar(&rootsRaw, "roots", defaultRoots, "comma-separated roots to scan")
	fs.BoolVar(&freeze, "freeze", false, "write current violations as the baseline")
	fs.BoolVar(&ratchet, "ratchet", false, "write eligible baseline improvements after a full check")
	fs.BoolVar(&strict, "strict", false, "ignore baseline and fail on all violations")
	fs.BoolVar(&jsonOut, "json", false, "write machine-readable result")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fullScan := len(fs.Args()) == 0 && rootsRaw == defaultRoots
	if ratchet && freeze {
		fmt.Fprintln(stderr, "code_guard: --ratchet and --freeze cannot be used together")
		return 2
	}
	if ratchet && !fullScan {
		fmt.Fprintln(stderr, "code_guard: --ratchet requires a full scan with default roots and no positional file arguments")
		return 2
	}

	root = cleanRoot(root)
	if !filepath.IsAbs(baselinePath) {
		baselinePath = filepath.Join(root, baselinePath)
	}
	opts := ScanOptions{
		Root:   root,
		Roots:  splitCSV(rootsRaw),
		Files:  fs.Args(),
		Limits: defaultLimits(),
	}
	violations, err := scan(opts)
	if err != nil {
		fmt.Fprintf(stderr, "code_guard: scan failed: %v\n", err)
		return 1
	}

	if jsonOut {
		if err := json.NewEncoder(stdout).Encode(struct {
			Violations []Violation `json:"violations"`
		}{Violations: violations}); err != nil {
			fmt.Fprintf(stderr, "code_guard: write json: %v\n", err)
			return 1
		}
	}

	if freeze {
		if err := saveBaseline(baselinePath, baselineFromViolations(violations)); err != nil {
			fmt.Fprintf(stderr, "code_guard: save baseline: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "code_guard: froze %d violation(s) in %s\n", len(violations), displayPath(root, baselinePath))
		return 0
	}

	if strict {
		return reportStrictResult(stdout, stderr, violations)
	}

	baseline, ok, err := loadBaseline(baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "code_guard: load baseline: %v\n", err)
		return 1
	}
	if !ok {
		if len(violations) == 0 {
			fmt.Fprintln(stdout, "code_guard: passed (no baseline needed)")
			return 0
		}
		fmt.Fprintf(stderr, "code_guard: %d violation(s) found and baseline is missing; run `go run ./scripts/code_guard -- --freeze` to record existing debt\n", len(violations))
		printViolations(stderr, violations)
		return 1
	}

	result := compareWithBaselineForScope(violations, baseline, fullScan)
	if !result.OK() {
		printCompareFailures(stderr, result)
		return 1
	}
	if !fullScan {
		fmt.Fprintln(stdout, "code_guard: passed (partial check; baseline unchanged)")
		return 0
	}
	if result.Changed {
		if !ratchet {
			fmt.Fprintln(stdout, "code_guard: passed (baseline changes available; rerun with --ratchet to apply)")
			return 0
		}
		if err := saveBaseline(baselinePath, result.Baseline); err != nil {
			fmt.Fprintf(stderr, "code_guard: save ratcheted baseline: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "code_guard: passed and ratcheted baseline (%s)\n", displayPath(root, baselinePath))
		return 0
	}
	fmt.Fprintln(stdout, "code_guard: passed")
	return 0
}

func scan(opts ScanOptions) ([]Violation, error) {
	root := cleanRoot(opts.Root)
	limits := opts.Limits.withDefaults()
	stats := map[string]*packageStat{}
	var violations []Violation

	if len(opts.Files) > 0 {
		for _, name := range opts.Files {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			fileViolations, err := scanGoFile(root, path, limits, stats)
			if err != nil {
				return nil, err
			}
			violations = append(violations, fileViolations...)
		}
		return sortedViolations(append(violations, packageViolations(stats, limits)...)), nil
	}

	for _, scanRoot := range opts.Roots {
		scanRoot = strings.TrimSpace(scanRoot)
		if scanRoot == "" {
			continue
		}
		absRoot := filepath.Join(root, filepath.FromSlash(scanRoot))
		if _, err := os.Stat(absRoot); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", scanRoot, err)
		}
		err := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if shouldSkipDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isProductionGoFile(path) {
				return nil
			}
			fileViolations, err := scanGoFile(root, path, limits, stats)
			if err != nil {
				return err
			}
			violations = append(violations, fileViolations...)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", scanRoot, err)
		}
	}
	violations = append(violations, packageViolations(stats, limits)...)
	return sortedViolations(violations), nil
}

func scanGoFile(root, path string, limits Limits, stats map[string]*packageStat) ([]Violation, error) {
	if !isProductionGoFile(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if isGenerated(data) {
		return nil, nil
	}
	rel := displayPath(root, path)
	rawLines := splitLines(data)
	codeLines := codeLineSet(data)
	fileLines := codeLinesInRange(codeLines, 1, len(rawLines))
	pkgDir := filepath.ToSlash(filepath.Dir(rel))
	stat := stats[pkgDir]
	if stat == nil {
		stat = &packageStat{}
		stats[pkgDir] = stat
	}
	stat.files++

	var violations []Violation
	if fileLines > limits.FileLines {
		violations = append(violations, violation("file_lines", rel, "", 1, fileLines, limits.FileLines))
	}

	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	for _, decl := range fileNode.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name == nil {
			continue
		}
		metrics := measureFunction(codeLines, fset, fn)
		if metrics.lines > limits.FuncLines {
			violations = append(violations, violation("func_lines", rel, metrics.name, metrics.line, metrics.lines, limits.FuncLines))
		}
		if metrics.nesting > limits.NestingDepth {
			violations = append(violations, violation("nesting_depth", rel, metrics.name, metrics.line, metrics.nesting, limits.NestingDepth))
		}
		if metrics.complexity > limits.Complexity {
			violations = append(violations, violation("complexity", rel, metrics.name, metrics.line, metrics.complexity, limits.Complexity))
		}
	}
	return violations, nil
}

type funcMetrics struct {
	name       string
	line       int
	lines      int
	nesting    int
	complexity int
}

func measureFunction(codeLines map[int]bool, fset *token.FileSet, fn *ast.FuncDecl) funcMetrics {
	start := fset.Position(fn.Body.Lbrace).Line + 1
	end := fset.Position(fn.Body.Rbrace).Line - 1
	return funcMetrics{
		name:       functionName(fn),
		line:       fset.Position(fn.Pos()).Line,
		lines:      codeLinesInRange(codeLines, start, end),
		nesting:    maxNesting(fn.Body, 0),
		complexity: cyclomaticComplexity(fn),
	}
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := receiverTypeName(fn.Recv.List[0].Type)
	if recv == "" {
		return fn.Name.Name
	}
	return recv + "." + fn.Name.Name
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		name := receiverTypeName(t.X)
		if name == "" {
			return ""
		}
		return "*" + name
	case *ast.SelectorExpr:
		prefix := receiverTypeName(t.X)
		if prefix == "" {
			return t.Sel.Name
		}
		return prefix + "." + t.Sel.Name
	default:
		return ""
	}
}

func packageViolations(stats map[string]*packageStat, limits Limits) []Violation {
	var out []Violation
	for pkg, stat := range stats {
		if stat.files > limits.PackageFiles {
			out = append(out, violation("package_files", pkg, "", 1, stat.files, limits.PackageFiles))
		}
	}
	return out
}

func compareWithBaseline(violations []Violation, baseline Baseline) CompareResult {
	return compareWithBaselineForScope(violations, baseline, true)
}

func compareWithBaselineForScope(violations []Violation, baseline Baseline, pruneMissing bool) CompareResult {
	next := cloneBaseline(baseline)
	if next.Version == 0 {
		next.Version = baselineVersion
	}
	if next.Entries == nil {
		next.Entries = map[string]BaselineEntry{}
	}
	current := map[string]Violation{}
	result := CompareResult{Baseline: next}

	for _, v := range violations {
		key := v.Key()
		current[key] = v
		entry, ok := baseline.Entries[key]
		if !ok {
			result.New = append(result.New, v)
			continue
		}
		if v.Got > entry.Got {
			result.Worse = append(result.Worse, v)
			continue
		}
		if v.Got < entry.Got || v.Line != entry.Line || v.Limit != entry.Limit {
			next.Entries[key] = entryFromViolation(v)
			result.Changed = true
		}
	}

	if pruneMissing {
		for key := range next.Entries {
			if _, ok := current[key]; !ok {
				delete(next.Entries, key)
				result.Changed = true
			}
		}
	}
	result.Baseline = next
	return result
}

func (r CompareResult) OK() bool {
	return len(r.New) == 0 && len(r.Worse) == 0
}

func baselineFromViolations(violations []Violation) Baseline {
	b := Baseline{Version: baselineVersion, Entries: map[string]BaselineEntry{}}
	for _, v := range violations {
		b.Entries[v.Key()] = entryFromViolation(v)
	}
	return b
}

func entryFromViolation(v Violation) BaselineEntry {
	return BaselineEntry{
		Kind:  v.Kind,
		File:  v.File,
		Func:  v.Func,
		Line:  v.Line,
		Got:   v.Got,
		Limit: v.Limit,
	}
}

func loadBaseline(path string) (Baseline, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Baseline{}, false, nil
		}
		return Baseline{}, false, err
	}
	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return Baseline{}, false, err
	}
	if baseline.Version != baselineVersion {
		return Baseline{}, false, fmt.Errorf("unsupported baseline version %d", baseline.Version)
	}
	if baseline.Entries == nil {
		baseline.Entries = map[string]BaselineEntry{}
	}
	return baseline, true, nil
}

func saveBaseline(path string, baseline Baseline) error {
	if baseline.Version == 0 {
		baseline.Version = baselineVersion
	}
	if baseline.Entries == nil {
		baseline.Entries = map[string]BaselineEntry{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func cloneBaseline(b Baseline) Baseline {
	out := Baseline{Version: b.Version, Entries: map[string]BaselineEntry{}}
	for key, entry := range b.Entries {
		out.Entries[key] = entry
	}
	return out
}

func (v Violation) Key() string {
	return baselineKey(v.Kind, v.File, v.Func)
}

func baselineKey(kind, file, fn string) string {
	return kind + "|" + file + "|" + fn
}

func violation(kind, file, fn string, line, got, limit int) Violation {
	msg := file
	if fn != "" {
		msg = fmt.Sprintf("%s:%d %s()", file, line, fn)
	}
	return Violation{
		Kind:    kind,
		File:    file,
		Func:    fn,
		Line:    line,
		Got:     got,
		Limit:   limit,
		Message: fmt.Sprintf("%s %s got=%d limit=%d", kind, msg, got, limit),
	}
}

func defaultLimits() Limits {
	return Limits{
		FileLines:    800,
		FuncLines:    80,
		NestingDepth: 4,
		Complexity:   10,
		PackageFiles: 30,
	}
}

func (l Limits) withDefaults() Limits {
	def := defaultLimits()
	if l.FileLines == 0 {
		l.FileLines = def.FileLines
	}
	if l.FuncLines == 0 {
		l.FuncLines = def.FuncLines
	}
	if l.NestingDepth == 0 {
		l.NestingDepth = def.NestingDepth
	}
	if l.Complexity == 0 {
		l.Complexity = def.Complexity
	}
	if l.PackageFiles == 0 {
		l.PackageFiles = def.PackageFiles
	}
	return l
}

func maxNesting(node ast.Node, depth int) int {
	maxDepth := depth
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil || n == node {
			return true
		}
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if got := maxNesting(n, depth+1); got > maxDepth {
				maxDepth = got
			}
			return false
		}
		return true
	})
	return maxDepth
}

func cyclomaticComplexity(fn *ast.FuncDecl) int {
	cc := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			cc++
		case *ast.CaseClause:
			if v.List != nil {
				cc++
			}
		case *ast.CommClause:
			if v.Comm != nil {
				cc++
			}
		case *ast.BinaryExpr:
			if v.Op.String() == "&&" || v.Op.String() == "||" {
				cc++
			}
		}
		return true
	})
	return cc
}

func codeLineSet(data []byte) map[int]bool {
	lines := map[int]bool{}
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(data))
	var s scanner.Scanner
	s.Init(file, data, nil, scanner.ScanComments)
	for {
		pos, tok, _ := s.Scan()
		switch tok {
		case token.EOF:
			return lines
		case token.COMMENT, token.SEMICOLON:
			continue
		default:
			lines[fset.Position(pos).Line] = true
		}
	}
}

func codeLinesInRange(lines map[int]bool, start, end int) int {
	if start < 1 {
		start = 1
	}
	if end < start {
		return 0
	}
	count := 0
	for line := start; line <= end; line++ {
		if lines[line] {
			count++
		}
	}
	return count
}

func splitLines(data []byte) []string {
	data = bytes.TrimSuffix(data, []byte("\n"))
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte("\n"))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSuffix(string(part), "\r"))
	}
	return out
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".worktrees", "node_modules", "vendor", "dist", "bin":
		return true
	default:
		return false
	}
}

func isProductionGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func isGenerated(data []byte) bool {
	return bytes.Contains(data, []byte("Code generated")) && bytes.Contains(data, []byte("DO NOT EDIT"))
}

func sortedViolations(violations []Violation) []Violation {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		if violations[i].Line != violations[j].Line {
			return violations[i].Line < violations[j].Line
		}
		if violations[i].Func != violations[j].Func {
			return violations[i].Func < violations[j].Func
		}
		return violations[i].Kind < violations[j].Kind
	})
	return violations
}

func stripArgSeparator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

func cleanRoot(root string) string {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return abs
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func displayPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func reportStrictResult(stdout, stderr io.Writer, violations []Violation) int {
	if len(violations) == 0 {
		fmt.Fprintln(stdout, "code_guard: strict passed")
		return 0
	}
	fmt.Fprintf(stderr, "code_guard: strict found %d violation(s)\n", len(violations))
	printViolations(stderr, violations)
	return 1
}

func printCompareFailures(w io.Writer, result CompareResult) {
	if len(result.New) > 0 {
		fmt.Fprintf(w, "code_guard: new violation(s) (%d)\n", len(result.New))
		printViolations(w, result.New)
	}
	if len(result.Worse) > 0 {
		fmt.Fprintf(w, "code_guard: worsened baseline violation(s) (%d)\n", len(result.Worse))
		printViolations(w, result.Worse)
	}
}

func printViolations(w io.Writer, violations []Violation) {
	for _, v := range sortedViolations(violations) {
		fmt.Fprintf(w, "  - %s\n", v.Message)
	}
}
