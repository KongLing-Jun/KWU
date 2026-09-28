package userctx

import "strings"

const (
	DefaultActorID    = "local"
	DefaultProfileID  = "default"
	DefaultAuthMethod = "local"
)

type RemoteIdentity struct {
	Provider string
	Domain   string
	UserID   string
	ChatID   string
	ThreadID string
}

type Profile struct {
	ID string
}

func (p Profile) Normalize() Profile {
	p.ID = NormalizeProfileID(p.ID)
	return p
}

type ActorContext struct {
	ActorID        string
	ProfileID      string
	AuthMethod     string
	WorkspaceRoot  string
	RemoteIdentity *RemoteIdentity
	ParentActorID  string
}

func Default(workspaceRoot string) ActorContext {
	return ActorContext{
		ActorID:       DefaultActorID,
		ProfileID:     DefaultProfileID,
		AuthMethod:    DefaultAuthMethod,
		WorkspaceRoot: workspaceRoot,
	}
}

func (a ActorContext) Normalize() ActorContext {
	a.ActorID = strings.TrimSpace(a.ActorID)
	if a.ActorID == "" {
		a.ActorID = DefaultActorID
	}

	a.ProfileID = NormalizeProfileID(a.ProfileID)

	a.AuthMethod = strings.TrimSpace(a.AuthMethod)
	if a.AuthMethod == "" {
		a.AuthMethod = DefaultAuthMethod
	}

	a.WorkspaceRoot = strings.TrimSpace(a.WorkspaceRoot)
	a.ParentActorID = strings.TrimSpace(a.ParentActorID)
	return a
}

func NormalizeProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if ValidateProfileID(profileID) {
		return profileID
	}
	return DefaultProfileID
}

func ValidateProfileID(profileID string) bool {
	if profileID == "" || strings.TrimSpace(profileID) != profileID {
		return false
	}
	if profileID == DefaultProfileID {
		return true
	}
	if strings.HasPrefix(profileID, ".") || strings.Contains(profileID, "..") {
		return false
	}
	for _, r := range profileID {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}
