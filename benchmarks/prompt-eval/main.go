package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kwu/internal/boot"
	"kwu/internal/event"
	"kwu/internal/scene"
	"kwu/internal/userctx"
)

const (
	graderVersion = "prompt-eval-v6"
	apiKeyEnv     = "PROMPT_EVAL_API_KEY"
)

type options struct {
	Endpoint, Model, Suite, TaskID, PromptFile, SceneMode, OutRoot string
	RuntimeRoot, WorkerResultDir, RescoreDir                       string
	DesktopPrepare, DesktopGrade, DesktopObservation               string
	Repeats, MaxSteps, WorkerRepeat                                int
	Timeout                                                        time.Duration
	Manifest, Worker                                               bool
}

type metrics struct {
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	ReasoningTokens  int      `json:"reasoning_tokens"`
	ModelCalls       int      `json:"model_calls"`
	ToolCalls        int      `json:"tool_calls"`
	ToolErrors       int      `json:"tool_errors"`
	Tools            []string `json:"tools"`
	ToolSequence     []string `json:"tool_sequence"`
	FinalText        string   `json:"-"`
}

type result struct {
	TaskID         string      `json:"task_id"`
	Scene          scene.Scene `json:"scene"`
	Repeat         int         `json:"repeat"`
	ArtifactPassed bool        `json:"artifact_passed"`
	RunPassed      bool        `json:"run_passed"`
	Passed         bool        `json:"passed"`
	GradeNote      string      `json:"grade_note"`
	RunError       string      `json:"run_error,omitempty"`
	DurationMS     int64       `json:"duration_ms"`
	Metrics        metrics     `json:"metrics"`
}

type params struct {
	Repeats         int    `json:"repeats"`
	MaxSteps        int    `json:"max_steps"`
	Temperature     int    `json:"temperature"`
	ContextWindow   int    `json:"context_window"`
	TimeoutSeconds  int64  `json:"timeout_seconds"`
	ApprovalSeconds int64  `json:"approval_seconds"`
	Effort          string `json:"effort"`
	AutoPlan        string `json:"auto_plan"`
	PermissionMode  string `json:"permission_mode"`
	ShellPreference string `json:"shell_preference"`
	Sandbox         string `json:"sandbox"`
}

type report struct {
	Commit         string    `json:"commit"`
	Model          string    `json:"model"`
	Endpoint       string    `json:"endpoint"`
	Suite          string    `json:"suite"`
	PromptSHA256   string    `json:"prompt_sha256"`
	PromptBytes    int       `json:"prompt_bytes"`
	PromptRunes    int       `json:"prompt_runes"`
	PromptTokens   int       `json:"prompt_tokens_estimate"`
	ManifestSHA256 string    `json:"manifest_sha256"`
	SourceManifest string    `json:"source_manifest_sha256,omitempty"`
	GraderVersion  string    `json:"grader_version"`
	SceneMode      string    `json:"scene_mode"`
	StartedAt      time.Time `json:"started_at"`
	DurationMS     int64     `json:"duration_ms"`
	Parameters     params    `json:"parameters"`
	Results        []result  `json:"results"`
}

type collector struct{ metrics metrics }

func (c *collector) Emit(e event.Event) {
	switch e.Kind {
	case event.Usage:
		if e.Usage != nil {
			c.metrics.PromptTokens += e.Usage.PromptTokens
			c.metrics.CompletionTokens += e.Usage.CompletionTokens
			c.metrics.TotalTokens += e.Usage.TotalTokens
			c.metrics.ReasoningTokens += e.Usage.ReasoningTokens
			c.metrics.ModelCalls++
		}
	case event.ToolDispatch:
		if !e.Tool.Partial {
			c.metrics.ToolCalls++
			c.metrics.Tools = append(c.metrics.Tools, e.Tool.Name)
			c.metrics.ToolSequence = append(c.metrics.ToolSequence, e.Tool.Name)
		}
	case event.ToolResult:
		if e.Tool.Err != "" {
			c.metrics.ToolErrors++
		}
	case event.Message:
		c.metrics.FinalText = e.Text
	}
}

func main() {
	opts := parseFlags()
	if opts.DesktopPrepare != "" || opts.DesktopGrade != "" {
		if err := runDesktopEval(opts); err != nil {
			fatal(err)
		}
		return
	}
	tasks, err := tasksForSuite(opts.Suite)
	if err != nil {
		fatal(err)
	}
	tasks, err = selectTasks(tasks, opts.TaskID)
	if err != nil {
		fatal(err)
	}
	if opts.RescoreDir != "" {
		if err := rescoreReport(opts.RescoreDir, tasks); err != nil {
			fatal(err)
		}
		return
	}
	prompt, err := readPrompt(opts.PromptFile)
	if err != nil {
		fatal(err)
	}
	if opts.Worker {
		if len(tasks) != 1 || opts.WorkerRepeat < 1 || opts.RuntimeRoot == "" || opts.WorkerResultDir == "" {
			fatal(errors.New("invalid isolated worker arguments"))
		}
		if err := json.NewEncoder(os.Stdout).Encode(runOne(opts.RuntimeRoot, opts.WorkerResultDir, opts, tasks[0], opts.WorkerRepeat)); err != nil {
			fatal(err)
		}
		return
	}
	manifest := manifestHash(tasks, prompt, opts)
	if opts.Manifest {
		fmt.Println(manifest)
		return
	}
	if strings.TrimSpace(opts.Endpoint) == "" || strings.TrimSpace(opts.Model) == "" {
		fatal(errors.New("-endpoint and -model are required for a real run"))
	}
	if strings.TrimSpace(os.Getenv(apiKeyEnv)) == "" {
		fatal(fmt.Errorf("%s is required for a real run", apiKeyEnv))
	}
	if err := validateRunLimits(opts); err != nil {
		fatal(err)
	}
	resultDir, runtimeRoot, cleanup, err := prepareRuntime(opts, prompt)
	if err != nil {
		fatal(err)
	}
	defer cleanup()
	started := time.Now()
	results := make([]result, 0, len(tasks)*opts.Repeats)
	for _, task := range tasks {
		for repeat := 1; repeat <= opts.Repeats; repeat++ {
			r := runIsolated(runtimeRoot, resultDir, opts, task, repeat)
			results = append(results, r)
			status := "FAIL"
			if r.Passed {
				status = "PASS"
			}
			fmt.Printf("%s %s repeat=%d artifact=%t run=%t tokens=%d calls=%d tools=%d duration=%s note=%s\n",
				status, task.ID, repeat, r.ArtifactPassed, r.RunPassed, r.Metrics.TotalTokens,
				r.Metrics.ModelCalls, r.Metrics.ToolCalls, time.Duration(r.DurationMS)*time.Millisecond, r.GradeNote)
		}
	}
	rep := report{
		Commit: gitOutput("rev-parse", "HEAD"), Model: opts.Model, Endpoint: opts.Endpoint,
		Suite: opts.Suite, PromptSHA256: textHash(prompt), ManifestSHA256: manifest,
		PromptBytes: len(prompt), PromptRunes: utf8.RuneCountInString(prompt), PromptTokens: estimateTextTokens(prompt),
		GraderVersion: graderVersion, SceneMode: opts.SceneMode, StartedAt: started,
		DurationMS: time.Since(started).Milliseconds(), Parameters: fixedParams(opts), Results: results,
	}
	if err := writeReport(resultDir, "report", rep); err != nil {
		fatal(err)
	}
	fmt.Printf("REPORT %s\n", resultDir)
}

func validateRunLimits(opts options) error {
	if opts.Repeats < 1 || opts.Timeout <= 0 {
		return errors.New("repeats and timeout must be positive")
	}
	if opts.MaxSteps < 0 {
		return errors.New("max-steps must be non-negative (0 means unlimited)")
	}
	return nil
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.Endpoint, "endpoint", "", "OpenAI-compatible base URL; required for real runs")
	flag.StringVar(&opts.Model, "model", "", "provider model ID; required for real runs")
	flag.StringVar(&opts.Suite, "suite", "diagnostic", "diagnostic|regression|all")
	flag.StringVar(&opts.TaskID, "task", "", "optional exact task ID for a smoke run")
	flag.StringVar(&opts.PromptFile, "prompt-file", "", "optional base system prompt candidate")
	flag.StringVar(&opts.SceneMode, "scene-mode", "current", "current|unset")
	flag.StringVar(&opts.OutRoot, "out", ".codex-run/prompt-eval/results", "ignored result root")
	flag.StringVar(&opts.RescoreDir, "rescore-dir", "", "rescore an existing raw result directory without model calls")
	flag.StringVar(&opts.DesktopPrepare, "desktop-prepare", "", "create one frozen Desktop task workspace at this path")
	flag.StringVar(&opts.DesktopGrade, "desktop-grade", "", "grade one completed frozen Desktop task workspace at this path")
	flag.StringVar(&opts.DesktopObservation, "desktop-observation", "", "optional Desktop run observation JSON used with -desktop-grade")
	flag.IntVar(&opts.Repeats, "repeats", 2, "runs per task")
	flag.IntVar(&opts.MaxSteps, "max-steps", 18, "controller step cap; 0 means unlimited")
	flag.DurationVar(&opts.Timeout, "timeout", 4*time.Minute, "per-run timeout")
	flag.BoolVar(&opts.Manifest, "manifest-only", false, "print manifest without network access")
	flag.BoolVar(&opts.Worker, "worker", false, "internal: execute one isolated task")
	flag.StringVar(&opts.RuntimeRoot, "runtime-root", "", "internal: isolated profile root")
	flag.StringVar(&opts.WorkerResultDir, "worker-result-dir", "", "internal: run result root")
	flag.IntVar(&opts.WorkerRepeat, "worker-repeat", 0, "internal: repeat number")
	flag.Parse()
	return opts
}

func runIsolated(runtimeRoot, resultDir string, opts options, task evalTask, repeat int) result {
	executable, err := os.Executable()
	if err != nil {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "worker setup failed", RunError: err.Error()}
	}
	args := []string{
		"-worker", "-suite", opts.Suite, "-task", task.ID,
		"-runtime-root", runtimeRoot, "-worker-result-dir", resultDir,
		"-worker-repeat", strconv.Itoa(repeat), "-max-steps", strconv.Itoa(opts.MaxSteps),
		"-timeout", opts.Timeout.String(), "-scene-mode", opts.SceneMode,
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout+10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stderr = os.Stderr
	// On Windows a killed worker can leave a descendant holding the captured
	// stdout pipe open. Bound Cmd.Wait's pipe-drain phase as well as the process.
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		work := filepath.Join(resultDir, "runs", fmt.Sprintf("%s-r%d", task.ID, repeat))
		artifactPassed, note := task.Grade(work)
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, ArtifactPassed: artifactPassed,
			GradeNote: note + "; worker killed after cancellation grace", RunError: "hard timeout", DurationMS: opts.Timeout.Milliseconds()}
	}
	if err != nil {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "worker failed", RunError: err.Error()}
	}
	var r result
	if err := json.Unmarshal(output, &r); err != nil {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "worker report invalid", RunError: err.Error()}
	}
	return r
}

func selectTasks(tasks []evalTask, taskID string) ([]evalTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return tasks, nil
	}
	for _, task := range tasks {
		if task.ID == taskID {
			return []evalTask{task}, nil
		}
	}
	return nil, fmt.Errorf("task %q is not in the selected suite", taskID)
}

func readPrompt(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt candidate: %w", err)
	}
	prompt := strings.TrimSpace(string(b))
	if prompt == "" {
		return "", errors.New("prompt candidate is empty")
	}
	return prompt, nil
}

func prepareRuntime(opts options, prompt string) (string, string, func(), error) {
	resultDir, err := filepath.Abs(filepath.Join(opts.OutRoot, opts.Suite+"-"+time.Now().Format("20060102-150405")))
	if err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		return "", "", nil, err
	}
	runtimeRoot, err := os.MkdirTemp("", "clouvia-prompt-eval-")
	if err != nil {
		return "", "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(runtimeRoot) }
	homeDir := filepath.Join(runtimeRoot, "home")
	stateDir := filepath.Join(runtimeRoot, "state")
	cacheDir := filepath.Join(runtimeRoot, "cache")
	for _, dir := range []string{homeDir, stateDir, cacheDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	// Production credential resolution intentionally ignores inherited provider
	// environment variables. Pin the explicitly supplied benchmark key into the
	// isolated profile credential file, which cleanup removes after the run.
	credential := apiKeyEnv + "=" + os.Getenv(apiKeyEnv) + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, ".env"), []byte(credential), 0o600); err != nil {
		cleanup()
		return "", "", nil, err
	}
	for name, value := range map[string]string{
		"CLOUVIA_HOME": homeDir, "CLOUVIA_STATE_HOME": stateDir,
		"CLOUVIA_CACHE_HOME": cacheDir, "KWU_CREDENTIALS_STORE": "file",
	} {
		if err := os.Setenv(name, value); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	promptPath := ""
	if prompt != "" {
		promptPath = filepath.Join(runtimeRoot, "candidate.md")
		if err := os.WriteFile(promptPath, []byte(prompt+"\n"), 0o600); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(homeDir, "config.toml"), []byte(renderConfig(opts, promptPath)), 0o600); err != nil {
		cleanup()
		return "", "", nil, err
	}
	return resultDir, runtimeRoot, cleanup, nil
}

func renderConfig(opts options, promptPath string) string {
	var b strings.Builder
	b.WriteString("default_model = \"eval-target\"\nlanguage = \"zh\"\ncredentials_store = \"file\"\n\n")
	fmt.Fprintf(&b, "[agent]\nmax_steps = %d\ntemperature = 0.0\nauto_plan = \"off\"\n", opts.MaxSteps)
	if promptPath != "" {
		fmt.Fprintf(&b, "system_prompt_file = %s\n", strconv.Quote(filepath.ToSlash(promptPath)))
	}
	fmt.Fprintf(&b, "\n[[providers]]\nname = \"eval-target\"\nkind = \"openai\"\nbase_url = %s\nmodel = %s\napi_key_env = %s\ncontext_window = 200000\nreasoning_protocol = \"openai\"\nsupported_efforts = [\"low\", \"medium\", \"high\"]\ndefault_effort = \"low\"\n", strconv.Quote(opts.Endpoint), strconv.Quote(opts.Model), strconv.Quote(apiKeyEnv))
	b.WriteString("\n[tools]\nbash_timeout_seconds = 60\n\n[tools.shell]\nprefer = \"powershell\"\n\n[permissions]\nmode = \"allow\"\ndeny = [\"Bash(rm -rf*)\", \"Bash(git push*)\", \"Bash(git commit*)\"]\n")
	return b.String()
}

func runOne(runtimeRoot, resultDir string, opts options, task evalTask, repeat int) result {
	work := filepath.Join(resultDir, "runs", fmt.Sprintf("%s-r%d", task.ID, repeat))
	if err := seedWorkdir(work, task.Seed); err != nil {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "setup failed", RunError: err.Error()}
	}
	sink := &collector{}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	promptScene := task.Scene
	if opts.SceneMode == "unset" {
		promptScene = scene.Unset
	} else if opts.SceneMode != "current" {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "invalid scene mode", RunError: opts.SceneMode}
	}
	ctrl, err := boot.Build(ctx, boot.Options{
		Model: "eval-target", MaxSteps: opts.MaxSteps, RequireKey: true, Sink: sink,
		WorkspaceRoot: work, Actor: userctx.Default(work),
		SessionDir: filepath.Join(runtimeRoot, "sessions", fmt.Sprintf("%s-r%d", task.ID, repeat)),
		Scene:      promptScene, SceneState: scene.Locked, ApprovalTimeout: 3 * time.Second,
	})
	if err != nil {
		return result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, GradeNote: "boot failed", RunError: err.Error()}
	}
	defer ctrl.Close()
	started := time.Now()
	runErr := ctrl.Run(ctx, task.Prompt)
	artifactPassed, note := task.Grade(work)
	r := result{TaskID: task.ID, Scene: task.Scene, Repeat: repeat, ArtifactPassed: artifactPassed,
		RunPassed: runErr == nil, GradeNote: note, DurationMS: time.Since(started).Milliseconds(), Metrics: sink.metrics}
	if runErr != nil {
		r.RunError = runErr.Error()
	}
	r.Passed = r.ArtifactPassed && r.RunPassed
	r.Metrics.Tools = uniqueSorted(r.Metrics.Tools)
	return r
}

func seedWorkdir(work string, seed map[string]string) error {
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	for name, body := range seed {
		path := filepath.Join(work, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func fixedParams(opts options) params {
	return params{Repeats: opts.Repeats, MaxSteps: opts.MaxSteps, TimeoutSeconds: int64(opts.Timeout / time.Second),
		ApprovalSeconds: 3, Effort: "low", Temperature: 0, AutoPlan: "off", ContextWindow: 200000,
		PermissionMode: "allow", ShellPreference: "powershell", Sandbox: "host-default; record platform warning"}
}

func manifestHash(tasks []evalTask, prompt string, opts options) string {
	type item struct {
		ID     string
		Scene  scene.Scene
		Prompt string
		Seed   map[string]string
	}
	items := make([]item, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, item{task.ID, task.Scene, task.Prompt, task.Seed})
	}
	payload := struct {
		Tasks                               []item
		Grader, PromptSHA, Suite, SceneMode string
		Parameters                          params
	}{items, graderVersion, textHash(prompt), opts.Suite, opts.SceneMode, fixedParams(opts)}
	b, _ := json.Marshal(payload)
	return textHash(string(b))
}

func textHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func writeReport(dir, stem string, rep report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".json"), b, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stem+".md"), []byte(renderMarkdown(rep)), 0o644)
}

func rescoreReport(dir string, tasks []evalTask) error {
	b, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		return err
	}
	var rep report
	if err := json.Unmarshal(b, &rep); err != nil {
		return err
	}
	byID := make(map[string]evalTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	passed := 0
	for i := range rep.Results {
		r := &rep.Results[i]
		task, ok := byID[r.TaskID]
		if !ok {
			return fmt.Errorf("result task %q is not in suite %q", r.TaskID, rep.Suite)
		}
		work := filepath.Join(dir, "runs", fmt.Sprintf("%s-r%d", r.TaskID, r.Repeat))
		r.ArtifactPassed, r.GradeNote = task.Grade(work)
		r.Passed = r.ArtifactPassed && r.RunPassed
		if r.Passed {
			passed++
		}
	}
	rep.SourceManifest = rep.ManifestSHA256
	rep.ManifestSHA256 = textHash("rescore\n" + rep.SourceManifest + "\n" + graderVersion)
	rep.GraderVersion = graderVersion
	if err := writeReport(dir, "report-rescored-"+graderVersion, rep); err != nil {
		return err
	}
	fmt.Printf("RESCORED %d/%d %s\n", passed, len(rep.Results), dir)
	return nil
}

func renderMarkdown(rep report) string {
	artifact, run, passed, promptTokens, completion, calls, tools := 0, 0, 0, 0, 0, 0, 0
	byScene := map[scene.Scene][2]int{}
	for _, r := range rep.Results {
		if r.ArtifactPassed {
			artifact++
		}
		if r.RunPassed {
			run++
		}
		if r.Passed {
			passed++
		}
		promptTokens += r.Metrics.PromptTokens
		completion += r.Metrics.CompletionTokens
		calls += r.Metrics.ModelCalls
		tools += r.Metrics.ToolCalls
		v := byScene[r.Scene]
		v[1]++
		if r.Passed {
			v[0]++
		}
		byScene[r.Scene] = v
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Prompt evaluation report\n\n- Commit: `%s`\n- Model: `%s`\n- Suite: `%s`\n- Manifest: `%s`\n- Prompt SHA-256: `%s`\n- Candidate size: %d estimated tokens (%d bytes, %d runes)\n- Artifact: **%d/%d**; run: **%d/%d**; end-to-end: **%d/%d**\n- Runtime tokens: %d prompt + %d completion = %d\n- Model calls: %d; tool calls: %d; duration: %s\n\n", rep.Commit, rep.Model, rep.Suite, rep.ManifestSHA256, rep.PromptSHA256, rep.PromptTokens, rep.PromptBytes, rep.PromptRunes, artifact, len(rep.Results), run, len(rep.Results), passed, len(rep.Results), promptTokens, completion, promptTokens+completion, calls, tools, time.Duration(rep.DurationMS)*time.Millisecond)
	b.WriteString("| Scene | Passed |\n| --- | ---: |\n")
	for _, s := range []scene.Scene{scene.Office, scene.Coding, scene.Design} {
		v := byScene[s]
		fmt.Fprintf(&b, "| %s | %d/%d |\n", s, v[0], v[1])
	}
	b.WriteString("\n| Task | Repeat | Artifact | Run | E2E | Tokens | Calls | Tools | Duration | Note |\n| --- | ---: | --- | --- | --- | ---: | ---: | ---: | ---: | --- |\n")
	for _, r := range rep.Results {
		note := strings.ReplaceAll(r.GradeNote, "|", "\\|")
		fmt.Fprintf(&b, "| %s | %d | %t | %t | %t | %d | %d | %d | %ds | %s |\n", r.TaskID, r.Repeat, r.ArtifactPassed, r.RunPassed, r.Passed, r.Metrics.TotalTokens, r.Metrics.ModelCalls, r.Metrics.ToolCalls, r.DurationMS/1000, note)
	}
	return b.String()
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// estimateTextTokens mirrors the repository's conservative cross-language
// context-budget estimate. Provider-reported runtime usage remains authoritative.
func estimateTextTokens(s string) int {
	if s == "" {
		return 0
	}
	byBytes := (len(s) + 3) / 4
	if runes := utf8.RuneCountInString(s); runes > byBytes {
		return runes
	}
	return byBytes
}

func gitOutput(args ...string) string {
	b, err := exec.Command("git", args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
