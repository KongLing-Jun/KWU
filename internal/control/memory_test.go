package control

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/internal/memory"
)

// TestMemoryWriteReflectsInSnapshot verifies that a memory write lands on disk
// and that Memory() returns a freshly reloaded snapshot afterwards — the behavior
// the memoryManager (off-c.mu) extraction must preserve.
func TestMemoryWriteReflectsInSnapshot(t *testing.T) {
	dir := t.TempDir()
	c := New(Options{Memory: memory.Load(memory.Options{CWD: dir})})

	before := c.Memory()
	if before == nil {
		t.Fatal("memory should be enabled")
	}

	path, err := c.QuickAdd(memory.ScopeProject, "prefer tabs over spaces")
	if err != nil {
		t.Fatalf("QuickAdd: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if !strings.Contains(string(body), "prefer tabs over spaces") {
		t.Fatalf("note not written to disk:\n%s", body)
	}

	after := c.Memory()
	if after == nil {
		t.Fatal("memory snapshot is nil after QuickAdd")
	}
	if after == before {
		t.Fatal("Memory() returned the stale snapshot; the manager did not swap in a reload")
	}
}

func TestControllerMemoryMutationsUseLoadedProfile(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "repo")
	profileA := filepath.Join(root, "profile-a")
	profileB := filepath.Join(root, "profile-b")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	c := New(Options{
		SystemPrompt: "base prompt",
		Memory:       memory.Load(memory.Options{CWD: project, UserDir: profileA}),
	})

	path, err := c.SaveMemory(memory.Memory{Name: "profile-fact", Description: "profile A fact", Type: memory.TypeUser, Body: "A only"})
	if err != nil {
		t.Fatalf("SaveMemory: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(profileA, "memory", "global")) {
		t.Fatalf("SaveMemory should write user memory under current profile GlobalDir, got %s", path)
	}
	if got := memory.Load(memory.Options{CWD: project, UserDir: profileB}).Store.List(); len(got) != 0 {
		t.Fatalf("other profile should not see saved memory: %+v", got)
	}

	quickPath, err := c.QuickAdd(memory.ScopeUser, "profile A quick note")
	if err != nil {
		t.Fatalf("QuickAdd user scope: %v", err)
	}
	if !strings.HasPrefix(quickPath, profileA) {
		t.Fatalf("QuickAdd user scope should write under loaded profile UserDir, got %s", quickPath)
	}
	if _, err := os.Stat(filepath.Join(profileB, filepath.Base(quickPath))); !os.IsNotExist(err) {
		t.Fatalf("other profile should not receive quick-add doc, stat err = %v", err)
	}

	composed := c.Compose("next turn")
	if !strings.Contains(composed, "<memory-update>") || !strings.Contains(composed, "profile A quick note") {
		t.Fatalf("quick-add should ride the next turn as a memory update:\n%s", composed)
	}
	if strings.Contains(c.systemPrompt, "profile A quick note") {
		t.Fatalf("quick-add should not rewrite the cache-stable system prompt: %q", c.systemPrompt)
	}

	c.Submit("# project hash note")
	c.Submit("/remember project slash note")
	projectBody, err := os.ReadFile(filepath.Join(project, "CLOUVIA.md"))
	if err != nil {
		t.Fatalf("read project memory doc: %v", err)
	}
	for _, want := range []string{"- project hash note", "- project slash note"} {
		if !strings.Contains(string(projectBody), want) {
			t.Fatalf("project quick-add doc missing %q:\n%s", want, projectBody)
		}
	}
	composed = c.Compose("after quick-add commands")
	for _, want := range []string{"project hash note", "project slash note"} {
		if !strings.Contains(composed, want) {
			t.Fatalf("quick-add command note %q should ride the next turn:\n%s", want, composed)
		}
		if strings.Contains(c.systemPrompt, want) {
			t.Fatalf("quick-add command should not rewrite cache-stable system prompt: %q", c.systemPrompt)
		}
	}

	if err := c.ForgetMemory("profile-fact"); err != nil {
		t.Fatalf("ForgetMemory: %v", err)
	}
	archived := c.Memory().Store.ListArchived()
	if len(archived) != 1 || archived[0].Name != "profile-fact" || !strings.HasPrefix(archived[0].Path, filepath.Join(profileA, "memory", "global", ".archive")) {
		t.Fatalf("ForgetMemory should archive under current profile store: %+v", archived)
	}
	if got := memory.Load(memory.Options{CWD: project, UserDir: profileB}).Store.ListArchived(); len(got) != 0 {
		t.Fatalf("other profile should not receive archived memory: %+v", got)
	}
}

// TestMemoryWritesConcurrencySafe hammers memory writes from many goroutines
// while c.mu-guarded reads run concurrently. Under -race this proves the
// memoryManager's writeMu/mu split has no data race and no deadlock — and that
// holding writeMu (off c.mu) across the disk I/O still serializes writes so every
// note lands.
func TestMemoryWritesConcurrencySafe(t *testing.T) {
	dir := t.TempDir()
	c := New(Options{Memory: memory.Load(memory.Options{CWD: dir})})

	const writers = 8
	const each = 5

	stop := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = c.Running()       // takes c.mu
				_ = c.RuntimeStatus() // takes c.mu
				_ = c.Memory()        // takes c.mu, returns the snapshot pointer
			}
		}
	}()

	var writersWG sync.WaitGroup
	for w := range writers {
		writersWG.Add(1)
		go func(w int) {
			defer writersWG.Done()
			for i := range each {
				if _, err := c.QuickAdd(memory.ScopeProject, fmt.Sprintf("note w%d-%d", w, i)); err != nil {
					t.Errorf("QuickAdd: %v", err)
				}
			}
		}(w)
	}
	writersWG.Wait()
	close(stop)
	readers.Wait()

	body, err := os.ReadFile(c.Memory().DocPath(memory.ScopeProject))
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	for w := range writers {
		for i := range each {
			want := fmt.Sprintf("note w%d-%d", w, i)
			if !strings.Contains(string(body), want) {
				t.Fatalf("memory doc missing %q after concurrent writes:\n%s", want, body)
			}
		}
	}
}
