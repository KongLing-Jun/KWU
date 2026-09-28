package automation

import "context"

func (s *service) validateBulkTaskQuota(ctx context.Context, existing, targets []Task) error {
	if s.taskQuota == nil || len(targets) <= len(existing) {
		return nil
	}
	view, err := s.taskQuota.View(ctx, targets)
	if err != nil {
		return err
	}
	if view.State != TaskQuotaStateReady {
		return ErrTaskQuotaUnavailable
	}
	if view.Unlimited {
		return nil
	}
	if len(targets) > view.Limit {
		return &TaskLimitError{Plan: view.Plan, Used: len(targets), Limit: view.Limit}
	}
	return nil
}

func (s *service) prepareCreateTask(ctx context.Context, task Task) (Task, []Task, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, nil, err
	}
	if err := s.requireStore(); err != nil {
		return Task{}, nil, err
	}
	existing, err := s.store.LoadTasks()
	if err != nil {
		return Task{}, nil, err
	}
	candidate, err := NormalizeTask(task)
	if err != nil {
		return Task{}, nil, err
	}
	if err := ValidateSchedule(candidate.Schedule); err != nil {
		return Task{}, nil, err
	}
	return candidate, existing, nil
}

func (s *service) validateCreateTaskQuota(ctx context.Context, existing []Task, candidate Task) error {
	if s.taskQuota == nil {
		return nil
	}
	view, err := s.taskQuota.View(ctx, append(append([]Task(nil), existing...), candidate))
	if err != nil {
		return err
	}
	if view.State != TaskQuotaStateReady {
		return ErrTaskQuotaUnavailable
	}
	if view.Unlimited {
		return nil
	}
	if len(existing)+1 > view.Limit {
		return &TaskLimitError{Plan: view.Plan, Used: len(existing) + 1, Limit: view.Limit}
	}
	return nil
}
