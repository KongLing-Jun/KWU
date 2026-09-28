package diagnostics

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHumanJournalAcceptedIDFormatPrivacyAndLazySession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	now := time.Date(2026, 8, 29, 9, 7, 6, 123456000, time.FixedZone("CST", 8*60*60))
	j := New(root, WithClock(func() time.Time { return now }))
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal root exists before first error: %v", err)
	}
	id := j.RecordWithID(Event{
		Severity: "error", Operation: "cloud.test", Code: "UPSTREAM_ERROR", UserVisible: true,
		Cause:     errors.New("open C:\\Users\\alice\\project\\data.db: password=hunter2; request_body=private; owner@example.com 13800138000"),
		RequestID: "req-safe-1",
	})
	if !regexp.MustCompile(`^[a-z0-9]{6}-[0-9a-z]{4}$`).MatchString(id) {
		t.Fatalf("log id = %q", id)
	}
	j.Close()

	files := journalLogFiles(t, root)
	if len(files) != 1 || !strings.Contains(filepath.ToSlash(files[0]), "/2026-08-29/090706-") || filepath.Ext(files[0]) != ".log" {
		t.Fatalf("session files = %v", files)
	}
	body, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	line := string(body)
	if !regexp.MustCompile(`^09:07:06\.123 ERROR ` + regexp.QuoteMeta(id) + ` cloud\.test \[UPSTREAM_ERROR\] \| `).MatchString(line) {
		t.Fatalf("human line = %q", line)
	}
	for _, want := range []string{`C:\Users\alice\project\data.db`, "[redacted-password]", "request_body=[redacted]", "[redacted-email]", "[redacted-phone]", "request_id=req-safe-1"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line missing %q: %q", want, line)
		}
	}
	for _, forbidden := range []string{"hunter2", "private", "owner@example.com", "13800138000", "{\""} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("line leaked %q: %q", forbidden, line)
		}
	}
}

func TestHumanJournalRecordHotPathAndDedupePolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	ops := defaultJournalFileOps()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	originalMkdir := ops.mkdirAll
	ops.mkdirAll = func(path string, mode os.FileMode) error {
		once.Do(func() { close(entered) })
		<-release
		return originalMkdir(path, mode)
	}
	j := New(root, withFileOps(ops))
	first := j.RecordWithID(Event{Operation: "ui.save", UserVisible: true, Cause: errors.New("same cause")})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach blocked disk")
	}
	started := time.Now()
	second := j.RecordWithID(Event{Operation: "ui.save", UserVisible: true, Cause: errors.New("same cause")})
	background1 := j.RecordWithID(Event{Operation: "worker.sync", Cause: errors.New("same background cause")})
	background2 := j.RecordWithID(Event{Operation: "worker.sync", Cause: errors.New("same background cause")})
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("record blocked behind worker I/O for %v", elapsed)
	}
	if first == "" || second == "" || first == second || background1 == "" || background2 == "" {
		t.Fatalf("accepted IDs = %q %q %q %q", first, second, background1, background2)
	}
	close(release)
	j.Close()
	body := readHumanJournal(t, root)
	if strings.Count(body, "ui.save") != 2 {
		t.Fatalf("user-visible errors were deduplicated: %q", body)
	}
	if strings.Count(body, "worker.sync") != 1 {
		t.Fatalf("background errors were not deduplicated: %q", body)
	}
}

func TestHumanJournalLineCapPreservesDeepestCauseTail(t *testing.T) {
	root := filepath.Join(t.TempDir(), "errors")
	j := New(root)
	deepest := "DEEPEST-CAUSE-TAIL"
	cause := errors.Join(errors.New(strings.Repeat("prefix ", 800)), errors.New(deepest))
	if id := j.RecordWithID(Event{Operation: "session.persist", Cause: cause}); id == "" {
		t.Fatal("event was not accepted")
	}
	j.Close()
	line := readHumanJournal(t, root)
	if len([]byte(line)) > maxLineBytes || !strings.Contains(line, "[truncated]") || !strings.Contains(line, deepest) {
		t.Fatalf("bounded line bytes=%d tail=%t: %q", len([]byte(line)), strings.Contains(line, deepest), line)
	}
}

func journalLogFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func readHumanJournal(t *testing.T, root string) string {
	t.Helper()
	var body strings.Builder
	for _, path := range journalLogFiles(t, root) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body.Write(data)
	}
	return body.String()
}
