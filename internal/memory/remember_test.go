package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRememberToolSaves drives the tool the way the agent does — raw JSON args —
// and verifies the fact lands in the store and the index.
func TestRememberToolSaves(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	tl := NewRememberTool(store)

	if tl.Name() != "remember" || tl.ReadOnly() {
		t.Fatalf("unexpected tool identity: name=%q readonly=%v", tl.Name(), tl.ReadOnly())
	}
	// Schema must be valid JSON the provider can forward.
	if !json.Valid(tl.Schema()) {
		t.Fatal("remember schema is not valid JSON")
	}

	args := []byte(`{"name":"likes-go","title":"Likes Go","description":"User likes Go","type":"user","body":"Default to Go for backend work."}`)
	out, err := tl.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "Saved memory") {
		t.Fatalf("unexpected tool output: %q", out)
	}

	list := store.List()
	if len(list) != 1 || list[0].Name != "likes-go" || list[0].Type != TypeUser {
		t.Fatalf("memory not saved correctly: %+v", list)
	}
	if list[0].Title != "Likes Go" {
		t.Fatalf("title not persisted through the tool: %q", list[0].Title)
	}
}

// TestRememberToolValidates rejects calls missing required fields rather than
// writing an empty memory.
func TestRememberToolValidates(t *testing.T) {
	tl := NewRememberTool(Store{Dir: t.TempDir()})
	if _, err := tl.Execute(context.Background(), []byte(`{"description":"d"}`)); err == nil {
		t.Fatal("expected error when body is missing")
	}
	if _, err := tl.Execute(context.Background(), []byte(`{"body":"b"}`)); err == nil {
		t.Fatal("expected error when description is missing")
	}
}

// TestRememberToolQueuesNote verifies a save injects a turn-tail note so the
// fact applies this session, not only the next.
func TestRememberToolQueuesNote(t *testing.T) {
	q := &fakeQueue{}
	ctx := WithQueue(context.Background(), q)
	tl := NewRememberTool(Store{Dir: t.TempDir()})
	if _, err := tl.Execute(ctx, []byte(`{"name":"uses-rmb","description":"balance is RMB","type":"user","body":"b"}`)); err != nil {
		t.Fatal(err)
	}
	if len(q.notes) != 1 || !strings.Contains(q.notes[0], "uses-rmb") {
		t.Fatalf("expected one queued note naming the saved memory, got %v", q.notes)
	}
}

func TestRememberToolUsesBoundProfileStore(t *testing.T) {
	root := t.TempDir()
	current := Store{
		Dir:       filepath.Join(root, "profile-a", "projects", "repo", "memory"),
		GlobalDir: filepath.Join(root, "profile-a", "memory", "global"),
	}
	other := Store{
		Dir:       filepath.Join(root, "profile-b", "projects", "repo", "memory"),
		GlobalDir: filepath.Join(root, "profile-b", "memory", "global"),
	}

	tl := NewRememberTool(current)
	if _, err := tl.Execute(context.Background(), []byte(`{"name":"user-pref","description":"current profile user pref","type":"user","body":"use profile A"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tl.Execute(context.Background(), []byte(`{"name":"project-note","description":"current profile project note","type":"project","body":"project scoped"}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(current.GlobalDir, "user-pref.md")); err != nil {
		t.Fatalf("user memory should be written under current profile GlobalDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(current.Dir, "project-note.md")); err != nil {
		t.Fatalf("project memory should be written under current profile project store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other.GlobalDir, "user-pref.md")); !os.IsNotExist(err) {
		t.Fatalf("other profile GlobalDir should not receive user memory, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(other.Dir, "project-note.md")); !os.IsNotExist(err) {
		t.Fatalf("other profile project store should not receive project memory, stat err = %v", err)
	}
	if got := other.List(); len(got) != 0 {
		t.Fatalf("other profile should not see current profile memories: %+v", got)
	}
}
