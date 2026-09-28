package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

func TestSubagentStoreContinueLoadsSavedTranscript(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	run.Session.Add(provider.Message{Role: provider.RoleAssistant, Content: "finding A"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	continued, err := store.PrepareContinue(run.Ref, spec)
	if err != nil {
		t.Fatalf("PrepareContinue: %v", err)
	}
	defer continued.Release()
	if continued.Ref != run.Ref {
		t.Fatalf("continued ref = %q, want %q", continued.Ref, run.Ref)
	}
	if got := continued.Session.Snapshot(); len(got) != 3 || got[2].Content != "finding A" {
		t.Fatalf("continued transcript = %+v, want saved messages", got)
	}
}

func TestSubagentStoreForkCreatesIndependentReference(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	forked, err := store.prepareFork(run.Ref, spec)
	if err != nil {
		t.Fatalf("PrepareFork: %v", err)
	}
	defer forked.Release()
	if forked.Ref == run.Ref {
		t.Fatalf("fork ref should be new, got %q", forked.Ref)
	}
	if got := forked.Session.Snapshot(); len(got) != 2 || got[1].Content != "review diff" {
		t.Fatalf("fork transcript = %+v, want copied messages", got)
	}
	if forked.Meta.ParentSession != spec.ParentSession {
		t.Fatalf("fork parent session = %q, want %q", forked.Meta.ParentSession, spec.ParentSession)
	}
}

func TestSubagentStoreFreshMetaIncludesParentActorContext(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		ParentActorID: "grandparent-actor",
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "lark",
			UserID:   "remote-user",
		},
	}

	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	meta, err := store.LoadMeta(run.Ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.ActorID != "parent-actor" || meta.ProfileID != "profile-a" || meta.AuthMethod != "oauth" {
		t.Fatalf("actor metadata = %+v, want parent actor/profile/auth", meta)
	}
	if filepath.Clean(meta.ActorWorkspaceRoot) != filepath.Clean(spec.Actor.WorkspaceRoot) {
		t.Fatalf("actor workspace root = %q, want %q", meta.ActorWorkspaceRoot, spec.Actor.WorkspaceRoot)
	}
	if meta.ParentActorID != "parent-actor" {
		t.Fatalf("parent actor id = %q, want parent-actor", meta.ParentActorID)
	}
	if meta.RemoteIdentity == nil || meta.RemoteIdentity.UserID != "remote-user" {
		t.Fatalf("remote identity = %+v, want remote user", meta.RemoteIdentity)
	}
	if meta.ActorID == meta.RemoteIdentity.UserID {
		t.Fatalf("remote identity user %q was promoted to ActorID", meta.RemoteIdentity.UserID)
	}
}

func TestSubagentStoreRejectsContinueWithMismatchedActorContext(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "lark",
			UserID:   "remote-user-a",
		},
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	other := spec
	other.Actor.ProfileID = "profile-b"
	continued, err := store.PrepareContinue(run.Ref, other)
	if err == nil {
		continued.Release()
		t.Fatal("PrepareContinue succeeded with mismatched actor profile, want rejection")
	}
	if !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareContinue error = %v, want actor profile mismatch", err)
	}

	other = spec
	remote := *spec.Actor.RemoteIdentity
	remote.UserID = "remote-user-b"
	other.Actor.RemoteIdentity = &remote
	continued, err = store.PrepareContinue(run.Ref, other)
	if err == nil {
		continued.Release()
		t.Fatal("PrepareContinue succeeded with mismatched remote identity, want rejection")
	}
	if !strings.Contains(err.Error(), "remote identity") {
		t.Fatalf("PrepareContinue error = %v, want remote identity mismatch", err)
	}
}

func TestSubagentStoreRejectsForkWithMismatchedActorContext(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	other := spec
	other.Actor.ProfileID = "profile-b"
	forked, err := store.prepareFork(run.Ref, other)
	if err == nil {
		forked.Release()
		t.Fatal("PrepareFork succeeded with mismatched actor profile, want rejection")
	}
	if !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareFork error = %v, want actor profile mismatch", err)
	}
}

func TestSubagentStoreContinueAllowsLegacyMetaWithoutActorContext(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	legacy := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(legacy)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "legacy prompt"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	current := legacy
	current.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
	}
	continued, err := store.PrepareContinue(run.Ref, current)
	if err != nil {
		t.Fatalf("PrepareContinue legacy actorless meta: %v", err)
	}
	defer continued.Release()
	if continued.Ref != run.Ref {
		t.Fatalf("continued ref = %q, want %q", continued.Ref, run.Ref)
	}
}

func TestSubagentStoreRejectsContinueFromSiblingSession(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "left"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "left", "root")
	saveTestBranchMeta(t, sessionDir, "right", "root")
	other := spec
	other.ParentSession = "right"
	if _, err := store.PrepareContinue(run.Ref, other); err == nil || !strings.Contains(err.Error(), "not in current parent session") {
		t.Fatalf("PrepareContinue error = %v, want lineage rejection", err)
	}
}

func TestSubagentStoreContinueFromAncestorCopiesIntoCurrentSession(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	continued, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue: %v", err)
	}
	defer continued.Release()
	if continued.Ref == run.Ref {
		t.Fatalf("continued ref should be copied into child session, got source ref %q", continued.Ref)
	}
	if continued.Meta.ParentSession != "child" {
		t.Fatalf("continued parent session = %q, want child", continued.Meta.ParentSession)
	}
	if continued.Meta.ForkedFrom != run.Ref {
		t.Fatalf("forkedFrom = %q, want %q", continued.Meta.ForkedFrom, run.Ref)
	}
	if got := continued.Session.Snapshot(); len(got) != 2 || got[1].Content != "review diff" {
		t.Fatalf("continued transcript = %+v, want copied source transcript", got)
	}
	sourceMeta, err := store.LoadMeta(run.Ref)
	if err != nil {
		t.Fatalf("LoadMeta source: %v", err)
	}
	if sourceMeta.ParentSession != "root" {
		t.Fatalf("source parent session = %q, want root", sourceMeta.ParentSession)
	}
}

func TestSubagentStoreRejectsAncestorContinueWithMismatchedActorContext(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	child.Actor.ProfileID = "profile-b"
	continued, err := store.PrepareContinue(run.Ref, child)
	if err == nil {
		continued.Release()
		t.Fatal("PrepareContinue from ancestor succeeded with mismatched actor profile, want rejection")
	}
	if !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareContinue error = %v, want actor profile mismatch", err)
	}
}

func TestSubagentStoreRejectsCrossProfileReuseOfCurrentSessionCopy(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	workspace := t.TempDir()
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: workspace,
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "root task"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted root: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	childRun, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue child: %v", err)
	}
	if err := store.SaveCompleted(childRun); err != nil {
		t.Fatalf("SaveCompleted child copy: %v", err)
	}
	childRun.Release()

	other := child
	other.Actor.ProfileID = "profile-b"
	continued, err := store.PrepareContinue(run.Ref, other)
	if err == nil {
		continued.Release()
		t.Fatal("PrepareContinue reused a current-session copy from another profile, want rejection")
	}
	if !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareContinue error = %v, want actor profile mismatch", err)
	}
}

func TestSubagentStoreLegacyForkFromAncestorConvertsToContinueCopy(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	continued, err := store.PrepareLegacyForkFrom(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareLegacyForkFrom: %v", err)
	}
	defer continued.Release()
	if continued.Ref == run.Ref {
		t.Fatalf("legacy fork ref should be copied into child session, got source ref %q", continued.Ref)
	}
	if continued.Meta.ParentSession != "child" {
		t.Fatalf("continued parent session = %q, want child", continued.Meta.ParentSession)
	}
	if continued.Meta.ForkedFrom != run.Ref {
		t.Fatalf("forkedFrom = %q, want %q", continued.Meta.ForkedFrom, run.Ref)
	}
	if got := continued.Session.Snapshot(); len(got) != 2 || got[1].Content != "review diff" {
		t.Fatalf("continued transcript = %+v, want copied source transcript", got)
	}
}

func TestSubagentStoreRejectsLegacyForkFromCurrentSession(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	if _, err := store.PrepareLegacyForkFrom(run.Ref, spec); err == nil || !strings.Contains(err.Error(), "cannot be safely converted") {
		t.Fatalf("PrepareLegacyForkFrom error = %v, want unsafe conversion rejection", err)
	}
}

func TestSubagentStoreRejectsLegacyForkFromCurrentSessionWithMismatchedActorContext(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	other := spec
	other.Actor.ProfileID = "profile-b"
	if _, err := store.PrepareLegacyForkFrom(run.Ref, other); err == nil || !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareLegacyForkFrom error = %v, want actor profile mismatch", err)
	}
}

func TestSubagentStoreContinueFromAncestorReusesCurrentSessionCopy(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	first, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("first PrepareContinue: %v", err)
	}
	firstRef := first.Ref
	if err := store.SaveCompleted(first); err != nil {
		t.Fatalf("SaveCompleted first: %v", err)
	}
	first.Release()

	second, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("second PrepareContinue: %v", err)
	}
	defer second.Release()
	if second.Ref != firstRef {
		t.Fatalf("second continuation ref = %q, want existing child copy %q", second.Ref, firstRef)
	}
}

func TestSubagentStoreContinueFromOlderAncestorUsesNearestLineageCopy(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	rootRun, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh root: %v", err)
	}
	rootRun.Session.Add(provider.Message{Role: provider.RoleUser, Content: "root task"})
	if err := store.SaveCompleted(rootRun); err != nil {
		t.Fatalf("SaveCompleted root: %v", err)
	}
	rootRun.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	saveTestBranchMeta(t, sessionDir, "grandchild", "child")

	child := spec
	child.ParentSession = "child"
	childRun, err := store.PrepareContinue(rootRun.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue child: %v", err)
	}
	childRun.Session.Add(provider.Message{Role: provider.RoleAssistant, Content: "child finding"})
	childRef := childRun.Ref
	if err := store.SaveCompleted(childRun); err != nil {
		t.Fatalf("SaveCompleted child: %v", err)
	}
	childRun.Release()

	grandchild := spec
	grandchild.ParentSession = "grandchild"
	fromRoot, err := store.PrepareContinue(rootRun.Ref, grandchild)
	if err != nil {
		t.Fatalf("PrepareContinue grandchild from root: %v", err)
	}
	grandchildRef := fromRoot.Ref
	if fromRoot.Meta.ForkedFrom != childRef {
		t.Fatalf("grandchild forkedFrom = %q, want nearest child copy %q", fromRoot.Meta.ForkedFrom, childRef)
	}
	if got := fromRoot.Session.Snapshot(); len(got) != 3 || got[2].Content != "child finding" {
		t.Fatalf("grandchild transcript = %+v, want child copy transcript", got)
	}
	if err := store.SaveCompleted(fromRoot); err != nil {
		t.Fatalf("SaveCompleted grandchild: %v", err)
	}
	fromRoot.Release()

	fromChild, err := store.PrepareContinue(childRef, grandchild)
	if err != nil {
		t.Fatalf("PrepareContinue grandchild from child: %v", err)
	}
	defer fromChild.Release()
	if fromChild.Ref != grandchildRef {
		t.Fatalf("grandchild ref from child copy = %q, want existing copy %q", fromChild.Ref, grandchildRef)
	}
}

func TestSubagentStoreReusesFailedCurrentCopyForContinuation(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted root: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	copyRun, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue child: %v", err)
	}
	if err := store.SaveFailed(copyRun); err != nil {
		t.Fatalf("SaveFailed child copy: %v", err)
	}
	copyRun.Release()

	continued, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue failed current copy: %v", err)
	}
	defer continued.Release()
	if continued.Ref != copyRun.Ref {
		t.Fatalf("continued ref = %q, want failed current copy %q", continued.Ref, copyRun.Ref)
	}
}

func TestSubagentStoreRejectsAncestorContinuationWithMultipleCurrentCopies(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted root: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	child := spec
	child.ParentSession = "child"
	first, err := store.PrepareContinue(run.Ref, child)
	if err != nil {
		t.Fatalf("PrepareContinue first: %v", err)
	}
	if err := store.SaveCompleted(first); err != nil {
		t.Fatalf("SaveCompleted first: %v", err)
	}
	first.Release()

	second, err := store.PrepareFresh(child)
	if err != nil {
		t.Fatalf("PrepareFresh second: %v", err)
	}
	second.Meta.ForkedFrom = run.Ref
	if err := store.SaveCompleted(second); err != nil {
		t.Fatalf("SaveCompleted second: %v", err)
	}
	second.Release()

	if _, err := store.PrepareContinue(run.Ref, child); err == nil || !strings.Contains(err.Error(), "multiple copied transcripts") {
		t.Fatalf("PrepareContinue error = %v, want multiple-copy rejection", err)
	}
}

func TestSubagentStoreForkFromAncestorSessionCreatesCurrentOwner(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	other := spec
	other.ParentSession = "child"
	forked, err := store.prepareFork(run.Ref, other)
	if err != nil {
		t.Fatalf("PrepareFork: %v", err)
	}
	defer forked.Release()
	if forked.Ref == run.Ref {
		t.Fatalf("fork ref should be new, got %q", forked.Ref)
	}
	if forked.Meta.ParentSession != "child" {
		t.Fatalf("fork parent session = %q, want child", forked.Meta.ParentSession)
	}
	sourceMeta, err := store.LoadMeta(run.Ref)
	if err != nil {
		t.Fatalf("LoadMeta source: %v", err)
	}
	if sourceMeta.ParentSession != spec.ParentSession {
		t.Fatalf("source parent session = %q, want %q", sourceMeta.ParentSession, spec.ParentSession)
	}
}

func TestSubagentStoreRejectsCrossProfileForkFromAncestorSession(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	workspace := t.TempDir()
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	spec.Actor = userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: workspace,
	}
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	other := spec
	other.ParentSession = "child"
	other.Actor.ProfileID = "profile-b"
	forked, err := store.prepareFork(run.Ref, other)
	if err == nil {
		forked.Release()
		t.Fatal("PrepareFork succeeded across profiles with matching lineage, want rejection")
	}
	if !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("PrepareFork error = %v, want actor profile mismatch", err)
	}
}

func TestSubagentStoreRejectsForkWhenSourceOwnerMetaMissing(t *testing.T) {
	sessionDir, store, ref, spec := prepareCompletedSubagentForLineageTest(t, "root")
	saveTestBranchMeta(t, sessionDir, "child", "root")

	other := spec
	other.ParentSession = "child"
	if _, err := store.prepareFork(ref, other); err == nil || !strings.Contains(err.Error(), "lineage could not be verified") {
		t.Fatalf("PrepareFork error = %v, want unverified lineage rejection", err)
	}
}

func TestSubagentStoreRejectsForkWhenSourceOwnerMetaCorrupt(t *testing.T) {
	sessionDir, store, ref, spec := prepareCompletedSubagentForLineageTest(t, "root")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	if err := os.WriteFile(filepath.Join(sessionDir, "root.jsonl.meta"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt branch meta: %v", err)
	}

	other := spec
	other.ParentSession = "child"
	if _, err := store.prepareFork(ref, other); err == nil || !strings.Contains(err.Error(), "lineage could not be verified") {
		t.Fatalf("PrepareFork error = %v, want unverified lineage rejection", err)
	}
}

func TestSubagentStoreRejectsForkWhenSourceOwnerMetaIDDiffers(t *testing.T) {
	sessionDir, store, ref, spec := prepareCompletedSubagentForLineageTest(t, "root")
	saveTestBranchMeta(t, sessionDir, "child", "root")
	if err := SaveBranchMeta(filepath.Join(sessionDir, "root.jsonl"), BranchMeta{ID: "other-root"}); err != nil {
		t.Fatalf("SaveBranchMeta(root): %v", err)
	}

	other := spec
	other.ParentSession = "child"
	if _, err := store.prepareFork(ref, other); err == nil || !strings.Contains(err.Error(), "lineage could not be verified") {
		t.Fatalf("PrepareFork error = %v, want unverified lineage rejection", err)
	}
}

func TestSubagentStoreRejectsForkFromSiblingSession(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "left"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "left", "root")
	saveTestBranchMeta(t, sessionDir, "right", "root")
	other := spec
	other.ParentSession = "right"
	if _, err := store.prepareFork(run.Ref, other); err == nil || !strings.Contains(err.Error(), "not in current parent session") {
		t.Fatalf("PrepareFork error = %v, want lineage rejection", err)
	}
}

func TestSubagentStoreRejectsForkFromUnrelatedSession(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "source"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	saveTestBranchMeta(t, sessionDir, "root", "")
	saveTestBranchMeta(t, sessionDir, "current", "root")
	other := spec
	other.ParentSession = "current"
	if _, err := store.prepareFork(run.Ref, other); err == nil || !strings.Contains(err.Error(), "not in current parent session") {
		t.Fatalf("PrepareFork error = %v, want unrelated session rejection", err)
	}
}

func TestSubagentStoreRejectsForkWhenLineageCannotBeProven(t *testing.T) {
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = "root"
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	other := spec
	other.ParentSession = "child"
	if _, err := store.prepareFork(run.Ref, other); err == nil || !strings.Contains(err.Error(), "lineage could not be verified") {
		t.Fatalf("PrepareFork error = %v, want unverified lineage rejection", err)
	}
}

func TestSubagentStoreForkReleasesSourceLockAfterCopy(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "review diff"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	forked, err := store.prepareFork(run.Ref, spec)
	if err != nil {
		t.Fatalf("PrepareFork: %v", err)
	}
	defer forked.Release()
	continued, err := store.PrepareContinue(run.Ref, spec)
	if err != nil {
		t.Fatalf("source should not stay locked by fork run: %v", err)
	}
	continued.Release()
}

func TestSubagentStoreRejectsIncompatibleTranscript(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	other := spec
	other.Name = "security-review"
	if _, err := store.PrepareContinue(run.Ref, other); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("PrepareContinue error = %v, want incompatible name", err)
	}
}

func TestSubagentStoreRejectsConcurrentContinue(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()

	first, err := store.PrepareContinue(run.Ref, spec)
	if err != nil {
		t.Fatalf("first PrepareContinue: %v", err)
	}
	defer first.Release()
	if _, err := store.PrepareContinue(run.Ref, spec); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second PrepareContinue error = %v, want lock error", err)
	}
}

func TestSubagentStoreSaveFailedPersistsTranscriptAndAllowsReuse(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "failed continuation"})
	if err := store.SaveFailed(run); err != nil {
		t.Fatalf("SaveFailed: %v", err)
	}
	run.Release()

	loaded, err := LoadSession(store.sessionPath(run.Ref))
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got := loaded.Snapshot(); len(got) != 2 || got[1].Content != "failed continuation" {
		t.Fatalf("failed transcript = %+v, want persisted failed prompt", got)
	}
	meta, err := store.LoadMeta(run.Ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Status != SubagentFailed {
		t.Fatalf("status = %q, want failed", meta.Status)
	}
	continued, err := store.PrepareContinue(run.Ref, spec)
	if err != nil {
		t.Fatalf("PrepareContinue failed ref: %v", err)
	}
	continued.Release()
	forked, err := store.prepareFork(run.Ref, spec)
	if err != nil {
		t.Fatalf("PrepareFork failed ref: %v", err)
	}
	forked.Release()
}

func TestSubagentStoreCleanupStaleRunningMarksInterrupted(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "interrupted prompt"})
	if err := store.MarkRunning(run); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	ref := run.Ref
	run.Release()

	cleaned, err := store.CleanupStaleRunning()
	if err != nil {
		t.Fatalf("CleanupStaleRunning: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Status != SubagentInterrupted {
		t.Fatalf("status = %q, want interrupted", meta.Status)
	}
	continued, err := store.PrepareContinue(ref, spec)
	if err != nil {
		t.Fatalf("PrepareContinue interrupted ref: %v", err)
	}
	continued.Release()
	forked, err := store.prepareFork(ref, spec)
	if err != nil {
		t.Fatalf("PrepareFork interrupted ref: %v", err)
	}
	forked.Release()
}

func TestSubagentStoreSkipsSaveForDestroyedParent(t *testing.T) {
	store := NewSubagentStore(t.TempDir()).WithDestroyedChecker(func(parentSession string) bool {
		return parentSession == "parent-session"
	})
	spec := testSubagentSpec(t, "review")
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	run.Session.Add(provider.Message{Role: provider.RoleUser, Content: "answer after destroy"})
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	if _, err := os.Stat(store.sessionPath(run.Ref)); !os.IsNotExist(err) {
		t.Fatalf("destroyed parent should not save session, stat err = %v", err)
	}
	if _, err := os.Stat(store.metaPath(run.Ref)); !os.IsNotExist(err) {
		t.Fatalf("destroyed parent should not save meta, stat err = %v", err)
	}
	if err := store.SaveFailed(run); err != nil {
		t.Fatalf("SaveFailed: %v", err)
	}
	if _, err := os.Stat(store.sessionPath(run.Ref)); !os.IsNotExist(err) {
		t.Fatalf("destroyed parent should still not save session, stat err = %v", err)
	}
	run.Release()
}

func testSubagentSpec(t *testing.T, name string) SubagentSpec {
	t.Helper()
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	return SubagentSpec{
		Kind:          "skill",
		Name:          name,
		WorkspaceRoot: t.TempDir(),
		ParentSession: "parent-session",
		SystemPrompt:  "review persona",
		Registry:      reg,
		Model:         "deepseek",
		Effort:        "max",
	}
}

func saveTestBranchMeta(t *testing.T, sessionDir, id, parent string) {
	t.Helper()
	if err := SaveBranchMeta(filepath.Join(sessionDir, id+".jsonl"), BranchMeta{ParentID: parent}); err != nil {
		t.Fatalf("SaveBranchMeta(%s): %v", id, err)
	}
}

func prepareCompletedSubagentForLineageTest(t *testing.T, parentSession string) (string, *SubagentStore, string, SubagentSpec) {
	t.Helper()
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	spec := testSubagentSpec(t, "review")
	spec.ParentSession = parentSession
	run, err := store.PrepareFresh(spec)
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted: %v", err)
	}
	run.Release()
	return sessionDir, store, run.Ref, spec
}
