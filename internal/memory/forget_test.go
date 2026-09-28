package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestForgetToolDeletes drives the tool with raw JSON args and verifies the fact
// is removed from the store.
func TestForgetToolDeletes(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if _, err := store.Save(Memory{Name: "stale-fact", Description: "d", Type: TypeProject, Body: "b"}); err != nil {
		t.Fatal(err)
	}

	tl := NewForgetTool(store)
	if tl.Name() != "forget" || tl.ReadOnly() {
		t.Fatalf("unexpected tool identity: name=%q readonly=%v", tl.Name(), tl.ReadOnly())
	}
	if !json.Valid(tl.Schema()) {
		t.Fatal("forget schema is not valid JSON")
	}

	out, err := tl.Execute(context.Background(), []byte(`{"name":"stale-fact"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "Forgot memory") || !strings.Contains(out, "archived to") {
		t.Fatalf("unexpected tool output: %q", out)
	}
	if len(store.List()) != 0 {
		t.Fatalf("memory not deleted: %+v", store.List())
	}
}

// TestForgetToolValidates rejects an empty name rather than deleting nothing
// silently.
func TestForgetToolValidates(t *testing.T) {
	tl := NewForgetTool(Store{Dir: t.TempDir()})
	if _, err := tl.Execute(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error when name is missing")
	}
}

// fakeQueue records the turn-tail notes the remember/forget tools queue.
type fakeQueue struct{ notes []string }

func (f *fakeQueue) QueueMemory(note string) { f.notes = append(f.notes, note) }

// TestForgetToolQueuesDisregardNote verifies a forget injects a turn-tail note so
// the model stops trusting the still-cached index line this session.
func TestForgetToolQueuesDisregardNote(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if _, err := store.Save(Memory{Name: "old-fact", Description: "d", Type: TypeProject, Body: "b"}); err != nil {
		t.Fatal(err)
	}
	q := &fakeQueue{}
	ctx := WithQueue(context.Background(), q)
	if _, err := NewForgetTool(store).Execute(ctx, []byte(`{"name":"old-fact"}`)); err != nil {
		t.Fatal(err)
	}
	if len(q.notes) != 1 || !strings.Contains(q.notes[0], "old-fact") {
		t.Fatalf("expected one queued note naming the deleted memory, got %v", q.notes)
	}
}

func TestForgetToolArchivesOnlyBoundProfileMemory(t *testing.T) {
	root := t.TempDir()
	current := Store{
		Dir:       filepath.Join(root, "profile-a", "projects", "repo", "memory"),
		GlobalDir: filepath.Join(root, "profile-a", "memory", "global"),
	}
	other := Store{
		Dir:       filepath.Join(root, "profile-b", "projects", "repo", "memory"),
		GlobalDir: filepath.Join(root, "profile-b", "memory", "global"),
	}

	if _, err := current.Save(Memory{Name: "same-name", Description: "current", Type: TypeUser, Body: "profile A"}); err != nil {
		t.Fatal(err)
	}
	otherPath, err := other.Save(Memory{Name: "same-name", Description: "other", Type: TypeUser, Body: "profile B"})
	if err != nil {
		t.Fatal(err)
	}

	out, err := NewForgetTool(current).Execute(context.Background(), []byte(`{"name":"same-name"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, filepath.Join(current.GlobalDir, ".archive")) {
		t.Fatalf("forget output should archive under current profile GlobalDir, got %q", out)
	}
	if got := current.List(); len(got) != 0 {
		t.Fatalf("current profile memory should be inactive after forget: %+v", got)
	}
	archived := current.ListArchived()
	if len(archived) != 1 || archived[0].Name != "same-name" || !strings.HasPrefix(archived[0].Path, filepath.Join(current.GlobalDir, ".archive")) {
		t.Fatalf("current profile archive mismatch: %+v", archived)
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatalf("other profile active memory should remain: %v", err)
	}
	if got := other.List(); len(got) != 1 || got[0].Name != "same-name" || got[0].Description != "other" {
		t.Fatalf("other profile memory should remain visible: %+v", got)
	}
	if got := other.ListArchived(); len(got) != 0 {
		t.Fatalf("other profile should not receive an archive: %+v", got)
	}
}
