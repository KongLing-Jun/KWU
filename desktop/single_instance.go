package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
)

const singleInstanceIDPrefix = "ai.kwu.desktop"

type singleInstanceIDDependencies struct {
	buildVersion      string
	executable        func() (string, error)
	evalSymlinks      func(string) (string, error)
	abs               func(string) (string, error)
	initializeProfile func() (desktopProfile, error)
}

func currentSingleInstanceIDDependencies() singleInstanceIDDependencies {
	return singleInstanceIDDependencies{
		buildVersion:      version,
		executable:        os.Executable,
		evalSymlinks:      filepath.EvalSymlinks,
		abs:               filepath.Abs,
		initializeProfile: ensureDesktopProfileInitialized,
	}
}

// resolveSingleInstanceID preserves the release identifier exactly while
// binding development instances to the already validated cloud environment.
func resolveSingleInstanceID(dependencies singleInstanceIDDependencies) (string, error) {
	if strings.TrimSpace(dependencies.buildVersion) != "dev" {
		return productionSingleInstanceID(dependencies), nil
	}
	if dependencies.initializeProfile == nil {
		return "", fmt.Errorf("single instance: development profile initializer is unavailable")
	}
	profile, err := dependencies.initializeProfile()
	if err != nil {
		return "", fmt.Errorf("single instance: initialize development profile: %w", err)
	}
	if !profile.Development || !validDevEnvironmentID(profile.EnvironmentID) {
		return "", fmt.Errorf("single instance: development environment ID is unavailable or invalid")
	}
	return singleInstanceIDPrefix + ".dev." + profile.EnvironmentID, nil
}

func productionSingleInstanceID(dependencies singleInstanceIDDependencies) string {
	abs, err := dependencies.executable()
	if err != nil {
		return singleInstanceIDPrefix
	}
	if resolved, err := dependencies.evalSymlinks(abs); err == nil {
		abs = resolved
	} else if fallback, err := dependencies.abs(abs); err == nil {
		abs = fallback
	}
	sum := sha256.Sum256([]byte(abs))
	return singleInstanceIDPrefix + "." + hex.EncodeToString(sum[:8])
}

func singleInstanceID() (string, error) {
	return resolveSingleInstanceID(currentSingleInstanceIDDependencies())
}

func singleInstanceLock(app *App) (*options.SingleInstanceLock, error) {
	lock, err := singleInstanceLockWithDependencies(app, currentSingleInstanceIDDependencies())
	if err != nil {
		return nil, err
	}
	closeSignal, err := startSecondInstanceSignal(lock.UniqueId, app.secondInstanceLaunch)
	if err != nil {
		return nil, fmt.Errorf("single instance: initialize notification signal: %w", err)
	}
	app.singleInstanceSignalClose = closeSignal
	return lock, nil
}

func singleInstanceLockWithDependencies(app *App, dependencies singleInstanceIDDependencies) (*options.SingleInstanceLock, error) {
	if app == nil {
		return nil, fmt.Errorf("single instance: desktop app is unavailable")
	}
	uniqueID, err := resolveSingleInstanceID(dependencies)
	if err != nil {
		return nil, err
	}
	return newSingleInstanceLock(uniqueID, app.secondInstanceLaunch)
}

func newSingleInstanceLock(uniqueID string, onSecondInstanceLaunch func()) (*options.SingleInstanceLock, error) {
	if strings.TrimSpace(uniqueID) == "" {
		return nil, fmt.Errorf("single instance: unique ID is unavailable")
	}
	if onSecondInstanceLaunch == nil {
		return nil, fmt.Errorf("single instance: second launch callback is unavailable")
	}
	return &options.SingleInstanceLock{
		UniqueId: uniqueID,
		OnSecondInstanceLaunch: func(options.SecondInstanceData) {
			onSecondInstanceLaunch()
		},
	}, nil
}
