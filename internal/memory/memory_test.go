package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeEmptyIsIdentity is the cache-first invariant: with no memory at
// all, Compose must return the base prompt byte-for-byte, so the cached system
// prefix is exactly what it was before memory existed.
func TestComposeEmptyIsIdentity(t *testing.T) {
	base := "You are a helpful coding agent.\nBe concise."
	if got := Compose(base, nil); got != base {
		t.Fatalf("nil memory changed the prompt: got %q", got)
	}
	got := Compose(base, &Set{})
	if got != base {
		t.Fatalf("empty memory changed the prompt:\n base=%q\n got =%q", base, got)
	}
	// A nil-ish set (no docs, blank index) must also be identity.
	if got := Compose(base, &Set{Index: "   \n"}); got != base {
		t.Fatalf("blank index changed the prompt: got %q", got)
	}
}

// TestComposeAppendsAfterBase verifies memory folds in *after* the base prompt,
// so the base stays a valid cache prefix even as memory changes between sessions.
func TestComposeAppendsAfterBase(t *testing.T) {
	base := "BASE PROMPT"
	set := &Set{Docs: []Source{{Path: "/p/KWU.md", Scope: ScopeProject, Body: "Use tabs."}}}
	got := Compose(base, set)
	if !strings.HasPrefix(got, base) {
		t.Fatalf("base is not the prefix of the composed prompt:\n%q", got)
	}
	if !strings.Contains(got, "Use tabs.") {
		t.Fatalf("doc body missing from composed prompt:\n%q", got)
	}
	if strings.Contains(got, "来自用户画像") {
		t.Fatalf("profile context must not be folded into memory.Compose:\n%q", got)
	}
}

func TestComposeSavedMemoryDoesNotIncludeProfilePathIdentity(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	mustMkdir(t, filepath.Join(proj, ".git"))
	profileA := filepath.Join(root, "profiles", "profile-alpha")
	profileB := filepath.Join(root, "profiles", "profile-beta")

	for _, profile := range []string{profileA, profileB} {
		store := StoreFor(profile, proj)
		if _, err := store.Save(Memory{Name: "same-pref", Description: "same profile memory", Type: TypeUser, Body: "same body"}); err != nil {
			t.Fatal(err)
		}
	}

	base := "BASE PROMPT"
	promptA := Compose(base, Load(Options{CWD: proj, UserDir: profileA}))
	promptB := Compose(base, Load(Options{CWD: proj, UserDir: profileB}))
	if promptA != promptB {
		t.Fatalf("same memory content should compose identically across profile paths:\nA:\n%s\nB:\n%s", promptA, promptB)
	}
	for _, forbidden := range []string{"profile-alpha", "profile-beta", profileA, profileB} {
		if strings.Contains(promptA, forbidden) || strings.Contains(promptB, forbidden) {
			t.Fatalf("composed prompt leaked profile/path identity %q:\nA:\n%s\nB:\n%s", forbidden, promptA, promptB)
		}
	}
}

func TestComposeUserDocMemoryDoesNotIncludeProfilePathIdentity(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	mustMkdir(t, filepath.Join(proj, ".git"))
	profileA := filepath.Join(root, "profiles", "profile-alpha")
	profileB := filepath.Join(root, "profiles", "profile-beta")
	body := "Shared user doc guidance: prefer careful cache-stable prompts."
	pathA := filepath.Join(profileA, "KWU.md")
	pathB := filepath.Join(profileB, "AGENTS.md")
	mustMkdir(t, profileA)
	mustMkdir(t, profileB)
	mustWrite(t, pathA, body)
	mustWrite(t, pathB, body)

	setA := Load(Options{CWD: proj, UserDir: profileA})
	setB := Load(Options{CWD: proj, UserDir: profileB})
	if len(setA.Docs) != 1 || setA.Docs[0].Path != pathA {
		t.Fatalf("profile A docs should retain real path %q, got %+v", pathA, setA.Docs)
	}
	if len(setB.Docs) != 1 || setB.Docs[0].Path != pathB {
		t.Fatalf("profile B docs should retain real path %q, got %+v", pathB, setB.Docs)
	}
	if got := setA.DocPath(ScopeUser); got != pathA {
		t.Fatalf("DocPath should keep the real user doc path: got %q want %q", got, pathA)
	}
	if got := setB.DocPath(ScopeUser); got != pathB {
		t.Fatalf("DocPath should keep the real user doc path: got %q want %q", got, pathB)
	}

	base := "BASE PROMPT"
	promptA := Compose(base, setA)
	promptB := Compose(base, setB)
	if promptA != promptB {
		t.Fatalf("same user doc content should compose identically across profile paths:\nA:\n%s\nB:\n%s", promptA, promptB)
	}
	for _, forbidden := range []string{root, profileA, profileB, "profile-alpha", "profile-beta"} {
		if strings.Contains(promptA, forbidden) || strings.Contains(promptB, forbidden) {
			t.Fatalf("composed prompt leaked profile/path identity %q:\nA:\n%s\nB:\n%s", forbidden, promptA, promptB)
		}
	}
	for _, want := range []string{body, "user doc (user)"} {
		if !strings.Contains(promptA, want) {
			t.Fatalf("composed prompt missing stable doc content/source marker %q:\n%s", want, promptA)
		}
	}

	written, err := setA.WriteDoc(pathA, body+"\nUpdated host-side path behavior.")
	if err != nil {
		t.Fatalf("WriteDoc should still allow the real user doc path: %v", err)
	}
	if written != pathA {
		t.Fatalf("WriteDoc returned %q, want %q", written, pathA)
	}
	b, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Updated host-side path behavior.") {
		t.Fatalf("WriteDoc did not update the real user doc path:\n%s", b)
	}
}

// TestDiscoverPrecedenceOrder checks user → ancestor → project → local ordering,
// which puts the most specific guidance last.
func TestDiscoverPrecedenceOrder(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "userconfig")
	proj := filepath.Join(root, "proj")
	mustMkdir(t, user)
	mustMkdir(t, proj)
	// Make proj a git root so discovery stops there.
	mustMkdir(t, filepath.Join(proj, ".git"))

	mustWrite(t, filepath.Join(user, "KWU.md"), "USER LEVEL")
	mustWrite(t, filepath.Join(proj, "KWU.md"), "PROJECT LEVEL")
	mustWrite(t, filepath.Join(proj, "KWU.local.md"), "LOCAL LEVEL")

	set := Load(Options{CWD: proj, UserDir: user})
	if len(set.Docs) != 3 {
		t.Fatalf("want 3 docs, got %d: %+v", len(set.Docs), set.Docs)
	}
	wantScopes := []Scope{ScopeUser, ScopeProject, ScopeLocal}
	for i, s := range wantScopes {
		if set.Docs[i].Scope != s {
			t.Fatalf("doc %d: want scope %q, got %q", i, s, set.Docs[i].Scope)
		}
	}
	// In the composed block, local must appear after project must appear after user.
	block := set.Block()
	iu, ip, il := strings.Index(block, "USER LEVEL"), strings.Index(block, "PROJECT LEVEL"), strings.Index(block, "LOCAL LEVEL")
	if !(iu >= 0 && iu < ip && ip < il) {
		t.Fatalf("precedence order wrong in block: user=%d project=%d local=%d\n%s", iu, ip, il, block)
	}
}

func TestLoadUsesProfileUserDirForGlobalMemoryAndSharedProjectStore(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	mustMkdir(t, filepath.Join(proj, ".git"))
	profileA := filepath.Join(root, "profiles", "a")
	profileB := filepath.Join(root, "profiles", "b")

	storeA := StoreForProjectRoot(profileA, root, proj)
	storeB := StoreForProjectRoot(profileB, root, proj)
	if storeA.GlobalDir == storeB.GlobalDir {
		t.Fatalf("profile stores should have different GlobalDir: %q", storeA.GlobalDir)
	}
	if storeA.Dir != storeB.Dir {
		t.Fatalf("same workspace project stores should be shared across profiles:\n A=%s\n B=%s", storeA.Dir, storeB.Dir)
	}
	if _, err := storeA.Save(Memory{Name: "alpha-user", Description: "profile a user memory", Type: TypeUser, Body: "alpha only"}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeA.Save(Memory{Name: "alpha-feedback", Description: "profile a feedback memory", Type: TypeFeedback, Body: "alpha feedback"}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeA.Save(Memory{Name: "shared-project", Description: "workspace project memory", Type: TypeProject, Body: "project visible"}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeA.Save(Memory{Name: "shared-reference", Description: "workspace reference memory", Type: TypeReference, Body: "reference visible"}); err != nil {
		t.Fatal(err)
	}

	setA := Load(Options{CWD: proj, UserDir: profileA, ProjectStoreRoot: root})
	setB := Load(Options{CWD: proj, UserDir: profileB, ProjectStoreRoot: root})
	if !strings.Contains(setA.Index, "alpha-user.md") {
		t.Fatalf("profile A index missing profile A user memory:\n%s", setA.Index)
	}
	if strings.Contains(setB.Index, "alpha-user.md") || strings.Contains(setB.Index, "alpha-feedback.md") {
		t.Fatalf("profile B index leaked profile A user/feedback memory:\n%s", setB.Index)
	}
	if !strings.Contains(setB.Index, "shared-project.md") || !strings.Contains(setB.Index, "shared-reference.md") {
		t.Fatalf("profile B index missing shared project/reference memory:\n%s", setB.Index)
	}
	if err := setB.Store.Delete("shared-project"); err != nil {
		t.Fatal(err)
	}
	setAAfterDelete := Load(Options{CWD: proj, UserDir: profileA, ProjectStoreRoot: root})
	if strings.Contains(setAAfterDelete.Index, "shared-project.md") {
		t.Fatalf("project memory deleted from profile B should disappear for profile A too:\n%s", setAAfterDelete.Index)
	}
}

func TestLoadComposeHidesSharedProjectUserFeedbackFromProfilePrompt(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	mustMkdir(t, filepath.Join(proj, ".git"))
	profileA := filepath.Join(root, "profiles", "a")
	store := StoreForProjectRoot(profileA, root, proj)

	writeMemoryForTest(t, store.GlobalDir, "alpha-user", TypeUser, "alpha profile user")
	writeMemoryForTest(t, store.Dir, "shared-project", TypeProject, "workspace project memory")
	writeMemoryForTest(t, store.Dir, "leaked-user", TypeUser, "other profile user")
	writeMemoryForTest(t, store.Dir, "leaked-feedback", TypeFeedback, "other profile feedback")

	prompt := Compose("BASE PROMPT", Load(Options{CWD: proj, UserDir: profileA, ProjectStoreRoot: root}))
	for _, want := range []string{"alpha-user.md", "shared-project.md"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("composed prompt missing visible memory %q:\n%s", want, prompt)
		}
	}
	for _, forbidden := range []string{"leaked-user.md", "leaked-feedback.md", "other profile user", "other profile feedback"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("composed prompt leaked shared project profile-scoped memory %q:\n%s", forbidden, prompt)
		}
	}
}

// TestImportResolution checks "@path" inlining, including a relative import.
func TestImportResolution(t *testing.T) {
	proj := t.TempDir()
	mustMkdir(t, filepath.Join(proj, ".git"))
	mustWrite(t, filepath.Join(proj, "shared.md"), "SHARED CONTENT")
	mustWrite(t, filepath.Join(proj, "KWU.md"), "Top line\n@shared.md\nBottom line")

	set := Load(Options{CWD: proj})
	if len(set.Docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(set.Docs))
	}
	body := set.Docs[0].Body
	if !strings.Contains(body, "SHARED CONTENT") {
		t.Fatalf("import not inlined: %q", body)
	}
	if strings.Contains(body, "@shared.md") {
		t.Fatalf("import directive left in body: %q", body)
	}
}

// TestImportCycleDoesNotHang verifies cycle detection terminates.
func TestImportCycleDoesNotHang(t *testing.T) {
	proj := t.TempDir()
	mustMkdir(t, filepath.Join(proj, ".git"))
	mustWrite(t, filepath.Join(proj, "a.md"), "A\n@b.md")
	mustWrite(t, filepath.Join(proj, "b.md"), "B\n@a.md")
	mustWrite(t, filepath.Join(proj, "KWU.md"), "@a.md")

	set := Load(Options{CWD: proj}) // must return, not loop forever
	body := set.Docs[0].Body
	if !strings.Contains(body, "A") || !strings.Contains(body, "B") {
		t.Fatalf("cycle import dropped content: %q", body)
	}
}

// TestImportTargetClassification guards the "@mention vs @import" heuristic.
func TestImportTargetClassification(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"@docs/setup.md", true},
		{"@./notes.txt", true},
		{"@/abs/path.md", true},
		{"@mention", false},      // prose-y, no separator/dot
		{"@", false},             // bare
		{"@a/b and more", false}, // not the only token
		{"plain text", false},
	}
	for _, c := range cases {
		if _, got := importTarget(c.line); got != c.want {
			t.Errorf("importTarget(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportDiamondAndCycle(t *testing.T) {
	proj := t.TempDir()
	mustMkdir(t, filepath.Join(proj, ".git"))

	mustWrite(t, filepath.Join(proj, "shared.md"), "SHARED CONTENT")
	mustWrite(t, filepath.Join(proj, "a.md"), "A\n@shared.md")
	mustWrite(t, filepath.Join(proj, "b.md"), "B\n@shared.md")
	mustWrite(t, filepath.Join(proj, "KWU.md"), "@a.md\n@b.md")

	set := Load(Options{CWD: proj})
	if len(set.Docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(set.Docs))
	}
	body := set.Docs[0].Body

	count := strings.Count(body, "SHARED CONTENT")
	if count != 2 {
		t.Errorf("expected 'SHARED CONTENT' to appear twice, got %d times. Body:\n%s", count, body)
	}
	if strings.Contains(body, "skipped: import cycle") {
		t.Errorf("body contains incorrect import cycle message:\n%s", body)
	}

	projCycle := t.TempDir()
	mustMkdir(t, filepath.Join(projCycle, ".git"))
	mustWrite(t, filepath.Join(projCycle, "cycle1.md"), "CYCLE1\n@cycle2.md")
	mustWrite(t, filepath.Join(projCycle, "cycle2.md"), "CYCLE2\n@cycle1.md")
	mustWrite(t, filepath.Join(projCycle, "KWU.md"), "@cycle1.md")

	setCycle := Load(Options{CWD: projCycle})
	if len(setCycle.Docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(setCycle.Docs))
	}
	bodyCycle := setCycle.Docs[0].Body
	if !strings.Contains(bodyCycle, "skipped: import cycle") {
		t.Errorf("expected import cycle to be detected and reported. Body:\n%s", bodyCycle)
	}
}
