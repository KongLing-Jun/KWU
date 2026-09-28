package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"kwu/internal/fileutil"
	"kwu/internal/scene"
	"kwu/internal/store"
)

// BranchMeta is the small sidecar record that turns flat session files into a
// navigable conversation tree. The conversation itself remains in the .jsonl
// file; metadata lives beside it at <session>.meta.
type BranchMeta struct {
	ID               string    `json:"id"`
	Name             string    `json:"name,omitempty"`
	ParentID         string    `json:"parent_id,omitempty"`
	ForkTurn         int       `json:"fork_turn,omitempty"`
	ForkMessageIndex int       `json:"fork_message_index,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Scope            string    `json:"scope,omitempty"`
	WorkspaceRoot    string    `json:"workspace_root,omitempty"`
	TopicID          string    `json:"topic_id,omitempty"`
	TopicTitle       string    `json:"topic_title,omitempty"`
	Model            string    `json:"model,omitempty"`
	BillingSource    string    `json:"billing_source,omitempty"`
	Effort           *string   `json:"effort,omitempty"`
	// ToolApprovalMode is pointer-valued so a legacy missing posture remains
	// distinguishable from an explicit safe "ask" selection.
	ToolApprovalMode *string `json:"tool_approval_mode,omitempty"`
	Scene            string  `json:"scene,omitempty"`
	SceneState       string  `json:"scene_state,omitempty"`
	// ProjectContext is an additive CloudProject identity snapshot. Scope and
	// WorkspaceRoot above retain their legacy LocalWorkspace meaning.
	ProjectContext *ProjectContextPin `json:"project_context,omitempty"`
	// ProjectSessionPinned is a device-local sidebar preference for one cloud
	// project transcript. It is intentionally not inherited by forks.
	ProjectSessionPinned bool `json:"project_session_pinned,omitempty"`
	// ProjectContextPermanentlyReadOnly marks an explicitly retained device-local
	// transcript after its Cloud account was removed. It never relaxes scope or
	// send authorization and is inherited by metadata-only updates.
	ProjectContextPermanentlyReadOnly bool `json:"project_context_permanently_read_only,omitempty"`
	// SchemaVersion records the BranchMeta version that last wrote the listing
	// fields (Turns/Preview) FROM the session's content. It is stamped only by the
	// writers that actually derive those counts — Controller.snapshot's
	// UpdateSessionMeta and Fork/Branch — never by EnsureBranchMeta / TouchBranchMeta
	// / rename / set-model, which don't know the turn count. So ListSessions can
	// tell a meta whose counts are authoritative (>= BranchMetaCountsVersion: trust
	// Turns even when 0 = genuinely empty) from a legacy/contentless one
	// (< version: decode once, then backfill + stamp).
	SchemaVersion int `json:"schema_version,omitempty"`
	// Turns and Preview are listing-only fields the desktop sidebar and CLI
	// pickers show ("5 turns · 'help me debug…'") without decoding the whole
	// .jsonl. The autosave path (Controller.snapshot) keeps them fresh from the
	// in-memory conversation, so ListSessions stays O(1) per session instead of
	// O(file size). Gated by SchemaVersion (above), not Turns == 0, so a
	// genuinely-empty session is recorded once and never re-decoded.
	Turns   int    `json:"turns,omitempty"`
	Preview string `json:"preview,omitempty"`
}

// ProjectEnvironmentIdentity and ProjectBindingScope mirror the validated
// cloud values at the persistence boundary without introducing an agent/cloud
// package cycle.
type ProjectEnvironmentIdentity struct {
	EnvironmentID string `json:"environment_id"`
	CloudBaseURL  string `json:"cloud_base_url"`
}

type ProjectBindingScope struct {
	ProfileID         string `json:"profile_id"`
	CloudUserID       string `json:"cloud_user_id"`
	BindingGeneration uint64 `json:"binding_generation"`
}

type ProjectContextPin struct {
	Environment        ProjectEnvironmentIdentity `json:"environment"`
	BindingScope       ProjectBindingScope        `json:"binding_scope"`
	ProjectID          string                     `json:"project_id"`
	InstructionVersion uint64                     `json:"instruction_version"`
	ContentHash        string                     `json:"content_hash"`
}

func (p ProjectContextPin) Validate() error {
	if !lowerSHA256(p.Environment.EnvironmentID) {
		return fmt.Errorf("project environment_id must be lowercase SHA-256")
	}
	if !validPinnedText(p.Environment.CloudBaseURL) {
		return fmt.Errorf("project cloud_base_url is invalid")
	}
	if !validPinnedText(p.BindingScope.ProfileID) {
		return fmt.Errorf("project profile_id is invalid")
	}
	if !validPinnedText(p.BindingScope.CloudUserID) {
		return fmt.Errorf("project cloud_user_id is invalid")
	}
	if p.BindingScope.BindingGeneration == 0 {
		return fmt.Errorf("project binding_generation must be positive")
	}
	if !validPinnedText(p.ProjectID) {
		return fmt.Errorf("project_id is invalid")
	}
	if p.InstructionVersion == 0 {
		return fmt.Errorf("project instruction_version must be positive")
	}
	if !lowerSHA256(p.ContentHash) {
		return fmt.Errorf("project content_hash must be lowercase SHA-256")
	}
	return nil
}

func validPinnedText(value string) bool { return value != "" && value == strings.TrimSpace(value) }

func lowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (m BranchMeta) ValidateProjectContextPin() error {
	if m.ProjectContext == nil {
		return nil
	}
	return m.ProjectContext.Validate()
}

func (m BranchMeta) IsCloudProjectBound() bool {
	return m.ProjectContext != nil && m.ProjectContext.Validate() == nil
}

// InheritComposerState copies the session-scoped composer selection into a
// newly-created child branch. The pointer is cloned so parent and child remain
// independent after the fork.
func (m *BranchMeta) InheritComposerState(parent BranchMeta) {
	if m == nil {
		return
	}
	m.Model = strings.TrimSpace(parent.Model)
	m.BillingSource = strings.TrimSpace(parent.BillingSource)
	m.Effort = nil
	if parent.Effort != nil {
		effort := *parent.Effort
		m.Effort = &effort
	}
	m.ToolApprovalMode = nil
	if parent.ToolApprovalMode != nil {
		mode := *parent.ToolApprovalMode
		m.ToolApprovalMode = &mode
	}
}

// InheritBranchComposerState loads a parent sidecar when available and applies
// its composer state to a child without making branching depend on metadata.
func InheritBranchComposerState(parentPath string, child BranchMeta) BranchMeta {
	parent, ok, err := LoadBranchMeta(parentPath)
	if err == nil && ok {
		child.InheritComposerState(parent)
		if parent.ProjectContext != nil {
			pin := *parent.ProjectContext
			child.ProjectContext = &pin
		}
		child.ProjectContextPermanentlyReadOnly = parent.ProjectContextPermanentlyReadOnly
	}
	return child
}

func (m BranchMeta) SceneStatus() (scene.Status, error) {
	return scene.RestoreStatus(m.Scene, m.SceneState)
}

// SetSceneStatus stores only public Draft/Locked values. Legacy and unadopted
// hosts remain represented by absent fields.
func (m *BranchMeta) SetSceneStatus(status scene.Status) error {
	if m == nil {
		return fmt.Errorf("nil branch metadata")
	}
	sceneValue, stateValue, ok := status.Persisted()
	if !ok {
		if err := status.Validate(); err != nil {
			return err
		}
		m.Scene = ""
		m.SceneState = ""
		return nil
	}
	m.Scene = sceneValue
	m.SceneState = stateValue
	return nil
}

// BranchMetaCountsVersion is stamped into BranchMeta.SchemaVersion whenever a
// writer records Turns/Preview from session content (UpdateSessionMeta,
// Fork/Branch). Bump it when the meaning of those listing fields changes so
// existing listings re-derive them instead of trusting a stale cache.
const BranchMetaCountsVersion = 2

func (m BranchMeta) DefaultScope() string {
	switch m.Scope {
	case "project":
		return "project"
	default:
		return "global"
	}
}

// BranchInfo combines sidecar metadata with the session file details needed for
// pickers and tree rendering.
type BranchInfo struct {
	BranchMeta
	Path    string
	ModTime time.Time
	Preview string
	Turns   int
}

func BranchID(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func BranchMetaPath(sessionPath string) string {
	return store.SessionMeta(sessionPath)
}

type branchMetaPathLock struct {
	mu   sync.Mutex
	refs int
}

var branchMetaPathLocks = struct {
	sync.Mutex
	entries map[string]*branchMetaPathLock
}{entries: make(map[string]*branchMetaPathLock)}

func canonicalBranchMetaPath(path string) string {
	path = filepath.Clean(path)
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func acquireBranchMetaPath(path string) func() {
	key := canonicalBranchMetaPath(path)
	branchMetaPathLocks.Lock()
	entry := branchMetaPathLocks.entries[key]
	if entry == nil {
		entry = &branchMetaPathLock{}
		branchMetaPathLocks.entries[key] = entry
	}
	entry.refs++
	branchMetaPathLocks.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		branchMetaPathLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(branchMetaPathLocks.entries, key)
		}
		branchMetaPathLocks.Unlock()
	}
}

const (
	maxBranchMetaReadRetries = 8
	branchMetaReadRetryBase  = 20 * time.Millisecond
)

func readBranchMetaFile(path string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return os.ReadFile(path)
	}

	var err error
	for attempt := 0; ; attempt++ {
		var payload []byte
		payload, err = os.ReadFile(path)
		if err == nil {
			return payload, nil
		}
		if attempt >= maxBranchMetaReadRetries || !isTransientBranchMetaReadError(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * branchMetaReadRetryBase)
	}
}

func isTransientBranchMetaReadError(err error) bool {
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33))
}

func LoadBranchMeta(sessionPath string) (BranchMeta, bool, error) {
	metaPath := BranchMetaPath(sessionPath)
	if metaPath == "" {
		return BranchMeta{}, false, nil
	}
	release := acquireBranchMetaPath(metaPath)
	defer release()
	return loadBranchMetaUnlocked(sessionPath, metaPath)
}

func loadBranchMetaUnlocked(sessionPath, metaPath string) (BranchMeta, bool, error) {
	b, err := readBranchMetaFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return BranchMeta{}, false, nil
		}
		return BranchMeta{}, false, err
	}
	var m BranchMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return BranchMeta{}, false, fmt.Errorf("decode branch meta %s: %w", metaPath, err)
	}
	if err := m.ValidateProjectContextPin(); err != nil {
		return BranchMeta{}, false, fmt.Errorf("decode branch meta %s: %w", metaPath, err)
	}
	if m.ID == "" {
		m.ID = BranchID(sessionPath)
	}
	return m, true, nil
}

func SaveBranchMeta(sessionPath string, m BranchMeta) error {
	return saveBranchMeta(sessionPath, m, true)
}

func SaveBranchMetaPreserveUpdated(sessionPath string, m BranchMeta) error {
	return saveBranchMeta(sessionPath, m, false)
}

func saveBranchMeta(sessionPath string, m BranchMeta, touchUpdated bool) error {
	metaPath := BranchMetaPath(sessionPath)
	if metaPath == "" {
		return fmt.Errorf("empty session path")
	}
	release := acquireBranchMetaPath(metaPath)
	err := saveBranchMetaUnlocked(sessionPath, metaPath, m, touchUpdated)
	release()
	if err != nil {
		return err
	}
	refreshSessionCatalogEntry(sessionPath, nil)
	return nil
}

func saveBranchMetaUnlocked(sessionPath, metaPath string, m BranchMeta, touchUpdated bool) error {
	if err := m.ValidateProjectContextPin(); err != nil {
		return fmt.Errorf("invalid branch project context: %w", err)
	}
	now := time.Now().UTC()
	if m.ID == "" {
		m.ID = BranchID(sessionPath)
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if touchUpdated || m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return err
	}
	return fileutil.AtomicWriteFileStrict(metaPath, b, 0o600)
}

func EnsureBranchMeta(sessionPath string) (BranchMeta, error) {
	if sessionPath == "" {
		return BranchMeta{}, fmt.Errorf("empty session path")
	}
	metaPath := BranchMetaPath(sessionPath)
	release := acquireBranchMetaPath(metaPath)
	m, created, err := ensureBranchMetaUnlocked(sessionPath, metaPath)
	release()
	if err == nil && created {
		refreshSessionCatalogEntry(sessionPath, nil)
	}
	return m, err
}

func ensureBranchMetaUnlocked(sessionPath, metaPath string) (BranchMeta, bool, error) {
	if m, ok, err := loadBranchMetaUnlocked(sessionPath, metaPath); err != nil || ok {
		return m, false, err
	}
	m := newBranchMeta(sessionPath)
	return m, true, saveBranchMetaUnlocked(sessionPath, metaPath, m, false)
}

func newBranchMeta(sessionPath string) BranchMeta {
	when := time.Now().UTC()
	if info, err := os.Stat(sessionPath); err == nil {
		when = info.ModTime().UTC()
	}
	return BranchMeta{
		ID:        BranchID(sessionPath),
		CreatedAt: when,
		UpdatedAt: when,
	}
}

// MutateBranchMeta serializes a read-modify-write transaction for one sidecar,
// creating the sidecar first when it is missing. The callback, validation,
// marshal, and strict atomic publish all run while the path lock is held.
func MutateBranchMeta(sessionPath string, touchUpdated bool, mutate func(*BranchMeta) error) error {
	_, err := mutateBranchMeta(sessionPath, touchUpdated, true, mutate)
	return err
}

// MutateExistingBranchMeta applies a transaction only when the sidecar already
// exists. It returns false without invoking mutate when the sidecar is missing.
func MutateExistingBranchMeta(sessionPath string, touchUpdated bool, mutate func(*BranchMeta) error) (bool, error) {
	return mutateBranchMeta(sessionPath, touchUpdated, false, mutate)
}

func mutateBranchMeta(sessionPath string, touchUpdated, ensure bool, mutate func(*BranchMeta) error) (bool, error) {
	if sessionPath == "" {
		return false, fmt.Errorf("empty session path")
	}
	if mutate == nil {
		return false, fmt.Errorf("nil branch metadata mutation")
	}
	metaPath := BranchMetaPath(sessionPath)
	release := acquireBranchMetaPath(metaPath)
	ok, err := mutateBranchMetaLocked(sessionPath, metaPath, touchUpdated, ensure, mutate)
	release()
	if err != nil || !ok {
		return ok, err
	}
	refreshSessionCatalogEntry(sessionPath, nil)
	return true, nil
}

func mutateBranchMetaLocked(sessionPath, metaPath string, touchUpdated, ensure bool, mutate func(*BranchMeta) error) (bool, error) {
	meta, ok, err := loadBranchMetaUnlocked(sessionPath, metaPath)
	if err != nil {
		return false, err
	}
	if !ok && !ensure {
		return false, nil
	}
	if !ok {
		meta = newBranchMeta(sessionPath)
	}
	if err := mutate(&meta); err != nil {
		return true, err
	}
	if err := saveBranchMetaUnlocked(sessionPath, metaPath, meta, touchUpdated); err != nil {
		return true, err
	}
	return true, nil
}

func TouchBranchMeta(sessionPath string) error {
	return MutateBranchMeta(sessionPath, false, func(m *BranchMeta) error {
		m.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func ListBranches(dir string) ([]BranchInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BranchInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !IsVisibleSession(path) {
			continue
		}
		preview, turns := previewSession(path)
		if turns == 0 {
			continue
		}
		meta, ok, err := LoadBranchMeta(path)
		if err != nil {
			continue
		}
		if !ok {
			meta = BranchMeta{
				ID:        BranchID(path),
				CreatedAt: info.ModTime().UTC(),
				UpdatedAt: info.ModTime().UTC(),
			}
		}
		if meta.ID == "" {
			meta.ID = BranchID(path)
		}
		out = append(out, BranchInfo{
			BranchMeta: meta,
			Path:       path,
			ModTime:    info.ModTime(),
			Preview:    preview,
			Turns:      turns,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// RenameSession updates the TopicTitle in the session's .jsonl.meta sidecar
// file. If no meta file exists yet, one is created. This is used by the
// /rename CLI command and desktop UI to give sessions human-readable names.
func RenameSession(sessionPath string, title string) error {
	if sessionPath == "" {
		return fmt.Errorf("empty session path")
	}
	return MutateBranchMeta(sessionPath, true, func(m *BranchMeta) error {
		m.TopicTitle = title
		return nil
	})
}

// LoadSessionModel reads the canonical provider/model ref saved beside a
// session transcript.
func LoadSessionModel(sessionPath string) (string, bool) {
	meta, ok, err := LoadBranchMeta(sessionPath)
	if err != nil || !ok {
		return "", false
	}
	model := strings.TrimSpace(meta.Model)
	if model == "" {
		return "", false
	}
	return model, true
}

// SetBranchModelPreserveUpdated stores the canonical provider/model ref without
// changing the session activity timestamp.
func SetBranchModelPreserveUpdated(sessionPath, model string) error {
	if sessionPath == "" {
		return fmt.Errorf("empty session path")
	}
	return MutateBranchMeta(sessionPath, false, func(meta *BranchMeta) error {
		meta.Model = strings.TrimSpace(model)
		return nil
	})
}

// UpdateSessionMeta refreshes the listing-only sidecar fields (model, preview,
// user-turn count) the sidebar and pickers read without decoding the .jsonl.
// markActivity bumps UpdatedAt (the autosave path passes true on a real turn);
// false preserves it (used to backfill legacy sessions during a read). An empty
// model leaves the stored model untouched.
func UpdateSessionMeta(sessionPath, model, preview string, turns int, markActivity bool) error {
	if sessionPath == "" {
		return fmt.Errorf("empty session path")
	}
	return MutateBranchMeta(sessionPath, markActivity, func(m *BranchMeta) error {
		if strings.TrimSpace(model) != "" {
			m.Model = strings.TrimSpace(model)
		}
		m.Preview = preview
		m.Turns = turns
		// These counts were derived from the current content, so mark them
		// authoritative — listing can then trust Turns (even 0) without re-decoding.
		m.SchemaVersion = BranchMetaCountsVersion
		return nil
	})
}
