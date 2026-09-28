package projecttrust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreIsFailClosedAndTrustIsProfileScoped(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	storeA := Store{Path: filepath.Join(root, "profile-a", "trust.json")}
	storeB := Store{Path: filepath.Join(root, "profile-b", "trust.json")}
	if storeA.IsTrusted(project) || storeB.IsTrusted(project) {
		t.Fatal("new project must start untrusted")
	}
	if err := storeA.Trust(project); err != nil {
		t.Fatal(err)
	}
	if !storeA.IsTrusted(project) {
		t.Fatal("trusted project was not persisted")
	}
	if storeB.IsTrusted(project) {
		t.Fatal("profile B inherited profile A trust")
	}
}

func TestStoreReadsLegacyFallbackWithoutRewritingIt(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	abs, err := filepath.Abs(project)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "legacy", "trust.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(trustFile{Projects: map[string]bool{abs: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, body, 0o644); err != nil {
		t.Fatal(err)
	}
	store := Store{Path: filepath.Join(root, "new", "trust.json"), Fallbacks: []string{legacy}}
	if !store.IsTrusted(project) {
		t.Fatal("legacy trust decision was not read")
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatalf("read-only fallback unexpectedly wrote primary store: %v", err)
	}
}
