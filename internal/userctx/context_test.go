package userctx

import "testing"

func TestZeroActorContextNormalizeUsesLocalDefaults(t *testing.T) {
	ctx := ActorContext{}.Normalize()

	if ctx.ActorID != DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", ctx.ActorID, DefaultActorID)
	}
	if ctx.ProfileID != DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", ctx.ProfileID, DefaultProfileID)
	}
	if ctx.AuthMethod != DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", ctx.AuthMethod, DefaultAuthMethod)
	}
}

func TestDefaultPreservesWorkspaceRoot(t *testing.T) {
	ctx := Default("D:/KWU").Normalize()

	if ctx.ActorID != DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", ctx.ActorID, DefaultActorID)
	}
	if ctx.ProfileID != DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", ctx.ProfileID, DefaultProfileID)
	}
	if ctx.AuthMethod != DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", ctx.AuthMethod, DefaultAuthMethod)
	}
	if ctx.WorkspaceRoot != "D:/KWU" {
		t.Fatalf("WorkspaceRoot = %q, want %q", ctx.WorkspaceRoot, "D:/KWU")
	}
}

func TestZeroProfileNormalizeUsesDefaultID(t *testing.T) {
	profile := Profile{}.Normalize()

	if profile.ID != DefaultProfileID {
		t.Fatalf("Profile.ID = %q, want %q", profile.ID, DefaultProfileID)
	}
}

func TestNormalizeTrimsExplicitSafeFields(t *testing.T) {
	ctx := ActorContext{
		ActorID:       " operator ",
		ProfileID:     " team.alpha-01 ",
		AuthMethod:    " token ",
		WorkspaceRoot: " D:/KWU ",
		ParentActorID: " parent ",
	}.Normalize()

	if ctx.ActorID != "operator" {
		t.Fatalf("ActorID = %q, want operator", ctx.ActorID)
	}
	if ctx.ProfileID != "team.alpha-01" {
		t.Fatalf("ProfileID = %q, want team.alpha-01", ctx.ProfileID)
	}
	if ctx.AuthMethod != "token" {
		t.Fatalf("AuthMethod = %q, want token", ctx.AuthMethod)
	}
	if ctx.WorkspaceRoot != "D:/KWU" {
		t.Fatalf("WorkspaceRoot = %q, want D:/KWU", ctx.WorkspaceRoot)
	}
	if ctx.ParentActorID != "parent" {
		t.Fatalf("ParentActorID = %q, want parent", ctx.ParentActorID)
	}
}

func TestNormalizeUnsafeProfileIDFallsBackToDefault(t *testing.T) {
	unsafeIDs := []string{
		"",
		" ",
		"../outside",
		"..",
		".hidden",
		"profile/name",
		`profile\name`,
		"has space",
		"name..withdots",
		"中文",
	}

	for _, id := range unsafeIDs {
		ctx := ActorContext{ProfileID: id}.Normalize()
		if ctx.ProfileID != DefaultProfileID {
			t.Fatalf("ProfileID %q normalized to %q, want %q", id, ctx.ProfileID, DefaultProfileID)
		}
		if ValidateProfileID(id) {
			t.Fatalf("ValidateProfileID(%q) = true, want false", id)
		}
	}
}

func TestRemoteIdentityDoesNotDefaultActorID(t *testing.T) {
	remote := &RemoteIdentity{
		Provider: "lark",
		Domain:   "example.com",
		UserID:   "remote-user",
		ChatID:   "chat-1",
		ThreadID: "thread-1",
	}

	ctx := ActorContext{RemoteIdentity: remote}.Normalize()

	if ctx.ActorID != DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", ctx.ActorID, DefaultActorID)
	}
	if ctx.RemoteIdentity != remote {
		t.Fatalf("RemoteIdentity pointer was not preserved")
	}
}

func TestValidateProfileIDAllowsDefaultAndSafeExplicitIDs(t *testing.T) {
	validIDs := []string{
		DefaultProfileID,
		"team_alpha-01",
		"profile.v1",
		"ABC123",
	}

	for _, id := range validIDs {
		if !ValidateProfileID(id) {
			t.Fatalf("ValidateProfileID(%q) = false, want true", id)
		}
	}
}
