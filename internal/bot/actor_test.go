package bot

import (
	"testing"

	"kwu/internal/userctx"
)

func TestActorContextForInboundUsesServiceActorAndRemoteIdentity(t *testing.T) {
	actor := ActorContextForInbound(InboundMessage{
		Platform:   PlatformFeishu,
		Domain:     "lark",
		ChatID:     "chat-123",
		UserID:     "remote-user-456",
		OperatorID: "allowlisted-operator",
		ThreadID:   "thread-789",
	}, "C:\\work\\project")

	if actor.ActorID != "bot-service" {
		t.Fatalf("ActorID = %q, want bot-service", actor.ActorID)
	}
	if actor.ActorID == "remote-user-456" || actor.ActorID == "allowlisted-operator" {
		t.Fatalf("ActorID = %q, want service actor rather than remote identity", actor.ActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != "bot" {
		t.Fatalf("AuthMethod = %q, want bot", actor.AuthMethod)
	}
	if actor.WorkspaceRoot != "C:\\work\\project" {
		t.Fatalf("WorkspaceRoot = %q, want C:\\work\\project", actor.WorkspaceRoot)
	}
	if actor.RemoteIdentity == nil {
		t.Fatal("RemoteIdentity is nil")
	}
	remote := *actor.RemoteIdentity
	if remote.Provider != "feishu" || remote.Domain != "lark" || remote.UserID != "remote-user-456" || remote.ChatID != "chat-123" || remote.ThreadID != "thread-789" {
		t.Fatalf("RemoteIdentity = %#v, want platform/domain/user/chat/thread copied", remote)
	}
}

func TestServiceActorContextKeepsOwnerProfile(t *testing.T) {
	owner := userctx.Default("ignored")
	owner.ActorID = "desktop-user"
	owner.ProfileID = "work"

	actor := ServiceActorContext(owner, "C:\\work\\project")

	if actor.ActorID != "bot-service" {
		t.Fatalf("ActorID = %q, want bot-service", actor.ActorID)
	}
	if actor.ProfileID != "work" {
		t.Fatalf("ProfileID = %q, want work", actor.ProfileID)
	}
	if actor.AuthMethod != "bot" {
		t.Fatalf("AuthMethod = %q, want bot", actor.AuthMethod)
	}
	if actor.WorkspaceRoot != "C:\\work\\project" {
		t.Fatalf("WorkspaceRoot = %q, want C:\\work\\project", actor.WorkspaceRoot)
	}
	if actor.RemoteIdentity != nil {
		t.Fatalf("RemoteIdentity = %+v, want nil before a remote inbound message", actor.RemoteIdentity)
	}
}
