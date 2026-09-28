package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"kwu/internal/cloud"
)

const scopeKeyDomain = "projects-v1:scope-key:v1"

// Scope binds project state to one Cloud environment and complete authenticated
// account generation. Key is the only scope identity intended for UI views.
type Scope struct {
	Environment cloud.EnvironmentIdentity
	Binding     cloud.BindingScope
	Key         string
}

// NewScope validates every identity component and derives the stable opaque key
// used to reject cross-environment and cross-account project state.
func NewScope(environment cloud.EnvironmentIdentity, binding cloud.BindingScope) (Scope, error) {
	if err := environment.Validate(); err != nil {
		return Scope{}, fmt.Errorf("projects-v1: invalid environment: %w", err)
	}
	if err := binding.Validate(); err != nil {
		return Scope{}, fmt.Errorf("projects-v1: invalid binding scope: %w", err)
	}
	framed := scopeKeyDomain +
		frameScopeField("environment_id", environment.EnvironmentID) +
		frameScopeField("cloud_base_url", environment.CloudBaseURL) +
		frameScopeField("profile_id", binding.ProfileID) +
		frameScopeField("cloud_user_id", binding.CloudUserID) +
		frameScopeField("binding_generation", fmt.Sprint(binding.BindingGeneration))
	sum := sha256.Sum256([]byte(framed))
	return Scope{
		Environment: environment,
		Binding:     binding,
		Key:         hex.EncodeToString(sum[:]),
	}, nil
}

// Validate rejects a scope whose identity is invalid or whose opaque key was
// replaced after construction.
func (s Scope) Validate() error {
	want, err := NewScope(s.Environment, s.Binding)
	if err != nil {
		return err
	}
	if len(s.Key) != sha256.Size*2 || s.Key != strings.ToLower(s.Key) || s.Key != want.Key {
		return fmt.Errorf("projects-v1: project scope key does not match its identity")
	}
	return nil
}

func frameScopeField(name, value string) string {
	return fmt.Sprintf("\n%s:%d\n%s", name, len([]byte(value)), value)
}
