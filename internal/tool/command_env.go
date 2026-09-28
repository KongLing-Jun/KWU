package tool

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
)

const CommandEnvPATH = "PATH"

var commandEnvNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type commandEnvContextKey struct{}

// CommandEnvProvider resolves trusted host environment values on first use.
// Providers run only after a tool call has passed the permission gate; the
// result is then frozen for the rest of that task context.
type CommandEnvProvider interface {
	ResolveCommandEnv(context.Context) (map[string]string, error)
}

// CommandEnvRequest describes the literal executable aliases that must bind to
// the environment selected for a task.
type CommandEnvRequest struct {
	Commands []string
	PathExt  string
}

// CommandEnvRequestProvider optionally selects an environment that can satisfy
// the first task execution request. The selected environment is still frozen
// for the rest of the task.
type CommandEnvRequestProvider interface {
	ResolveCommandEnvForRequest(context.Context, CommandEnvRequest) (map[string]string, error)
}

type commandEnvProviderContextKey struct{}

type commandEnvResolution struct {
	ready   chan struct{}
	request CommandEnvRequest
	env     map[string]string
	err     error
}

type commandEnvSnapshot struct {
	provider    CommandEnvProvider
	taskContext context.Context
	taskID      string
	state       atomic.Pointer[commandEnvResolution]
}

// ValidateCommandEnv validates and clones the host-only task environment. An
// empty non-PATH value is an explicit request to remove that variable from the
// child process environment.
func ValidateCommandEnv(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		key, err := normalizeCommandEnvKey(key, out)
		if err != nil {
			return nil, err
		}
		if err := validateCommandEnvValue(key, value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}

func normalizeCommandEnvKey(key string, existing map[string]string) (string, error) {
	if !commandEnvNamePattern.MatchString(key) {
		return "", errors.New("invalid command environment key")
	}
	if runtime.GOOS != "windows" {
		return key, nil
	}
	key = strings.ToUpper(key)
	if _, exists := existing[key]; exists {
		return "", errors.New("duplicate command environment key")
	}
	return key, nil
}

func validateCommandEnvValue(key, value string) error {
	if strings.ContainsAny(value, "\x00\r\n") {
		return errors.New("invalid command environment value")
	}
	if key != CommandEnvPATH {
		return nil
	}
	if value == "" {
		return errors.New("command environment PATH is empty")
	}
	parts := filepath.SplitList(value)
	if len(parts) == 0 {
		return errors.New("command environment PATH is empty")
	}
	for _, part := range parts {
		if !filepath.IsAbs(part) {
			return errors.New("command environment PATH must contain absolute managed directories")
		}
	}
	return nil
}

func WithCommandEnv(ctx context.Context, input map[string]string) context.Context {
	if len(input) == 0 {
		return ctx
	}
	copy := make(map[string]string, len(input))
	for key, value := range input {
		copy[key] = value
	}
	return context.WithValue(ctx, commandEnvContextKey{}, copy)
}

// WithCommandEnvProvider binds a provider through a task-local atomic snapshot.
// Rebinding the same context preserves its first snapshot so derived calls and
// concurrent subagents cannot observe different interpreter selections.
func WithCommandEnvProvider(ctx context.Context, provider CommandEnvProvider) context.Context {
	return withCommandEnvProviderForTask(ctx, "", provider)
}

// WithCommandEnvProviderForTask binds one provider snapshot to an opaque task
// identity. Rebinding the same task preserves its snapshot; a new top-level
// task receives a fresh snapshot even when its context inherits the old one.
func WithCommandEnvProviderForTask(ctx context.Context, taskID string, provider CommandEnvProvider) context.Context {
	return withCommandEnvProviderForTask(ctx, taskID, provider)
}

func withCommandEnvProviderForTask(ctx context.Context, taskID string, provider CommandEnvProvider) context.Context {
	if provider == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if snapshot, ok := ctx.Value(commandEnvProviderContextKey{}).(*commandEnvSnapshot); ok {
		if taskID == "" || snapshot.taskID == taskID {
			return ctx
		}
	}
	return context.WithValue(ctx, commandEnvProviderContextKey{}, &commandEnvSnapshot{provider: provider, taskContext: ctx, taskID: taskID})
}

func CommandEnvFromContext(ctx context.Context) map[string]string {
	if ctx == nil {
		return nil
	}
	input, _ := ctx.Value(commandEnvContextKey{}).(map[string]string)
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// ResolveCommandEnv returns the immutable provider snapshot merged with static
// host values. Provider PATH entries stay first so a selected managed runtime
// cannot be shadowed by a connector or the process PATH; other static values win.
func ResolveCommandEnv(ctx context.Context) (map[string]string, error) {
	return ResolveCommandEnvForExecution(ctx, true)
}

// ResolveCommandEnvForExecution returns the static host environment and, when
// requested by the command being executed, the task-frozen runtime provider.
func ResolveCommandEnvForExecution(ctx context.Context, includeProvider bool) (map[string]string, error) {
	return ResolveCommandEnvForExecutionRequest(ctx, includeProvider, CommandEnvRequest{})
}

// ResolveCommandEnvForExecutionRequest resolves the environment for the first
// execution request. The first request and result are frozen atomically; later
// requests observe the same result and must validate against it themselves.
func ResolveCommandEnvForExecutionRequest(ctx context.Context, includeProvider bool, request CommandEnvRequest) (map[string]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	static, err := ValidateCommandEnv(CommandEnvFromContext(ctx))
	if err != nil {
		return nil, err
	}
	if !includeProvider {
		return static, nil
	}
	snapshot, _ := ctx.Value(commandEnvProviderContextKey{}).(*commandEnvSnapshot)
	if snapshot == nil {
		return static, nil
	}
	request, err = validateCommandEnvRequest(request)
	if err != nil {
		return nil, err
	}
	if pathExt, ok := static["PATHEXT"]; ok {
		request.PathExt = pathExt
	}
	provided, err := snapshot.resolve(ctx, request)
	if err != nil {
		return nil, err
	}
	return mergeCommandEnv(provided, static), nil
}

func (s *commandEnvSnapshot) resolve(ctx context.Context, request CommandEnvRequest) (map[string]string, error) {
	state := s.state.Load()
	if state == nil {
		candidate := &commandEnvResolution{ready: make(chan struct{}), request: request}
		if s.state.CompareAndSwap(nil, candidate) {
			state = candidate
			go s.publish(candidate)
		} else {
			state = s.state.Load()
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-state.ready:
		return cloneCommandEnv(state.env), state.err
	}
}

func (s *commandEnvSnapshot) publish(candidate *commandEnvResolution) {
	defer close(candidate.ready)
	defer func() {
		if recovered := recover(); recovered != nil {
			candidate.err = fmt.Errorf("resolve task command environment: provider panic: %v", recovered)
		}
	}()
	var input map[string]string
	var err error
	if provider, ok := s.provider.(CommandEnvRequestProvider); ok {
		input, err = provider.ResolveCommandEnvForRequest(s.taskContext, candidate.request)
	} else {
		input, err = s.provider.ResolveCommandEnv(s.taskContext)
	}
	if err == nil {
		input, err = ValidateCommandEnv(input)
	}
	if err != nil {
		candidate.err = fmt.Errorf("resolve task command environment: %w", err)
		return
	}
	candidate.env = input
}

func validateCommandEnvRequest(request CommandEnvRequest) (CommandEnvRequest, error) {
	if len(request.Commands) == 0 {
		return CommandEnvRequest{}, nil
	}
	commands := make([]string, len(request.Commands))
	for i, command := range request.Commands {
		if strings.ContainsAny(command, "\x00\r\n") {
			return CommandEnvRequest{}, errors.New("invalid command environment request")
		}
		commands[i] = command
	}
	if strings.ContainsAny(request.PathExt, "\x00\r\n") {
		return CommandEnvRequest{}, errors.New("invalid command environment request")
	}
	return CommandEnvRequest{Commands: commands, PathExt: request.PathExt}, nil
}

func mergeCommandEnv(provided, static map[string]string) map[string]string {
	if len(provided) == 0 {
		return cloneCommandEnv(static)
	}
	out := cloneCommandEnv(provided)
	for key, value := range static {
		if key == CommandEnvPATH {
			out[key] = mergeManagedPathLists(out[key], value)
			continue
		}
		if providedValue, exists := provided[key]; exists && providedValue == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func cloneCommandEnv(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func mergeManagedPathLists(first, second string) string {
	seen := make(map[string]bool)
	parts := make([]string, 0, len(filepath.SplitList(first))+len(filepath.SplitList(second)))
	add := func(list string) {
		for _, part := range filepath.SplitList(list) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			clean := filepath.Clean(part)
			key := clean
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			parts = append(parts, clean)
		}
	}
	add(first)
	add(second)
	return strings.Join(parts, string(filepath.ListSeparator))
}
