package automation

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"kwu/internal/fileutil"
)

const (
	runsDirName        = "runs"
	runTrashDirName    = "run-trash"
	manifestFileName   = "manifest.json"
	eventsFileName     = "events.jsonl"
	sessionFileName    = "session.jsonl"
	resultFileName     = "result.md"
	deliveriesFileName = "deliveries.jsonl"
	assistFileName     = "assist.md"
)

var runSeq atomic.Uint64

// RunPaths contains the stable file layout for one archived automation run.
type RunPaths struct {
	Dir        string
	Manifest   string
	Events     string
	Session    string
	Result     string
	Deliveries string
}

// RunSummary is a compact manifest projection for run listings.
type RunSummary struct {
	RunID           string       `json:"runId"`
	TaskID          string       `json:"taskId"`
	TaskName        string       `json:"taskName"`
	DisplayName     string       `json:"displayName,omitempty"`
	Scope           string       `json:"scope,omitempty"`
	ProjectID       string       `json:"projectId,omitempty"`
	Status          RunStatus    `json:"status"`
	Trigger         Trigger      `json:"trigger"`
	TriggerReason   string       `json:"triggerReason,omitempty"`
	SkipReason      string       `json:"skipReason,omitempty"`
	ConditionReason string       `json:"conditionReason,omitempty"`
	Owner           string       `json:"owner,omitempty"`
	Model           string       `json:"model,omitempty"`
	BillingSource   string       `json:"billingSource,omitempty"`
	WorkspaceRoot   string       `json:"workspaceRoot,omitempty"`
	StartedAt       time.Time    `json:"startedAt"`
	FinishedAt      time.Time    `json:"finishedAt,omitempty"`
	DurationMs      int64        `json:"durationMs,omitempty"`
	ResultPath      string       `json:"resultPath,omitempty"`
	Summary         string       `json:"summary,omitempty"`
	ErrorCode       RunErrorCode `json:"errorCode,omitempty"`
	ErrorMessage    string       `json:"errorMessage,omitempty"`
	Error           string       `json:"error,omitempty"`
}

// RunPaths returns safe archive paths for taskID/runID under the Store root.
func (s *Store) RunPaths(taskID, runID string) (RunPaths, error) {
	taskPart, err := pathID(taskID)
	if err != nil {
		return RunPaths{}, fmt.Errorf("task id: %w", err)
	}
	runPart, err := pathID(runID)
	if err != nil {
		return RunPaths{}, fmt.Errorf("run id: %w", err)
	}
	dir := filepath.Join(s.dir, runsDirName, taskPart, runPart)
	if err := ensureUnderRoot(s.dir, dir); err != nil {
		return RunPaths{}, err
	}
	return RunPaths{
		Dir:        dir,
		Manifest:   filepath.Join(dir, manifestFileName),
		Events:     filepath.Join(dir, eventsFileName),
		Session:    filepath.Join(dir, sessionFileName),
		Result:     filepath.Join(dir, resultFileName),
		Deliveries: filepath.Join(dir, deliveriesFileName),
	}, nil
}

// ListDeliveries returns the latest delivery state for one archived run.
func (s *Store) ListDeliveries(taskID, runID string) ([]DeliveryAttempt, error) {
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return nil, err
	}
	store, err := OpenDeliveryStore(paths.Deliveries)
	if err != nil {
		return nil, err
	}
	return store.List(runID), nil
}

// CreateRun creates a run directory and writes its initial queued manifest.
func (s *Store) CreateRun(task Task, trigger string, owner string) (RunManifest, RunPaths, error) {
	now := time.Now()
	return s.createRun(task, trigger, "", owner, newRunID(now), now.UTC())
}

func (s *Store) createRun(task Task, trigger, triggerReason, owner, runID string, startedAt time.Time) (RunManifest, RunPaths, error) {
	normalized, err := NormalizeTask(task)
	if err != nil {
		return RunManifest{}, RunPaths{}, err
	}
	tr, err := normalizeTrigger(trigger)
	if err != nil {
		return RunManifest{}, RunPaths{}, err
	}
	paths, err := s.RunPaths(normalized.ID, runID)
	if err != nil {
		return RunManifest{}, RunPaths{}, err
	}
	if err := os.MkdirAll(paths.Dir, 0o755); err != nil {
		return RunManifest{}, RunPaths{}, fmt.Errorf("create run dir: %w", err)
	}
	manifest := RunManifest{
		RunID:         runID,
		TaskID:        normalized.ID,
		TaskName:      normalized.Name,
		TaskRevision:  normalized.Revision,
		Scope:         normalized.Scope,
		ProjectID:     normalized.ProjectID,
		Status:        RunStatusQueued,
		Trigger:       tr,
		TriggerReason: defaultTriggerReason(tr, triggerReason),
		Owner:         strings.TrimSpace(owner),
		HostOwner:     strings.TrimSpace(owner),
		LeaseID:       runID,
		StartedAt:     startedAt.UTC(),
		Model:         normalized.Prompt.Model,
		BillingSource: normalized.Prompt.BillingSource,
		Capabilities: CapabilitySnapshot{
			ModelRef:     normalized.Capabilities.ModelRef,
			ApprovalMode: normalized.Execution.ApprovalMode,
			SkillIDs:     append([]string(nil), normalized.Capabilities.SkillIDs...),
			ExpertID:     normalized.Capabilities.ExpertID,
			ConnectorIDs: append([]string(nil), normalized.Capabilities.ConnectorIDs...),
			CapturedAt:   startedAt.UTC(),
		},
		Delivery:       normalized.Delivery,
		WorkspaceRoot:  normalized.WorkspaceRoot,
		Schedule:       normalized.Schedule,
		EffectiveRange: normalized.EffectiveRange,
		SessionPath:    paths.Session,
		EventsPath:     paths.Events,
		ResultPath:     paths.Result,
	}
	if err := s.SaveRunManifest(manifest); err != nil {
		return RunManifest{}, RunPaths{}, err
	}
	return manifest, paths, nil
}

// SaveRunManifest atomically writes manifest.json for its task/run IDs.
func (s *Store) SaveRunManifest(manifest RunManifest) error {
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	if paths, err := s.RunPaths(manifest.TaskID, manifest.RunID); err == nil {
		if current, readErr := readRunManifest(paths.Manifest); readErr == nil {
			manifest.DisplayName = current.DisplayName
		}
	}
	return s.saveRunManifestLocked(manifest)
}

func (s *Store) saveRunManifestLocked(manifest RunManifest) error {
	indexNeedsRebuild, err := s.runsIndexNeedsRebuildLocked()
	if err != nil {
		return err
	}
	paths, err := s.RunPaths(manifest.TaskID, manifest.RunID)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("save run manifest: encode: %w", err)
	}
	b = append(b, '\n')
	if err := s.markRunsIndexDirtyLocked(); err != nil {
		return err
	}
	if err := fileutil.AtomicWriteFile(paths.Manifest, b, 0o600); err != nil {
		return fmt.Errorf("save run manifest: %w", err)
	}
	// The manifest is authoritative. If the disposable index cannot be updated,
	// leave its dirty marker behind so the next read rebuilds it.
	if indexNeedsRebuild {
		err = s.rebuildRunsIndexLocked(context.Background())
	} else {
		err = s.updateRunsIndexLocked(context.Background(), runSummary(manifest))
	}
	if err != nil {
		return nil
	}
	if err := os.Remove(s.runsIndexDirtyPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear automation run index marker: %w", err)
	}
	return nil
}

// RenameRun changes only the user-facing name of a run. TaskName remains
// the immutable task snapshot captured when the run was created.
func (s *Store) RenameRun(taskID, runID, displayName string) (RunManifest, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 120 {
		return RunManifest{}, errors.New("automation: run display name must contain 1 to 120 characters")
	}
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	manifest, err := s.ReadRunManifest(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	manifest.DisplayName = displayName
	if err := s.saveRunManifestLocked(manifest); err != nil {
		return RunManifest{}, err
	}
	return manifest, nil
}

// TrashRun atomically moves every artifact for a completed run into run-trash.
func (s *Store) TrashRun(taskID, runID string) (RunManifest, error) {
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	manifest, err := readRunManifest(paths.Manifest)
	if err != nil {
		return RunManifest{}, err
	}
	if !terminalRunStatus(manifest.Status) {
		return RunManifest{}, errors.New("automation: active runs must be canceled before deletion")
	}
	trashDir, err := s.runTrashPath(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	if err := requireMissingPath(trashDir, "automation: run trash destination already exists"); err != nil {
		return RunManifest{}, err
	}
	if err := os.MkdirAll(filepath.Dir(trashDir), 0o755); err != nil {
		return RunManifest{}, fmt.Errorf("create run trash dir: %w", err)
	}
	if err := s.markRunsIndexDirtyLocked(); err != nil {
		return RunManifest{}, err
	}
	if err := os.Rename(paths.Dir, trashDir); err != nil {
		return RunManifest{}, fmt.Errorf("trash automation run: %w", err)
	}
	if err := s.rebuildRunsIndexLocked(context.Background()); err != nil {
		return manifest, nil
	}
	return manifest, nil
}

// RestoreRun moves a previously trashed run back to the authoritative runs tree.
func (s *Store) RestoreRun(taskID, runID string) (RunManifest, error) {
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	trashDir, err := s.runTrashPath(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	manifest, err := readRunManifest(filepath.Join(trashDir, manifestFileName))
	if err != nil {
		return RunManifest{}, err
	}
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	if err := requireMissingPath(paths.Dir, "automation: run restore destination already exists"); err != nil {
		return RunManifest{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Dir), 0o755); err != nil {
		return RunManifest{}, fmt.Errorf("create run restore dir: %w", err)
	}
	if err := s.markRunsIndexDirtyLocked(); err != nil {
		return RunManifest{}, err
	}
	if err := os.Rename(trashDir, paths.Dir); err != nil {
		return RunManifest{}, fmt.Errorf("restore automation run: %w", err)
	}
	if err := s.rebuildRunsIndexLocked(context.Background()); err != nil {
		return manifest, nil
	}
	return manifest, nil
}

func requireMissingPath(path, conflictMessage string) error {
	_, err := os.Stat(path)
	if err == nil {
		return errors.New(conflictMessage)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) ReadTrashedRunManifest(taskID, runID string) (RunManifest, error) {
	trashDir, err := s.runTrashPath(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	return readRunManifest(filepath.Join(trashDir, manifestFileName))
}

func (s *Store) runTrashPath(taskID, runID string) (string, error) {
	taskPart, err := pathID(taskID)
	if err != nil {
		return "", fmt.Errorf("task id: %w", err)
	}
	runPart, err := pathID(runID)
	if err != nil {
		return "", fmt.Errorf("run id: %w", err)
	}
	dir := filepath.Join(s.dir, runTrashDirName, taskPart, runPart)
	if err := ensureUnderRoot(s.dir, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func terminalRunStatus(status RunStatus) bool {
	switch status {
	case RunStatusSkipped, RunStatusSucceeded, RunStatusFailed, RunStatusBlocked, RunStatusCanceled:
		return true
	default:
		return false
	}
}

// ReadRunManifest reads manifest.json for one task run.
func (s *Store) ReadRunManifest(taskID, runID string) (RunManifest, error) {
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return RunManifest{}, err
	}
	return readRunManifest(paths.Manifest)
}

// ListRuns scans archived run directories and returns newest runs first.
func (s *Store) ListRuns(taskID string, limit int) ([]RunSummary, error) {
	runs, err := s.loadRuns(RunFilter{TaskID: taskID})
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// ListRunsPage returns a stable newest-first page across one task or all tasks.
func (s *Store) ListRunsPage(filter RunFilter, cursor string) (RunPage, error) {
	page, _, err := s.indexedRunPage(context.Background(), filter, cursor, nil)
	return page, err
}

func (s *Store) loadRuns(filter RunFilter) ([]RunSummary, error) {
	root := filepath.Join(s.dir, runsDirName)
	dirs, err := runTaskDirectories(root, filter.TaskID)
	if err != nil {
		return nil, err
	}
	runs := make([]RunSummary, 0)
	for _, dir := range dirs {
		loaded, err := loadRunDirectory(dir, filter)
		if err != nil {
			return nil, err
		}
		runs = append(runs, loaded...)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].StartedAt.Equal(runs[j].StartedAt) {
			if runs[i].RunID == runs[j].RunID {
				return runs[i].TaskID > runs[j].TaskID
			}
			return runs[i].RunID > runs[j].RunID
		}
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}

func runTaskDirectories(root, taskID string) ([]string, error) {
	if taskID = strings.TrimSpace(taskID); taskID != "" {
		taskPart, err := pathID(taskID)
		if err != nil {
			return nil, fmt.Errorf("task id: %w", err)
		}
		return []string{filepath.Join(root, taskPart)}, nil
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list run tasks: %w", err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	return dirs, nil
}

func loadRunDirectory(dir string, filter RunFilter) ([]RunSummary, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []RunSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	runs := make([]RunSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readRunManifest(filepath.Join(dir, entry.Name(), manifestFileName))
		if err != nil {
			return nil, err
		}
		summary := runSummary(manifest)
		if !runMatchesFilter(summary, filter) {
			continue
		}
		runs = append(runs, summary)
	}
	return runs, nil
}

func runMatchesFilter(summary RunSummary, filter RunFilter) bool {
	if !runMatchesScope(summary, filter) {
		return false
	}
	if filter.Trigger != "" && summary.Trigger != filter.Trigger {
		return false
	}
	return filter.Search == "" || strings.Contains(strings.ToLower(summary.TaskName), filter.Search)
}

func runMatchesScope(summary RunSummary, filter RunFilter) bool {
	return filter.Scope == "" || strings.EqualFold(summary.Scope, strings.TrimSpace(filter.Scope))
}

type runCursor struct {
	Version    int       `json:"version"`
	FilterHash string    `json:"filterHash"`
	StartedAt  time.Time `json:"startedAt"`
	RunID      string    `json:"runId"`
	TaskID     string    `json:"taskId"`
}

func runPageStart(runs []RunSummary, encoded string, filter RunFilter) (int, error) {
	if strings.TrimSpace(encoded) == "" {
		return 0, nil
	}
	cursor, err := decodeRunCursor(encoded, filter)
	if err != nil {
		return 0, err
	}
	for index, run := range runs {
		if run.StartedAt.Before(cursor.StartedAt) ||
			(run.StartedAt.Equal(cursor.StartedAt) && run.RunID < cursor.RunID) ||
			(run.StartedAt.Equal(cursor.StartedAt) && run.RunID == cursor.RunID && run.TaskID < cursor.TaskID) {
			return index, nil
		}
	}
	return len(runs), nil
}

func encodeRunCursor(run RunSummary, filter RunFilter) (string, error) {
	payload, err := json.Marshal(runCursor{Version: 1, FilterHash: runFilterHash(filter), StartedAt: run.StartedAt, RunID: run.RunID, TaskID: run.TaskID})
	if err != nil {
		return "", fmt.Errorf("encode run cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeRunCursor(encoded string, filter RunFilter) (runCursor, error) {
	encoded = strings.TrimSpace(encoded)
	if len(encoded) > 2048 {
		return runCursor{}, fmt.Errorf("automation: invalid run cursor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return runCursor{}, fmt.Errorf("automation: invalid run cursor")
	}
	var cursor runCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Version != 1 || cursor.FilterHash != runFilterHash(filter) || cursor.RunID == "" || cursor.TaskID == "" || cursor.StartedAt.IsZero() {
		return runCursor{}, fmt.Errorf("automation: invalid run cursor")
	}
	return cursor, nil
}

func runFilterHash(filter RunFilter) string {
	limit, _ := normalizedRunPageLimit(filter.Limit)
	canonical := strings.ToLower(strings.TrimSpace(filter.Scope)) + "\x00" +
		strings.TrimSpace(filter.TaskID) + "\x00" +
		strings.ToLower(strings.TrimSpace(string(filter.Trigger))) + "\x00" +
		strings.ToLower(strings.TrimSpace(filter.Search)) + "\x00" + strconv.Itoa(limit)
	digest := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("sha256:%x", digest)
}

func normalizeRunFilter(filter RunFilter) (RunFilter, error) {
	filter.TaskID = strings.TrimSpace(filter.TaskID)
	filter.Scope = strings.ToLower(strings.TrimSpace(filter.Scope))
	filter.Search = strings.ToLower(strings.TrimSpace(filter.Search))
	if len(filter.Search) > 256 || strings.ContainsRune(filter.Search, '\x00') {
		return RunFilter{}, fmt.Errorf("automation: run search must be at most 256 bytes")
	}
	trigger := strings.ToLower(strings.TrimSpace(string(filter.Trigger)))
	if trigger == "all" {
		trigger = ""
	}
	filter.Trigger = Trigger(trigger)
	if filter.Trigger != "" && filter.Trigger != TriggerSchedule && filter.Trigger != TriggerManual {
		return RunFilter{}, fmt.Errorf("automation: invalid run trigger %q", trigger)
	}
	limit, err := normalizedRunPageLimit(filter.Limit)
	if err != nil {
		return RunFilter{}, err
	}
	filter.Limit = limit
	return filter, nil
}

// WriteResult atomically writes result.md for one task run.
func (s *Store) WriteResult(taskID, runID, body string) error {
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicWriteFile(paths.Result, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write run result: %w", err)
	}
	return nil
}

// ReadResult reads result.md for one task run.
func (s *Store) ReadResult(taskID, runID string) (string, error) {
	paths, err := s.RunPaths(taskID, runID)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(paths.Result)
	if err != nil {
		return "", fmt.Errorf("read run result: %w", err)
	}
	return string(b), nil
}

func readRunManifest(path string) (RunManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return RunManifest{}, fmt.Errorf("read run manifest: %w", err)
	}
	var manifest RunManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return RunManifest{}, fmt.Errorf("read run manifest: decode %s: %w", path, err)
	}
	return manifest, nil
}

func runSummary(manifest RunManifest) RunSummary {
	return RunSummary{
		RunID:           manifest.RunID,
		TaskID:          manifest.TaskID,
		TaskName:        manifest.TaskName,
		DisplayName:     manifest.DisplayName,
		Scope:           manifest.Scope,
		ProjectID:       manifest.ProjectID,
		Status:          manifest.Status,
		Trigger:         manifest.Trigger,
		TriggerReason:   manifest.TriggerReason,
		SkipReason:      manifest.SkipReason,
		ConditionReason: manifest.ConditionReason,
		Owner:           firstNonEmptyString(manifest.Owner, manifest.HostOwner),
		Model:           manifest.Model,
		BillingSource:   manifest.BillingSource,
		WorkspaceRoot:   manifest.WorkspaceRoot,
		StartedAt:       manifest.StartedAt,
		FinishedAt:      manifest.FinishedAt,
		DurationMs:      manifest.DurationMs,
		ResultPath:      manifest.ResultPath,
		Summary:         manifest.Summary,
		ErrorCode:       manifest.ErrorCode,
		ErrorMessage:    manifest.ErrorMessage,
		Error:           manifest.Error,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeTrigger(trigger string) (Trigger, error) {
	tr := Trigger(strings.ToLower(strings.TrimSpace(trigger)))
	switch tr {
	case TriggerSchedule, TriggerManual:
		return tr, nil
	default:
		return "", fmt.Errorf("automation: invalid trigger %q", trigger)
	}
}

func defaultTriggerReason(trigger Trigger, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	switch trigger {
	case TriggerManual:
		return "manual trigger"
	case TriggerSchedule:
		return "schedule due"
	default:
		return string(trigger)
	}
}

func newRunID(now time.Time) string {
	return now.UTC().Format("20060102T150405.000000000Z") + "-" + strconv.FormatUint(runSeq.Add(1), 36)
}

func pathID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("empty id")
	}
	if id == "." || id == ".." {
		return "", fmt.Errorf("unsafe id %q", id)
	}
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String(), nil
}

func ensureUnderRoot(root, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes automation root: %s", path)
	}
	return nil
}
