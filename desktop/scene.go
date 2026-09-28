package main

import (
	"kwu/internal/boot"
	"kwu/internal/scene"
)

type sceneStatusSource interface {
	SceneStatus() scene.Status
}

func newDesktopSceneStatus() scene.Status {
	return scene.Status{Scene: scene.Office, State: scene.Draft}
}

func legacyDesktopSceneStatus() scene.Status {
	return scene.Status{Scene: scene.Unset, State: scene.Legacy}
}

func sceneStatusFromPersistedEntry(entry desktopTabEntry) (scene.Status, error) {
	return scene.RestoreStatus(entry.Scene, entry.SceneState)
}

func tabSceneStatus(tab *WorkspaceTab) scene.Status {
	if tab == nil {
		return scene.Status{}
	}
	if source, ok := tab.Ctrl.(sceneStatusSource); ok {
		status := source.SceneStatus()
		if status.State != scene.StateUnset {
			return status
		}
	}
	return scene.Status{Scene: tab.Scene, State: tab.SceneState}
}

func setTabSceneStatus(tab *WorkspaceTab, status scene.Status) {
	if tab == nil {
		return
	}
	tab.Scene = status.Scene
	tab.SceneState = status.State
}

func applyTabSceneBootOptions(options *boot.Options, tab *WorkspaceTab) {
	if options == nil {
		return
	}
	status := tabSceneStatus(tab)
	options.Scene = status.Scene
	options.SceneState = status.State
}

func syncTabSceneFromController(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	if source, ok := tab.Ctrl.(sceneStatusSource); ok {
		status := source.SceneStatus()
		if status.State != scene.StateUnset {
			setTabSceneStatus(tab, status)
		}
	}
}
