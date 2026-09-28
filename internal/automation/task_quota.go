package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/fileutil"
)

const taskQuotaFileName = "task-quota.json"

const (
	TaskQuotaStateLoading TaskQuotaState = "loading"
	TaskQuotaStateReady   TaskQuotaState = "ready"

	TaskQuotaPlanAnonymous = "anonymous"
	TaskQuotaPlanFree      = "free"
	TaskQuotaPlanLite      = "lite"
	TaskQuotaPlanPlus      = "plus"
	TaskQuotaPlanPro       = "pro"
)

var (
	ErrTaskLimitExceeded    = errors.New("automation: task limit exceeded")
	ErrTaskQuotaUnavailable = errors.New("automation: task quota unavailable")
)

type TaskQuotaState string

// TaskQuotaEntitlement is the effective local entitlement used by automation.
type TaskQuotaEntitlement struct {
	State     TaskQuotaState `json:"state"`
	Plan      string         `json:"plan,omitempty"`
	Limit     int            `json:"limit"`
	Unlimited bool           `json:"unlimited"`
}

// TaskQuotaSnapshot is the locally persisted, non-sensitive subscription fact.
// Paid plans retain their period end so offline restarts can downgrade them to
// Free after the verified subscription expires.
type TaskQuotaSnapshot struct {
	Plan       string     `json:"plan"`
	PeriodEnd  *time.Time `json:"periodEnd,omitempty"`
	VerifiedAt time.Time  `json:"verifiedAt"`
}

// TaskQuotaView is returned to hosts and the frontend.
type TaskQuotaView struct {
	State         TaskQuotaState `json:"state"`
	Plan          string         `json:"plan,omitempty"`
	Used          int            `json:"used"`
	Limit         int            `json:"limit"`
	Unlimited     bool           `json:"unlimited"`
	CanCreate     bool           `json:"canCreate"`
	PausedTaskIDs []string       `json:"pausedTaskIds"`
}

type TaskQuotaResolver interface {
	ResolveTaskQuota(context.Context) (TaskQuotaEntitlement, error)
}

type TaskQuotaResolverFunc func(context.Context) (TaskQuotaEntitlement, error)

func (f TaskQuotaResolverFunc) ResolveTaskQuota(ctx context.Context) (TaskQuotaEntitlement, error) {
	return f(ctx)
}

// FileTaskQuotaResolver shares one local entitlement between Desktop and local
// serve processes. MissingState is Loading for Desktop startup and Ready for a
// standalone local serve process, where a missing snapshot means anonymous.
type FileTaskQuotaResolver struct {
	dir          string
	missingState TaskQuotaState
	now          func() time.Time
}

func NewFileTaskQuotaResolver(dir string, missingState TaskQuotaState) *FileTaskQuotaResolver {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir()
	}
	if missingState != TaskQuotaStateReady {
		missingState = TaskQuotaStateLoading
	}
	return &FileTaskQuotaResolver{dir: dir, missingState: missingState, now: time.Now}
}

func (r *FileTaskQuotaResolver) ResolveTaskQuota(ctx context.Context) (TaskQuotaEntitlement, error) {
	if err := ctx.Err(); err != nil {
		return TaskQuotaEntitlement{}, err
	}
	data, err := os.ReadFile(r.path())
	if errors.Is(err, os.ErrNotExist) {
		if r.missingState == TaskQuotaStateReady {
			return entitlementForPlan(TaskQuotaPlanAnonymous), nil
		}
		return TaskQuotaEntitlement{State: TaskQuotaStateLoading}, nil
	}
	if err != nil {
		return TaskQuotaEntitlement{}, fmt.Errorf("%w: %v", ErrTaskQuotaUnavailable, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return TaskQuotaEntitlement{}, fmt.Errorf("%w: empty local quota snapshot", ErrTaskQuotaUnavailable)
	}
	var snapshot TaskQuotaSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return TaskQuotaEntitlement{}, fmt.Errorf("%w: decode local quota snapshot: %v", ErrTaskQuotaUnavailable, err)
	}
	return entitlementForSnapshot(snapshot, r.now().UTC())
}

func entitlementForSnapshot(snapshot TaskQuotaSnapshot, now time.Time) (TaskQuotaEntitlement, error) {
	plan := normalizeTaskQuotaPlan(snapshot.Plan)
	if plan == "" {
		return TaskQuotaEntitlement{}, fmt.Errorf("%w: invalid local quota plan %q", ErrTaskQuotaUnavailable, snapshot.Plan)
	}
	if isPaidTaskQuotaPlan(plan) {
		if snapshot.PeriodEnd == nil || !snapshot.PeriodEnd.After(now) {
			plan = TaskQuotaPlanFree
		}
	}
	return entitlementForPlan(plan), nil
}

func (r *FileTaskQuotaResolver) Save(snapshot TaskQuotaSnapshot) error {
	plan := normalizeTaskQuotaPlan(snapshot.Plan)
	if plan == "" {
		return fmt.Errorf("automation: invalid task quota plan %q", snapshot.Plan)
	}
	snapshot.Plan = plan
	if isPaidTaskQuotaPlan(plan) && snapshot.PeriodEnd == nil {
		return fmt.Errorf("automation: paid task quota requires period end")
	}
	if !isPaidTaskQuotaPlan(plan) {
		snapshot.PeriodEnd = nil
	}
	if snapshot.VerifiedAt.IsZero() {
		snapshot.VerifiedAt = r.now().UTC()
	} else {
		snapshot.VerifiedAt = snapshot.VerifiedAt.UTC()
	}
	if snapshot.PeriodEnd != nil {
		periodEnd := snapshot.PeriodEnd.UTC()
		snapshot.PeriodEnd = &periodEnd
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("automation: encode task quota snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := fileutil.AtomicWriteFile(r.path(), data, 0o600); err != nil {
		return fmt.Errorf("automation: save task quota snapshot: %w", err)
	}
	return nil
}

func (r *FileTaskQuotaResolver) Clear() error {
	if err := os.Remove(r.path()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("automation: clear task quota snapshot: %w", err)
	}
	return nil
}

func (r *FileTaskQuotaResolver) path() string {
	return filepath.Join(r.dir, taskQuotaFileName)
}

type TaskQuotaPolicy struct {
	resolver TaskQuotaResolver
}

func NewTaskQuotaPolicy(resolver TaskQuotaResolver) *TaskQuotaPolicy {
	if resolver == nil {
		return nil
	}
	return &TaskQuotaPolicy{resolver: resolver}
}

func (p *TaskQuotaPolicy) View(ctx context.Context, tasks []Task) (TaskQuotaView, error) {
	if p == nil || p.resolver == nil {
		return TaskQuotaView{}, nil
	}
	entitlement, err := p.resolver.ResolveTaskQuota(ctx)
	if err != nil {
		if errors.Is(err, ErrTaskQuotaUnavailable) {
			return TaskQuotaView{}, err
		}
		return TaskQuotaView{}, fmt.Errorf("%w: %v", ErrTaskQuotaUnavailable, err)
	}
	view := TaskQuotaView{
		State:     entitlement.State,
		Plan:      entitlement.Plan,
		Used:      len(tasks),
		Limit:     entitlement.Limit,
		Unlimited: entitlement.Unlimited,
	}
	if entitlement.State != TaskQuotaStateReady {
		view.PausedTaskIDs = taskIDsByAge(tasks)
		return view, nil
	}
	if entitlement.Unlimited {
		view.CanCreate = true
		return view, nil
	}
	view.CanCreate = len(tasks) < entitlement.Limit
	ordered := tasksByAge(tasks)
	if entitlement.Limit < len(ordered) {
		view.PausedTaskIDs = make([]string, 0, len(ordered)-entitlement.Limit)
		for _, task := range ordered[entitlement.Limit:] {
			view.PausedTaskIDs = append(view.PausedTaskIDs, task.ID)
		}
	}
	return view, nil
}

func entitlementForPlan(plan string) TaskQuotaEntitlement {
	normalized := normalizeTaskQuotaPlan(plan)
	return TaskQuotaEntitlement{
		State:     TaskQuotaStateReady,
		Plan:      plan,
		Limit:     TaskLimitForPlan(plan),
		Unlimited: normalized == TaskQuotaPlanPro,
	}
}

func TaskLimitForPlan(plan string) int {
	switch normalizeTaskQuotaPlan(plan) {
	case TaskQuotaPlanAnonymous:
		return 5
	case TaskQuotaPlanLite:
		return 25
	case TaskQuotaPlanPlus:
		return 50
	case TaskQuotaPlanPro:
		return 0
	default:
		return 10
	}
}

func normalizeTaskQuotaPlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case TaskQuotaPlanAnonymous:
		return TaskQuotaPlanAnonymous
	case TaskQuotaPlanFree:
		return TaskQuotaPlanFree
	case TaskQuotaPlanLite:
		return TaskQuotaPlanLite
	case TaskQuotaPlanPlus:
		return TaskQuotaPlanPlus
	case TaskQuotaPlanPro:
		return TaskQuotaPlanPro
	default:
		return ""
	}
}

func isPaidTaskQuotaPlan(plan string) bool {
	return plan == TaskQuotaPlanLite || plan == TaskQuotaPlanPlus || plan == TaskQuotaPlanPro
}

func tasksByAge(tasks []Task) []Task {
	ordered := append([]Task(nil), tasks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})
	return ordered
}

func taskIDsByAge(tasks []Task) []string {
	ordered := tasksByAge(tasks)
	ids := make([]string, len(ordered))
	for i, task := range ordered {
		ids[i] = task.ID
	}
	return ids
}

func taskQuotaAllows(view TaskQuotaView, taskID string) bool {
	if view.State != TaskQuotaStateReady {
		return false
	}
	for _, pausedID := range view.PausedTaskIDs {
		if pausedID == taskID {
			return false
		}
	}
	return true
}

type TaskLimitError struct {
	TaskID string
	Plan   string
	Used   int
	Limit  int
}

func (e *TaskLimitError) Error() string {
	if e.TaskID != "" {
		return fmt.Sprintf("当前套餐最多可运行 %d 个自动化任务；任务 %s 已因超出额度暂停", e.Limit, e.TaskID)
	}
	return fmt.Sprintf("当前套餐最多可创建 %d 个自动化任务（当前 %d 个）", e.Limit, e.Used)
}

func (e *TaskLimitError) Unwrap() error {
	return ErrTaskLimitExceeded
}
