package serve

import (
	"testing"

	"kwu/internal/userctx"
)

func TestActorContextForServeMapsAuthMode(t *testing.T) {
	for _, tt := range []struct {
		name       string
		mode       string
		actorID    string
		authMethod string
	}{
		{name: "default", mode: "", actorID: "local", authMethod: "local"},
		{name: "none", mode: "none", actorID: "local", authMethod: "local"},
		{name: "token", mode: " TOKEN ", actorID: "serve", authMethod: "serve-token"},
		{name: "password", mode: "password", actorID: "serve", authMethod: "serve-password"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actor, err := ActorContextForServe(tt.mode, " workspace ")
			if err != nil {
				t.Fatalf("ActorContextForServe(%q): %v", tt.mode, err)
			}
			if actor.ActorID != tt.actorID {
				t.Fatalf("ActorID = %q, want %q", actor.ActorID, tt.actorID)
			}
			if actor.ProfileID != userctx.DefaultProfileID {
				t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
			}
			if actor.AuthMethod != tt.authMethod {
				t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, tt.authMethod)
			}
			if actor.WorkspaceRoot != "workspace" {
				t.Fatalf("WorkspaceRoot = %q, want trimmed workspace", actor.WorkspaceRoot)
			}
			if actor.RemoteIdentity != nil {
				t.Fatalf("RemoteIdentity = %+v, want nil", actor.RemoteIdentity)
			}
		})
	}
}

func TestActorContextForServeRejectsUnknownAuthMode(t *testing.T) {
	if _, err := ActorContextForServe("tokne", ""); err == nil {
		t.Fatal("unknown auth mode should be rejected")
	}
}
