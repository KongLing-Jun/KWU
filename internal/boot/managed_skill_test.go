package boot

import (
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/skill"
)

func TestValidateManagedSkillPathsUsesExistingLoaderAndRejectsCollision(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	managed := filepath.Join(root, "managed")
	writeBootTestSkill(t, base, "existing")
	writeBootTestSkill(t, managed, "managed")
	options := skill.Options{HomeDir: filepath.Join(root, "home"), KWUHomeDir: filepath.Join(root, "kwu"), CustomPaths: []string{base}, DisableBuiltins: true}
	if err := validateManagedSkillPaths(options, []string{managed}); err != nil {
		t.Fatalf("valid managed path: %v", err)
	}
	writeBootTestSkill(t, managed, "existing")
	if err := validateManagedSkillPaths(options, []string{managed}); err == nil {
		t.Fatal("expected managed skill name collision")
	}
}

func writeBootTestSkill(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
}
