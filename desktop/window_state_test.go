package main

import (
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/userctx"
)

func TestDesktopWindowSizeContract(t *testing.T) {
	if defaultWindowWidth != 1240 || defaultWindowHeight != 800 {
		t.Fatalf("default window size = %dx%d, want 1240x800", defaultWindowWidth, defaultWindowHeight)
	}
	if minimumWindowWidth != 1200 || minimumWindowHeight != 800 {
		t.Fatalf("minimum window size = %dx%d, want 1200x800", minimumWindowWidth, minimumWindowHeight)
	}
	if defaultWindowWidth < minimumWindowWidth || defaultWindowHeight < minimumWindowHeight {
		t.Fatalf(
			"default window size %dx%d must not be below minimum %dx%d",
			defaultWindowWidth,
			defaultWindowHeight,
			minimumWindowWidth,
			minimumWindowHeight,
		)
	}
}

func TestWindowStateUsesCurrentProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	actorA := userctx.ActorContext{ActorID: "desktop-a", ProfileID: "profile-a", AuthMethod: "local"}
	actorB := userctx.ActorContext{ActorID: "desktop-b", ProfileID: "profile-b", AuthMethod: "local"}

	appA := NewApp()
	appA.setCurrentActorContext(actorA)
	if err := appA.SaveWindowState(DesktopWindowState{Width: 1300, Height: 900, X: 10, Y: 20}); err != nil {
		t.Fatalf("SaveWindowState profile A: %v", err)
	}

	appB := NewApp()
	appB.setCurrentActorContext(actorB)
	if _, ok := appB.loadWindowState(); ok {
		t.Fatal("profile B should not read profile A window state")
	}

	got, ok := appA.loadWindowState()
	if !ok {
		t.Fatal("profile A should read its saved window state")
	}
	if got.Width != 1300 || got.Height != 900 || got.X != 10 || got.Y != 20 {
		t.Fatalf("profile A window state = %+v, want saved state", got)
	}
	if filepath.Clean(appA.windowStatePath()) == filepath.Clean(appB.windowStatePath()) {
		t.Fatalf("window state paths should differ by profile: %q", appA.windowStatePath())
	}
}

func TestLoadWindowStateNormalizesSavedDimensions(t *testing.T) {
	tests := []struct {
		name   string
		state  string
		want   DesktopWindowState
		wantOK bool
	}{
		{
			name:   "both dimensions below minimum",
			state:  `{"width":800,"height":500,"x":10,"y":20,"maximised":true}`,
			want:   DesktopWindowState{Width: minimumWindowWidth, Height: minimumWindowHeight, X: 10, Y: 20, Maximised: true},
			wantOK: true,
		},
		{
			name:   "only width below minimum",
			state:  `{"width":800,"height":800,"x":-10,"y":30}`,
			want:   DesktopWindowState{Width: minimumWindowWidth, Height: 800, X: -10, Y: 30},
			wantOK: true,
		},
		{
			name:   "only height below minimum",
			state:  `{"width":1300,"height":500,"x":40,"y":-20}`,
			want:   DesktopWindowState{Width: 1300, Height: minimumWindowHeight, X: 40, Y: -20},
			wantOK: true,
		},
		{
			name:   "dimensions above minimum unchanged",
			state:  `{"width":1300,"height":900,"x":50,"y":60,"maximised":true}`,
			want:   DesktopWindowState{Width: 1300, Height: 900, X: 50, Y: 60, Maximised: true},
			wantOK: true,
		},
		{
			name:   "non-positive dimensions retain fallback semantics",
			state:  `{"width":-1,"height":0,"x":70,"y":80}`,
			want:   DesktopWindowState{X: 70, Y: 80},
			wantOK: true,
		},
		{
			name:   "all zero migration guard",
			state:  `{}`,
			want:   DesktopWindowState{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "window-state.json")
			if err := os.WriteFile(path, []byte(tt.state), 0o644); err != nil {
				t.Fatalf("write window state: %v", err)
			}

			got, ok := loadWindowStateFromPath(path)
			if ok != tt.wantOK {
				t.Fatalf("loadWindowStateFromPath() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("loadWindowStateFromPath() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
