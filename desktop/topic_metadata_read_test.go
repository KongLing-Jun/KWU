package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
)

func isolateMetadataReads(t *testing.T) {
	t.Helper()
	oldReadFile := metadataReadFile
	oldSlots := readFileWithTimeoutSlots
	metadataReadFile = os.ReadFile
	readFileWithTimeoutSlots = make(chan struct{}, 16)
	projectTreeMetadataSnapshots.reset()
	t.Cleanup(func() {
		metadataReadFile = oldReadFile
		readFileWithTimeoutSlots = oldSlots
		projectTreeMetadataSnapshots.reset()
	})
}

func writeMetadataFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTopicMetadataReadClassifiesSuccessMissingCorruptPathAndIOErrors(t *testing.T) {
	isolateMetadataReads(t)
	root := t.TempDir()
	path := filepath.Join(root, topicTitlesFile)
	resolver := func(string, string) (string, error) { return path, nil }

	writeMetadataFixture(t, path, `{"topic_a":"Persisted title"}`)
	result := loadTopicStringMap(root, topicTitlesFile, resolver)
	if result.status != metadataReadSuccess || result.values["topic_a"] != "Persisted title" || result.usedSnapshot {
		t.Fatalf("success result = %#v", result)
	}

	missingPath := filepath.Join(root, "missing.json")
	missing := loadTopicStringMap(root, topicTitlesFile, func(string, string) (string, error) { return missingPath, nil })
	if missing.status != metadataReadFileNotFound || len(missing.values) != 0 || missing.usedSnapshot {
		t.Fatalf("missing result = %#v", missing)
	}

	writeMetadataFixture(t, path, `{not-json`)
	corrupt := loadTopicStringMap(root, topicTitlesFile, resolver)
	if corrupt.status != metadataReadInvalidJSON || corrupt.values["topic_a"] != "Persisted title" || !corrupt.usedSnapshot {
		t.Fatalf("corrupt result = %#v", corrupt)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missingAfterSuccess := loadTopicStringMap(root, topicTitlesFile, resolver)
	if missingAfterSuccess.status != metadataReadFileNotFound || missingAfterSuccess.values["topic_a"] != "Persisted title" || !missingAfterSuccess.shouldRetry() {
		t.Fatalf("missing-after-success result = %#v", missingAfterSuccess)
	}

	pathFailure := loadTopicStringMap(root, topicTitlesFile, func(string, string) (string, error) {
		return "", errors.New("profile unavailable")
	})
	if pathFailure.status != metadataReadPathError || len(pathFailure.values) != 0 {
		t.Fatalf("path failure result = %#v", pathFailure)
	}

	metadataReadFile = func(string) ([]byte, error) { return nil, errors.New("temporary I/O failure") }
	ioFailure := loadTopicStringMap(root, topicTitlesFile, resolver)
	if ioFailure.status != metadataReadOtherError || ioFailure.values["topic_a"] != "Persisted title" || !ioFailure.usedSnapshot {
		t.Fatalf("I/O failure result = %#v", ioFailure)
	}
}

func TestTopicMetadataReadWaitsForSlotsAndUsesLastKnownGoodOnSlotTimeout(t *testing.T) {
	isolateMetadataReads(t)
	path := filepath.Join(t.TempDir(), topicTitlesFile)
	writeMetadataFixture(t, path, `{"topic_a":"Known title"}`)
	resolver := func(string, string) (string, error) { return path, nil }
	if result := loadTopicStringMap("", topicTitlesFile, resolver); result.status != metadataReadSuccess {
		t.Fatalf("prime status = %s", result.status)
	}
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		readFileWithTimeoutSlots <- struct{}{}
	}
	started := time.Now()
	result := loadTopicStringMap("", topicTitlesFile, resolver)
	if result.status != metadataReadSlotWaitTimeout || result.values["topic_a"] != "Known title" || !result.usedSnapshot {
		t.Fatalf("slot timeout result = %#v", result)
	}
	if elapsed := time.Since(started); elapsed < topicFileReadTimeout/2 {
		t.Fatalf("slot acquisition failed fast after %v", elapsed)
	}
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		<-readFileWithTimeoutSlots
	}
}

func TestTopicMetadataReadTimeoutRetainsSnapshotThenRecoversAuthority(t *testing.T) {
	isolateMetadataReads(t)
	path := filepath.Join(t.TempDir(), topicTitlesFile)
	writeMetadataFixture(t, path, `{"topic_a":"Old authority"}`)
	resolver := func(string, string) (string, error) { return path, nil }
	if result := loadTopicStringMap("", topicTitlesFile, resolver); result.status != metadataReadSuccess {
		t.Fatalf("prime status = %s", result.status)
	}

	release := make(chan struct{})
	readFinished := make(chan struct{})
	metadataReadFile = func(readPath string) ([]byte, error) {
		<-release
		defer close(readFinished)
		return os.ReadFile(readPath)
	}
	timedOut := loadTopicStringMap("", topicTitlesFile, resolver)
	if timedOut.status != metadataReadTimeout || timedOut.values["topic_a"] != "Old authority" || !timedOut.usedSnapshot {
		t.Fatalf("timeout result = %#v", timedOut)
	}
	writeMetadataFixture(t, path, `{"topic_a":"New authority"}`)
	close(release)
	<-readFinished
	metadataReadFile = os.ReadFile
	recovered := loadTopicStringMap("", topicTitlesFile, resolver)
	if recovered.status != metadataReadSuccess || recovered.values["topic_a"] != "New authority" || recovered.usedSnapshot {
		t.Fatalf("recovered result = %#v", recovered)
	}
}

func TestTopicMetadataColdReadFailureHasNoIdentityFallback(t *testing.T) {
	isolateMetadataReads(t)
	path := filepath.Join(t.TempDir(), topicTitlesFile)
	resolver := func(string, string) (string, error) { return path, nil }
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		readFileWithTimeoutSlots <- struct{}{}
	}
	result := loadTopicStringMap("", topicTitlesFile, resolver)
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		<-readFileWithTimeoutSlots
	}
	if result.status != metadataReadSlotWaitTimeout || len(result.values) != 0 || result.usedSnapshot {
		t.Fatalf("cold failure result = %#v", result)
	}
	if got := safeSessionTreeLabel("legacy_internal", "legacy_internal", ""); got != defaultTopicTitle {
		t.Fatalf("safe fallback = %q, want %q", got, defaultTopicTitle)
	}
	if got := runtimeSessionTreeLabel(&WorkspaceTab{TopicID: "topic_internal", TopicTitle: "topic_internal", SessionPath: "legacy_session.jsonl"}, agentSessionInfoWithPath("legacy_session.jsonl"), ""); got != defaultTopicTitle {
		t.Fatalf("runtime fallback = %q, want %q", got, defaultTopicTitle)
	}
}

func TestRuntimeSessionTreeLabelPrefersTopicTitleOverInternalPreview(t *testing.T) {
	tab := &WorkspaceTab{TopicID: "automation-topic", TopicTitle: "每日办公软件技巧"}
	info := agent.SessionInfo{Preview: "# Automation Task\n\nYou are running an automation task."}
	if got := runtimeSessionTreeLabel(tab, info, ""); got != "每日办公软件技巧" {
		t.Fatalf("runtime automation label = %q, want task title", got)
	}
}

func TestCloudProjectSessionTreeLabelPreservesTitleAfterRuntimeSwitch(t *testing.T) {
	meta := agent.BranchMeta{TopicID: "cloud-topic", TopicTitle: "整理季度经营复盘"}
	info := agent.SessionInfo{TopicID: "cloud-topic", TopicTitle: defaultTopicTitle, Preview: "原始用户消息"}
	if got := cloudProjectSessionTreeLabel(meta, info, ""); got != meta.TopicTitle {
		t.Fatalf("inactive cloud project label = %q, want durable topic title %q", got, meta.TopicTitle)
	}
	if got := cloudProjectSessionTreeLabel(meta, info, "自定义会话标题"); got != "自定义会话标题" {
		t.Fatalf("inactive cloud project custom label = %q, want custom session title", got)
	}
	meta.TopicTitle = defaultTopicTitle
	info.Preview = "11"
	if got := cloudProjectSessionTreeLabel(meta, info, ""); got != "11" {
		t.Fatalf("legacy cloud project label = %q, want recovered preview title", got)
	}
}

func agentSessionInfoWithPath(path string) agent.SessionInfo {
	return agent.SessionInfo{Path: path}
}

func TestTopicMetadataConcurrentReadsAreBoundedAndCompleteBeyondSlotCapacity(t *testing.T) {
	isolateMetadataReads(t)
	const count = 40
	root := t.TempDir()
	var active atomic.Int32
	var maximum atomic.Int32
	metadataReadFile = func(path string) ([]byte, error) {
		current := active.Add(1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return os.ReadFile(path)
	}

	results := make([]metadataStringMapResult, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		path := filepath.Join(root, fmt.Sprintf("project-%02d", i), topicTitlesFile)
		writeMetadataFixture(t, path, fmt.Sprintf(`{"topic_%02d":"Title %02d"}`, i, i))
		wg.Add(1)
		go func(index int, readPath string) {
			defer wg.Done()
			results[index] = readMetadataStringMap(readPath)
		}(i, path)
	}
	wg.Wait()
	if got := maximum.Load(); got > int32(cap(readFileWithTimeoutSlots)) {
		t.Fatalf("maximum concurrent reads = %d, capacity = %d", got, cap(readFileWithTimeoutSlots))
	}
	for i, result := range results {
		key := fmt.Sprintf("topic_%02d", i)
		if result.status != metadataReadSuccess || result.values[key] != fmt.Sprintf("Title %02d", i) {
			t.Fatalf("result %d = %#v", i, result)
		}
	}
}

func TestTopicMetadataSnapshotsArePathScopedAndLogsRedactPathsAndTitles(t *testing.T) {
	isolateMetadataReads(t)
	root := t.TempDir()
	pathA := filepath.Join(root, "account-a", topicTitlesFile)
	pathB := filepath.Join(root, "account-b", topicTitlesFile)
	writeMetadataFixture(t, pathA, `{"topic":"Secret A"}`)
	writeMetadataFixture(t, pathB, `{"topic":"Secret B"}`)
	if result := readMetadataStringMap(pathA); result.status != metadataReadSuccess {
		t.Fatalf("prime A = %#v", result)
	}
	if result := readMetadataStringMap(pathB); result.status != metadataReadSuccess {
		t.Fatalf("prime B = %#v", result)
	}

	previous := slog.Default()
	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	metadataReadFile = func(string) ([]byte, error) { return nil, errors.New("temporary failure") }
	resultA := loadTopicStringMap("workspace-a", topicTitlesFile, func(string, string) (string, error) { return pathA, nil })
	resultB := loadTopicStringMap("workspace-b", topicTitlesFile, func(string, string) (string, error) { return pathB, nil })
	if !reflect.DeepEqual(resultA.values, map[string]string{"topic": "Secret A"}) || !reflect.DeepEqual(resultB.values, map[string]string{"topic": "Secret B"}) {
		t.Fatalf("scoped snapshots crossed: A=%v B=%v", resultA.values, resultB.values)
	}
	logText := output.String()
	for _, forbidden := range []string{root, pathA, pathB, "Secret A", "Secret B"} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("metadata log exposed sensitive content %q: %s", forbidden, logText)
		}
	}
	if !strings.Contains(logText, `status=other_error`) || !strings.Contains(logText, `last_known_good=true`) {
		t.Fatalf("metadata failure was not observable: %s", logText)
	}
}

func TestListProjectTreeReadsPersistedProjectTopicTitles(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	projectRoot := t.TempDir()
	if err := addProject(projectRoot, "Project"); err != nil {
		t.Fatal(err)
	}
	if afterAdd := loadProjectsFile(); len(afterAdd.Projects) != 1 {
		t.Fatalf("projects immediately after add = %#v; dir=%q", afterAdd, desktopConfigDir())
	}
	if err := setTopicTitle(projectRoot, "topic_persisted", "Persisted user title"); err != nil {
		t.Fatal(err)
	}
	if afterTitle := loadProjectsFile(); len(afterTitle.Projects) != 1 {
		t.Fatalf("projects after title write = %#v; dir=%q", afterTitle, desktopConfigDir())
	}
	titlesResult := loadTopicTitlesResult(projectRoot)
	if titlesResult.status != metadataReadSuccess || titlesResult.values["topic_persisted"] != "Persisted user title" {
		t.Fatalf("persisted title read = %#v", titlesResult)
	}
	f := loadProjectsFile()
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v; projects = %#v; titles = %#v", nodes, f, titlesResult)
	}
	if got := nodes[0].Children[0].Label; got != "Persisted user title" {
		t.Fatalf("project topic label = %q", got)
	}
}

func TestListProjectTreeKeepsLastKnownGoodTitlesWhenReadSlotsAreOccupied(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	projectRoot := t.TempDir()
	if err := saveProjectsFile(desktopProjectFile{
		GlobalTopics: []string{"topic_global"},
		Projects:     []desktopProject{{Root: projectRoot, Title: "Project", Topics: []string{"topic_project"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicTitles("", map[string]string{"topic_global": "Global authority"}); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicTitles(projectRoot, map[string]string{"topic_project": "Project authority"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		readFileWithTimeoutSlots <- struct{}{}
	}
	nodes := app.ListProjectTree()
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		<-readFileWithTimeoutSlots
	}
	labels := map[string]string{}
	for _, parent := range nodes {
		for _, child := range parent.Children {
			labels[child.TopicID] = child.Label
		}
	}
	if labels["topic_global"] != "Global authority" || labels["topic_project"] != "Project authority" {
		t.Fatalf("project tree lost last-known-good titles: labels=%v nodes=%#v", labels, nodes)
	}
}

func TestListProjectTreeColdMetadataFailureUsesSafeTopicFallbacks(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	projectRoot := t.TempDir()
	if err := saveProjectsFile(desktopProjectFile{
		GlobalTopics: []string{"legacy_global_internal"},
		Projects:     []desktopProject{{Root: projectRoot, Title: "Project", Topics: []string{"topic_project_internal"}}},
	}); err != nil {
		t.Fatal(err)
	}
	projectTreeMetadataSnapshots.reset()
	projectSessionCache.invalidate()
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		readFileWithTimeoutSlots <- struct{}{}
	}
	nodes := app.ListProjectTree()
	for i := 0; i < cap(readFileWithTimeoutSlots); i++ {
		<-readFileWithTimeoutSlots
	}
	for _, parent := range nodes {
		for _, child := range parent.Children {
			if child.Label != defaultTopicTitle {
				t.Fatalf("cold fallback exposed internal identity: %#v", child)
			}
		}
	}
}

func TestListProjectTreeLoadsAllProjectTitlesWithMoreProjectsThanReadSlots(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	const projectCount = 24
	projects := make([]desktopProject, 0, projectCount)
	for i := 0; i < projectCount; i++ {
		root := filepath.Join(t.TempDir(), fmt.Sprintf("workspace-%02d", i))
		topicID := fmt.Sprintf("topic_%02d", i)
		projects = append(projects, desktopProject{Root: root, Title: fmt.Sprintf("Project %02d", i), Topics: []string{topicID}})
		if err := saveTopicTitles(root, map[string]string{topicID: fmt.Sprintf("Authority %02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveProjectsFile(desktopProjectFile{Projects: projects}); err != nil {
		t.Fatal(err)
	}
	projectTreeMetadataSnapshots.reset()
	projectSessionCache.invalidate()
	var active atomic.Int32
	var maximum atomic.Int32
	metadataReadFile = func(path string) ([]byte, error) {
		current := active.Add(1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return os.ReadFile(path)
	}
	nodes := app.ListProjectTree()
	if got := maximum.Load(); got > int32(cap(readFileWithTimeoutSlots)) {
		t.Fatalf("maximum concurrent project-tree reads = %d", got)
	}
	seen := map[string]string{}
	for _, parent := range nodes {
		for _, child := range parent.Children {
			seen[child.TopicID] = child.Label
		}
	}
	if len(seen) != projectCount {
		t.Fatalf("loaded %d project topics, want %d; nodes=%#v", len(seen), projectCount, nodes)
	}
	for i := 0; i < projectCount; i++ {
		topicID := fmt.Sprintf("topic_%02d", i)
		if got, want := seen[topicID], fmt.Sprintf("Authority %02d", i); got != want {
			t.Fatalf("%s label = %q, want %q", topicID, got, want)
		}
	}
}

func TestListProjectTreeMetadataFailureSchedulesRecoveryRefresh(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	projectRoot := t.TempDir()
	topicID := "topic_recovery"
	if err := saveProjectsFile(desktopProjectFile{Projects: []desktopProject{{Root: projectRoot, Title: "Project", Topics: []string{topicID}}}}); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicTitles(projectRoot, map[string]string{topicID: "Old authority"}); err != nil {
		t.Fatal(err)
	}
	titlePath, err := topicTitlesPath(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	var failTitleRead atomic.Bool
	failTitleRead.Store(true)
	metadataReadFile = func(path string) ([]byte, error) {
		if failTitleRead.Load() && metadataSnapshotKey(path) == metadataSnapshotKey(titlePath) {
			return nil, errors.New("temporary metadata failure")
		}
		return os.ReadFile(path)
	}
	refreshed := make(chan struct{}, 1)
	app.projectTreeChangedHook = func() {
		select {
		case refreshed <- struct{}{}:
		default:
		}
	}
	first := app.ListProjectTree()
	if len(first) != 1 || len(first[0].Children) != 1 || first[0].Children[0].Label != "Old authority" {
		t.Fatalf("transient read did not retain authority: %#v", first)
	}
	writeMetadataFixture(t, titlePath, `{"topic_recovery":"New authority"}`)
	failTitleRead.Store(false)
	select {
	case <-refreshed:
	case <-time.After(2 * time.Second):
		t.Fatal("metadata failure did not schedule a project-tree refresh")
	}
	second := app.ListProjectTree()
	if len(second) != 1 || len(second[0].Children) != 1 || second[0].Children[0].Label != "New authority" {
		t.Fatalf("recovery refresh did not load new authority: %#v", second)
	}
}

func TestListProjectTreeRepeatedRefreshKeepsStableAuthorityAcrossIntermittentReads(t *testing.T) {
	isolateDesktopUserDirs(t)
	isolateMetadataReads(t)
	app := NewApp()
	projectRoot := t.TempDir()
	if err := saveProjectsFile(desktopProjectFile{
		GlobalTopics: []string{"topic_global_stress"},
		Projects:     []desktopProject{{Root: projectRoot, Title: "Project", Topics: []string{"topic_project_stress"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicTitles("", map[string]string{"topic_global_stress": "Stable global title"}); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicTitles(projectRoot, map[string]string{"topic_project_stress": "Stable project title"}); err != nil {
		t.Fatal(err)
	}
	var titleReads atomic.Int32
	metadataReadFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == topicTitlesFile && titleReads.Add(1)%7 == 0 {
			return nil, errors.New("intermittent cold-read failure")
		}
		return os.ReadFile(path)
	}
	for refresh := 0; refresh < 50; refresh++ {
		nodes := app.ListProjectTree()
		labels := map[string]string{}
		for _, parent := range nodes {
			for _, child := range parent.Children {
				labels[child.TopicID] = child.Label
			}
		}
		if labels["topic_global_stress"] != "Stable global title" || labels["topic_project_stress"] != "Stable project title" {
			t.Fatalf("refresh %d changed authority: labels=%v nodes=%#v", refresh, labels, nodes)
		}
	}
	metadataReadFile = os.ReadFile
	nodes := app.ListProjectTree()
	for _, parent := range nodes {
		for _, child := range parent.Children {
			if strings.Contains(child.Label, child.TopicID) || child.Label == "" {
				t.Fatalf("recovered refresh exposed internal identity: %#v", child)
			}
		}
	}
}
