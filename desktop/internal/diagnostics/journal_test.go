package diagnostics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandlerKeepsDelegateAndCapturesErrorAndMarkedWarning(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	var delegate bytes.Buffer
	logger := slog.New(NewHandler(slog.NewTextHandler(&delegate, nil), j))
	logger.Error("save failed", "err", errors.New("disk cause"), "password", "delegate-only")
	logger.Warn("recovery degraded", FailureMarkerKey, FailureMarker, "scope", "desktop.startup", "err", errors.New("snapshot cause"))
	logger.Warn("ordinary warning", "err", errors.New("not selected"))
	j.Close()
	if !strings.Contains(delegate.String(), "delegate-only") {
		t.Fatalf("delegate did not receive original records: %s", delegate.String())
	}
	body := readHumanJournal(t, root)
	for _, want := range []string{"go.error | save failed: *errors.errorString: disk cause", "desktop.startup | recovery degraded: *errors.errorString: snapshot cause"} {
		if !strings.Contains(body, want) {
			t.Fatalf("journal missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"delegate-only", "ordinary warning", "not selected"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("journal persisted %q: %s", forbidden, body)
		}
	}
}

func TestHandlerCapturesMessageWhenErrIsAbsent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	logger := slog.New(NewHandler(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}), j))
	logger.Error("bootstrap failed without err", "code", "startup_failed")
	j.Close()
	body := readHumanJournal(t, root)
	if !strings.Contains(body, "go.error [startup_failed] | bootstrap failed without err") {
		t.Fatalf("slog message was not preserved as the cause: %s", body)
	}
}

func TestAuthFailureEnqueueIsNonblockingAndStructured(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	originalMkdir := ops.mkdirAll
	ops.mkdirAll = func(path string, mode os.FileMode) error {
		once.Do(func() { close(entered) })
		<-release
		return originalMkdir(path, mode)
	}
	j := New(root, withFileOps(ops))
	h := NewHandler(slog.NewTextHandler(&bytes.Buffer{}, nil), j).(*fanoutHandler)
	if !h.EnqueueAuthFailure([]slog.Attr{
		slog.String(SlogSourceKey, "cloud"), slog.String(SlogEventKey, "auth.failure"),
		slog.String("code", "TOKEN_EXPIRED"), slog.String("request_id", "req-auth"), slog.Int("http_status", 401),
	}) {
		t.Fatal("first auth error was not accepted")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach blocked disk")
	}
	started := time.Now()
	if !h.EnqueueAuthFailure([]slog.Attr{
		slog.String(SlogSourceKey, "cloud"), slog.String(SlogEventKey, "auth.failure"), slog.String("code", "AUTH_REQUIRED"),
	}) {
		t.Fatal("second auth error was not accepted")
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("auth enqueue blocked for %v", elapsed)
	}
	close(release)
	j.Close()
	body := readHumanJournal(t, root)
	if !strings.Contains(body, "cloud.auth [TOKEN_EXPIRED] | authentication failure | request_id=req-auth status=401") || strings.Count(body, "cloud.auth") != 1 {
		t.Fatalf("auth projection = %s", body)
	}
}

func TestJournalQueueFullAndClosedReturnNoID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	ops.mkdirAll = func(string, os.FileMode) error {
		once.Do(func() { close(entered) })
		<-release
		return errors.New("blocked")
	}
	j := New(root, withFileOps(ops))
	firstID := j.RecordWithID(Event{Operation: "queue.first", Cause: errors.New("first")})
	if firstID == "" {
		t.Fatal("first event rejected")
	}
	prefix, _, ok := strings.Cut(firstID, "-")
	if !ok {
		t.Fatalf("first ID %q has no session prefix", firstID)
	}
	<-entered
	accepted, dropped := 0, 0
	for index := 0; index < maxQueue+8; index++ {
		if j.RecordWithID(Event{Operation: "queue.fill", Cause: fmt.Errorf("cause-%d", index), DiagnosticID: prefix + "-zzzzzz"}) == "" {
			dropped++
		} else {
			accepted++
		}
	}
	if accepted != maxQueue || dropped == 0 {
		t.Fatalf("accepted=%d dropped=%d", accepted, dropped)
	}
	if j.OwnsLogID(prefix + "-zzzzzz") {
		t.Fatal("queue-full caller-supplied ID became owned")
	}
	close(release)
	var afterFullID string
	for afterFullID == "" {
		afterFullID = j.RecordWithID(Event{Operation: "queue.after-full", Cause: errors.New("accepted later"), DiagnosticID: prefix + "-zzzzzz"})
		runtime.Gosched()
	}
	if afterFullID == prefix+"-zzzzzz" || !j.OwnsLogID(afterFullID) {
		t.Fatalf("accepted ID = %q, caller-supplied rejected ID was reused or accepted ID is not owned", afterFullID)
	}
	j.Close()
	if id := j.RecordWithID(Event{Operation: "after.close", Cause: errors.New("closed")}); id != "" {
		t.Fatalf("closed journal returned ID %q", id)
	}
}

func TestJournalDoesNotOwnUnissuedCurrentSessionID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	issued := j.RecordWithID(Event{Operation: "issued", Cause: errors.New("issued cause")})
	if issued == "" {
		t.Fatal("accepted event returned no ID")
	}
	prefix, _, ok := strings.Cut(issued, "-")
	if !ok {
		t.Fatalf("issued ID %q has no session prefix", issued)
	}
	unissued := prefix + "-zzzzzz"
	if j.OwnsLogID(unissued) {
		t.Fatalf("journal owns never-issued ID %q", unissued)
	}
	alias := prefix + "-00001"
	if j.OwnsLogID(alias) {
		t.Fatalf("journal owns noncanonical alias %q for issued ID %q", alias, issued)
	}
	j.Close()
}

func TestJournalConcurrentRecordWithIDIsUniqueAndOwnedOnReturn(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	const count = 128
	ids := make(chan string, count)
	var wg sync.WaitGroup
	for index := 0; index < count; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			id := j.RecordWithID(Event{Operation: "concurrent.record", Cause: fmt.Errorf("cause-%d", index)})
			if id == "" {
				t.Errorf("record %d was rejected", index)
				return
			}
			if !j.OwnsLogID(id) {
				t.Errorf("accepted ID %q is not owned on return", id)
			}
			ids <- id
		}(index)
	}
	wg.Wait()
	close(ids)
	seen := make(map[string]struct{}, count)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Errorf("duplicate accepted ID %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("unique accepted IDs = %d, want %d", len(seen), count)
	}
	j.Close()
}

func TestJournalCloseWithAcceptedEventAwaitingIDIsBounded(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	accepted, release := make(chan struct{}), make(chan struct{})
	j := New(root, withRecordObserver(func() {
		close(accepted)
		<-release
	}))
	idResult := make(chan string, 1)
	go func() {
		idResult <- j.RecordWithID(Event{Operation: "close.inflight", Cause: errors.New("inflight cause")})
	}()
	<-accepted
	started := time.Now()
	j.Close()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Close blocked for %v with an accepted event awaiting ID publication", elapsed)
	}
	close(release)
	select {
	case id := <-idResult:
		if id == "" || !j.OwnsLogID(id) {
			t.Fatalf("accepted inflight event returned unowned ID %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted inflight RecordWithID did not finish")
	}
	select {
	case <-j.done:
	case <-time.After(time.Second):
		t.Fatal("journal worker did not finish after inflight publication")
	}
}

func TestJournalCloseIsConcurrentAndBounded(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	ops.mkdirAll = func(string, os.FileMode) error {
		once.Do(func() { close(entered) })
		<-release
		return errors.New("blocked")
	}
	j := New(root, withFileOps(ops))
	j.Record(Event{Operation: "close.block", Cause: errors.New("cause")})
	<-entered
	started := time.Now()
	var wg sync.WaitGroup
	for index := 0; index < 8; index++ {
		wg.Add(1)
		go func() { defer wg.Done(); j.Close() }()
	}
	wg.Wait()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("concurrent Close took %v", elapsed)
	}
	close(release)
}

func TestJournalRotatesAtTwoMiBParts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	originalWrite := ops.write
	allWritten := make(chan struct{})
	var writeCount atomic.Int32
	ops.write = func(file *os.File, payload []byte) (int, error) {
		written, err := originalWrite(file, payload)
		if err == nil && written == len(payload) && writeCount.Add(1) == 1120 {
			close(allWritten)
		}
		return written, err
	}
	j := New(root, withFileOps(ops))
	for index := 0; index < 1120; index++ {
		cause := fmt.Errorf("%04d %s", index, strings.Repeat("payload-chunk ", 145))
		if j.RecordWithID(Event{Operation: "rotation.test", UserVisible: true, Cause: cause}) == "" {
			index--
			runtime.Gosched()
		}
	}
	select {
	case <-allWritten:
	case <-time.After(10 * time.Second):
		t.Fatalf("writer completed %d of 1120 writes", writeCount.Load())
	}
	j.Close()
	files := journalLogFiles(t, root)
	hasFirstPart := false
	for _, path := range files {
		hasFirstPart = hasFirstPart || strings.Contains(filepath.Base(path), ".001.log")
	}
	if len(files) < 2 || !hasFirstPart {
		t.Fatalf("parts = %v", files)
	}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > maxCurrentBytes {
			t.Fatalf("part %s size=%d", path, info.Size())
		}
	}
}

func TestJournalRetentionAgesLegacyAndCapsSessionsOldestFirst(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.Local)
	legacyExpired := filepath.Join(root, "errors-old.jsonl")
	legacyRecent := filepath.Join(root, "errors-recent.jsonl")
	if err := os.WriteFile(legacyExpired, []byte("{old}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyRecent, []byte("{recent}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(legacyExpired, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour))
	_ = os.Chtimes(legacyRecent, now.Add(-24*time.Hour), now.Add(-24*time.Hour))
	day := filepath.Join(root, "2026-08-28")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < maxErrorSessions+2; index++ {
		path := filepath.Join(day, fmt.Sprintf("%06d-aaaaaa.log", index))
		if err := os.WriteFile(path, []byte("error\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		at := now.Add(-time.Duration(maxErrorSessions+2-index) * time.Minute)
		_ = os.Chtimes(path, at, at)
	}
	j := New(root, WithClock(func() time.Time { return now }))
	j.Record(Event{Operation: "retention.trigger", UserVisible: true, Cause: errors.New("trigger")})
	j.Close()
	if _, err := os.Stat(legacyExpired); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired legacy file remains: %v", err)
	}
	if _, err := os.Stat(legacyRecent); err != nil {
		t.Fatalf("recent legacy file removed: %v", err)
	}
	files := journalLogFiles(t, root)
	if len(groupSessionsFromPaths(t, files)) > maxErrorSessions {
		t.Fatalf("session count exceeds %d: %v", maxErrorSessions, files)
	}
	if _, err := os.Stat(filepath.Join(day, "000000-aaaaaa.log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest session remains: %v", err)
	}
}

func TestJournalQuietStartupHousekeepingAgesLegacyWithoutSessionFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	expired := filepath.Join(root, "errors-expired.jsonl")
	if err := os.WriteFile(expired, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = os.Chtimes(expired, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour))
	j := New(root, WithClock(func() time.Time { return now }))
	t.Cleanup(j.Close)
	deadline := time.Now().Add(time.Second)
	for {
		_, err := os.Stat(expired)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("quiet housekeeping did not remove expired legacy file: %v", err)
		}
		runtime.Gosched()
	}
	if files := journalLogFiles(t, root); len(files) != 0 {
		t.Fatalf("quiet housekeeping created session files: %v", files)
	}
}

func TestJournalCapacityEvictsOldestFileBeforeWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	oldest := filepath.Join(root, "errors-oldest.jsonl")
	recent := filepath.Join(root, "errors-recent.jsonl")
	if err := os.WriteFile(oldest, make([]byte, 6<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recent, make([]byte, 4<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = os.Chtimes(oldest, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	_ = os.Chtimes(recent, now.Add(-time.Hour), now.Add(-time.Hour))
	j := New(root, WithClock(func() time.Time { return now }))
	j.Record(Event{Operation: "capacity.write", UserVisible: true, Cause: errors.New("cause")})
	j.Close()
	if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest file remains: %v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("recent file removed: %v", err)
	}
	if !strings.Contains(readHumanJournal(t, root), "capacity.write") {
		t.Fatal("new event missing")
	}
}

func TestJournalRemoveFailureFailsClosedAtCapacity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "errors-full.jsonl")
	if err := os.WriteFile(archive, make([]byte, maxJournalBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	ops := defaultJournalFileOps()
	ops.remove = func(string) error { return errors.New("injected remove failure") }
	j := New(root, withFileOps(ops))
	j.Record(Event{Operation: "capacity.rejected", UserVisible: true, Cause: errors.New("cause")})
	j.Close()
	if files := journalLogFiles(t, root); len(files) != 0 {
		t.Fatalf("wrote past capacity: %v", files)
	}
}

func TestJournalBlockingOpenKeepsCloseBounded(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	originalOpen := ops.openFile
	ops.openFile = func(path string, flags int, mode os.FileMode) (*os.File, error) {
		if filepath.Base(path) != lockFilename {
			once.Do(func() { close(entered) })
			<-release
		}
		return originalOpen(path, flags, mode)
	}
	j := New(root, withFileOps(ops))
	j.Record(Event{Operation: "open.block", Cause: errors.New("cause")})
	<-entered
	started := time.Now()
	j.Close()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Close blocked for %v", elapsed)
	}
	close(release)
}

func TestJournalPermanentLockFailureStaysBounded(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root, withLockAcquire(func(string, time.Duration) (*journalLock, bool) { return nil, false }))
	for index := 0; index < maxQueue*2; index++ {
		_ = j.RecordWithID(Event{Operation: "lock.failure", Cause: fmt.Errorf("%d", index)})
	}
	started := time.Now()
	j.Close()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Close took %v", elapsed)
	}
	if files := journalLogFiles(t, root); len(files) != 0 {
		t.Fatalf("lock failure wrote files: %v", files)
	}
}

func TestJournalCrossProcessSessionsRemainComplete(t *testing.T) {
	if os.Getenv("KWU_DIAGNOSTIC_HELPER") == "1" {
		t.Skip("parent-only")
	}
	root := filepath.Join(t.TempDir(), "errors")
	commands := make([]*exec.Cmd, 2)
	for index := range commands {
		commands[index] = exec.Command(os.Args[0], "-test.run=TestJournalCrossProcessHelper", "-test.count=1")
		commands[index].Env = append(os.Environ(), "KWU_DIAGNOSTIC_HELPER=1", "KWU_DIAGNOSTIC_ROOT="+root, fmt.Sprintf("KWU_DIAGNOSTIC_INDEX=%d", index))
		if err := commands[index].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	files := journalLogFiles(t, root)
	if len(groupSessionsFromPaths(t, files)) != 2 {
		t.Fatalf("cross-process sessions = %v", files)
	}
	body := readHumanJournal(t, root)
	for index := 0; index < 2; index++ {
		if !strings.Contains(body, fmt.Sprintf("cross.process.%d", index)) {
			t.Fatalf("missing process %d: %s", index, body)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if len(line) == 0 || !strings.Contains(line, " | ") {
			t.Fatalf("partial line: %q", line)
		}
	}
}

func TestJournalCrossProcessHelper(t *testing.T) {
	if os.Getenv("KWU_DIAGNOSTIC_HELPER") != "1" {
		t.Skip("helper-only")
	}
	root := os.Getenv("KWU_DIAGNOSTIC_ROOT")
	index := os.Getenv("KWU_DIAGNOSTIC_INDEX")
	j := New(root)
	if id := j.RecordWithID(Event{Operation: "cross.process." + index, UserVisible: true, Cause: errors.New("process cause")}); id == "" {
		t.Fatal("helper event rejected")
	}
	j.Close()
}

func TestJournalFaultedWriteRollsBackCompleteLines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	originalWrite := ops.write
	var calls atomic.Int32
	ops.write = func(file *os.File, payload []byte) (int, error) {
		if calls.Add(1) == 1 {
			half := len(payload) / 2
			n, _ := originalWrite(file, payload[:half])
			return n, errors.New("injected")
		}
		return originalWrite(file, payload)
	}
	j := New(root, withFileOps(ops))
	j.Record(Event{Operation: "write.first", UserVisible: true, Cause: errors.New("first")})
	j.Record(Event{Operation: "write.second", UserVisible: true, Cause: errors.New("second")})
	j.Close()
	body := readHumanJournal(t, root)
	if strings.Contains(body, "write.first") || !strings.Contains(body, "write.second") || !strings.HasSuffix(body, "\n") {
		t.Fatalf("rollback result = %q", body)
	}
}

func TestJournalPrivateModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	j.Record(Event{Operation: "mode.test", Cause: errors.New("cause")})
	j.Close()
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("root mode=%o", rootInfo.Mode().Perm())
	}
	for _, path := range journalLogFiles(t, root) {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file mode=%o", info.Mode().Perm())
		}
	}
}

func genericErrorEventAtStableCallsite() (Event, uintptr) {
	pc, _, _, _ := runtime.Caller(0)
	record := slog.NewRecord(time.Now(), slog.LevelError, "ignored", pc)
	event, _ := eventFromRecord(record, nil)
	return event, pc
}

func TestGenericErrorStableHashedCallsite(t *testing.T) {
	first, firstPC := genericErrorEventAtStableCallsite()
	second, _ := genericErrorEventAtStableCallsite()
	if first.CallsiteID == 0 || first.CallsiteID != second.CallsiteID || first.CallsiteID == uint64(firstPC) {
		t.Fatalf("callsite IDs=%d/%d raw=%d", first.CallsiteID, second.CallsiteID, firstPC)
	}
}

func TestFanoutWithAttrsAndGroups(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	logger := slog.New(NewHandler(slog.NewTextHandler(&bytes.Buffer{}, nil), j)).With("code", "startup_failed").WithGroup("nested")
	logger.ErrorContext(context.Background(), "startup failed", "err", errors.New("nested cause"))
	j.Close()
	if body := readHumanJournal(t, root); !strings.Contains(body, "[startup_failed] | startup failed: *errors.errorString: nested cause") {
		t.Fatalf("attrs not retained: %s", body)
	}
}

func groupSessionsFromPaths(t *testing.T, paths []string) map[string]struct{} {
	t.Helper()
	result := make(map[string]struct{})
	for _, path := range paths {
		name := filepath.Base(path)
		base := strings.TrimSuffix(name, ".log")
		if match := partPattern.FindStringSubmatch(name); len(match) == 2 {
			base = match[1]
		}
		result[filepath.Join(filepath.Dir(path), base)] = struct{}{}
	}
	return result
}
