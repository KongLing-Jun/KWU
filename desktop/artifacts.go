package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/fileutil"
	"kwu/internal/provider"
	"kwu/internal/tool/builtin"
)

const (
	legacyArtifactSidecarVersion = 1
	artifactSidecarVersion       = 2
	deliverFilesToolName         = "deliver_files"
)

var deliveredFileLinkPattern = regexp.MustCompile(`!?\[[^\]\r\n]+\]\((<[^>\r\n]+>|[^)\s]+)\)`)

type artifactSidecar struct {
	Version                 int                     `json:"version"`
	ArtifactDeliveryVersion int                     `json:"artifactDeliveryVersion,omitempty"`
	LegacyArtifacts         []artifactSidecarRecord `json:"legacyArtifacts,omitempty"`
	Artifacts               []artifactSidecarRecord `json:"artifacts"`
}

type artifactSidecarRecord struct {
	Path        string `json:"path"`
	DeliveredAt int64  `json:"deliveredAt"`
}

type artifactDeliveryBatch struct {
	sessionPath   string
	workspaceRoot string
	candidates    []string
	deliveredAt   int64
	contract      bool
}

// artifactDeliveryQueue is immutable after publication through WorkspaceTab's
// atomic pointer. Writers copy the slice before a compare-and-swap.
type artifactDeliveryQueue struct {
	items []artifactDeliveryBatch
}

// ArtifactView is the cross-session, live-file projection consumed by the
// desktop artifact library. Session stays complete so source navigation can
// reuse the existing resume flow.
type ArtifactView struct {
	Path          string      `json:"path"`
	Name          string      `json:"name"`
	Extension     string      `json:"extension"`
	Type          string      `json:"type"`
	Size          int64       `json:"size"`
	ModifiedAt    int64       `json:"modifiedAt"`
	DeliveredAt   int64       `json:"deliveredAt"`
	SessionTitle  string      `json:"sessionTitle"`
	WorkspaceName string      `json:"workspaceName"`
	Session       SessionMeta `json:"session"`
}

func artifactSidecarPath(sessionPath string) string {
	if strings.HasSuffix(strings.ToLower(sessionPath), ".jsonl") {
		return sessionPath[:len(sessionPath)-len(".jsonl")] + ".artifacts.json"
	}
	return sessionPath + ".artifacts.json"
}

// captureArtifactDelivery freezes the just-completed visible user turn before
// autosave coalescing can advance controller history to a later turn. New
// runtimes provide an explicit contract; this remains the compatibility path
// for older runtimes and transcripts.
func captureArtifactDelivery(ctrl control.SessionAPI, deliveredAt int64) (artifactDeliveryBatch, bool) {
	if ctrl == nil {
		return artifactDeliveryBatch{}, false
	}
	sessionPath := strings.TrimSpace(ctrl.SessionPath())
	workspaceRoot := strings.TrimSpace(ctrl.WorkspaceRoot())
	if sessionPath == "" || workspaceRoot == "" || deliveredAt <= 0 {
		return artifactDeliveryBatch{}, false
	}
	turn := lastVisibleUserTurn(ctrl.History())
	if len(turn) == 0 {
		return artifactDeliveryBatch{}, false
	}
	candidates := artifactCandidatesFromTurn(turn)
	if len(candidates) == 0 {
		return artifactDeliveryBatch{}, false
	}
	return artifactDeliveryBatch{
		sessionPath:   sessionPath,
		workspaceRoot: workspaceRoot,
		candidates:    append([]string(nil), candidates...),
		deliveredAt:   deliveredAt,
	}, true
}

// artifactDeliveryBatchFromContract creates the artifact-library projection of
// an already committed Controller contract. It never infers delivery from
// transcript text or task receipts.
func artifactDeliveryBatchFromContract(sessionPath, workspaceRoot string, delivery *provider.ArtifactDelivery) (artifactDeliveryBatch, bool) {
	if delivery == nil || delivery.Version != provider.ArtifactDeliveryVersion || delivery.DeliveredAt <= 0 {
		return artifactDeliveryBatch{}, false
	}
	sessionPath = strings.TrimSpace(sessionPath)
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if sessionPath == "" || workspaceRoot == "" {
		return artifactDeliveryBatch{}, false
	}
	return artifactDeliveryBatch{
		sessionPath:   sessionPath,
		workspaceRoot: workspaceRoot,
		candidates:    append([]string(nil), delivery.Paths...),
		deliveredAt:   delivery.DeliveredAt,
		contract:      true,
	}, true
}

func mergeArtifactDeliveryBatches(batches []artifactDeliveryBatch) error {
	for _, batch := range batches {
		paths := resolvedArtifactPaths(batch.workspaceRoot, batch.candidates)
		if batch.contract {
			if err := mergeArtifactSidecar(batch.sessionPath, paths, batch.deliveredAt); err != nil {
				return err
			}
			continue
		}
		if len(paths) == 0 {
			continue
		}
		if err := mergeLegacyArtifactSidecar(batch.sessionPath, paths, batch.deliveredAt); err != nil {
			return err
		}
	}
	return nil
}

func lastVisibleUserTurn(messages []provider.Message) []provider.Message {
	start := -1
	for index, message := range messages {
		if message.Role != provider.RoleUser || message.IsProjectContext() || control.IsSyntheticUserMessage(message.Content) {
			continue
		}
		start = index
	}
	if start < 0 || start+1 >= len(messages) {
		return nil
	}
	return messages[start+1:]
}

func deliveredArtifactPaths(workspaceRoot string, turn []provider.Message) []string {
	return resolvedArtifactPaths(workspaceRoot, artifactCandidatesFromTurn(turn))
}

func artifactCandidatesFromTurn(turn []provider.Message) []string {
	var candidates []string
	toolNames := make(map[string]string)
	for _, message := range turn {
		if message.Role == provider.RoleAssistant {
			for _, call := range message.ToolCalls {
				toolNames[call.ID] = call.Name
			}
			if message.MessageKind == provider.MessageKindFinalAnswer {
				candidates = append(candidates, finalAnswerArtifactCandidates(message.Content)...)
			}
			continue
		}
		if message.Role != provider.RoleTool {
			continue
		}
		name := strings.TrimSpace(message.Name)
		if name == "" {
			name = toolNames[message.ToolCallID]
		}
		switch name {
		case deliverFilesToolName:
			paths, ok := decodeDeliverFilesResult(message.Content)
			if ok {
				candidates = append(candidates, paths...)
			}
		case "wait_task", "stop_task":
			receipts, ok := decodeTaskLifecycleViews(message.Content)
			if !ok {
				continue
			}
			candidates = append(candidates, verifiedTaskArtifactCandidates(receipts)...)
		}
	}
	return candidates
}

func decodeDeliverFilesResult(output string) ([]string, bool) {
	paths, ok := builtin.DecodeDeliverFilesResult(output)
	if !ok || len(paths) == 0 {
		return nil, false
	}
	return append([]string(nil), paths...), true
}

func resolvedArtifactPaths(workspaceRoot string, candidates []string) []string {
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		path, ok := normalizedWorkspaceArtifactPath(workspaceRoot, candidate)
		if !ok {
			continue
		}
		key := artifactRelativePathKey(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, path)
	}
	return paths
}

func (t *WorkspaceTab) enqueueArtifactDelivery(batch artifactDeliveryBatch) {
	if t == nil {
		return
	}
	for {
		current := t.pendingArtifactDeliveries.Load()
		count := 0
		if current != nil {
			count = len(current.items)
		}
		items := make([]artifactDeliveryBatch, count+1)
		if current != nil {
			copy(items, current.items)
		}
		items[count] = batch
		next := &artifactDeliveryQueue{items: items}
		if t.pendingArtifactDeliveries.CompareAndSwap(current, next) {
			return
		}
	}
}

func (t *WorkspaceTab) takeArtifactDeliveries() []artifactDeliveryBatch {
	if t == nil {
		return nil
	}
	queue := t.pendingArtifactDeliveries.Swap(nil)
	if queue == nil {
		return nil
	}
	return queue.items
}

func (t *WorkspaceTab) requeueArtifactDeliveries(batches []artifactDeliveryBatch) {
	if t == nil || len(batches) == 0 {
		return
	}
	for {
		current := t.pendingArtifactDeliveries.Load()
		count := 0
		if current != nil {
			count = len(current.items)
		}
		items := make([]artifactDeliveryBatch, len(batches)+count)
		copy(items, batches)
		if current != nil {
			copy(items[len(batches):], current.items)
		}
		next := &artifactDeliveryQueue{items: items}
		if t.pendingArtifactDeliveries.CompareAndSwap(current, next) {
			return
		}
	}
}

// transferArtifactDeliveriesTo moves the frozen batches after the source
// autosave has quiesced and while App.mu excludes new event-sink lookups.
func (t *WorkspaceTab) transferArtifactDeliveriesTo(target *WorkspaceTab) {
	if t == nil || target == nil || t == target {
		return
	}
	target.requeueArtifactDeliveries(t.takeArtifactDeliveries())
}

func (t *WorkspaceTab) hasPendingArtifactDeliveries() bool {
	if t == nil {
		return false
	}
	queue := t.pendingArtifactDeliveries.Load()
	return queue != nil && len(queue.items) > 0
}

func finalAnswerArtifactCandidates(content string) []string {
	matches := deliveredFileLinkPattern.FindAllStringSubmatch(content, -1)
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if path, ok := localArtifactHrefPath(match[1]); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

func localArtifactHrefPath(raw string) (string, bool) {
	target := strings.TrimSpace(raw)
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
		target = strings.TrimSpace(target[1 : len(target)-1])
	}
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "//") {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(target), "file:") {
		parsed, err := url.Parse(target)
		if err != nil || !strings.EqualFold(parsed.Scheme, "file") {
			return "", false
		}
		path, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", false
		}
		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		return filepath.FromSlash(path), strings.TrimSpace(path) != ""
	}
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" && filepath.VolumeName(target) == "" {
		return "", false
	}
	path, err := url.PathUnescape(target)
	if err != nil {
		return "", false
	}
	return filepath.FromSlash(path), strings.TrimSpace(path) != ""
}

func verifiedTaskArtifactCandidates(receipts []TaskLifecycleView) []string {
	var paths []string
	for _, receipt := range receipts {
		if receipt.TurnStatus != agent.SubagentCompleted {
			continue
		}
		observed := make(map[string]bool)
		for _, validation := range receipt.Validations {
			if validation.Kind == "artifact_observed" && validation.Success {
				observed[artifactCandidateKey(validation.Path)] = true
			}
		}
		for _, artifact := range receipt.Artifacts {
			path := strings.TrimSpace(artifact.Path)
			if path == "" || !observed[artifactCandidateKey(path)] {
				continue
			}
			if artifact.Source != agent.SubagentArtifactSourceWriter && hasHiddenArtifactDirectory(path) {
				continue
			}
			paths = append(paths, path)
		}
	}
	return paths
}

func artifactCandidateKey(path string) string {
	path = filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if os.PathSeparator == '\\' {
		path = strings.ToLower(path)
	}
	return path
}

func hasHiddenArtifactDirectory(path string) bool {
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if len(part) > 1 && strings.HasPrefix(part, ".") && part != ".." {
			return true
		}
	}
	return false
}

func normalizedWorkspaceArtifactPath(workspaceRoot, candidate string) (string, bool) {
	base, err := historicalWorkspaceBaseFromRoot(workspaceRoot)
	if err != nil {
		return "", false
	}
	path, ok, err := workspacePathForBase(base, strings.TrimSpace(candidate))
	if err != nil || !ok {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	relative, err := filepath.Rel(base, path)
	if err != nil || relative == "." || relative == "" {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func mergeLegacyArtifactSidecar(sessionPath string, paths []string, deliveredAt int64) error {
	path := artifactSidecarPath(sessionPath)
	sidecar, exists, err := readArtifactSidecar(path)
	if err != nil {
		return err
	}
	if !exists {
		sidecar = artifactSidecar{Version: legacyArtifactSidecarVersion}
	}
	var records *[]artifactSidecarRecord
	switch {
	case sidecar.Version == legacyArtifactSidecarVersion:
		records = &sidecar.Artifacts
	case artifactSidecarUsesDeliveryContract(sidecar):
		records = &sidecar.LegacyArtifacts
	case sidecar.Version == artifactSidecarVersion:
		return fmt.Errorf("unsupported artifact delivery version %d", sidecar.ArtifactDeliveryVersion)
	default:
		return fmt.Errorf("unsupported artifact sidecar version %d", sidecar.Version)
	}
	byPath := make(map[string]int, len(*records))
	for index, artifact := range *records {
		byPath[artifactRelativePathKey(artifact.Path)] = index
	}
	changed := !exists
	for _, artifactPath := range paths {
		artifactPath = filepath.ToSlash(artifactPath)
		key := artifactRelativePathKey(artifactPath)
		if index, ok := byPath[key]; ok {
			if (*records)[index].DeliveredAt < deliveredAt {
				(*records)[index].DeliveredAt = deliveredAt
				changed = true
			}
			continue
		}
		byPath[key] = len(*records)
		*records = append(*records, artifactSidecarRecord{Path: artifactPath, DeliveredAt: deliveredAt})
		changed = true
	}
	if !changed {
		return nil
	}
	raw, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, raw, 0o600)
}

func mergeArtifactSidecar(sessionPath string, paths []string, deliveredAt int64) error {
	path := artifactSidecarPath(sessionPath)
	sidecar, exists, err := readArtifactSidecar(path)
	if err != nil {
		return err
	}
	sidecar, changed, err := activateArtifactSidecar(sidecar, exists)
	if err != nil {
		return err
	}
	byPath := make(map[string]int, len(sidecar.Artifacts))
	for index, artifact := range sidecar.Artifacts {
		byPath[artifactRelativePathKey(artifact.Path)] = index
	}
	for _, artifactPath := range paths {
		artifactPath = filepath.ToSlash(artifactPath)
		key := artifactRelativePathKey(artifactPath)
		if index, ok := byPath[key]; ok {
			if sidecar.Artifacts[index].DeliveredAt < deliveredAt {
				sidecar.Artifacts[index].DeliveredAt = deliveredAt
				changed = true
			}
			continue
		}
		byPath[key] = len(sidecar.Artifacts)
		sidecar.Artifacts = append(sidecar.Artifacts, artifactSidecarRecord{Path: artifactPath, DeliveredAt: deliveredAt})
		changed = true
	}
	if !changed {
		return nil
	}
	raw, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, raw, 0o600)
}

func readArtifactSidecar(path string) (artifactSidecar, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return artifactSidecar{}, false, nil
	}
	if err != nil {
		return artifactSidecar{}, false, err
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		return artifactSidecar{}, false, fmt.Errorf("decode artifact sidecar: %w", err)
	}
	return sidecar, true, nil
}

func activateArtifactSidecar(sidecar artifactSidecar, exists bool) (artifactSidecar, bool, error) {
	if !exists {
		return artifactSidecar{
			Version:                 artifactSidecarVersion,
			ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
		}, true, nil
	}
	switch sidecar.Version {
	case legacyArtifactSidecarVersion:
		return artifactSidecar{
			Version:                 artifactSidecarVersion,
			ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
			LegacyArtifacts:         append([]artifactSidecarRecord(nil), sidecar.Artifacts...),
		}, true, nil
	case artifactSidecarVersion:
		if sidecar.ArtifactDeliveryVersion != provider.ArtifactDeliveryVersion {
			return artifactSidecar{}, false, fmt.Errorf(
				"unsupported artifact delivery version %d", sidecar.ArtifactDeliveryVersion,
			)
		}
		return sidecar, false, nil
	default:
		return artifactSidecar{}, false, fmt.Errorf("unsupported artifact sidecar version %d", sidecar.Version)
	}
}

func artifactSidecarUsesDeliveryContract(sidecar artifactSidecar) bool {
	return sidecar.Version == artifactSidecarVersion &&
		sidecar.ArtifactDeliveryVersion == provider.ArtifactDeliveryVersion
}

func artifactRelativePathKey(path string) string {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
	if os.PathSeparator == '\\' {
		path = strings.ToLower(path)
	}
	return path
}

func artifactRecordsForSession(sessionPath, workspaceRoot string) []artifactSidecarRecord {
	records := make([]artifactSidecarRecord, 0)
	byPath := make(map[string]int)
	modernPaths := make(map[string]bool)
	appendRecord := func(record artifactSidecarRecord) {
		path, ok := normalizedWorkspaceArtifactPath(workspaceRoot, record.Path)
		if !ok {
			return
		}
		record.Path = path
		key := artifactRelativePathKey(path)
		if index, exists := byPath[key]; exists {
			if records[index].DeliveredAt < record.DeliveredAt {
				records[index].DeliveredAt = record.DeliveredAt
			}
			return
		}
		byPath[key] = len(records)
		records = append(records, record)
	}
	appendModernRecord := func(record artifactSidecarRecord) {
		path, ok := normalizedWorkspaceArtifactPath(workspaceRoot, record.Path)
		if !ok {
			return
		}
		record.Path = path
		key := artifactRelativePathKey(path)
		if index, exists := byPath[key]; exists {
			if !modernPaths[key] || records[index].DeliveredAt < record.DeliveredAt {
				records[index].DeliveredAt = record.DeliveredAt
			}
			modernPaths[key] = true
			return
		}
		modernPaths[key] = true
		byPath[key] = len(records)
		records = append(records, record)
	}

	fallbackDeliveredAt := int64(0)
	if info, err := os.Stat(sessionPath); err == nil {
		fallbackDeliveredAt = info.ModTime().UnixMilli()
	}
	sidecarPath := artifactSidecarPath(sessionPath)
	sidecar, sidecarExists, sidecarErr := readArtifactSidecar(sidecarPath)
	appendLegacyHistory := func() {
		if sidecarErr != nil || !sidecarExists {
			return
		}
		switch {
		case artifactSidecarUsesDeliveryContract(sidecar):
			for _, record := range sidecar.LegacyArtifacts {
				appendRecord(record)
			}
		case sidecar.Version == legacyArtifactSidecarVersion:
			for _, record := range sidecar.Artifacts {
				appendRecord(record)
			}
		}
	}

	loadedContracts, err := agent.LoadArtifactDeliveryContracts(sessionPath)
	if err != nil {
		// A corrupt transcript cannot make a modern projection authoritative,
		// but it also must not erase independently persisted legacy history.
		appendLegacyHistory()
		return records
	}
	deliveries := loadedContracts.Deliveries

	deliveryContractActive := sidecarErr == nil && sidecarExists && artifactSidecarUsesDeliveryContract(sidecar)
	validContractExists := len(deliveries) != 0

	// The first persisted v1 contract upgrades the sidecar even when its path
	// list is explicitly empty. Version 2 separates the one-time legacy import
	// from the modern projection and is the durable activation marker after a
	// rewind or compaction removes every current JSONL contract.
	if validContractExists && !deliveryContractActive {
		if sidecarErr == nil {
			if activated, _, activateErr := activateArtifactSidecar(sidecar, sidecarExists); activateErr == nil {
				sidecar = activated
				if err := mergeArtifactSidecar(sessionPath, nil, 0); err != nil {
					recordDesktopBackgroundFailure("session.artifacts.activate", err)
				}
			}
		}
	}

	// An invalid-only transcript is an explicit suppression marker even after
	// a v2 sidecar has been activated. LegacyArtifacts may otherwise resurrect
	// files that the current host cannot safely interpret as delivered.
	if !validContractExists && loadedContracts.HasContract() {
		return records
	}

	appendLegacyHistory()
	for _, delivery := range deliveries {
		deliveredAt := delivery.DeliveredAt
		if deliveredAt <= 0 {
			deliveredAt = fallbackDeliveredAt
		}
		for _, path := range delivery.Paths {
			appendModernRecord(artifactSidecarRecord{Path: path, DeliveredAt: deliveredAt})
		}
	}
	return records
}

// ListArtifacts returns live files delivered by every visible saved session.
func (a *App) ListArtifacts() []ArtifactView {
	workspaceNames := make(map[string]string)
	for _, workspace := range a.ListWorkspaces() {
		workspaceNames[cleanDesktopPath(workspace.Path)] = workspace.Name
	}
	out := make([]ArtifactView, 0)
	sessions, cloudProjectNames := a.artifactLibrarySessions()
	for _, session := range sessions {
		workspaceRoot := strings.TrimSpace(session.WorkspaceRoot)
		if workspaceRoot == "" && session.Scope != "project" {
			workspaceRoot = globalWorkspaceRoot()
			session.WorkspaceRoot = workspaceRoot
		}
		if workspaceRoot == "" {
			continue
		}
		workspaceName := artifactWorkspaceName(session, workspaceRoot, workspaceNames)
		if name := strings.TrimSpace(cloudProjectNames[sessionRuntimeKey(session.Path)]); name != "" {
			workspaceName = name
		}
		for _, artifact := range artifactRecordsForSession(session.Path, workspaceRoot) {
			absolute, ok, err := workspacePathForBase(workspaceRoot, artifact.Path)
			if err != nil || !ok {
				continue
			}
			info, err := os.Stat(absolute)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(artifact.Path)), ".")
			out = append(out, ArtifactView{
				Path:          filepath.ToSlash(artifact.Path),
				Name:          filepath.Base(artifact.Path),
				Extension:     extension,
				Type:          artifactFileType(extension),
				Size:          info.Size(),
				ModifiedAt:    info.ModTime().UnixMilli(),
				DeliveredAt:   artifact.DeliveredAt,
				SessionTitle:  artifactSessionTitle(session),
				WorkspaceName: workspaceName,
				Session:       session,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeliveredAt > out[j].DeliveredAt })
	return out
}

func (a *App) artifactLibrarySessions() ([]SessionMeta, map[string]string) {
	sessions := a.ListAllSessions()
	infos := make(map[string]agent.SessionInfo, len(sessions))
	for _, session := range sessions {
		infos[sessionRuntimeKey(session.Path)] = agent.SessionInfo{
			Path: session.Path, Scope: session.Scope, WorkspaceRoot: session.WorkspaceRoot, TopicID: session.TopicID,
		}
	}
	catalog := a.cloudProjectTreeCatalog(infos)
	visibleCloudSessions := make(map[string]bool)
	cloudProjectNames := make(map[string]string)
	projectNames := make(map[string]string, len(catalog.projects))
	for _, project := range catalog.projects {
		name := strings.TrimSpace(project.Name)
		if name == "" {
			name = project.ProjectID
		}
		projectNames[project.ProjectID] = name
	}
	for projectID, projectSessions := range catalog.sessionsByProject {
		for _, session := range projectSessions {
			key := sessionRuntimeKey(session.info.Path)
			visibleCloudSessions[key] = true
			cloudProjectNames[key] = projectNames[projectID]
		}
	}
	visible := make([]SessionMeta, 0, len(sessions))
	for _, session := range sessions {
		key := sessionRuntimeKey(session.Path)
		// A workspace path is not project ownership: only pinned cloud sessions
		// follow the sidebar's current project directory and account scope.
		if catalog.cloudSessionPaths[key] && !visibleCloudSessions[key] {
			continue
		}
		visible = append(visible, session)
	}
	return visible, cloudProjectNames
}

func artifactWorkspaceName(session SessionMeta, workspaceRoot string, names map[string]string) string {
	if session.Scope != "project" {
		return globalProjectTitle()
	}
	if name := strings.TrimSpace(names[cleanDesktopPath(workspaceRoot)]); name != "" {
		return name
	}
	return workspaceName(workspaceRoot)
}

func artifactSessionTitle(session SessionMeta) string {
	for _, title := range []string{session.Title, session.TopicTitle, session.Preview} {
		if title = strings.TrimSpace(title); title != "" {
			return title
		}
	}
	return "未命名对话"
}

func artifactFileType(extension string) string {
	switch extension {
	case "md", "markdown":
		return "markdown"
	case "pdf":
		return "pdf"
	case "doc", "docx", "txt", "rtf":
		return "document"
	case "xls", "xlsx", "xlsm", "csv", "tsv":
		return "spreadsheet"
	case "ppt", "pptx":
		return "presentation"
	case "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico":
		return "image"
	case "mp4", "mov", "mkv", "avi", "webm":
		return "video"
	case "mp3", "wav", "m4a", "aac", "flac", "ogg":
		return "audio"
	case "go", "py", "js", "jsx", "ts", "tsx", "java", "kt", "rs", "c", "cc", "cpp", "h", "hpp", "cs", "rb", "php", "swift", "sh", "sql", "json", "yaml", "yml", "toml", "xml", "html", "htm", "css", "scss", "vue", "svelte":
		return "code"
	default:
		return "other"
	}
}
