package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
)

func TestTaskToolHandsOffOnlyHostObservedArtifactWithoutFinalLink(t *testing.T) {
	const prompt = "实现单文件贪吃蛇浏览器游戏"

	root := t.TempDir()
	target := filepath.Join(root, "games", "snake.html")
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactBundleWriter{files: map[string]string{
		target: "<!doctype html><title>Snake</title>",
		filepath.Join(root, "reports", "run.log"): "passed",
		filepath.Join(root, "reports", "run.yml"): "status: passed",
	}})
	provider := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "export", Name: "opaque_bundle_export", Arguments: `{}`}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "游戏文件已完成。"},
			{Type: provider.ChunkDone},
		},
	}}
	task := newArtifactHandoffTestTask(t, provider, registry, root)
	request := newExpertTeamA2ARequest(testTaskContext(), "Implementation Expert", prompt, false)
	args, err := json.Marshal(map[string]any{
		"member_name": "Implementation Expert",
		"prompt":      prompt,
		"tools":       []string{"opaque_bundle_export"},
		"a2a":         request,
	})
	if err != nil {
		t.Fatal(err)
	}

	output, err := task.Execute(testTaskContext(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	response := expertTeamA2AResponseFromOutput(t, output)
	var got []string
	if response.Result != nil {
		for _, artifact := range response.Result.Task.Artifacts {
			for _, part := range artifact.Parts {
				if path, _ := part.Data["path"].(string); strings.TrimSpace(path) != "" {
					got = append(got, path)
				}
			}
		}
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("A2A workspace artifacts = %q, want only linked delta %q\n%s", got, want, output)
	}
}

func TestFailedSubagentRunDoesNotReturnSnapshotArtifacts(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "failed.html")
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactBundleWriter{files: map[string]string{target: "partial"}})
	provider := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "export", Name: "opaque_bundle_export", Arguments: `{}`}},
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkError, Err: errors.New("child failed after writing")}},
	}}
	task := newArtifactHandoffTestTask(t, provider, registry, root)

	_, artifacts, err := task.runSubSessionWithArtifactOutcome(
		testTaskContext(),
		"Generate a file in the workspace.",
		registry,
		event.Discard,
		20,
		subSessionRuntime{provider: provider},
		NewSession("sys"),
	)
	if err == nil || !strings.Contains(err.Error(), "child failed after writing") {
		t.Fatalf("run error = %v, want child failure", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("failed run returned artifacts: %+v", artifacts)
	}
}

func TestDefaultTaskSystemPromptDefinesLocalDeliverableLinkContract(t *testing.T) {
	if !strings.Contains(DefaultTaskSystemPrompt, "local file intended as a deliverable") ||
		!strings.Contains(DefaultTaskSystemPrompt, "Markdown link") {
		t.Fatalf("task system prompt lacks the local deliverable link contract:\n%s", DefaultTaskSystemPrompt)
	}
	if strings.Contains(DefaultTaskSystemPrompt, "create, generate, export, or save") {
		t.Fatalf("task system prompt still enumerates intent verbs:\n%s", DefaultTaskSystemPrompt)
	}
}

func TestTaskToolReturnsRuntimeDetectedWorkspaceFileWhenChildOmitsPath(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file") {
		registry.Add(workspaceTool)
	}
	prov := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:        "write",
				Name:      "write_file",
				Arguments: `{"path":"report.html","content":"<!doctype html><title>Report</title>"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{
			// Deliberately omit the path. The runtime, not model prose, owns the
			// file handoff contract.
			{Type: provider.ChunkText, Text: "Done."},
			{Type: provider.ChunkDone},
		},
	}}
	task := newArtifactHandoffTestTask(t, prov, registry, root)

	output, err := task.Execute(testTaskContext(), json.RawMessage(`{
		"prompt":"Generate an HTML file named report.html in the workspace.",
		"tools":["write_file"]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "report.html"))
	if err != nil {
		t.Fatalf("canonical generated file: %v", err)
	}
	encodedWantPath, _ := json.Marshal(wantPath)
	if !strings.Contains(output, "Subagent workspace files (runtime-detected") ||
		!strings.Contains(output, string(encodedWantPath)) {
		t.Fatalf("task output did not hand the generated file to its parent:\n%s", output)
	}
	if strings.Contains(output, `"path":"report.html"`) {
		t.Fatalf("task output returned only a relative model-authored path:\n%s", output)
	}
}

func TestExpertTeamParentReceivesChildGeneratedFile(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file") {
		registry.Add(workspaceTool)
	}
	child := &mockProvider{name: "child", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:        "write",
				Name:      "write_file",
				Arguments: `{"path":"team-report.html","content":"<!doctype html><title>Team report</title>"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{
			// This is the original failure mode: the file exists, but the child
			// does not volunteer its path in its final prose.
			{Type: provider.ChunkText, Text: "Child work completed."},
			{Type: provider.ChunkDone},
		},
	}}
	task := newArtifactHandoffTestTask(t, child, registry, root)
	registry.Add(task)
	parent := &mockProvider{name: "parent", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:   "delegate",
				Name: "task",
				Arguments: `{
					"member_name":"File Expert",
					"prompt":"Generate the requested HTML file as team-report.html in the workspace.",
					"tools":["write_file"]
				}`,
			}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "The team report is ready."},
			{Type: provider.ChunkDone},
		},
	}}
	parentAgent := New(parent, registry, NewSession("parent"), Options{}, event.Discard)
	input := `<system-prompt hidden="true" source="expert" mode="team" member-names='["File Expert"]'>Coordinate the configured experts.</system-prompt>
Generate an HTML file named team-report.html.`
	if err := parentAgent.Run(WithParentSession(context.Background(), "parent-session"), input); err != nil {
		t.Fatalf("parent expert run: %v", err)
	}

	generatedPath, err := filepath.EvalSymlinks(filepath.Join(root, "team-report.html"))
	if err != nil {
		t.Fatalf("canonical generated file: %v", err)
	}
	encodedPath, _ := json.Marshal(generatedPath)
	taskResult := lastToolResult(parentAgent.session, "task")
	if !strings.Contains(taskResult, string(encodedPath)) {
		t.Fatalf("parent task result did not receive child file path:\n%s", taskResult)
	}
	for _, contract := range []string{
		"A2A response:",
		`"jsonrpc":"2.0"`,
		`"state":"TASK_STATE_COMPLETED"`,
		`"mode":"lead-member-only"`,
		`"senderRole":"lead"`,
		`"recipientRole":"member"`,
		`"name":"team-report.html"`,
	} {
		if !strings.Contains(taskResult, contract) {
			t.Fatalf("parent task result missing A2A contract %q:\n%s", contract, taskResult)
		}
	}
}

func TestTaskToolDetectsArtifactCreatedByOpaqueWriter(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactWriter{path: filepath.Join(root, "opaque.pdf")})
	prov := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:        "export",
				Name:      "opaque_export",
				Arguments: `{}`,
			}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "Export finished."},
			{Type: provider.ChunkDone},
		},
	}}
	task := newArtifactHandoffTestTask(t, prov, registry, root)

	output, err := task.Execute(testTaskContext(), json.RawMessage(`{
		"prompt":"Generate a PDF file in the workspace.",
		"tools":["opaque_export"]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "opaque.pdf"))
	if err != nil {
		t.Fatalf("canonical generated file: %v", err)
	}
	encodedWantPath, _ := json.Marshal(wantPath)
	if !strings.Contains(output, string(encodedWantPath)) {
		t.Fatalf("opaque generated file was not recovered by the workspace delta:\n%s", output)
	}
}

func TestBackgroundTaskResultReturnsRuntimeDetectedWorkspaceFile(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file") {
		registry.Add(workspaceTool)
	}
	prov := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:        "write",
				Name:      "write_file",
				Arguments: `{"path":"background.xlsx","content":"workbook"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "Background export finished."},
			{Type: provider.ChunkDone},
		},
	}}
	task := newArtifactHandoffTestTask(t, prov, registry, root)
	wrapped := expertTeamBoundedTaskTool{inner: task, allowedMembers: []string{"File Expert"}}
	manager := jobs.NewManager(event.Discard)
	defer manager.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), manager)

	started, err := wrapped.Execute(ctx, json.RawMessage(`{
		"member_name":"File Expert",
		"prompt":"Generate an XLSX file named background.xlsx in the workspace.",
		"tools":["write_file"],
		"run_in_background":true
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(started, "A2A response:") ||
		!strings.Contains(started, `"state":"TASK_STATE_WORKING"`) ||
		!strings.Contains(started, `"recipientName":"File Expert"`) {
		t.Fatalf("background start did not return an A2A working task:\n%s", started)
	}
	working := expertTeamA2AResponseFromOutput(t, started)
	ref := subagentRefFromOutput(t, started)
	result, ok := manager.WaitControlledForSession(context.Background(), "parent-session", "task", ref, 5)
	if !ok || result.Status != jobs.Done {
		t.Fatalf("background result = %+v, ok=%v, want one completed task", result, ok)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "background.xlsx"))
	if err != nil {
		t.Fatalf("canonical generated file: %v", err)
	}
	encodedWantPath, _ := json.Marshal(wantPath)
	if !strings.Contains(result.Output, string(encodedWantPath)) ||
		!strings.Contains(result.Output, "A2A response:") ||
		!strings.Contains(result.Output, `"recipientName":"File Expert"`) {
		t.Fatalf("background result did not hand the generated file to its parent:\n%s", result.Output)
	}
	completed := expertTeamA2AResponseFromOutput(t, result.Output)
	if working.Result == nil || completed.Result == nil ||
		working.Result.Task.ID != completed.Result.Task.ID ||
		working.Result.Task.ContextID != completed.Result.Task.ContextID {
		t.Fatalf("A2A background lifecycle changed task/context identity: working=%+v completed=%+v", working, completed)
	}
}

func TestWaitTaskReturnsHostObservedArtifactsAndParentEvidence(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file") {
		registry.Add(workspaceTool)
	}
	childProvider := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID: "write", Name: "write_file", Arguments: `{"path":"receipt.pdf","content":"%PDF-1.7"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "Artifact complete."}, {Type: provider.ChunkDone}},
	}}
	task := newArtifactHandoffTestTask(t, childProvider, registry, root)
	manager := jobs.NewManager(event.Discard)
	defer manager.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), manager)

	started, err := task.Execute(ctx, json.RawMessage(`{"prompt":"Generate receipt.pdf in the workspace.","tools":["write_file"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ref := subagentRefFromOutput(t, started)
	waitOutput, err := NewWaitTaskTool(task).Execute(ctx, json.RawMessage(`{"subagent_ref":"`+ref+`","timeout_seconds":5}`))
	if err != nil {
		t.Fatal(err)
	}
	var receipt taskTurnReceipt
	if err := json.Unmarshal([]byte(waitOutput), &receipt); err != nil {
		t.Fatalf("decode wait receipt: %v\n%s", err, waitOutput)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "receipt.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.FinalAnswer != "Artifact complete." || len(receipt.Artifacts) != 1 ||
		receipt.Artifacts[0].Path != wantPath || receipt.Artifacts[0].Source != SubagentArtifactSourceWriter {
		t.Fatalf("wait receipt = %+v", receipt)
	}
	if len(receipt.Validations) != 2 || receipt.Validations[1].Kind != "artifact_observed" || !receipt.Validations[1].Success || receipt.Validations[1].Path != wantPath {
		t.Fatalf("host validations = %+v", receipt.Validations)
	}

	parent := &Agent{evidence: evidence.NewLedger()}
	parent.recordToolReceipt(provider.ToolCall{ID: "wait", Name: "wait_task", Arguments: `{"subagent_ref":"` + ref + `"}`}, NewWaitTaskTool(task), waitOutput, nil, false)
	if !parent.evidence.HasSuccessfulWrite([]string{wantPath}) {
		t.Fatalf("parent ledger did not receive host-observed artifact %q", wantPath)
	}
}

func TestParallelTaskResultReturnsRuntimeDetectedWorkspaceFile(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file") {
		registry.Add(workspaceTool)
	}
	prov := parallelArtifactProvider{}
	task := newArtifactHandoffTestTask(t, prov, registry, root)
	parallel := NewParallelTasksTool(task, registry)
	wrapped := expertTeamBoundedParallelTasksTool{
		inner:          parallel,
		allowedMembers: []string{"File Expert", "Status Expert"},
	}

	ctx := WithCallContext(context.Background(), "parallel-parent", event.Discard, nil, false)
	output, err := wrapped.Execute(ctx, json.RawMessage(`{
		"tasks":[
			{
				"member_name":"File Expert",
				"prompt":"Generate a DOCX file named parallel.docx in the workspace.",
				"tools":["write_file"]
			},
			{
				"member_name":"Status Expert",
				"prompt":"Summarize the status without creating a file.",
				"tools":["write_file"]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "parallel.docx"))
	if err != nil {
		t.Fatalf("canonical generated file: %v", err)
	}
	encodedWantPath, _ := json.Marshal(wantPath)
	if !strings.Contains(output, string(encodedWantPath)) ||
		strings.Count(output, "A2A response:") != 2 ||
		!strings.Contains(output, `"recipientName":"File Expert"`) ||
		!strings.Contains(output, `"recipientName":"Status Expert"`) {
		t.Fatalf("parallel result did not hand the generated file to its parent:\n%s", output)
	}
}

func TestParallelTaskOpaqueWriterWithoutTypedReceiptFailsClosed(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "parallel", "snake.html")
	noise := filepath.Join(root, "reports", "parallel-run.yml")
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactBundleWriter{files: map[string]string{
		target: "<!doctype html><title>Snake</title>",
		noise:  "status: passed",
	}})
	provider := parallelArtifactProvider{}
	task := newArtifactHandoffTestTask(t, provider, registry, root)
	parallel := NewParallelTasksTool(task, registry)

	ctx := WithCallContext(context.Background(), "parallel-parent", event.Discard, nil, false)
	output, err := parallel.Execute(ctx, json.RawMessage(`{
		"tasks":[
			{"prompt":"实现单文件贪吃蛇浏览器游戏","tools":["opaque_bundle_export"]},
			{"prompt":"Summarize status without creating a file.","tools":["opaque_bundle_export"]}
		]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	canonicalNoise, err := filepath.EvalSymlinks(noise)
	if err != nil {
		t.Fatal(err)
	}
	encodedWant, _ := json.Marshal(want)
	encodedNoise, _ := json.Marshal(canonicalNoise)
	if strings.Contains(output, string(encodedWant)) || strings.Contains(output, string(encodedNoise)) ||
		strings.Contains(output, "Subagent workspace files") || strings.Contains(output, `"source":"snapshot"`) {
		t.Fatalf("parallel opaque writer leaked an unattributed workspace scan:\n%s", output)
	}
}

func TestSubagentArtifactHandoffRejectsFileOutsideOwningWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := newSubagentArtifactHandoff(root, tool.NewRegistry(), nil)
	handoff.recordArtifacts([]provider.ToolArtifact{{Path: outside, Source: SubagentArtifactSourceWriter}})
	if got := handoff.files(""); len(got) != 0 {
		t.Fatalf("outside workspace file leaked into parent handoff: %+v", got)
	}
}

func TestSubagentArtifactHandoffCandidateDedupUsesPlatformPathSemantics(t *testing.T) {
	root := t.TempDir()
	upper := filepath.Join(root, "Report.md")
	lower := filepath.Join(root, "report.md")
	if err := os.WriteFile(upper, []byte("upper"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(lower, []byte("lower"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	handoff := newSubagentArtifactHandoff(root, tool.NewRegistry(), nil)
	filesByPath := make(map[string]subagentWorkspaceFile)
	handoff.addArtifactCandidate(filesByPath, subagentArtifactCandidate{
		path: upper, source: SubagentArtifactSourceSnapshot,
	})
	handoff.addArtifactCandidate(filesByPath, subagentArtifactCandidate{
		path: lower, source: SubagentArtifactSourceWriter,
	})

	if runtime.GOOS == "windows" {
		if len(filesByPath) != 1 {
			t.Fatalf("case-equivalent Windows candidates = %+v, want one file", filesByPath)
		}
		for _, file := range filesByPath {
			if file.Source != SubagentArtifactSourceWriter {
				t.Fatalf("case-equivalent Windows candidate source = %q, want writer", file.Source)
			}
		}
		return
	}

	got := make(map[string]string, len(filesByPath))
	for _, file := range filesByPath {
		got[filepath.Base(file.Path)] = file.Source
	}
	want := map[string]string{
		"Report.md": SubagentArtifactSourceSnapshot,
		"report.md": SubagentArtifactSourceWriter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("case-distinct POSIX candidates = %q, want %q", got, want)
	}
}

func TestSubagentArtifactHandoffCandidateKeepsHighestPrioritySource(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "report.md")
	if err := os.WriteFile(target, []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}

	handoff := newSubagentArtifactHandoff(root, tool.NewRegistry(), nil)
	filesByPath := make(map[string]subagentWorkspaceFile)
	for _, source := range []string{
		SubagentArtifactSourceSnapshot,
		SubagentArtifactSourceWriter,
		SubagentArtifactSourceSnapshot,
		SubagentArtifactSourceManifest,
		SubagentArtifactSourceWriter,
	} {
		handoff.addArtifactCandidate(filesByPath, subagentArtifactCandidate{path: target, source: source})
	}

	if len(filesByPath) != 1 {
		t.Fatalf("same-path candidates = %+v, want one file", filesByPath)
	}
	for _, file := range filesByPath {
		if file.Source != SubagentArtifactSourceManifest {
			t.Fatalf("same-path candidate source = %q, want manifest", file.Source)
		}
	}
}

func TestSubagentArtifactHandoffUsesStructuredReceiptsAndIgnoresProseNoise(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactWriter{path: filepath.Join(root, "unused")})
	oldPath := filepath.Join(root, "reports", "old.yml")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old: true"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := newSubagentArtifactHandoff(root, registry, nil)

	for path, content := range map[string]string{
		"artifact.html":                          "<!doctype html>",
		"writer-output.json":                     `{"kind":"writer"}`,
		"writer-output.yaml":                     "kind: writer",
		"writer-output.customer-format":          "writer",
		"manifest-output.json":                   `{"kind":"manifest"}`,
		"manifest-output.yaml":                   "kind: manifest",
		"manifest-output.customer-format":        "manifest",
		"run.log":                                "operational log",
		"src/source-output.json":                 `{"kind":"source"}`,
		"tests/test-output.yaml":                 "kind: test",
		"reports/browser-run.log":                "passed",
		"reports/results.yml":                    "status: passed",
		"absolute-output.json":                   `{"status":"passed"}`,
		".playwright-cli/console-private.log":    "browser internals",
		".playwright-cli/page-private.yml":       "snapshot: private",
		".playwright-cli/private.txt":            "private",
		"build/generated.pdf":                    "build output",
		".customer-delivery/explicit-output.yml": "user requested",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	handoff.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		Name: "write_file", Artifacts: []provider.ToolArtifact{{Path: filepath.Join(root, ".customer-delivery", "explicit-output.yml"), Source: SubagentArtifactSourceWriter}},
	}})
	handoff.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		Name: "write_file", Artifacts: []provider.ToolArtifact{{Path: filepath.Join(root, "artifact.html"), Source: SubagentArtifactSourceWriter}},
	}})
	for _, relative := range []string{
		"writer-output.json", "writer-output.yaml", "writer-output.customer-format",
		"manifest-output.json", "manifest-output.yaml", "manifest-output.customer-format",
		"run.log", "src/source-output.json", "tests/test-output.yaml",
	} {
		handoff.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
			Name: "write_file", Artifacts: []provider.ToolArtifact{{Path: filepath.Join(root, filepath.FromSlash(relative)), Source: SubagentArtifactSourceWriter}},
		}})
	}
	handoff.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		Name: "deliver_files", ReadOnly: true, Artifacts: []provider.ToolArtifact{
			{Path: filepath.Join(root, "manifest-output.json"), Source: SubagentArtifactSourceManifest},
			{Path: filepath.Join(root, "manifest-output.yaml"), Source: SubagentArtifactSourceManifest},
			{Path: filepath.Join(root, "manifest-output.customer-format"), Source: SubagentArtifactSourceManifest},
			{Path: filepath.Join(root, "run.log"), Source: SubagentArtifactSourceManifest},
			{Path: filepath.Join(root, "src", "source-output.json"), Source: SubagentArtifactSourceManifest},
		},
	}})
	handoff.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		Name: "opaque_export", Artifacts: []provider.ToolArtifact{
			{Path: filepath.Join(root, "reports", "results.yml"), Source: SubagentArtifactSourceSnapshot},
			{Path: filepath.Join(root, "absolute-output.json"), Source: SubagentArtifactSourceSnapshot},
			{Path: filepath.Join(root, ".playwright-cli", "private.txt"), Source: SubagentArtifactSourceSnapshot},
			{Path: filepath.Join(root, "build", "generated.pdf"), Source: SubagentArtifactSourceSnapshot},
		},
	}})

	answer := "[results](reports\\results.yml)\n" +
		"[absolute](<" + filepath.ToSlash(filepath.Join(root, "absolute-output.json")) + ">)\n" +
		"[old](reports/old.yml)\n" +
		"[hidden](.playwright-cli/page-private.yml)\n" +
		"`[code-only](reports/browser-run.log)`"
	got := make(map[string]string)
	for _, file := range handoff.files(answer) {
		rel, err := filepath.Rel(root, file.Path)
		if err != nil {
			t.Fatal(err)
		}
		got[filepath.ToSlash(rel)] = file.Source
	}
	want := map[string]string{
		".customer-delivery/explicit-output.yml": SubagentArtifactSourceWriter,
		"artifact.html":                          SubagentArtifactSourceWriter,
		"manifest-output.customer-format":        SubagentArtifactSourceWriter,
		"manifest-output.json":                   SubagentArtifactSourceWriter,
		"manifest-output.yaml":                   SubagentArtifactSourceWriter,
		"writer-output.customer-format":          SubagentArtifactSourceWriter,
		"writer-output.json":                     SubagentArtifactSourceWriter,
		"writer-output.yaml":                     SubagentArtifactSourceWriter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workspace artifacts = %q, want %q", got, want)
	}
}

func TestChildCannotDeclareDeliveryAndObservedWritesRemainParentCandidates(t *testing.T) {
	root := t.TempDir()
	ownedPaths := []string{
		"output.json",
		"output.yaml",
		"output.customer-format",
	}
	manifestPaths := []string{
		ownedPaths[0],
		ownedPaths[1],
		ownedPaths[2],
		"run.log",
		"build/build-output.json",
		"src/source-output.yaml",
	}
	// These files exist, but the child never produced them. Mere existence must
	// not let the child's manifest mint ownership.
	for _, relative := range manifestPaths[len(ownedPaths):] {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(relative), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeChunks := make([]provider.Chunk, 0, len(ownedPaths)+1)
	for i, relative := range ownedPaths {
		args, marshalErr := json.Marshal(map[string]string{"path": relative, "content": relative})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		writeChunks = append(writeChunks, provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: fmt.Sprintf("child-write-%d", i), Name: "write_file", Arguments: string(args),
		}})
	}
	writeChunks = append(writeChunks, provider.Chunk{Type: provider.ChunkDone})
	child := &mockProvider{name: "child-manifest", streams: [][]provider.Chunk{
		writeChunks,
		{{Type: provider.ChunkText, Text: "Child files delivered."}, {Type: provider.ChunkDone}},
	}}
	registry := tool.NewRegistry()
	for _, workspaceTool := range (builtin.Workspace{Dir: root, WriteRoots: []string{root}}).Tools("write_file", "deliver_files") {
		registry.Add(workspaceTool)
	}
	task := newArtifactHandoffTestTask(t, child, registry, root)
	registry.Add(task)
	parent := New(nil, registry, NewSession("parent"), Options{}, event.Discard)
	parent.session.BeginArtifactDeliveryTurn("parent-turn")
	ctx := WithWorkspaceArtifactObservation(testTaskContext(), NewWorkspaceArtifactObservation(root))

	outcome := parent.executeOneWithRegistry(ctx, registry, provider.ToolCall{
		ID: "delegate", Name: "task", Arguments: `{"prompt":"write outputs for the root agent","tools":["write_file"]}`,
	})
	if outcome.errMsg != "" {
		t.Fatalf("task outcome error = %q\n%s", outcome.errMsg, outcome.output)
	}
	wantNames := []string{"output.customer-format", "output.json", "output.yaml"}
	gotNames := make([]string, 0, len(outcome.artifacts))
	for _, artifact := range outcome.artifacts {
		gotNames = append(gotNames, filepath.Base(artifact.Path))
		if artifact.Source != SubagentArtifactSourceWriter {
			t.Fatalf("typed child artifact source = %q, want host-observed writer", artifact.Source)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("typed parent candidates = %v, want %v", gotNames, wantNames)
	}
	if pending, required := parent.evidence.PendingArtifactDelivery(); !required || len(pending) != len(wantNames) {
		t.Fatalf("parent pending delivery = %v, %v; child writes must remain candidates", pending, required)
	}

	parentManifest, err := json.Marshal(map[string]any{"paths": []string{"output.json", "output.yaml", "output.customer-format"}})
	if err != nil {
		t.Fatal(err)
	}
	verified := parent.executeOneWithRegistry(ctx, registry, provider.ToolCall{
		ID: "parent-manifest", Name: "deliver_files", Arguments: string(parentManifest),
	})
	if verified.errMsg != "" {
		t.Fatalf("parent delivery error = %q\n%s", verified.errMsg, verified.output)
	}
	if pending, required := parent.evidence.PendingArtifactDelivery(); required || len(pending) != 0 {
		t.Fatalf("parent pending delivery after own verify = %v, %v", pending, required)
	}
}

func TestIndependentManagedTaskObservationCannotClaimFailedSiblingFile(t *testing.T) {
	root := t.TempDir()
	ownerStartedWriting := make(chan struct{}, 1)
	siblingFailedWritten := make(chan struct{}, 1)
	registry := tool.NewRegistry()
	registry.Add(managedOwnershipWriter{
		root:          root,
		ownerStarted:  ownerStartedWriting,
		failedWritten: siblingFailedWritten,
	})
	task := newArtifactHandoffTestTask(t, managedOwnershipProvider{}, registry, root)
	manager := jobs.NewManager(event.Discard)
	defer manager.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), manager)

	ownerStarted, err := task.Execute(ctx, json.RawMessage(`{
		"prompt":"owner A",
		"tools":["write_file"]
	}`))
	if err != nil {
		t.Fatalf("dispatch owner: %v", err)
	}
	ownerRef := subagentRefFromOutput(t, ownerStarted)
	select {
	case <-ownerStartedWriting:
	case <-time.After(5 * time.Second):
		t.Fatal("owner writer did not start")
	}

	siblingStarted, err := task.Execute(ctx, json.RawMessage(`{
		"prompt":"sibling B",
		"tools":["write_file"],
		"independent":true
	}`))
	if err != nil {
		t.Fatalf("dispatch sibling: %v", err)
	}
	siblingRef := subagentRefFromOutput(t, siblingStarted)
	if result, ok := manager.WaitControlledForSession(context.Background(), "parent-session", "task", ownerRef, 5); !ok || result.Status != jobs.Done {
		t.Fatalf("owner result = %+v, ok=%v", result, ok)
	}
	if result, ok := manager.WaitControlledForSession(context.Background(), "parent-session", "task", siblingRef, 5); !ok || result.Status != jobs.Failed {
		t.Fatalf("sibling result = %+v, ok=%v", result, ok)
	}

	waitOutput, err := NewWaitTaskTool(task).Execute(ctx, json.RawMessage(`{"subagent_ref":"`+ownerRef+`","timeout_seconds":5}`))
	if err != nil {
		t.Fatalf("wait owner: %v", err)
	}
	var receipt taskTurnReceipt
	if err := json.Unmarshal([]byte(waitOutput), &receipt); err != nil {
		t.Fatalf("decode owner receipt: %v\n%s", err, waitOutput)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, "owner.customer-format"))
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Artifacts) != 1 || receipt.Artifacts[0].Path != wantPath || receipt.Artifacts[0].Source != SubagentArtifactSourceWriter {
		t.Fatalf("owner artifacts = %+v, want only its host-observed write", receipt.Artifacts)
	}
	failedPath, err := filepath.EvalSymlinks(filepath.Join(root, "failed.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range receipt.Artifacts {
		if artifactDeliveryPathKey(artifact.Path) == artifactDeliveryPathKey(failedPath) {
			t.Fatalf("owner cross-attributed failed sibling artifact: %+v", receipt.Artifacts)
		}
	}
}

func newArtifactHandoffTestTask(t *testing.T, prov provider.Provider, registry *tool.Registry, root string) *TaskTool {
	t.Helper()
	return NewTaskTool(prov, nil, registry, 20, 0, 0, 0, 0, 0, 0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(NewSubagentStore(filepath.Join(t.TempDir(), "subagents")), root, "base-model", "base-effort")
}

func expertTeamA2AResponseFromOutput(t *testing.T, output string) expertTeamA2AResponse {
	t.Helper()
	const marker = "A2A response: "
	start := strings.LastIndex(output, marker)
	if start < 0 {
		t.Fatalf("output has no A2A response:\n%s", output)
	}
	var response expertTeamA2AResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output[start+len(marker):])), &response); err != nil {
		t.Fatalf("decode A2A response: %v\n%s", err, output)
	}
	return response
}

type opaqueArtifactWriter struct {
	path string
}

type opaqueArtifactBundleWriter struct {
	files map[string]string
}

type managedOwnershipProvider struct{}

type managedOwnershipWriter struct {
	root          string
	ownerStarted  chan<- struct{}
	failedWritten chan struct{}
}

func (managedOwnershipProvider) Name() string { return "managed-artifact-ownership" }

func (managedOwnershipProvider) Stream(_ context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	prompt := lastUser(request)
	toolResults := make([]provider.Message, 0)
	for _, message := range request.Messages {
		if message.Role == provider.RoleTool {
			toolResults = append(toolResults, message)
		}
	}
	chunks := make(chan provider.Chunk, 2)
	switch {
	case strings.Contains(prompt, "owner A") && len(toolResults) == 0:
		chunks <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "owner-write", Name: "write_file", Arguments: `{"path":"owner.customer-format","content":"owner"}`}}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
	case strings.Contains(prompt, "owner A"):
		chunks <- provider.Chunk{Type: provider.ChunkText, Text: "Owner artifact complete."}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
	case strings.Contains(prompt, "sibling B") && len(toolResults) == 0:
		chunks <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "failed-write", Name: "write_file", Arguments: `{"path":"failed.pdf","content":"partial"}`}}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
	default:
		chunks <- provider.Chunk{Type: provider.ChunkError, Err: errors.New("sibling failed after writing")}
	}
	close(chunks)
	return chunks, nil
}

func (managedOwnershipWriter) Name() string { return "write_file" }
func (managedOwnershipWriter) Description() string {
	return "write an owned artifact or a failed sibling artifact"
}
func (managedOwnershipWriter) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (managedOwnershipWriter) ReadOnly() bool { return false }
func (w managedOwnershipWriter) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}
	path := filepath.Join(w.root, filepath.FromSlash(params.Path))
	if params.Path == "owner.customer-format" {
		w.ownerStarted <- struct{}{}
		<-w.failedWritten
		if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
			return "", err
		}
		return "owner written", nil
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
		return "", err
	}
	w.failedWritten <- struct{}{}
	return "partial", errors.New("writer failed after mutation")
}

type parallelArtifactProvider struct{}

func (parallelArtifactProvider) Name() string { return "parallel-artifact" }

func (parallelArtifactProvider) Stream(_ context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	chunks := []provider.Chunk{
		{Type: provider.ChunkText, Text: "No file needed."},
		{Type: provider.ChunkDone},
	}
	prompt := lastUser(request)
	if strings.Contains(prompt, "贪吃蛇") {
		hasWriteResult := false
		for _, message := range request.Messages {
			if message.Role == provider.RoleTool && message.Name == "opaque_bundle_export" {
				hasWriteResult = true
				break
			}
		}
		if hasWriteResult {
			chunks = []provider.Chunk{
				{Type: provider.ChunkText, Text: "游戏文件已完成。"},
				{Type: provider.ChunkDone},
			}
		} else {
			chunks = []provider.Chunk{
				{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "export", Name: "opaque_bundle_export", Arguments: `{}`}},
				{Type: provider.ChunkDone},
			}
		}
	}
	if strings.Contains(prompt, "parallel.docx") {
		hasWriteResult := false
		for _, message := range request.Messages {
			if message.Role == provider.RoleTool && message.Name == "write_file" {
				hasWriteResult = true
				break
			}
		}
		if hasWriteResult {
			chunks = []provider.Chunk{
				{Type: provider.ChunkText, Text: "Parallel export finished."},
				{Type: provider.ChunkDone},
			}
		} else {
			chunks = []provider.Chunk{
				{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
					ID:        "write",
					Name:      "write_file",
					Arguments: `{"path":"parallel.docx","content":"document"}`,
				}},
				{Type: provider.ChunkDone},
			}
		}
	}
	stream := make(chan provider.Chunk, len(chunks))
	for _, chunk := range chunks {
		stream <- chunk
	}
	close(stream)
	return stream, nil
}

func (opaqueArtifactWriter) Name() string { return "opaque_export" }

func (opaqueArtifactWriter) Description() string {
	return "Generate an artifact without exposing its destination in tool arguments."
}

func (opaqueArtifactWriter) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (opaqueArtifactWriter) ReadOnly() bool { return false }
func (opaqueArtifactWriter) ObserveWorkspaceArtifactChanges(json.RawMessage) bool {
	return true
}

func (w opaqueArtifactWriter) Execute(context.Context, json.RawMessage) (string, error) {
	if err := os.WriteFile(w.path, []byte("%PDF-1.7\n"), 0o644); err != nil {
		return "", err
	}
	return "export complete", nil
}

func (opaqueArtifactBundleWriter) Name() string { return "opaque_bundle_export" }

func (opaqueArtifactBundleWriter) Description() string {
	return "Generate files without exposing their destinations in tool arguments."
}

func (opaqueArtifactBundleWriter) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (opaqueArtifactBundleWriter) ReadOnly() bool { return false }
func (opaqueArtifactBundleWriter) ObserveWorkspaceArtifactChanges(json.RawMessage) bool {
	return true
}

func (w opaqueArtifactBundleWriter) Execute(context.Context, json.RawMessage) (string, error) {
	for path, content := range w.files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return "export complete", nil
}
