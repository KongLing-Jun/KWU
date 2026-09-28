package serve

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kwu/internal/automation"
)

func (s *Server) SetAutomationService(svc automation.Service) {
	s.automation = svc
}

func (s *Server) automationService(w http.ResponseWriter) (automation.Service, bool) {
	if s.automation == nil {
		http.Error(w, "automation disabled", http.StatusServiceUnavailable)
		return nil, false
	}
	return s.automation, true
}

func (s *Server) automationListTasks(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	tasks, err := svc.ListTasks(r.Context())
	writeAutomationJSON(w, tasks, err)
}

func (s *Server) automationHeartbeatMigrationStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	status, err := svc.MigrationStatus(r.Context())
	writeAutomationJSON(w, status, err)
}

func (s *Server) automationSaveTasks(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	var req automation.SaveTasksRequest
	if err := decodeJSONRequest(w, r, &req); err != nil {
		return
	}
	writeAutomationJSON(w, nil, svc.SaveTasks(r.Context(), req))
}

func (s *Server) automationTriggerNow(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	res, err := svc.TriggerNow(r.Context(), automation.TriggerNowRequest{
		TaskID: strings.TrimSpace(r.PathValue("id")),
		Reason: "serve manual trigger",
	})
	writeAutomationJSON(w, res, err)
}

func (s *Server) automationListRuns(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	if taskID == "" {
		http.Error(w, "taskId required", http.StatusBadRequest)
		return
	}
	limit, err := automationLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runs, err := svc.ListRuns(r.Context(), automation.RunFilter{TaskID: taskID, Limit: limit})
	writeAutomationJSON(w, runs, err)
}

func (s *Server) automationReadRun(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	detail, err := svc.ReadRun(r.Context(), strings.TrimSpace(r.PathValue("taskID")), strings.TrimSpace(r.PathValue("runID")))
	writeAutomationJSON(w, detail, err)
}

func (s *Server) automationCancelRun(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.automationService(w)
	if !ok {
		return
	}
	res, err := svc.CancelRun(r.Context(), automation.CancelRunRequest{
		TaskID: strings.TrimSpace(r.PathValue("taskID")),
		RunID:  strings.TrimSpace(r.PathValue("runID")),
	})
	writeAutomationJSON(w, res, err)
}

func automationLimit(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		return 0, errors.New("limit must be a non-negative integer")
	}
	return limit, nil
}

func writeAutomationJSON(w http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(w, err.Error(), automationErrorStatus(err))
		return
	}
	if value == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, value)
}

func automationErrorStatus(err error) int {
	switch {
	case errors.Is(err, automation.ErrTaskNotFound):
		return http.StatusNotFound
	case errors.Is(err, automation.ErrTaskLimitExceeded):
		return http.StatusConflict
	case errors.Is(err, automation.ErrTaskQuotaUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, automation.ErrInvalidTask),
		errors.Is(err, automation.ErrInvalidSchedule),
		errors.Is(err, automation.ErrInvalidApprovalMode),
		errors.Is(err, automation.ErrDuplicateTaskID):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
