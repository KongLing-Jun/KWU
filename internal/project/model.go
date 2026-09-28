package project

import "time"

// ProjectStatus is the Cloud-owned lifecycle state exposed by Core v1.
type ProjectStatus string

const (
	// ProjectActive permits owner operations subject to version checks.
	ProjectActive ProjectStatus = "active"
	// ProjectArchived is retained for recovery but is read-only.
	ProjectArchived ProjectStatus = "archived"
)

// Project is the validated CloudProject domain snapshot for one server version.
type Project struct {
	ProjectID   string
	OwnerUserID string
	Name        string
	Description *string
	Status      ProjectStatus
	Version     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProjectPage is a validated server-ordered page stamped with its decode-time
// scope so later view conversion cannot restamp a stale response.
type ProjectPage struct {
	Items      []Project
	NextCursor *string
	scopeKey   string
}

// ProjectView is the Wails-safe project representation. It deliberately omits
// owner and local workspace identity.
type ProjectView struct {
	ProjectID   string  `json:"project_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Version     uint64  `json:"version"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ProjectPageView carries the opaque scope stamp required for UI race checks.
type ProjectPageView struct {
	ProjectScopeKey string        `json:"project_scope_key"`
	Items           []ProjectView `json:"items"`
	NextCursor      *string       `json:"next_cursor"`
}

// View converts validated domain values without accepting a replacement scope.
func (p ProjectPage) View() ProjectPageView {
	items := make([]ProjectView, len(p.Items))
	for i, project := range p.Items {
		items[i] = ProjectView{
			ProjectID: project.ProjectID, Name: project.Name, Description: cloneString(project.Description),
			Status: string(project.Status), Version: project.Version,
			CreatedAt: project.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: project.UpdatedAt.Format(time.RFC3339Nano),
		}
	}
	return ProjectPageView{ProjectScopeKey: p.scopeKey, Items: items, NextCursor: cloneString(p.NextCursor)}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
