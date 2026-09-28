package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"kwu/internal/evidence"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

// workspaceArtifactObservationOptIn lets an audited first-party opaque writer
// request before/after observation. Unknown and remote tools do not implement
// it and therefore fail closed.
type workspaceArtifactObservationOptIn interface {
	ObserveWorkspaceArtifactChanges(json.RawMessage) bool
}

func workspaceArtifactObservationAllowed(candidate tool.Tool, args json.RawMessage) bool {
	if candidate == nil || candidate.ReadOnly() {
		return false
	}
	switch candidate.Name() {
	case "write_file", "edit_file", "multi_edit", "move_file", "notebook_edit", "delete_range", "delete_symbol":
		return true
	case "bash":
		var params struct {
			RunInBackground             bool `json:"run_in_background"`
			PreserveBackgroundProcesses bool `json:"preserve_background_processes"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return false
		}
		return !params.RunInBackground && !params.PreserveBackgroundProcesses
	default:
		optIn, ok := candidate.(workspaceArtifactObservationOptIn)
		return ok && optIn.ObserveWorkspaceArtifactChanges(args)
	}
}

func workspaceArtifactExactWriterName(name string) bool {
	switch name {
	case "write_file", "edit_file", "multi_edit", "move_file", "notebook_edit", "delete_range", "delete_symbol":
		return true
	default:
		return false
	}
}

const (
	workspaceArtifactScanEntryLimit     = 50_000
	workspaceArtifactScanCandidateLimit = 4_096
	workspaceArtifactReceiptLimit       = 32
	workspaceArtifactHashSampleSize     = int64(64 << 10)
)

// WorkspaceArtifactObservation binds filesystem evidence to individual tool
// calls. It never scans at turn start: exact path-bearing writers stat only
// their declared targets, while opaque writers take a filtered before/after
// snapshot for the duration of that call.
type WorkspaceArtifactObservation struct {
	root                string
	realRoot            string
	valid               bool
	previouslyDelivered map[string]bool
}

type workspaceArtifactObservationContextKey struct{}
type workspaceArtifactOpaqueScanDisabledContextKey struct{}

type workspaceArtifactFileState struct {
	size        int64
	fingerprint [sha256.Size]byte
}

type workspaceArtifactSnapshotFile struct {
	path     string
	relative string
	state    workspaceArtifactFileState
}

type workspaceArtifactToolCall struct {
	ctx         context.Context
	observation *WorkspaceArtifactObservation
	exact       []string
	before      map[string]workspaceArtifactSnapshotFile
	source      string
}

type workspaceArtifactSnapshotScan struct {
	ctx         context.Context
	observation *WorkspaceArtifactObservation
	files       map[string]workspaceArtifactSnapshotFile
	entries     int
	candidates  int
}

// NewWorkspaceArtifactObservation resolves the owning workspace once. The
// optional paths are files delivered by earlier turns; changing one of those is
// itself a new delivery even when its format is otherwise source-like.
func NewWorkspaceArtifactObservation(root string, previouslyDelivered ...string) *WorkspaceArtifactObservation {
	observation := &WorkspaceArtifactObservation{
		root:                strings.TrimSpace(root),
		previouslyDelivered: make(map[string]bool, len(previouslyDelivered)),
	}
	if observation.root == "" {
		return observation
	}
	rootAbs, err := filepath.Abs(observation.root)
	if err != nil {
		return observation
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return observation
	}
	info, err := os.Stat(realRoot)
	if err != nil || !info.IsDir() {
		return observation
	}
	observation.root = rootAbs
	observation.realRoot = realRoot
	observation.valid = true
	for _, path := range previouslyDelivered {
		if relative, ok := observation.relativeExistingFile(path); ok {
			observation.previouslyDelivered[workspaceArtifactSnapshotKey(relative)] = true
		}
	}
	return observation
}

// WithWorkspaceArtifactObservation shares a top-level turn's immutable
// workspace boundary with root and child agents.
func WithWorkspaceArtifactObservation(ctx context.Context, observation *WorkspaceArtifactObservation) context.Context {
	if ctx == nil || observation == nil {
		return ctx
	}
	return context.WithValue(ctx, workspaceArtifactObservationContextKey{}, observation)
}

func ensureWorkspaceArtifactObservation(ctx context.Context, root string) context.Context {
	if ctx == nil {
		return ctx
	}
	if existing, _ := ctx.Value(workspaceArtifactObservationContextKey{}).(*WorkspaceArtifactObservation); existing != nil {
		return ctx
	}
	return WithWorkspaceArtifactObservation(ctx, NewWorkspaceArtifactObservation(root))
}

func withoutOpaqueWorkspaceArtifactObservation(ctx context.Context) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, workspaceArtifactOpaqueScanDisabledContextKey{}, true)
}

func beginWorkspaceArtifactToolCall(ctx context.Context, toolName string, args json.RawMessage, readOnly bool) *workspaceArtifactToolCall {
	if ctx == nil || readOnly {
		return nil
	}
	observation, _ := ctx.Value(workspaceArtifactObservationContextKey{}).(*WorkspaceArtifactObservation)
	if observation == nil || !observation.valid {
		return nil
	}
	call := &workspaceArtifactToolCall{ctx: ctx, observation: observation}
	if workspaceArtifactExactWriterName(toolName) {
		receipt := evidence.ReceiptFromToolCall(toolName, args, true, false)
		if len(receipt.Paths) == 0 {
			return nil
		}
		call.exact = append([]string(nil), receipt.Paths...)
		call.before = observation.snapshotExactPaths(call.exact)
		call.source = SubagentArtifactSourceWriter
		return call
	}
	if disabled, _ := ctx.Value(workspaceArtifactOpaqueScanDisabledContextKey{}).(bool); disabled {
		return nil
	}
	before, ok := observation.snapshotCandidateFiles(ctx)
	if !ok {
		return nil
	}
	call.before = before
	call.source = SubagentArtifactSourceSnapshot
	return call
}

func (call *workspaceArtifactToolCall) complete(success bool) []provider.ToolArtifact {
	if call == nil || call.observation == nil || !success {
		return nil
	}
	candidates, ok := call.changedCandidates()
	if !ok {
		return nil
	}
	return call.artifactReceipts(candidates)
}

func (call *workspaceArtifactToolCall) changedCandidates() ([]workspaceArtifactSnapshotFile, bool) {
	if len(call.exact) > 0 {
		return call.changedExactCandidates(), true
	}
	return call.changedSnapshotCandidates()
}

func (call *workspaceArtifactToolCall) changedExactCandidates() []workspaceArtifactSnapshotFile {
	var candidates []workspaceArtifactSnapshotFile
	for _, raw := range call.exact {
		file, ok := call.observation.existingFile(raw)
		if !ok || !call.isDeliveryChange(file, workspaceArtifactSnapshotKey(file.path)) {
			continue
		}
		candidates = append(candidates, file)
	}
	return candidates
}

func (call *workspaceArtifactToolCall) changedSnapshotCandidates() ([]workspaceArtifactSnapshotFile, bool) {
	after, ok := call.observation.snapshotCandidateFiles(call.ctx)
	if !ok {
		return nil, false
	}
	var candidates []workspaceArtifactSnapshotFile
	for key, file := range after {
		if call.isDeliveryChange(file, key) {
			candidates = append(candidates, file)
		}
	}
	return candidates, true
}

func (call *workspaceArtifactToolCall) isDeliveryChange(file workspaceArtifactSnapshotFile, key string) bool {
	previous, existed := call.before[key]
	return (!existed || previous.state != file.state) && call.observation.deliveryCandidate(file.relative, existed, call.source)
}

func (call *workspaceArtifactToolCall) artifactReceipts(candidates []workspaceArtifactSnapshotFile) []provider.ToolArtifact {
	sort.Slice(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].relative) < strings.ToLower(candidates[j].relative)
	})
	artifacts := make([]provider.ToolArtifact, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		key := workspaceArtifactSnapshotKey(candidate.path)
		if seen[key] {
			continue
		}
		seen[key] = true
		artifacts = append(artifacts, provider.ToolArtifact{Path: candidate.path, Source: call.source})
		if len(artifacts) == workspaceArtifactReceiptLimit {
			break
		}
	}
	return artifacts
}

func (o *WorkspaceArtifactObservation) snapshotExactPaths(paths []string) map[string]workspaceArtifactSnapshotFile {
	files := make(map[string]workspaceArtifactSnapshotFile, len(paths))
	for _, raw := range paths {
		file, ok := o.existingFile(raw)
		if ok {
			files[workspaceArtifactSnapshotKey(file.path)] = file
		}
	}
	return files
}

func (o *WorkspaceArtifactObservation) snapshotCandidateFiles(ctx context.Context) (map[string]workspaceArtifactSnapshotFile, bool) {
	if o == nil || !o.valid {
		return nil, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	scan := &workspaceArtifactSnapshotScan{
		ctx:         ctx,
		observation: o,
		files:       make(map[string]workspaceArtifactSnapshotFile),
	}
	err := filepath.WalkDir(o.realRoot, scan.visit)
	return scan.files, err == nil
}

func (scan *workspaceArtifactSnapshotScan) visit(path string, entry fs.DirEntry, walkErr error) error {
	if err := scan.enter(walkErr); err != nil {
		return err
	}
	if entry.IsDir() {
		return scan.visitDirectory(path, entry)
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return nil
	}
	info, err := entry.Info()
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	return scan.addCandidate(path, info)
}

func (scan *workspaceArtifactSnapshotScan) enter(walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if err := scan.ctx.Err(); err != nil {
		return err
	}
	scan.entries++
	if scan.entries > workspaceArtifactScanEntryLimit {
		return fmt.Errorf("workspace artifact scan exceeded %d entries", workspaceArtifactScanEntryLimit)
	}
	return nil
}

func (scan *workspaceArtifactSnapshotScan) visitDirectory(path string, entry fs.DirEntry) error {
	if path == scan.observation.realRoot {
		return nil
	}
	relative, err := filepath.Rel(scan.observation.realRoot, path)
	if err != nil {
		return err
	}
	if !workspaceArtifactCustomerDeliveryDirectory(relative) && workspaceArtifactSnapshotSkippedDir(entry.Name()) {
		return filepath.SkipDir
	}
	return nil
}

func (scan *workspaceArtifactSnapshotScan) addCandidate(path string, info fs.FileInfo) error {
	relative, err := workspaceArtifactCandidateRelative(scan.observation.realRoot, path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	if !IsAutomaticArtifactDeliveryPath(relative) {
		return nil
	}
	if err := scan.countCandidate(); err != nil {
		return err
	}
	state, err := workspaceArtifactState(scan.ctx, path, info)
	if err != nil {
		return err
	}
	scan.files[workspaceArtifactSnapshotKey(path)] = workspaceArtifactSnapshotFile{path: path, relative: relative, state: state}
	return nil
}

func (scan *workspaceArtifactSnapshotScan) countCandidate() error {
	scan.candidates++
	if scan.candidates > workspaceArtifactScanCandidateLimit {
		return fmt.Errorf("workspace artifact scan exceeded %d candidates", workspaceArtifactScanCandidateLimit)
	}
	return nil
}

func workspaceArtifactCandidateRelative(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == "." || filepath.IsAbs(relative) {
		return "", fmt.Errorf("invalid workspace artifact path %q", path)
	}
	return relative, nil
}

func (o *WorkspaceArtifactObservation) lexicalCandidate(raw string) (string, bool) {
	if o == nil || !o.valid {
		return "", false
	}
	path := filepath.FromSlash(strings.TrimSpace(raw))
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(o.root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil || !workspaceArtifactPathWithinRoot(o.root, path) {
		return "", false
	}
	return path, true
}

func (o *WorkspaceArtifactObservation) existingFile(raw string) (workspaceArtifactSnapshotFile, bool) {
	path, ok := o.lexicalCandidate(raw)
	if !ok {
		return workspaceArtifactSnapshotFile{}, false
	}
	realPath, relative, info, ok := o.canonicalExistingFile(path)
	if !ok {
		return workspaceArtifactSnapshotFile{}, false
	}
	state, err := workspaceArtifactState(context.Background(), realPath, info)
	if err != nil {
		return workspaceArtifactSnapshotFile{}, false
	}
	return workspaceArtifactSnapshotFile{path: realPath, relative: filepath.ToSlash(relative), state: state}, true
}

func (o *WorkspaceArtifactObservation) canonicalExistingFile(path string) (string, string, fs.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", nil, false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil || !workspaceArtifactPathWithinRoot(o.realRoot, realPath) {
		return "", "", nil, false
	}
	relative, err := workspaceArtifactCandidateRelative(o.realRoot, realPath)
	if err != nil {
		return "", "", nil, false
	}
	info, err = os.Stat(realPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", nil, false
	}
	return realPath, relative, info, true
}

func workspaceArtifactState(ctx context.Context, path string, info fs.FileInfo) (workspaceArtifactFileState, error) {
	state := workspaceArtifactFileState{size: info.Size()}
	file, err := os.Open(path)
	if err != nil {
		return state, err
	}
	defer file.Close()

	fingerprint, err := workspaceArtifactFingerprint(ctx, file, path, info)
	if err != nil {
		return state, err
	}
	if err := workspaceArtifactVerifyUnchanged(path, info); err != nil {
		return state, err
	}
	copy(state.fingerprint[:], fingerprint)
	return state, nil
}

func workspaceArtifactFingerprint(ctx context.Context, file *os.File, path string, info fs.FileInfo) ([]byte, error) {
	digest := sha256.New()
	var err error
	if info.Size() <= workspaceArtifactHashSampleSize*3 {
		err = writeFullWorkspaceArtifactFingerprint(digest, file, path, info.Size())
	} else {
		err = writeSampledWorkspaceArtifactFingerprint(ctx, digest, file, info.Size())
	}
	if err != nil {
		return nil, err
	}
	return digest.Sum(nil), nil
}

func writeFullWorkspaceArtifactFingerprint(dst io.Writer, file *os.File, path string, size int64) error {
	copied, err := io.Copy(dst, io.LimitReader(file, size+1))
	if err != nil {
		return err
	}
	if copied != size {
		return fmt.Errorf("workspace artifact changed while fingerprinting %q", path)
	}
	return nil
}

func writeSampledWorkspaceArtifactFingerprint(ctx context.Context, dst io.Writer, file *os.File, size int64) error {
	offsets := []int64{0, size/2 - workspaceArtifactHashSampleSize/2, size - workspaceArtifactHashSampleSize}
	buffer := make([]byte, workspaceArtifactHashSampleSize)
	for _, offset := range offsets {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := file.ReadAt(buffer, offset)
		if err != nil && err != io.EOF {
			return err
		}
		_, _ = fmt.Fprintf(dst, "%d:%d:", offset, n)
		_, _ = dst.Write(buffer[:n])
	}
	return nil
}

func workspaceArtifactVerifyUnchanged(path string, before fs.FileInfo) error {
	after, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("workspace artifact changed while fingerprinting %q", path)
	}
	if after.Size() != before.Size() || after.ModTime() != before.ModTime() {
		return fmt.Errorf("workspace artifact changed while fingerprinting %q", path)
	}
	return nil
}

func (o *WorkspaceArtifactObservation) relativeExistingFile(raw string) (string, bool) {
	file, ok := o.existingFile(raw)
	return file.relative, ok
}

func (o *WorkspaceArtifactObservation) deliveryCandidate(relative string, existed bool, source string) bool {
	if !workspaceArtifactDeliveryPathAllowed(relative, source) {
		return false
	}
	key := workspaceArtifactSnapshotKey(filepath.ToSlash(relative))
	if o.previouslyDelivered[key] || explicitCustomerDeliveryPath(relative) || !existed {
		return true
	}
	switch strings.ToLower(filepath.Ext(relative)) {
	case ".pdf", ".doc", ".docx", ".rtf", ".xls", ".xlsx", ".xlsm", ".csv", ".tsv", ".ppt", ".pptx",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".ico",
		".mp4", ".mov", ".mkv", ".avi", ".webm", ".mp3", ".wav", ".m4a", ".aac", ".flac", ".ogg",
		".zip", ".7z", ".tar", ".gz", ".tgz":
		return true
	default:
		return false
	}
}

func workspaceArtifactDeliveryPathAllowed(path, source string) bool {
	switch source {
	case SubagentArtifactSourceWriter, SubagentArtifactSourceManifest:
		return IsExplicitArtifactDeliveryPath(path)
	case SubagentArtifactSourceSnapshot:
		return IsAutomaticArtifactDeliveryPath(path)
	default:
		return false
	}
}

// IsExplicitArtifactDeliveryPath applies the automatic-candidate policy for
// exact writer receipts. Unlike opaque workspace observation, it does not use
// an extension allowlist. Explicit deliver_files declarations are validated by
// the tool and do not pass through this heuristic filter.
func IsExplicitArtifactDeliveryPath(path string) bool {
	path = filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if path == "" || path == "." || path == ".." || filepath.IsAbs(path) ||
		strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return false
	}
	return !workspaceArtifactExcludedPath(path)
}

// IsAutomaticArtifactDeliveryPath applies the automatic-delivery policy to a
// workspace-relative path. Callers must resolve absolute paths against their
// owning workspace before using it so hidden parent directories do not suppress
// valid files inside the workspace.
func IsAutomaticArtifactDeliveryPath(path string) bool {
	return IsExplicitArtifactDeliveryPath(path) && automaticArtifactDeliveryPath(path)
}

func explicitCustomerDeliveryPath(path string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))), "/")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if part == ".customer-delivery" {
			return true
		}
	}
	return false
}

func workspaceArtifactCustomerDeliveryDirectory(path string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))), "/")
	for _, part := range parts {
		if part == ".customer-delivery" {
			return true
		}
	}
	return false
}

func workspaceArtifactExcludedPath(path string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))), "/")
	if len(parts) == 0 {
		return true
	}
	for _, part := range parts[:max(0, len(parts)-1)] {
		if workspaceArtifactExplicitExcludedDir(part) {
			return true
		}
	}
	return workspaceArtifactEngineeringFileName(parts[len(parts)-1])
}

func workspaceArtifactExplicitExcludedDir(name string) bool {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, ".customer-delivery") {
		return false
	}
	if len(name) > 1 && strings.HasPrefix(name, ".") && name != ".." {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "__pycache__", "dist", "build", "coverage", "target", "bin", "obj",
		"src", "internal", "cmd", "pkg", "app", "frontend", "backend", "scripts",
		"test", "tests", "testdata", "fixtures", "logs", "log", "tmp", "temp":
		return true
	default:
		return false
	}
}

func workspaceArtifactEngineeringFileName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) > 1 && strings.HasPrefix(name, ".") {
		return true
	}
	for _, stem := range []string{
		"readme", "changelog", "contributing", "code_of_conduct", "security",
		"license", "notice", "authors", "agents", "claude",
	} {
		if name == stem || strings.HasPrefix(name, stem+".") {
			return true
		}
	}
	switch name {
	case "cmakelists.txt", "requirements.txt", "constraints.txt",
		"package.json", "package-lock.json", "pnpm-lock.yaml", "pnpm-lock.yml", "yarn.lock",
		"go.mod", "go.sum", "cargo.toml", "cargo.lock", "pyproject.toml", "composer.json",
		"dockerfile", "makefile", "lcov.info", "coverage.out", "coverage.json", "jacoco.xml":
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".log", ".trace",
		".go", ".py", ".pyi", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx",
		".java", ".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx",
		".rs", ".rb", ".php", ".swift", ".kt", ".kts", ".cs", ".fs", ".fsx",
		".scala", ".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd",
		".vue", ".svelte", ".css", ".scss", ".sass", ".less":
		return true
	default:
		return false
	}
}

func workspaceArtifactSnapshotSkippedDir(name string) bool {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, ".customer-delivery") {
		return false
	}
	if len(name) > 1 && strings.HasPrefix(name, ".") && name != ".." {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "__pycache__", "dist", "build", "coverage", "target", "bin", "obj",
		"src", "internal", "cmd", "pkg", "app", "frontend", "backend", "scripts",
		"docs", "documentation", "test", "tests", "testdata", "fixtures", "examples",
		"assets", "public", "static", "logs", "log", "tmp", "temp":
		return true
	default:
		return false
	}
}

func workspaceArtifactSnapshotKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

func workspaceArtifactPathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
