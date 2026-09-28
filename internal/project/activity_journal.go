package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/fileutil"
)

const activityJournalRetention = 24 * time.Hour

type ActivityMessageRecord struct {
	ProjectScopeKey string    `json:"project_scope_key"`
	ProjectID       string    `json:"project_id"`
	ClientMessageID string    `json:"client_message_id"`
	IdempotencyKey  string    `json:"idempotency_key"`
	Text            string    `json:"text"`
	MentionUserIDs  []string  `json:"mention_user_ids"`
	AssetIDs        []string  `json:"asset_ids"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type activityJournalDocument struct {
	SchemaVersion int                     `json:"schema_version"`
	ScopeKey      string                  `json:"scope_key"`
	Records       []ActivityMessageRecord `json:"records"`
}

type ActivityMessageJournalOptions struct {
	Root   string
	Scopes ProjectScopeResolver
	Now    func() time.Time
	Random io.Reader
}

type ActivityMessageJournal struct {
	root   string
	scopes ProjectScopeResolver
	now    func() time.Time
	random io.Reader
	mu     sync.Mutex
}

func NewActivityMessageJournal(options ActivityMessageJournalOptions) (*ActivityMessageJournal, error) {
	if strings.TrimSpace(options.Root) == "" || nilGatewayDependency(options.Scopes) {
		return nil, fmt.Errorf("projects-v2: activity journal dependencies are not configured")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	return &ActivityMessageJournal{root: options.Root, scopes: options.Scopes, now: options.Now, random: options.Random}, nil
}

func (j *ActivityMessageJournal) Prepare(ctx context.Context, scope Scope, projectID, text string, mentions, assets []string) (ActivityMessageRecord, error) {
	if err := j.validateScope(ctx, scope); err != nil {
		return ActivityMessageRecord{}, err
	}
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ActivityMessageRecord{}, err
	}
	if strings.TrimSpace(text) == "" || !validProjectText(text, 20_480) {
		return ActivityMessageRecord{}, fmt.Errorf("projects-v2: invalid activity message")
	}
	clientID, err := j.randomID("msg_")
	if err != nil {
		return ActivityMessageRecord{}, err
	}
	key, err := j.randomID("")
	if err != nil {
		return ActivityMessageRecord{}, err
	}
	now := j.now().UTC()
	record := ActivityMessageRecord{ProjectScopeKey: scope.Key, ProjectID: projectID, ClientMessageID: clientID, IdempotencyKey: key, Text: text, MentionUserIDs: append([]string(nil), mentions...), AssetIDs: append([]string(nil), assets...), State: "prepared", CreatedAt: now, UpdatedAt: now}
	j.mu.Lock()
	defer j.mu.Unlock()
	document, err := j.load(scope)
	if err != nil {
		return record, err
	}
	document.Records = append(document.Records, record)
	return record, j.commit(scope, document)
}

func (j *ActivityMessageJournal) Pending(ctx context.Context, scope Scope, projectID string) ([]ActivityMessageRecord, error) {
	if err := j.validateScope(ctx, scope); err != nil {
		return nil, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	document, err := j.load(scope)
	if err != nil {
		return nil, err
	}
	changed := j.expire(&document)
	out := make([]ActivityMessageRecord, 0)
	for _, record := range document.Records {
		if record.ProjectID == projectID && record.State != "unresolved_expired" {
			out = append(out, record)
		}
	}
	if changed {
		err = j.commit(scope, document)
	}
	return out, err
}

func (j *ActivityMessageJournal) Complete(ctx context.Context, scope Scope, clientID string) error {
	if err := j.validateScope(ctx, scope); err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	document, err := j.load(scope)
	if err != nil {
		return err
	}
	kept := document.Records[:0]
	found := false
	for _, record := range document.Records {
		if record.ClientMessageID == clientID {
			found = true
			continue
		}
		kept = append(kept, record)
	}
	if !found {
		return fmt.Errorf("projects-v2: activity journal record not found")
	}
	document.Records = kept
	return j.commit(scope, document)
}

func (j *ActivityMessageJournal) expire(document *activityJournalDocument) bool {
	cutoff := j.now().UTC().Add(-activityJournalRetention)
	changed := false
	for i := range document.Records {
		if document.Records[i].State != "unresolved_expired" && !document.Records[i].CreatedAt.After(cutoff) {
			document.Records[i].State = "unresolved_expired"
			document.Records[i].Text = ""
			document.Records[i].MentionUserIDs = []string{}
			document.Records[i].AssetIDs = []string{}
			document.Records[i].UpdatedAt = j.now().UTC()
			changed = true
		}
	}
	return changed
}

func (j *ActivityMessageJournal) load(scope Scope) (activityJournalDocument, error) {
	document := activityJournalDocument{SchemaVersion: 1, ScopeKey: scope.Key, Records: []ActivityMessageRecord{}}
	payload, err := os.ReadFile(j.path(scope))
	if os.IsNotExist(err) {
		return document, nil
	}
	if err != nil {
		return document, err
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return document, fmt.Errorf("projects-v2: decode activity journal: %w", err)
	}
	if document.SchemaVersion != 1 || document.ScopeKey != scope.Key || document.Records == nil {
		return document, fmt.Errorf("projects-v2: invalid activity journal identity")
	}
	return document, nil
}

func (j *ActivityMessageJournal) commit(scope Scope, document activityJournalDocument) error {
	sort.SliceStable(document.Records, func(i, k int) bool { return document.Records[i].CreatedAt.Before(document.Records[k].CreatedAt) })
	payload, err := json.Marshal(document)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return fileutil.AtomicWriteFileStrict(j.path(scope), payload, 0o600)
}

func (j *ActivityMessageJournal) path(scope Scope) string {
	return filepath.Join(j.root, "project-activity", "v2", scope.Key, "messages.json")
}

func (j *ActivityMessageJournal) randomID(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(j.random, raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func (j *ActivityMessageJournal) validateScope(ctx context.Context, expected Scope) error {
	current, err := j.scopes.CurrentProjectScope(ctx)
	if err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "activity journal operation belongs to a stale scope"}
	}
	return nil
}
