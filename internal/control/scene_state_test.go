package control

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
	"kwu/internal/scene"
)

func sceneStateTestController(t *testing.T, status scene.Status) (*Controller, *agent.Agent) {
	t.Helper()
	dir := t.TempDir()
	prompt := fmt.Sprintf("%s prompt", status.Scene)
	exec := agent.New(nil, nil, agent.NewSession(prompt), agent.Options{}, event.Discard)
	c := New(Options{
		Executor:     exec,
		SystemPrompt: prompt,
		SessionDir:   dir,
		SessionPath:  filepath.Join(dir, "current.jsonl"),
		Label:        "test",
		Scene:        status.Scene,
		SceneState:   status.State,
		ScenePromptBuilder: func(s scene.Scene, _, _ string) (string, expert.Selection, error) {
			return fmt.Sprintf("%s prompt", s), expert.Selection{}, nil
		},
	})
	return c, exec
}

func TestNewAndClearSessionCreateOfficeDraft(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Controller) error
	}{
		{name: "new", run: (*Controller).NewSession},
		{name: "clear", run: (*Controller).ClearSession},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, exec := sceneStateTestController(t, scene.Status{Scene: scene.Coding, State: scene.Locked})
			exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "old"})
			if err := tc.run(c); err != nil {
				t.Fatal(err)
			}
			if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Office, State: scene.Draft}) {
				t.Fatalf("scene status = %+v, want Office/Draft", got)
			}
			messages := exec.Session().Snapshot()
			if len(messages) != 1 || messages[0].Role != provider.RoleSystem || messages[0].Content != "office prompt" {
				t.Fatalf("fresh session = %+v, want one Office system message", messages)
			}
		})
	}
}

func TestResumeSceneCompatibility(t *testing.T) {
	t.Run("known locked rebuilds prompt", func(t *testing.T) {
		c, _ := sceneStateTestController(t, scene.Status{Scene: scene.Coding, State: scene.Locked})
		loaded := agent.NewSession("saved old prompt")
		loaded.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
		c.Resume(loaded, filepath.Join(t.TempDir(), "known.jsonl"))
		messages := c.History()
		if len(messages) != 2 || messages[0].Content != "coding prompt" {
			t.Fatalf("known resume = %+v, want rebuilt Coding prompt", messages)
		}
	})

	t.Run("legacy preserves saved prompt", func(t *testing.T) {
		c, _ := sceneStateTestController(t, scene.Status{Scene: scene.Unset, State: scene.Legacy})
		loaded := agent.NewSession("saved legacy prompt")
		loaded.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
		c.Resume(loaded, filepath.Join(t.TempDir(), "legacy.jsonl"))
		messages := c.History()
		if len(messages) != 2 || messages[0].Content != "saved legacy prompt" {
			t.Fatalf("legacy resume = %+v, want exact saved prompt", messages)
		}
		if got := c.SceneStatus(); !got.IsLegacy() {
			t.Fatalf("legacy status = %+v", got)
		}
	})
}

func TestBranchInheritsSceneStatus(t *testing.T) {
	for _, status := range []scene.Status{
		{Scene: scene.Design, State: scene.Locked},
		{Scene: scene.Unset, State: scene.Legacy},
	} {
		c, exec := sceneStateTestController(t, status)
		exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "branch me"})
		path, err := c.Branch("child")
		if err != nil {
			t.Fatal(err)
		}
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			t.Fatalf("load branch metadata: ok=%v err=%v", ok, err)
		}
		got, err := meta.SceneStatus()
		if err != nil {
			t.Fatal(err)
		}
		if got != status {
			t.Fatalf("branch scene = %+v, want %+v", got, status)
		}
	}
}

func TestTurnZeroForkInheritsLockedScene(t *testing.T) {
	c, _ := sceneStateTestController(t, scene.Status{Scene: scene.Office, State: scene.Locked})
	c.checkpoints.mu.Lock()
	c.checkpoints.bound[0] = 1
	c.checkpoints.mu.Unlock()
	path, err := c.ForkSession(0, "turn zero")
	if err != nil {
		t.Fatal(err)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load turn-zero fork metadata: ok=%v err=%v", ok, err)
	}
	if got, err := meta.SceneStatus(); err != nil || got != c.SceneStatus() {
		t.Fatalf("turn-zero fork scene = %+v err=%v, want %+v", got, err, c.SceneStatus())
	}
}

func TestLockedSceneRemainsMonotonicAcrossConversationMaintenance(t *testing.T) {
	c, exec := sceneStateTestController(t, scene.Status{Scene: scene.Coding, State: scene.Locked})
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "old turn"})
	exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "old answer"})
	c.checkpoints.mu.Lock()
	c.checkpoints.bound[0] = 1
	c.checkpoints.mu.Unlock()
	if err := c.Rewind(0, RewindConversation); err != nil {
		t.Fatal(err)
	}
	_ = c.Compact(context.Background(), "keep the decision")
	c.Cancel()
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Locked}) {
		t.Fatalf("maintenance changed locked scene: %+v", got)
	}
}
