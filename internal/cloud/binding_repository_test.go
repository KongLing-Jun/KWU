package cloud

import (
	"context"
	"errors"
	"testing"

	"kwu/internal/userctx"
)

func TestBindingRepositoryConfirmsCurrentProfileAndRequiresExplicitRebind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewBindingRepository()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}

	initial, err := repository.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatalf("ResolveBinding initial: %v", err)
	}
	if initial.Status != BindingUnbound || initial.ProfileID != "profile-a" || initial.ActorID != "actor-a" {
		t.Fatalf("initial binding = %+v, want current actor's unbound profile", initial)
	}

	confirmed, err := repository.ConfirmBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatalf("ConfirmBinding: %v", err)
	}
	if confirmed.Status != BindingBound || confirmed.CloudUserID != "usr-a" || confirmed.BindingGeneration != 1 {
		t.Fatalf("confirmed binding = %+v, want usr-a generation 1", confirmed)
	}

	_, err = repository.ConfirmBinding(ctx, actor, "usr-b")
	assertProfileScopeMismatch(t, err)
	unchanged, err := repository.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatalf("ResolveBinding after candidate: %v", err)
	}
	if unchanged != confirmed {
		t.Fatalf("binding after candidate = %+v, want unchanged %+v", unchanged, confirmed)
	}

	rebound, err := repository.Rebind(ctx, actor, "usr-b")
	if err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	if rebound.Status != BindingBound || rebound.CloudUserID != "usr-b" || rebound.BindingGeneration != 2 {
		t.Fatalf("rebound binding = %+v, want usr-b generation 2", rebound)
	}
}

func TestBindingRepositoryNeverCreatesOrSwitchesAnotherProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewBindingRepository()
	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-b"}

	if _, err := repository.ConfirmBinding(ctx, actorA, "usr-a"); err != nil {
		t.Fatalf("ConfirmBinding profile-a: %v", err)
	}
	other, err := repository.ResolveBinding(ctx, actorB)
	if err != nil {
		t.Fatalf("ResolveBinding profile-b: %v", err)
	}
	if other.Status != BindingUnbound || other.ProfileID != "profile-b" || other.CloudUserID != "" || other.BindingGeneration != 0 {
		t.Fatalf("profile-b binding = %+v, want unbound profile-b", other)
	}
}

func TestBindingRepositoryRejectsAnotherActorForTheSameProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewBindingRepository()
	owner := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	another := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-a"}
	if _, err := repository.ConfirmBinding(ctx, owner, "usr-a"); err != nil {
		t.Fatalf("ConfirmBinding owner: %v", err)
	}

	if _, err := repository.ResolveBinding(ctx, another); err == nil {
		t.Fatal("ResolveBinding another actor succeeded, want PROFILE_SCOPE_MISMATCH")
	} else {
		assertProfileScopeMismatch(t, err)
	}
	if _, err := repository.ConfirmBinding(ctx, another, "usr-a"); err == nil {
		t.Fatal("ConfirmBinding another actor succeeded, want PROFILE_SCOPE_MISMATCH")
	} else {
		assertProfileScopeMismatch(t, err)
	}
	if _, err := repository.Rebind(ctx, another, "usr-b"); err == nil {
		t.Fatal("Rebind another actor succeeded, want PROFILE_SCOPE_MISMATCH")
	} else {
		assertProfileScopeMismatch(t, err)
	}

	binding, err := repository.ResolveBinding(ctx, owner)
	if err != nil {
		t.Fatalf("ResolveBinding owner: %v", err)
	}
	if binding.ActorID != "actor-a" || binding.CloudUserID != "usr-a" || binding.BindingGeneration != 1 || binding.Status != BindingBound {
		t.Fatalf("owner binding = %+v, want original binding unchanged", binding)
	}
}

func assertProfileScopeMismatch(t *testing.T, err error) {
	t.Helper()
	var cloudErr *Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorProfileScopeMismatch {
		t.Fatalf("error = %v, want %s", err, ErrorProfileScopeMismatch)
	}
}
