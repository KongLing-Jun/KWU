package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"kwu/internal/cloud"
)

func TestLocalWorkspaceBindingPersistsExactIdentityAndSupportsSharedWorkspace(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), resolver)
	firstPath := filepath.Join(t.TempDir(), "First Workspace")
	secondPath := filepath.Join(t.TempDir(), "Second Workspace")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	displayPath := firstPath + string(os.PathSeparator) + "."
	bound, err := repository.Bind(context.Background(), scope, "project-1", displayPath)
	if err != nil {
		t.Fatal(err)
	}
	if bound.ProjectID != "project-1" || bound.ProjectScopeKey != scope.Key || bound.DisplayPath != displayPath || bound.Status != LocalWorkspaceAvailable || !filepath.IsAbs(bound.CanonicalPath) {
		t.Fatalf("Bind = %#v", bound)
	}
	shared, err := repository.Bind(context.Background(), scope, "project-2", firstPath)
	if err != nil || shared.CanonicalPath != bound.CanonicalPath {
		t.Fatalf("shared workspace Bind = %#v, %v", shared, err)
	}
	rebound, err := repository.Bind(context.Background(), scope, "project-1", secondPath)
	if err != nil || rebound.CanonicalPath == bound.CanonicalPath {
		t.Fatalf("rebind = %#v, %v", rebound, err)
	}
	loaded, found, err := repository.Get(context.Background(), scope, "project-1")
	if err != nil || !found || loaded.CanonicalPath != rebound.CanonicalPath {
		t.Fatalf("Get = %#v found=%t err=%v", loaded, found, err)
	}

	raw, err := os.ReadFile(repository.path(scope))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{`"schema":"reasonix.local-workspace-bindings"`, `"environment":`, `"binding":`, `"project_id":"project-1"`, `"display_path":`, `"canonical_path":`} {
		if !strings.Contains(text, required) {
			t.Errorf("binding document missing %s", required)
		}
	}
	if strings.Contains(text, "desktop-projects.json") {
		t.Fatal("binding document references LocalWorkspace registry")
	}

	otherScope := gatewayTestScope(t, "user-2", 8)
	resolver.Set(otherScope)
	if bindings, err := repository.List(context.Background(), otherScope); err != nil || len(bindings) != 0 {
		t.Fatalf("other scope List = %#v, %v", bindings, err)
	}
	resolver.Set(scope)
	if err := repository.Unbind(context.Background(), scope, "project-1"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repository.Get(context.Background(), scope, "project-1"); err != nil || found {
		t.Fatalf("Get after Unbind found=%t err=%v", found, err)
	}
	if loaded, found, err := repository.Get(context.Background(), scope, "project-2"); err != nil || !found || loaded.CanonicalPath != shared.CanonicalPath {
		t.Fatalf("shared binding after other project Unbind = %#v found=%t err=%v", loaded, found, err)
	}
}

func TestLocalWorkspaceBindingCanonicalPathAndAvailability(t *testing.T) {
	t.Run("missing and files cannot be bound", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
		missing := filepath.Join(t.TempDir(), "missing")
		if _, err := repository.Bind(context.Background(), scope, "project-1", missing); err == nil {
			t.Fatal("Bind accepted a missing directory")
		}
		file := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.Bind(context.Background(), scope, "project-1", file); err == nil {
			t.Fatal("Bind accepted a file")
		}
	})

	t.Run("symlink or junction aliases can be shared by projects", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
		root := t.TempDir()
		target := filepath.Join(root, "target")
		alias := filepath.Join(root, "alias")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := createLocalWorkspaceDirectoryAlias(target, alias); err != nil {
			t.Fatalf("create workspace alias: %v", err)
		}
		_, err := repository.Bind(context.Background(), scope, "project-1", target)
		if err != nil {
			t.Fatal(err)
		}
		shared, err := repository.Bind(context.Background(), scope, "project-2", alias)
		if err != nil || shared.ProjectID != "project-2" || shared.Status != LocalWorkspaceAvailable {
			t.Fatalf("shared alias binding = %#v, %v", shared, err)
		}
	})

	t.Run("later missing directory is retained as unavailable", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
		workspace := filepath.Join(t.TempDir(), "workspace")
		moved := workspace + "-moved"
		if err := os.Mkdir(workspace, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.Bind(context.Background(), scope, "project-1", workspace); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(workspace, moved); err != nil {
			t.Fatal(err)
		}
		binding, found, err := repository.Get(context.Background(), scope, "project-1")
		if err != nil || !found || binding.Status != LocalWorkspaceUnavailable || binding.DisplayPath != workspace {
			t.Fatalf("missing binding = %#v found=%t err=%v", binding, found, err)
		}
		if bindings, err := repository.List(context.Background(), scope); err != nil || len(bindings) != 1 {
			t.Fatalf("retained bindings = %#v err=%v", bindings, err)
		}
		if err := os.Rename(moved, workspace); err != nil {
			t.Fatal(err)
		}
		alias := workspace + "-alias"
		if err := createLocalWorkspaceDirectoryAlias(workspace, alias); err != nil {
			t.Fatal(err)
		}
		if shared, err := repository.Bind(context.Background(), scope, "project-2", alias); err != nil || shared.Status != LocalWorkspaceAvailable {
			t.Fatalf("recovered shared alias binding = %#v, %v", shared, err)
		}
	})
}

func createLocalWorkspaceDirectoryAlias(target, alias string) error {
	if err := os.Symlink(target, alias); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", alias, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create junction: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func TestLocalWorkspaceBindingWindowsVolumeAwareIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		if workspacePathIdentity(`C:\Work\Project`) != workspacePathIdentity(`c:\work\PROJECT`) {
			t.Fatal("Windows case variants did not share an identity")
		}
		if workspacePathIdentity(`C:\Work\Project`) == workspacePathIdentity(`D:\Work\Project`) {
			t.Fatal("different Windows volumes shared an identity")
		}
		return
	}
	if workspacePathIdentity("/Work/Project") == workspacePathIdentity("/work/project") {
		t.Fatal("case-sensitive paths unexpectedly shared an identity")
	}
}

func TestLocalWorkspaceBindingCorruptionIsStrictAndAtomic(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
	workspace := t.TempDir()
	if _, err := repository.Bind(context.Background(), scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	path := repository.path(scope)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(raw), `"bindings":`, `"unexpected":true,"bindings":`, 1)
	if err := os.WriteFile(path, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.List(context.Background(), scope); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("corrupt List error = %v", err)
	}
	if matches, _ := filepath.Glob(path + ".invalid-*"); len(matches) != 1 {
		t.Fatalf("isolated corrupt documents = %v", matches)
	}
	if temporary, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(temporary) != 0 {
		t.Fatalf("binding temporary files = %v", temporary)
	}
}

func TestLocalWorkspaceBindingRejectsCaseAliasesInvalidUTF8AndNUL(t *testing.T) {
	for name, mutate := range map[string]func([]byte) []byte{
		"case alias": func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `"schema":"reasonix.local-workspace-bindings"`, `"schema":"reasonix.local-workspace-bindings","Schema":"reasonix.local-workspace-bindings"`, 1))
		},
		"invalid UTF-8": func(raw []byte) []byte {
			index := strings.Index(string(raw), `"display_path":"`)
			if index < 0 {
				return raw
			}
			index += len(`"display_path":"`)
			raw[index] = 0xff
			return raw
		},
		"NUL path": func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `"display_path":"`, `"display_path":"\u0000`, 1))
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := gatewayTestScope(t, "user-1", 7)
			repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
			if _, err := repository.Bind(context.Background(), scope, "project-1", t.TempDir()); err != nil {
				t.Fatal(err)
			}
			path := repository.path(scope)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, mutate(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.List(context.Background(), scope); err == nil {
				t.Fatal("List accepted an unsafe binding document")
			}
		})
	}
}

func TestLocalWorkspaceBindingDoesNotCompareUnrelatedOfflineWorkspace(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope})
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	if err := os.Mkdir(first, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(second, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Bind(context.Background(), scope, "project-1", first); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(first, first+"-offline"); err != nil {
		t.Fatal(err)
	}
	if bound, err := repository.Bind(context.Background(), scope, "project-2", second); err != nil || bound.Status != LocalWorkspaceAvailable {
		t.Fatalf("independent Bind with another offline workspace = %#v, %v", bound, err)
	}
}

func TestLocalWorkspaceBindingScopeRaceAndConcurrentWindows(t *testing.T) {
	t.Run("scope switch rejects commit", func(t *testing.T) {
		scopeA := gatewayTestScope(t, "user-1", 7)
		scopeB := gatewayTestScope(t, "user-2", 8)
		resolver := &gatewayTestScopeResolver{scope: scopeA}
		repository := newLocalWorkspaceBindingRepositoryForTest(t, t.TempDir(), resolver)
		entered := make(chan struct{})
		release := make(chan struct{})
		repository.beforeCommit = func() { close(entered); <-release }
		workspace := t.TempDir()
		result := make(chan error, 1)
		go func() {
			_, err := repository.Bind(context.Background(), scopeA, "project-1", workspace)
			result <- err
		}()
		<-entered
		resolver.Set(scopeB)
		close(release)
		if err := <-result; err == nil || !strings.Contains(err.Error(), string(cloud.ErrorProfileScopeMismatch)) {
			t.Fatalf("late Bind error = %v", err)
		}
		if _, err := os.Stat(repository.path(scopeA)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("late binding commit exists: %v", err)
		}
	})

	t.Run("repository instances preserve concurrent binds", func(t *testing.T) {
		scope := gatewayTestScope(t, "user-1", 7)
		resolver := &gatewayTestScopeResolver{scope: scope}
		root := t.TempDir()
		first := newLocalWorkspaceBindingRepositoryForTest(t, root, resolver)
		second := newLocalWorkspaceBindingRepositoryForTest(t, root, resolver)
		const count = 24
		workspaces := make([]string, count)
		for index := range workspaces {
			workspaces[index] = filepath.Join(root, fmt.Sprintf("workspace-%02d", index))
			if err := os.Mkdir(workspaces[index], 0o700); err != nil {
				t.Fatal(err)
			}
		}
		var wait sync.WaitGroup
		for index, workspace := range workspaces {
			index, workspace := index, workspace
			wait.Add(1)
			go func() {
				defer wait.Done()
				repository := first
				if index%2 == 1 {
					repository = second
				}
				if _, err := repository.Bind(context.Background(), scope, fmt.Sprintf("project-%02d", index), workspace); err != nil {
					t.Errorf("Bind(%d): %v", index, err)
				}
			}()
		}
		wait.Wait()
		bindings, err := first.List(context.Background(), scope)
		if err != nil || len(bindings) != count {
			t.Fatalf("concurrent List count=%d err=%v", len(bindings), err)
		}
	})
}

func newLocalWorkspaceBindingRepositoryForTest(t *testing.T, root string, scopes ProjectScopeResolver) *LocalWorkspaceBindingRepository {
	t.Helper()
	repository, err := NewLocalWorkspaceBindingRepository(LocalWorkspaceBindingRepositoryOptions{Root: root, Scopes: scopes})
	if err != nil {
		t.Fatal(err)
	}
	return repository
}
