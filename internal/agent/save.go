package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/fileutil"
	"kwu/internal/provider"
	"kwu/internal/store"
)

const cleanupPendingExt = ".cleanup-pending.json"

// Save writes the session's messages to path in JSONL — one provider.Message
// per line — so a user can resume the conversation later. Explicit Save calls
// always rewrite the target; periodic controller snapshots use SaveIfChanged.
func (s *Session) Save(path string) error {
	_, _, err := s.save(path, false)
	return err
}

// SaveIfChanged writes the session only when its content generation differs
// from the generation already persisted to path. The returned snapshot is the
// exact message slice written and can be reused for listing metadata without a
// second full-session copy.
func (s *Session) SaveIfChanged(path string) ([]provider.Message, bool, error) {
	return s.save(path, true)
}

func (s *Session) save(path string, onlyIfChanged bool) ([]provider.Message, bool, error) {
	if path == "" {
		return nil, false, fmt.Errorf("empty session path")
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.saveLocked(path, onlyIfChanged)
}

func (s *Session) saveLocked(path string, onlyIfChanged bool) ([]provider.Message, bool, error) {
	cleanPath := filepath.Clean(path)
	messages, generation, changed := s.snapshotForSave(cleanPath, onlyIfChanged)
	if !changed {
		return nil, false, nil
	}
	if err := persistSessionMessages(path, messages); err != nil {
		return nil, false, err
	}
	s.markPersisted(cleanPath, generation)
	refreshSessionCatalogEntry(path, messages)
	return messages, true, nil
}

// CommitArtifactDeliveryAndSave publishes an artifact contract only when the
// matching final answer can be persisted in the same JSONL snapshot. It reuses
// the Session's existing save serialization so periodic autosaves can observe
// only the durable state before or after this transition, never an intermediate
// promoted answer whose write later failed.
func (s *Session) CommitArtifactDeliveryAndSave(path, artifactTurnID string, delivery provider.ArtifactDelivery) (provider.ArtifactDelivery, bool, error) {
	if path == "" {
		return provider.ArtifactDelivery{}, false, fmt.Errorf("empty session path")
	}
	if artifactTurnID == "" || !provider.IsDurableArtifactDeliveryV1(&delivery) {
		return provider.ArtifactDelivery{}, false, nil
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.Lock()
	committed, ok := s.commitArtifactDeliveryLocked(artifactTurnID, delivery)
	s.mu.Unlock()
	if !ok {
		return provider.ArtifactDelivery{}, false, nil
	}
	if _, _, err := s.saveLocked(path, false); err == nil {
		return committed, true, nil
	} else {
		cause := err
		s.mu.Lock()
		s.downgradeArtifactDeliveryFinalLocked(artifactTurnID)
		s.mu.Unlock()
		if _, _, rollbackErr := s.saveLocked(path, false); rollbackErr != nil {
			return provider.ArtifactDelivery{}, false, errors.Join(cause, fmt.Errorf("persist uncommitted artifact final: %w", rollbackErr))
		}
		return provider.ArtifactDelivery{}, false, cause
	}
}

// DowngradeArtifactDeliveryFinalAndSave removes a previously promoted contract
// and persists the fail-closed answer before returning. The in-memory answer
// remains uncommitted even when persistence fails.
func (s *Session) DowngradeArtifactDeliveryFinalAndSave(path, artifactTurnID string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("empty session path")
	}
	if artifactTurnID == "" {
		return false, nil
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.Lock()
	found := s.downgradeArtifactDeliveryFinalLocked(artifactTurnID)
	s.mu.Unlock()
	if !found {
		return false, nil
	}
	_, _, err := s.saveLocked(path, false)
	return true, err
}

func (s *Session) snapshotForSave(cleanPath string, onlyIfChanged bool) ([]provider.Message, uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if onlyIfChanged &&
		!s.normalizedDirty &&
		s.persistedPath == cleanPath &&
		s.persistedGeneration == s.contentGeneration {
		return nil, 0, false
	}
	generation := s.contentGeneration
	messages := append([]provider.Message(nil), s.Messages...)
	return messages, generation, true
}

func persistSessionMessages(path string, messages []provider.Message) error {
	return persistSessionMessagesWith(path, messages, (*os.File).Sync, fileutil.ReplaceFileStrict)
}

func persistSessionMessagesWith(
	path string,
	messages []provider.Message,
	syncFile func(*os.File) error,
	replaceFile func(string, string) error,
) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	// Write to a sibling tmp file then rename, so a crash mid-write can't
	// leave a partial JSONL that won't reload.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session.*.tmp")
	if err != nil {
		return fmt.Errorf("create session tmp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	enc := json.NewEncoder(tmp)
	for _, m := range messages {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("encode message: %w", err)
		}
	}
	if err := syncFile(tmp); err != nil {
		return fmt.Errorf("sync session tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session tmp: %w", err)
	}
	if err := replaceFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace session file: %w", err)
	}
	return nil
}

// LoadSession reads a JSONL file written by Save into a fresh Session value.
// Missing files surface as os.IsNotExist so callers can fall through to a
// new session.
func LoadSession(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := &Session{}
	// Decode a stream of JSON values rather than scanning lines: a single
	// message (e.g. a multi-MiB bash output) can exceed any line-buffer cap, and
	// Save's json.Encoder has no such limit — a Scanner here made sessions that
	// saved fine fail to reload.
	dec := json.NewDecoder(f)
	for {
		var m provider.Message
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		s.Messages = append(s.Messages, m)
	}
	// Repair persisted-history-safe issues before anything reads the session.
	// Old sessions (pre adde2d3e) and interrupted turns can carry empty tool-call
	// names, dangling tool_calls, or half-streamed argument JSON that DeepSeek
	// rejects with a 400 on replay. Wire-only cleanup, such as dropping orphan
	// tool messages, stays in the provider send path so Save/LoadSession keeps
	// its round-trip contract. The fast path returns the input slice unchanged
	// for a well-formed history, so we detect an actual repair by comparing
	// slice headers: when NormalizeSession allocated a new backing array, the
	// session is marked dirty so the next Save persists the fix.
	normalized := NormalizeSession(s.Messages)
	if len(normalized) != len(s.Messages) || (len(s.Messages) > 0 && &normalized[0] != &s.Messages[0]) {
		s.normalizedDirty = true
	}
	s.Messages = normalized
	s.contentGeneration = 1
	s.persistedGeneration = 1
	s.persistedPath = filepath.Clean(path)
	if s.normalizedDirty {
		s.persistedGeneration = 0
	}
	return s, nil
}

// SessionInfo summarises a saved session for the --resume picker: where it is on
// disk, when it was created/last active, the first user message as a preview, and
// a rough turn count.
type SessionInfo struct {
	Path           string
	CreatedAt      time.Time
	LastActivityAt time.Time
	ModTime        time.Time // compatibility alias for LastActivityAt
	Preview        string
	Turns          int
	ExpertID       string
	ExpertName     string
	Scope          string
	WorkspaceRoot  string
	TopicID        string
	TopicTitle     string
}

type ExpertMeta struct {
	ExpertID         string   `json:"expert_id,omitempty"`
	ExpertName       string   `json:"expert_name,omitempty"`
	Source           string   `json:"source,omitempty"`
	ProjectExpertIDs []string `json:"project_expert_ids,omitempty"`
	TeamState        string   `json:"team_state,omitempty"`
	// Missing usage is unknown in legacy sidecars, never proof of non-use.
	TeamUsage         string                 `json:"team_usage,omitempty"`
	Lead              *ExpertTeamMemberMeta  `json:"lead,omitempty"`
	Members           []ExpertTeamMemberMeta `json:"members,omitempty"`
	TeamBindingsReset bool                   `json:"team_bindings_reset,omitempty"`
	UpdatedAt         int64                  `json:"updated_at,omitempty"`
}

const (
	ExpertTeamActive   = "team_active"
	ExpertTeamArchived = "team_archived"
	ExpertTeamUnused   = "unused"
	ExpertTeamUsed     = "used"
)

// ExpertTeamMemberMeta is the non-sensitive member snapshot kept with the
// parent session. It intentionally excludes prompts, tasks, and A2A payloads.
type ExpertTeamMemberMeta struct {
	MemberKey   string `json:"member_key"`
	ExpertID    string `json:"expert_id,omitempty"`
	Name        string `json:"name"`
	Role        string `json:"role,omitempty"`
	AvatarKey   string `json:"avatar_key,omitempty"`
	SubagentRef string `json:"subagent_ref,omitempty"`
}

func SaveSessionExpert(path string, meta ExpertMeta) error {
	sidecar := store.SessionExpert(path)
	if sidecar == "" {
		return nil
	}
	if strings.TrimSpace(meta.ExpertID) == "" {
		return fmt.Errorf("expert metadata requires an expert ID")
	}
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		return err
	}
	meta.UpdatedAt = time.Now().Unix()
	tmp, err := os.CreateTemp(filepath.Dir(sidecar), ".expert.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(meta); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := fileutil.ReplaceFile(tmpPath, sidecar); err != nil {
		os.Remove(tmpPath)
		return err
	}
	refreshSessionCatalogEntry(path, nil)
	return nil
}

// DeleteSessionExpert explicitly removes the expert metadata owned by path.
// Saving an empty snapshot must never double as deletion because ordinary
// autosaves can legitimately have no current expert while an archived team
// remains attached to the session.
func DeleteSessionExpert(path string) error {
	sidecar := store.SessionExpert(path)
	if sidecar == "" {
		return nil
	}
	if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
		return err
	}
	refreshSessionCatalogEntry(path, nil)
	return nil
}

func LoadSessionExpert(path string) (ExpertMeta, bool, error) {
	sidecar := store.SessionExpert(path)
	if sidecar == "" {
		return ExpertMeta{}, false, nil
	}
	f, err := os.Open(sidecar)
	if err != nil {
		if os.IsNotExist(err) {
			return ExpertMeta{}, false, nil
		}
		return ExpertMeta{}, false, err
	}
	defer f.Close()
	var meta ExpertMeta
	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return ExpertMeta{}, false, err
	}
	return meta, strings.TrimSpace(meta.ExpertID) != "", nil
}

// SessionOrderInfo is the lightweight sidecar/mtime ordering record shared by
// session pickers and prompt-history navigation. It intentionally avoids reading
// JSONL content; callers that need previews can layer that on afterwards.
type SessionOrderInfo struct {
	Path           string
	CreatedAt      time.Time
	LastActivityAt time.Time
	ModTime        time.Time // compatibility alias for LastActivityAt
	Scope          string
	WorkspaceRoot  string
	TopicID        string
	TopicTitle     string
	// Turns and Preview are the cached listing fields from the sidecar; SchemaVersion
	// >= agent.BranchMetaCountsVersion means they were recorded from content and can
	// be trusted (even Turns == 0). ListSessions uses them to skip the whole-file decode.
	Turns         int
	Preview       string
	SchemaVersion int
}

// CleanupPendingMeta records that a session was logically removed but still has
// artifacts waiting for a background job to unwind before physical cleanup.
type CleanupPendingMeta struct {
	Operation string `json:"operation"`
	CreatedAt int64  `json:"createdAt"`
}

// CleanupPendingInfo describes one durable delayed-cleanup marker and the
// session transcript it belongs to.
type CleanupPendingInfo struct {
	SessionPath string
	MarkerPath  string
	Meta        CleanupPendingMeta
}

// CleanupPendingPath returns the durable marker path for a session transcript.
func CleanupPendingPath(sessionPath string) string {
	return store.SessionCleanupPending(sessionPath)
}

// MarkCleanupPending hides a logically removed session from resume/list surfaces
// until delayed physical cleanup has finished.
func MarkCleanupPending(sessionPath, operation string) error {
	path := CleanupPendingPath(sessionPath)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	meta := CleanupPendingMeta{Operation: strings.TrimSpace(operation), CreatedAt: time.Now().UnixMilli()}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	removeSessionCatalogEntry(sessionPath)
	return nil
}

// ClearCleanupPending removes a delayed-cleanup marker after physical cleanup.
func ClearCleanupPending(sessionPath string) error {
	path := CleanupPendingPath(sessionPath)
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	refreshSessionCatalogEntry(sessionPath, nil)
	return nil
}

// IsCleanupPending reports whether a session is hidden pending delayed cleanup.
func IsCleanupPending(sessionPath string) bool {
	path := CleanupPendingPath(sessionPath)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// IsVisibleSession reports whether a persisted session should appear on normal
// user/agent-facing list, restore, and retrieval surfaces.
func IsVisibleSession(sessionPath string) bool {
	return strings.TrimSpace(sessionPath) != "" && !IsCleanupPending(sessionPath)
}

// ListCleanupPending returns delayed-cleanup markers left in dir. A missing
// directory is not an error.
func ListCleanupPending(dir string) ([]CleanupPendingInfo, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []CleanupPendingInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), cleanupPendingExt) {
			continue
		}
		markerPath := filepath.Join(dir, e.Name())
		var meta CleanupPendingMeta
		b, err := os.ReadFile(markerPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if strings.TrimSpace(string(b)) != "" {
			if err := json.Unmarshal(b, &meta); err != nil {
				return nil, fmt.Errorf("read cleanup-pending marker %s: %w", markerPath, err)
			}
		}
		name := strings.TrimSuffix(e.Name(), cleanupPendingExt) + ".jsonl"
		out = append(out, CleanupPendingInfo{
			SessionPath: filepath.Join(dir, name),
			MarkerPath:  markerPath,
			Meta:        meta,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SessionPath < out[j].SessionPath
	})
	return out, nil
}

// ReconcileCleanupPending retries physical cleanup for leftover delayed-cleanup
// markers. It keeps going after individual cleanup errors and returns them joined.
func ReconcileCleanupPending(dir string, cleanup func(CleanupPendingInfo) error) error {
	pending, err := ListCleanupPending(dir)
	if err != nil {
		return err
	}
	if cleanup == nil {
		return nil
	}
	var errs []error
	for _, item := range pending {
		if err := cleanup(item); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", item.SessionPath, err))
		}
	}
	return errors.Join(errs...)
}

// ListSessionOrder returns every *.jsonl session under dir in the same
// most-recently-active order used by ListSessions, using only file metadata and
// branch sidecars. A missing directory is not an error.
func ListSessionOrder(dir string) ([]SessionOrderInfo, error) {
	out, pending, err := ListSessionOrderContext(context.Background(), dir)
	if err != nil || !pending {
		return out, err
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		return nil, err
	}
	out, _, err = ListSessionOrderContext(context.Background(), dir)
	return out, err
}

// ListSessionOrderContext returns a catalog-backed order and whether exact
// legacy metadata is still being repaired.
func ListSessionOrderContext(ctx context.Context, dir string) ([]SessionOrderInfo, bool, error) {
	if sessionCatalogEnabled() {
		if rows, pending, err := loadSessionCatalogRows(ctx, dir); err == nil {
			out := make([]SessionOrderInfo, 0, len(rows))
			for _, row := range rows {
				out = append(out, SessionOrderInfo{
					Path: row.Path, CreatedAt: row.CreatedAt, LastActivityAt: row.LastActivityAt,
					ModTime: row.ModTime, Scope: row.Scope, WorkspaceRoot: row.WorkspaceRoot,
					TopicID: row.TopicID, TopicTitle: row.TopicTitle, Turns: row.Turns,
					Preview: row.Preview, SchemaVersion: row.SchemaVersion,
				})
			}
			return out, pending, nil
		} else {
			return nil, false, err
		}
	}
	out, err := scanSessionOrder(dir)
	return out, false, err
}

func scanSessionOrder(dir string) ([]SessionOrderInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionOrderInfo
	for _, e := range entries {
		if session, ok := sessionOrderInfoFromEntry(dir, e); ok {
			out = append(out, session)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastActivityAt.Equal(out[j].LastActivityAt) {
			return out[i].Path < out[j].Path
		}
		return out[i].LastActivityAt.After(out[j].LastActivityAt)
	})
	return out, nil
}

func sessionOrderInfoFromEntry(dir string, entry os.DirEntry) (SessionOrderInfo, bool) {
	if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
		return SessionOrderInfo{}, false
	}
	info, err := entry.Info()
	if err != nil {
		return SessionOrderInfo{}, false
	}
	full := filepath.Join(dir, entry.Name())
	if !IsVisibleSession(full) {
		return SessionOrderInfo{}, false
	}
	session := SessionOrderInfo{
		Path: full, CreatedAt: info.ModTime(), LastActivityAt: info.ModTime(),
		ModTime: info.ModTime(), Scope: "global",
	}
	if meta, ok, err := LoadBranchMeta(full); err == nil && ok {
		applyBranchMetaToSessionOrder(&session, meta)
	}
	return session, true
}

func applyBranchMetaToSessionOrder(session *SessionOrderInfo, meta BranchMeta) {
	if !meta.CreatedAt.IsZero() {
		session.CreatedAt = meta.CreatedAt
	}
	if !meta.UpdatedAt.IsZero() {
		session.LastActivityAt = meta.UpdatedAt
		session.ModTime = meta.UpdatedAt
	}
	session.Scope = meta.DefaultScope()
	session.WorkspaceRoot = meta.WorkspaceRoot
	session.TopicID = meta.TopicID
	session.TopicTitle = meta.TopicTitle
	session.Turns = meta.Turns
	session.Preview = meta.Preview
	session.SchemaVersion = meta.SchemaVersion
}

// ListSessions returns every non-empty *.jsonl session under dir,
// most-recently-active first, each with a preview line so the picker can show
// something the user recognises. A missing directory is not an error — it just
// means there's nothing to resume yet.
func ListSessions(dir string) ([]SessionInfo, error) {
	infos, pending, err := ListSessionsContext(context.Background(), dir)
	if err != nil || !pending {
		return infos, err
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		return nil, err
	}
	infos, _, err = ListSessionsContext(context.Background(), dir)
	return infos, err
}

// ListSessionsContext returns catalog-backed session summaries and reports
// whether an exact, cancellable legacy repair is still running in the
// background. Callers may wait with WaitSessionCatalogRepair and refresh.
func ListSessionsContext(ctx context.Context, dir string) ([]SessionInfo, bool, error) {
	rows, pending, err := loadSessionCatalogRows(ctx, dir)
	if err != nil {
		return nil, false, err
	}
	var out []SessionInfo
	for _, row := range rows {
		session := SessionOrderInfo{
			Path: row.Path, CreatedAt: row.CreatedAt, LastActivityAt: row.LastActivityAt,
			ModTime: row.ModTime, Scope: row.Scope, WorkspaceRoot: row.WorkspaceRoot,
			TopicID: row.TopicID, TopicTitle: row.TopicTitle, Turns: row.Turns,
			Preview: row.Preview, SchemaVersion: row.SchemaVersion,
		}
		preview, turns := session.Preview, session.Turns
		if !sessionCatalogEnabled() && session.SchemaVersion < BranchMetaCountsVersion {
			// The sidecar's counts weren't recorded from content (a legacy session
			// from before they were persisted). Decode the .jsonl once, then backfill
			// + stamp the sidecar so every later listing is O(1) — and so a genuinely
			// empty session is recorded once instead of being re-decoded forever.
			preview, turns = previewSession(session.Path)
			// Best-effort: a failure here just means we decode again next time.
			_ = UpdateSessionMeta(session.Path, "", preview, turns, false)
		}
		if turns == 0 {
			// Never had user interaction — an empty conversation that should not
			// appear in the history panel or the resume picker.
			continue
		}
		out = append(out, SessionInfo{
			Path:           session.Path,
			CreatedAt:      session.CreatedAt,
			LastActivityAt: session.LastActivityAt,
			ModTime:        session.ModTime,
			Preview:        preview,
			Turns:          turns,
			ExpertID:       row.ExpertID,
			ExpertName:     row.ExpertName,
			Scope:          session.Scope,
			WorkspaceRoot:  session.WorkspaceRoot,
			TopicID:        session.TopicID,
			TopicTitle:     session.TopicTitle,
		})
	}
	return out, pending, nil
}

// SessionPreview returns the same preview and user-turn count used by
// ListSessions for one session file.
func SessionPreview(path string) (string, int) {
	return previewSession(path)
}

// SessionPreviewFromMessages computes the same preview line and user-turn count
// as previewSession, but from an in-memory message slice. Session.Save writes
// exactly these messages to the .jsonl, so this is byte-for-byte equivalent to
// decoding the file — letting the autosave path persist the counts into the
// sidecar without a disk read.
func SessionPreviewFromMessages(msgs []provider.Message) (string, int) {
	first := ""
	turns := 0
	for _, m := range msgs {
		if m.Role == provider.RoleUser && !m.IsProjectContext() {
			if IsTurnInterruptedMessage(m.Content) {
				continue
			}
			turns++
			if first == "" {
				first = truncatePreview(UserPreviewText(m.Content))
			}
		}
	}
	return first, turns
}

// previewSession returns the first user message (truncated) and the number of
// user-role messages so the picker can show "5 turns · 'help me debug the…'".
// Errors are swallowed — a malformed file just shows up with an empty preview.
func previewSession(path string) (string, int) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	first := ""
	turns := 0
	for {
		var m provider.Message
		if err := dec.Decode(&m); err != nil {
			break // EOF or a malformed tail — return the preview gathered so far
		}
		if m.Role == provider.RoleUser && !m.IsProjectContext() {
			if IsTurnInterruptedMessage(m.Content) {
				continue
			}
			turns++
			if first == "" {
				first = truncatePreview(UserPreviewText(m.Content))
			}
		}
	}
	return first, turns
}

// ProjectContextInspection is the explicit data seam for an authorized
// inspector. Ordinary history projections must never expose Body.
type ProjectContextInspection struct {
	Source      string
	Version     uint64
	ContentHash string
	Body        string
}

// InspectProjectContext returns the first valid typed context snapshot. Scope
// authorization belongs to the caller; this function never infers from body.
func InspectProjectContext(msgs []provider.Message) *ProjectContextInspection {
	for _, msg := range msgs {
		if !msg.IsProjectContext() || msg.ValidateProjectContext() != nil {
			continue
		}
		return &ProjectContextInspection{
			Source: msg.Metadata.Source, Version: msg.Metadata.Version,
			ContentHash: msg.Metadata.ContentHash, Body: msg.Content,
		}
	}
	return nil
}

// truncatePreview clamps a preview line to 80 runes with an ellipsis, matching
// what the pickers render.
func truncatePreview(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:77]) + "…"
	}
	return s
}

// ContinueSessionPath returns where a conversation carried into a rebuilt
// controller (model switch, config change) should keep auto-saving: its existing
// file when it has one, so the continued session stays a single file instead of
// the old one being orphaned as an identical duplicate (#2807). A session with no
// file yet gets a fresh path; "" when persistence is disabled.
func ContinueSessionPath(prevPath, dir, model string) string {
	if prevPath != "" {
		return prevPath
	}
	if dir == "" {
		return ""
	}
	return NewSessionPath(dir, model)
}

// NewSessionPath returns the path to use for a fresh session, namespaced by
// the model so the filename hints at what the conversation was with. dir is
// typically config.SessionDir().
func NewSessionPath(dir, model string) string {
	safe := strings.NewReplacer("/", "-", "\\", "-").Replace(model)
	if safe == "" {
		safe = "session"
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s.jsonl", time.Now().UTC().Format("20060102-150405.000000000"), safe))
}
