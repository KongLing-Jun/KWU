package cloud

import (
	"context"
	"testing"

	"kwu/internal/userctx"
)

func TestProfileSessionRepositoryOnlyReadsCurrentBoundScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bindings := NewBindingRepository()
	sessions := NewMemorySessionStore()
	repository := NewProfileSessionRepository(bindings, sessions)
	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-b"}
	binding, err := bindings.ConfirmBinding(ctx, actorA, "usr-a")
	if err != nil {
		t.Fatalf("ConfirmBinding: %v", err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-a", RefreshToken: "refresh-a"}
	if err := repository.Save(ctx, actorA, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, ok, err := repository.Load(ctx, actorA)
	if err != nil || !ok || loaded.RefreshToken != "refresh-a" {
		t.Fatalf("Load current profile = (%+v, %t, %v), want stored session", loaded, ok, err)
	}
	_, _, err = repository.Load(ctx, actorB)
	assertProfileScopeMismatch(t, err)

	wrongScope := session
	wrongScope.Scope = BindingScope{ProfileID: "profile-b", CloudUserID: "usr-a", BindingGeneration: 1}
	assertProfileScopeMismatch(t, repository.Save(ctx, actorA, wrongScope))

	if _, err := bindings.Rebind(ctx, actorA, "usr-b"); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	if _, ok, err := repository.Load(ctx, actorA); err != nil || ok {
		t.Fatalf("Load after rebind = (_, %t, %v), want no old-generation session", ok, err)
	}
	assertProfileScopeMismatch(t, repository.Save(ctx, actorA, session))
}
