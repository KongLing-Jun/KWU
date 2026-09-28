package control

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
	"kwu/internal/scene"
)

type sceneCommitRunner struct {
	exec   *agent.Agent
	calls  int
	system string
	err    error
}

func (r *sceneCommitRunner) Run(context.Context, string) error {
	r.calls++
	for _, message := range r.exec.Session().Snapshot() {
		if message.Role == provider.RoleSystem {
			r.system = message.Content
		}
	}
	return r.err
}

func newSceneCommitController(t *testing.T, runner *sceneCommitRunner, mutate func(*Options)) (*Controller, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.jsonl")
	exec := agent.New(nil, nil, agent.NewSession("office prompt"), agent.Options{}, event.Discard)
	runner.exec = exec
	opts := Options{
		Runner:       runner,
		Executor:     exec,
		SystemPrompt: "office prompt",
		SessionDir:   dir,
		SessionPath:  path,
		Label:        "test",
		Scene:        scene.Coding,
		SceneState:   scene.Draft,
		ScenePromptBuilder: func(s scene.Scene, _, _ string) (string, expert.Selection, error) {
			return fmt.Sprintf("%s prompt", s), expert.Selection{}, nil
		},
	}
	if mutate != nil {
		mutate(&opts)
	}
	return New(opts), path
}

func TestRootRunnerCommitsDraftSceneBeforeDispatch(t *testing.T) {
	runner := &sceneCommitRunner{}
	c, path := newSceneCommitController(t, runner, nil)
	if err := c.runRootRunner(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || runner.system != "coding prompt" {
		t.Fatalf("runner calls=%d system=%q, want one Coding dispatch", runner.calls, runner.system)
	}
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Locked}) {
		t.Fatalf("scene status = %+v, want Coding/Locked", got)
	}
	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	messages := loaded.Snapshot()
	if len(messages) != 1 || messages[0].Role != provider.RoleSystem || messages[0].Content != "coding prompt" {
		t.Fatalf("saved session = %+v, want one Coding system message", messages)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load branch metadata: ok=%v err=%v", ok, err)
	}
	if got, err := meta.SceneStatus(); err != nil || got != c.SceneStatus() {
		t.Fatalf("saved scene = %+v err=%v, runtime=%+v", got, err, c.SceneStatus())
	}
	if meta.SchemaVersion != agent.BranchMetaCountsVersion || meta.Turns != 0 || meta.Preview != "" {
		t.Fatalf("saved listing metadata = schema %d turns %d preview %q", meta.SchemaVersion, meta.Turns, meta.Preview)
	}
}

func TestDraftSceneCommitSerializesConcurrentSnapshot(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	runner := &sceneCommitRunner{}
	c, _ := newSceneCommitController(t, runner, func(opts *Options) {
		opts.SceneSessionSaver = func(*agent.Session, string) error {
			close(entered)
			<-release
			return nil
		}
		opts.SceneStatusSaver = func(string, scene.Status) error { return nil }
	})

	commitDone := make(chan error, 1)
	go func() { commitDone <- c.runRootRunner(context.Background(), "hello") }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("scene commit did not reach session persistence")
	}

	snapshotDone := make(chan error, 1)
	go func() { snapshotDone <- c.Snapshot() }()
	select {
	case err := <-snapshotDone:
		t.Fatalf("snapshot completed during scene persistence: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-commitDone; err != nil {
		t.Fatalf("scene commit: %v", err)
	}
	if err := <-snapshotDone; err != nil {
		t.Fatalf("snapshot after scene commit: %v", err)
	}
}

func TestDraftSceneCommitFailuresDoNotDispatchOrLock(t *testing.T) {
	nextExpert := expert.Selection{ID: "design-expert", Name: "Design Expert", Source: expert.SourceSession, Selected: true}
	tests := []struct {
		name   string
		mutate func(*Options)
	}{
		{
			name: "prompt build",
			mutate: func(opts *Options) {
				opts.ScenePromptBuilder = func(scene.Scene, string, string) (string, expert.Selection, error) {
					return "", expert.Selection{}, errors.New("prompt failed")
				}
			},
		},
		{
			name: "session save",
			mutate: func(opts *Options) {
				opts.ScenePromptBuilder = func(scene.Scene, string, string) (string, expert.Selection, error) {
					return "coding prompt", nextExpert, nil
				}
				opts.SceneSessionSaver = func(*agent.Session, string) error { return errors.New("session failed") }
				opts.SceneStatusSaver = func(string, scene.Status) error {
					t.Fatal("status save ran after session failure")
					return nil
				}
			},
		},
		{
			name: "status save",
			mutate: func(opts *Options) {
				opts.ScenePromptBuilder = func(scene.Scene, string, string) (string, expert.Selection, error) {
					return "coding prompt", nextExpert, nil
				}
				opts.SceneSessionSaver = func(*agent.Session, string) error { return nil }
				opts.SceneStatusSaver = func(string, scene.Status) error { return errors.New("status failed") }
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &sceneCommitRunner{}
			c, _ := newSceneCommitController(t, runner, tc.mutate)
			beforeSession := c.executor.Session()
			beforeMessages := beforeSession.Snapshot()
			beforePrompt := c.systemPrompt
			beforeExpert := c.expert
			if err := c.runRootRunner(context.Background(), "hello"); err == nil {
				t.Fatal("commit unexpectedly succeeded")
			}
			if runner.calls != 0 {
				t.Fatalf("runner calls = %d, want 0", runner.calls)
			}
			if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
				t.Fatalf("scene status = %+v, want retryable Coding/Draft", got)
			}
			if c.executor.Session() != beforeSession {
				t.Fatal("executor session pointer changed before persistence completed")
			}
			if got := c.executor.Session().Snapshot(); fmt.Sprint(got) != fmt.Sprint(beforeMessages) {
				t.Fatalf("executor messages = %+v, want unchanged %+v", got, beforeMessages)
			}
			if c.systemPrompt != beforePrompt {
				t.Fatalf("system prompt = %q, want unchanged %q", c.systemPrompt, beforePrompt)
			}
			if fmt.Sprintf("%#v", c.expert) != fmt.Sprintf("%#v", beforeExpert) {
				t.Fatalf("expert = %+v, want unchanged %+v", c.expert, beforeExpert)
			}
		})
	}
}

func TestRunnerFailureDoesNotUnlockCommittedScene(t *testing.T) {
	runner := &sceneCommitRunner{err: errors.New("runner failed")}
	c, _ := newSceneCommitController(t, runner, nil)
	if err := c.runRootRunner(context.Background(), "hello"); err == nil {
		t.Fatal("runner failure was not returned")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Locked}) {
		t.Fatalf("scene status = %+v, want committed Coding/Locked", got)
	}
}

func TestSetDraftScenePersistsAndLockedRejectsChanges(t *testing.T) {
	runner := &sceneCommitRunner{}
	c, path := newSceneCommitController(t, runner, nil)
	if err := c.SetDraftScene(scene.Design); err != nil {
		t.Fatal(err)
	}
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Design, State: scene.Draft}) {
		t.Fatalf("draft status = %+v, want Design/Draft", got)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load draft metadata: ok=%v err=%v", ok, err)
	}
	if got, err := meta.SceneStatus(); err != nil || got != c.SceneStatus() {
		t.Fatalf("saved draft = %+v err=%v", got, err)
	}
	if err := c.SetDraftScene(scene.Unset); err == nil {
		t.Fatal("unset was accepted as a selectable scene")
	}
	if err := c.runRootRunner(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if runner.system != "design prompt" {
		t.Fatalf("first runner system = %q, want final Design prompt", runner.system)
	}
	if err := c.SetDraftScene(scene.Office); err == nil {
		t.Fatal("locked scene was changed")
	}
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Design, State: scene.Locked}) {
		t.Fatalf("locked status = %+v, want unchanged Design/Locked", got)
	}
}

func TestUnadoptedAndLegacyScenesBypassCommit(t *testing.T) {
	for _, status := range []scene.Status{
		{},
		{Scene: scene.Unset, State: scene.Legacy},
	} {
		runner := &sceneCommitRunner{}
		c, _ := newSceneCommitController(t, runner, func(opts *Options) {
			opts.Scene = status.Scene
			opts.SceneState = status.State
			opts.SceneSessionSaver = func(*agent.Session, string) error {
				t.Fatal("compatibility scene attempted a commit")
				return nil
			}
		})
		if err := c.runRootRunner(context.Background(), "hello"); err != nil {
			t.Fatal(err)
		}
		if runner.calls != 1 || runner.system != "office prompt" {
			t.Fatalf("compatibility dispatch calls=%d system=%q", runner.calls, runner.system)
		}
	}
}
