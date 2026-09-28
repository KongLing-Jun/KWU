package project

import "time"

type AggregateCreateInput struct {
	Name             string
	Description      NullableStringUpdate
	HumanDescription string
	AgentInstruction string
	Bindings         []ProjectBindingInput
}

type ProjectBindingInput struct {
	Kind        string         `json:"kind"`
	RegistryRef string         `json:"registry_ref"`
	Position    int            `json:"position"`
	Config      map[string]any `json:"config"`
}

type PatchProjectBindingsInput struct {
	ExpectedVersion uint64
	SkillID         NullableStringUpdate
	ExpertID        NullableStringUpdate
	ConnectorID     NullableStringUpdate
}

type ProjectBinding struct {
	BindingID      string         `json:"binding_id"`
	ProjectID      string         `json:"project_id"`
	Kind           string         `json:"kind"`
	RegistryRef    string         `json:"registry_ref"`
	Position       int            `json:"position"`
	Config         map[string]any `json:"config"`
	PublicDisabled bool           `json:"public_disabled"`
	Version        uint64         `json:"version"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type ProjectBindingList struct {
	ProjectScopeKey string
	ProjectID       string           `json:"project_id"`
	ProjectVersion  uint64           `json:"project_version"`
	Items           []ProjectBinding `json:"items"`
}

type AggregateCreateResult struct {
	ProjectScopeKey    string
	ProjectID          string    `json:"project_id"`
	ProjectVersion     uint64    `json:"project_version"`
	InstructionVersion uint64    `json:"instruction_version"`
	BindingIDs         []string  `json:"binding_ids"`
	CreatedAt          time.Time `json:"created_at"`
}

type ProjectMember struct {
	ProjectID   string    `json:"project_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	DisplayName string    `json:"display_name"`
	Version     uint64    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectMemberList struct {
	ProjectScopeKey string
	ProjectID       string
	Items           []ProjectMember
}

// ProjectJoinResult is the current user's authoritative membership returned by
// the idempotent project join endpoint.
type ProjectJoinResult struct {
	ProjectScopeKey string
	Member          ProjectMember
}

type ProjectInvite struct {
	InviteID  string    `json:"invite_id"`
	ProjectID string    `json:"project_id"`
	Status    string    `json:"status"`
	Role      string    `json:"role"`
	Version   uint64    `json:"version"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type ProjectInviteCreateResult struct {
	ProjectScopeKey        string
	InviteID               string    `json:"invite_id"`
	Token                  string    `json:"token,omitempty"`
	ExpiresAt              time.Time `json:"expires_at,omitempty"`
	CommittedWithoutSecret bool      `json:"committed_without_secret,omitempty"`
}

type ProjectInviteAcceptResult struct {
	ProjectScopeKey   string
	Status            string `json:"status"`
	ProjectID         string `json:"project_id,omitempty"`
	MembershipVersion uint64 `json:"membership_version,omitempty"`
}

type ActivityFilter string

const (
	ActivityRelatedToMe ActivityFilter = "related_to_me"
	ActivityMember      ActivityFilter = "member_activity"
	ActivityAllMembers  ActivityFilter = "all_members"
)

type ProjectActivity struct {
	ActivityID             string                           `json:"activity_id"`
	ProjectID              string                           `json:"project_id"`
	Kind                   string                           `json:"kind"`
	SchemaVersion          int                              `json:"schema_version"`
	ActorUserID            *string                          `json:"actor_user_id"`
	AuthorDisplayName      string                           `json:"author_display_name"`
	ClientMessageID        string                           `json:"client_message_id,omitempty"`
	MessageText            string                           `json:"message_text,omitempty"`
	EventName              string                           `json:"event_name,omitempty"`
	EventPayload           map[string]any                   `json:"event_payload,omitempty"`
	MentionUserIDs         []string                         `json:"mention_user_ids"`
	Assets                 []ProjectActivityAsset           `json:"assets"`
	Unread                 bool                             `json:"unread"`
	OtherMembersUnread     *bool                            `json:"other_members_unread,omitempty"`
	UnreadOtherMemberCount *int64                           `json:"unread_other_member_count,omitempty"`
	OtherMemberReadStates  *[]ProjectMessageMemberReadState `json:"other_member_read_states,omitempty"`
	CreatedAt              time.Time                        `json:"created_at"`
}
type ProjectMessageMemberReadState struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}
type ProjectActivityAsset struct {
	AssetID     string `json:"asset_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type ActivityPage struct {
	Items      []ProjectActivity `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	ReadCursor *string           `json:"read_cursor"`
	scopeKey   string
	projectID  string
}

type ListActivitiesInput struct {
	PageSize      int
	Cursor        string
	Filter        ActivityFilter
	MemberUserIDs []string
}

type ListMessagesInput struct {
	PageSize int
	Cursor   string
}

type PostMessageInput struct {
	ClientMessageID string
	Text            string
	MentionUserIDs  []string
	AssetIDs        []string
}

type ActivityReadResult struct {
	ThroughActivityID string    `json:"through_activity_id"`
	UpdatedAt         time.Time `json:"updated_at"`
}
