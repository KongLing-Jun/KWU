package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

const testDevEnvironmentID = "0123456789abcdef"

func TestProductionSingleInstanceIDPreservesExecutablePathAlgorithmWithoutProfile(t *testing.T) {
	profileCalls := 0
	resolvedExecutable := filepath.Join("resolved", "kwu-desktop.exe")
	dependencies := singleInstanceIDDependencies{
		buildVersion: "v1.0.2",
		executable: func() (string, error) {
			return filepath.Join("original", "kwu-desktop.exe"), nil
		},
		evalSymlinks: func(string) (string, error) {
			return resolvedExecutable, nil
		},
		abs: func(string) (string, error) {
			t.Fatal("absolute fallback must not run after symlink resolution")
			return "", nil
		},
		initializeProfile: func() (desktopProfile, error) {
			profileCalls++
			return desktopProfile{}, errors.New("production must not initialize a development profile")
		},
	}

	got, err := resolveSingleInstanceID(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(resolvedExecutable))
	want := singleInstanceIDPrefix + "." + hex.EncodeToString(sum[:8])
	if got != want {
		t.Fatalf("resolveSingleInstanceID() = %q, want %q", got, want)
	}
	if profileCalls != 0 {
		t.Fatalf("development profile initializer called %d times in production", profileCalls)
	}
}

func TestProductionSingleInstanceIDPreservesExecutableFailureFallback(t *testing.T) {
	got, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion: "stable",
		executable: func() (string, error) {
			return "", os.ErrNotExist
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != singleInstanceIDPrefix {
		t.Fatalf("resolveSingleInstanceID() = %q, want legacy fallback %q", got, singleInstanceIDPrefix)
	}
}

func TestProductionSingleInstanceIDPreservesAbsolutePathFallback(t *testing.T) {
	absoluteExecutable := filepath.Join("absolute", "kwu-desktop.exe")
	got, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion: "stable",
		executable: func() (string, error) {
			return filepath.Join("relative", "kwu-desktop.exe"), nil
		},
		evalSymlinks: func(string) (string, error) {
			return "", os.ErrNotExist
		},
		abs: func(string) (string, error) {
			return absoluteExecutable, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(absoluteExecutable))
	want := singleInstanceIDPrefix + "." + hex.EncodeToString(sum[:8])
	if got != want {
		t.Fatalf("resolveSingleInstanceID() = %q, want %q", got, want)
	}
}

func TestDevelopmentSingleInstanceIDStableAcrossExecutableAndWorkingDirectory(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })

	executableCalls := 0
	resolve := func(executablePath string) string {
		got, resolveErr := resolveSingleInstanceID(singleInstanceIDDependencies{
			buildVersion: "dev",
			executable: func() (string, error) {
				executableCalls++
				return executablePath, nil
			},
			initializeProfile: developmentProfileInitializer(testDevEnvironmentID),
		})
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		return got
	}

	first := resolve(filepath.Join("worktree-a", "kwu-desktop.exe"))
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	second := resolve(filepath.Join("worktree-b", "kwu-desktop.exe"))
	if err := os.Chdir(originalWorkingDirectory); err != nil {
		t.Fatal(err)
	}
	want := singleInstanceIDPrefix + ".dev." + testDevEnvironmentID
	if first != want || second != want {
		t.Fatalf("development IDs = %q and %q, want %q", first, second, want)
	}
	if executableCalls != 0 {
		t.Fatalf("executable resolver called %d times for development IDs", executableCalls)
	}
}

func TestDevelopmentSingleInstanceIDsDifferByEnvironment(t *testing.T) {
	first, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion:      "dev",
		initializeProfile: developmentProfileInitializer("0123456789abcdef"),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion:      "dev",
		initializeProfile: developmentProfileInitializer("fedcba9876543210"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("development IDs unexpectedly match: %q", first)
	}
}

func TestDevelopmentAndProductionSingleInstanceIDsDiffer(t *testing.T) {
	development, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion:      "dev",
		initializeProfile: developmentProfileInitializer(testDevEnvironmentID),
	})
	if err != nil {
		t.Fatal(err)
	}
	production, err := resolveSingleInstanceID(singleInstanceIDDependencies{
		buildVersion: "v1.0.2",
		executable: func() (string, error) {
			return testDevEnvironmentID, nil
		},
		evalSymlinks: func(path string) (string, error) { return path, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if development == production {
		t.Fatalf("development and production IDs unexpectedly match: %q", development)
	}
}

func TestKWUDevDoesNotSkipSingleInstanceLock(t *testing.T) {
	t.Setenv("KWU_DEV", "1")
	lock, err := singleInstanceLockWithDependencies(&App{}, singleInstanceIDDependencies{
		buildVersion:      "dev",
		initializeProfile: developmentProfileInitializer(testDevEnvironmentID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		t.Fatal("singleInstanceLockWithDependencies() returned nil with KWU_DEV=1")
	}
	want := singleInstanceIDPrefix + ".dev." + testDevEnvironmentID
	if lock.UniqueId != want {
		t.Fatalf("UniqueId = %q, want %q", lock.UniqueId, want)
	}
}

func TestDevelopmentSingleInstanceLockFailsClosedForInvalidProfile(t *testing.T) {
	tests := []struct {
		name        string
		initializer func() (desktopProfile, error)
	}{
		{
			name: "initialization error",
			initializer: func() (desktopProfile, error) {
				return desktopProfile{}, os.ErrPermission
			},
		},
		{name: "non-development profile", initializer: developmentProfileInitializerWithDevelopment(false, testDevEnvironmentID)},
		{name: "missing environment ID", initializer: developmentProfileInitializer("")},
		{name: "invalid environment ID", initializer: developmentProfileInitializer("0123456789abcdeg")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock, err := singleInstanceLockWithDependencies(&App{}, singleInstanceIDDependencies{
				buildVersion:      "dev",
				initializeProfile: test.initializer,
			})
			if err == nil {
				t.Fatal("singleInstanceLockWithDependencies() error = nil")
			}
			if lock != nil {
				t.Fatalf("singleInstanceLockWithDependencies() returned lock %#v after profile failure", lock)
			}
		})
	}
}

func TestSingleInstanceCallbackOnlyInvokesOriginalAppRecovery(t *testing.T) {
	originalApp := &singleInstanceCallbackTestApp{}
	otherApp := &singleInstanceCallbackTestApp{}
	lock, err := newSingleInstanceLock("test-id", originalApp.secondInstanceLaunch)
	if err != nil {
		t.Fatal(err)
	}

	lock.OnSecondInstanceLaunch(options.SecondInstanceData{})
	if originalApp.calls != 1 || otherApp.calls != 0 {
		t.Fatalf("recovery calls = original:%d other:%d, want original:1 other:0", originalApp.calls, otherApp.calls)
	}
}

type singleInstanceCallbackTestApp struct {
	calls int
}

func (app *singleInstanceCallbackTestApp) secondInstanceLaunch() {
	app.calls++
}

func developmentProfileInitializer(environmentID string) func() (desktopProfile, error) {
	return developmentProfileInitializerWithDevelopment(true, environmentID)
}

func developmentProfileInitializerWithDevelopment(development bool, environmentID string) func() (desktopProfile, error) {
	return func() (desktopProfile, error) {
		return desktopProfile{Development: development, EnvironmentID: environmentID}, nil
	}
}
