package evidence

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"kwu/internal/provider"
)

func TestLedgerRecordsSuccessAndFailureReceipts(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "bash",
		Args:     json.RawMessage(`{"command":"go test ./..."}`),
		Success:  true,
		Command:  "go test ./...",
	})
	ledger.Record(Receipt{
		ToolName: "bash",
		Args:     json.RawMessage(`{"command":"go test ./internal/..."}`),
		Success:  false,
		Command:  "go test ./internal/...",
	})

	if !ledger.HasSuccessfulCommand("go test ./...") {
		t.Fatal("successful bash command should verify")
	}
	if ledger.HasSuccessfulCommand("go test ./internal/...") {
		t.Fatal("failed bash command must not verify")
	}
}

func TestLedgerMatchesFileReadAndWriteReceipts(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "read_file", Success: true, Paths: []string{`internal/tool/builtin/completestep.go`}, Read: true})
	ledger.Record(Receipt{ToolName: "write_file", Success: true, Paths: []string{`internal/evidence/evidence.go`}, Write: true})
	ledger.Record(Receipt{ToolName: "edit_file", Success: false, Paths: []string{`failed.go`}, Write: true})

	if !ledger.HasSuccessfulReadOrWrite([]string{`internal\tool\builtin\completestep.go`}) {
		t.Fatal("read receipt should verify the same path across separators")
	}
	if !ledger.HasSuccessfulWrite([]string{`internal/evidence/evidence.go`}) {
		t.Fatal("write receipt should verify written path")
	}
	if ledger.HasSuccessfulWrite([]string{`failed.go`}) {
		t.Fatal("failed write receipt must not verify")
	}
}

func TestWorkspacePathMatchingUsesLexicalRootEquivalence(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "internal", "same.go")
	outside := filepath.Join(filepath.Dir(root), "outside", "same.go")
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "read_file", Success: true, Paths: []string{inside}, Read: true})
	ledger.Record(Receipt{ToolName: "write_file", Success: true, Paths: []string{"internal/reverse.go"}, Write: true})

	if !ledger.HasSuccessfulReadOrWriteInWorkspace([]string{"internal/same.go"}, root) {
		t.Fatal("relative evidence should match an absolute receipt inside the workspace")
	}
	if !ledger.HasSuccessfulWriteInWorkspace([]string{filepath.Join(root, "internal", "reverse.go")}, root) {
		t.Fatal("absolute evidence should match a relative receipt inside the workspace")
	}
	if ledger.HasSuccessfulReadOrWriteInWorkspace([]string{outside}, root) {
		t.Fatal("an outside path with the same basename must not match")
	}
	if ledger.HasSuccessfulReadOrWriteInWorkspace([]string{"internal/same.go"}, "") {
		t.Fatal("empty workdir must preserve the old relative-versus-absolute distinction")
	}
}

func TestSessionPathMatchingUsesWorkspaceAndSkipsFailures(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "internal", "session.go")
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "ok", Name: "write_file", Arguments: `{"path":"internal/session.go"}`}, {ID: "bad", Name: "write_file", Arguments: `{"path":"internal/failed.go"}`}}},
		{Role: provider.RoleTool, ToolCallID: "ok", Name: "write_file", Content: "wrote file"},
		{Role: provider.RoleTool, ToolCallID: "bad", Name: "write_file", Content: "error: denied"},
	}
	if !PathsProvenInSessionWithWorkspace(msgs, []string{abs}, true, root) {
		t.Fatal("session relative writer should prove the workspace-absolute path")
	}
	if PathsProvenInSessionWithWorkspace(msgs, []string{filepath.Join(root, "internal", "failed.go")}, true, root) {
		t.Fatal("failed session writer must not prove a path")
	}
}

func TestWorkspaceBashPathMentionRequiresWholeShellToken(t *testing.T) {
	root := t.TempDir()
	for _, command := range []string{
		"type NOTREADME.md",
		"type README.md.bak",
		"type docs/README.md",
		"type ./docs/README.md",
	} {
		ledger := NewLedger()
		ledger.Record(Receipt{ToolName: "bash", Success: true, Command: command})
		if ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, root) {
			t.Fatalf("shell token collision %q must not prove README.md", command)
		}
	}

	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "bash", Success: false, Command: `type "README.md"`})
	if ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, root) {
		t.Fatal("failed command must not prove a whole-token path mention")
	}
	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: `git diff -- "README.md"`})
	if !ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, root) {
		t.Fatal("quoted whole-token relative path should be accepted")
	}
	for _, command := range []string{`cat ./README.md`, `Get-Content .\README.md`} {
		ledger := NewLedger()
		ledger.Record(Receipt{ToolName: "bash", Success: true, Command: command})
		if !ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, root) {
			t.Fatalf("dot-relative whole-token path %q should be accepted", command)
		}
	}
}

func TestWorkspaceBashPathMentionCaseSemanticsFollowOS(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: "type readme.md"})
	got := ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, t.TempDir())
	if want := runtime.GOOS == "windows"; got != want {
		t.Fatalf("case-only path match = %v, want %v on %s", got, want, runtime.GOOS)
	}
}

func TestWorkspacePathEntrypointsPreserveLegacyBehaviorForEmptyRoot(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: "type NOTREADME.md && type readme.md"})
	ledger.Record(Receipt{ToolName: "read_file", Success: true, Paths: []string{"read.go"}, Read: true})
	ledger.Record(Receipt{ToolName: "write_file", Success: true, Paths: []string{"write.go"}, Write: true})

	if got, want := ledger.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, "  "), ledger.HasSuccessfulBashMentioningPaths([]string{"README.md"}); got != want || !got {
		t.Fatalf("empty-root bash match = %v, legacy = %v; legacy substring/case behavior must be preserved", got, want)
	}
	if got, want := ledger.HasSuccessfulReadOrWriteInWorkspace([]string{"read.go"}, ""), ledger.HasSuccessfulReadOrWrite([]string{"read.go"}); got != want {
		t.Fatalf("empty-root read/write match = %v, legacy = %v", got, want)
	}
	if got, want := ledger.HasSuccessfulWriteInWorkspace([]string{"write.go"}, ""), ledger.HasSuccessfulWrite([]string{"write.go"}); got != want {
		t.Fatalf("empty-root write match = %v, legacy = %v", got, want)
	}
	gotIndex, gotOK := ledger.LatestSuccessfulWriteIndexInWorkspace([]string{"write.go"}, "")
	wantIndex, wantOK := ledger.LatestSuccessfulWriteIndex([]string{"write.go"})
	if gotIndex != wantIndex || gotOK != wantOK {
		t.Fatalf("empty-root latest writer = (%d,%v), legacy = (%d,%v)", gotIndex, gotOK, wantIndex, wantOK)
	}

	msgs := []provider.Message{{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "w", Name: "write_file", Arguments: `{"path":"write.go"}`}}}, {Role: provider.RoleTool, ToolCallID: "w", Name: "write_file", Content: "ok"}}
	if got, want := PathsProvenInSessionWithWorkspace(msgs, []string{"write.go"}, true, ""), PathsProvenInSession(msgs, []string{"write.go"}, true); got != want {
		t.Fatalf("empty-root session path match = %v, legacy = %v", got, want)
	}
	collision := NewLedger()
	collision.Record(Receipt{ToolName: "bash", Success: true, Command: "type NOTREADME.md"})
	if collision.HasSuccessfulBashMentioningPathsInWorkspace([]string{"README.md"}, t.TempDir()) {
		t.Fatal("non-empty workspace root must still reject prefix/suffix collisions")
	}
}

func TestLedgerReportsFinalReadinessReceiptsAfterWriter(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: "go test ./..."})
	ledger.Record(Receipt{ToolName: "write_file", Success: true, Paths: []string{"changed.go"}, Write: true})
	ledger.Record(Receipt{ToolName: "bash", Success: false, Command: "git diff --check"})
	ledger.Record(Receipt{ToolName: "todo_write", Success: true, Todos: []TodoItem{{Content: "Edit code", Status: "in_progress"}}})

	writer, ok := ledger.LatestSuccessfulWriterIndex()
	if !ok {
		t.Fatal("expected latest successful writer")
	}
	if ledger.HasSuccessfulCommandAfter("go test ./...", writer) {
		t.Fatal("command before latest writer must not satisfy final readiness")
	}
	if ledger.HasSuccessfulCommandAfter("git diff --check", writer) {
		t.Fatal("failed command must not satisfy final readiness")
	}
	if ledger.HasSuccessfulCompleteStepAfter(writer) {
		t.Fatal("missing complete_step must not satisfy final readiness")
	}
	if !ledger.HasSuccessfulTodoWrite() {
		t.Fatal("successful todo_write receipt should be reported")
	}

	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: "git diff --check"})
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "Edit code"})
	if !ledger.HasSuccessfulCommandAfter("git diff --check", writer) {
		t.Fatal("command after latest writer should satisfy final readiness")
	}
	if !ledger.HasSuccessfulCompleteStepAfter(writer) {
		t.Fatal("complete_step after latest writer should satisfy final readiness")
	}
}

func TestLedgerResetClearsTurnReceipts(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "bash", Success: true, Command: "go test ./..."})

	ledger.Reset()

	if ledger.HasSuccessfulCommand("go test ./...") {
		t.Fatal("reset should clear prior-turn evidence")
	}
}

func TestContextCarriesLedger(t *testing.T) {
	ledger := NewLedger()
	ctx := WithLedger(context.Background(), ledger)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("ledger missing from context")
	}
	if got != ledger {
		t.Fatal("context returned a different ledger")
	}
}

func TestReceiptFromToolCallExtractsEvidenceFields(t *testing.T) {
	bash := ReceiptFromToolCall("bash", json.RawMessage(`{"command":"git diff --check"}`), true, false)
	if bash.Command != "git diff --check" {
		t.Fatalf("bash command = %q", bash.Command)
	}
	if bash.Write {
		t.Fatal("bash should not be treated as a verified file writer")
	}

	write := ReceiptFromToolCall("write_file", json.RawMessage(`{"path":"internal/evidence/evidence.go","content":"x"}`), true, false)
	if !write.Write || len(write.Paths) != 1 || write.Paths[0] != `internal/evidence/evidence.go` {
		t.Fatalf("write receipt not extracted: %+v", write)
	}

	read := ReceiptFromToolCall("read_file", json.RawMessage(`{"path":"internal/tool/builtin/completestep.go"}`), true, true)
	if !read.Read || len(read.Paths) != 1 {
		t.Fatalf("read receipt not extracted: %+v", read)
	}

	glob := ReceiptFromToolCall("glob", json.RawMessage(`{"pattern":"**/*.go"}`), true, true)
	if !glob.Read {
		t.Fatalf("generic read-only tool should be treated as read context: %+v", glob)
	}
}

func TestReceiptFromToolCallExtractsTodoWriteItems(t *testing.T) {
	receipt := ReceiptFromToolCall("todo_write", json.RawMessage(`{"todos":[
		{"content":"Add parser","status":"in_progress","activeForm":"Adding parser"},
		{"content":"Wire parser","status":"pending","level":1}
	]}`), true, true)

	if len(receipt.Todos) != 2 {
		t.Fatalf("todos not extracted: %+v", receipt)
	}
	if receipt.Todos[0].Content != "Add parser" || receipt.Todos[0].Status != "in_progress" || receipt.Todos[0].ActiveForm != "Adding parser" {
		t.Fatalf("first todo not extracted: %+v", receipt.Todos[0])
	}
	if receipt.Todos[1].Level != 1 {
		t.Fatalf("todo level not extracted: %+v", receipt.Todos[1])
	}
}

func TestReceiptFromToolCallExtractsCompleteStep(t *testing.T) {
	receipt := ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Add parser",
		"result":"parser added",
		"evidence":[{"kind":"manual","summary":"checked manually"}]
	}`), true, true)

	if receipt.Step != "Add parser" {
		t.Fatalf("complete_step step = %q", receipt.Step)
	}
	if !receipt.StepProof {
		t.Fatalf("complete_step evidence proof not extracted: %+v", receipt)
	}
	if receipt.Read {
		t.Fatalf("complete_step should not be treated as read-only context: %+v", receipt)
	}

	missingResult := ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Add parser",
		"evidence":[{"kind":"manual","summary":"checked manually"}]
	}`), false, true)
	if missingResult.StepProof {
		t.Fatalf("complete_step without result should not count as proof: %+v", missingResult)
	}

	missingCommand := ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Add parser",
		"result":"parser added",
		"evidence":[{"kind":"verification","summary":"checked manually"}]
	}`), false, true)
	if missingCommand.StepProof {
		t.Fatalf("verification evidence without command should not count as proof: %+v", missingCommand)
	}

	emptyProof := ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Add parser",
		"result":"parser added",
		"evidence":[]
	}`), false, true)
	if emptyProof.StepProof {
		t.Fatalf("empty complete_step evidence should not count as proof: %+v", emptyProof)
	}
}

func TestLedgerMatchesLatestSuccessfulTodoStep(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  false,
		Todos:    []TodoItem{{Content: "Failed only", Status: "in_progress"}},
	})
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Add parser", Status: "in_progress", ActiveForm: "Adding parser"},
			{Content: "Wire parser", Status: "completed"},
			{Content: "Document parser", Status: "pending"},
		},
	})

	for _, step := range []string{"Add parser", "Adding parser", "2"} {
		match, ok := ledger.MatchLatestTodoStep(step)
		if !ok {
			t.Fatalf("latest todo receipt missing for %q", step)
		}
		if !match.Found {
			t.Fatalf("step %q did not match latest todo list", step)
		}
		if step == "2" && match.Content != "Wire parser" {
			t.Fatalf("numeric step matched %q, want Wire parser", match.Content)
		}
	}

	match, ok := ledger.MatchLatestTodoStep("Failed only")
	if !ok {
		t.Fatal("successful todo receipt should exist")
	}
	if match.Found {
		t.Fatal("failed todo_write receipt must not match")
	}
}

func TestMatchTodoStepToleratesCitationDrift(t *testing.T) {
	// Verbatim shape from discussion #3970: todo authored with a fullwidth
	// colon, cited back with a halfwidth one — and stuck forever.
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Phase 4：环境准备", Status: "completed"},
			{Content: "Phase 5：脚本编辑与执行代码", Status: "in_progress"},
			{Content: "Review notes", Status: "pending"},
		},
	})

	matches := map[string]int{
		"Phase 5: 脚本编辑与执行代码":  2,
		"phase 5：脚本编辑与执行代码":   2,
		"  Phase　5：脚本编辑与执行代码": 2,
		"脚本编辑与执行代码":           2,
		"Phase 4：环境":          1,
		"REVIEW NOTES":        3,
		"２":                   2,
	}
	for step, want := range matches {
		match, ok := ledger.MatchLatestTodoStep(step)
		if !ok || !match.Found {
			t.Fatalf("step %q should match todo %d, got found=%v", step, want, match.Found)
		}
		if match.Index != want {
			t.Errorf("step %q matched todo %d, want %d", step, match.Index, want)
		}
	}

	for _, step := range []string{"deploy backend", "代码", "Phase 9：不存在的阶段"} {
		if match, _ := ledger.MatchLatestTodoStep(step); match.Found {
			t.Errorf("step %q should not match, got todo %d (%q)", step, match.Index, match.Content)
		}
	}
}

func TestMatchTodoStepAmbiguousContainmentStaysUnmatched(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Deploy backend service", Status: "in_progress"},
			{Content: "Deploy backend worker", Status: "pending"},
		},
	})
	if match, _ := ledger.MatchLatestTodoStep("Deploy backend"); match.Found {
		t.Fatalf("ambiguous citation should stay unmatched, got todo %d (%q)", match.Index, match.Content)
	}
	if match, _ := ledger.MatchLatestTodoStep("Deploy backend worker"); !match.Found || match.Index != 2 {
		t.Fatal("exact citation must still resolve despite shared prefix")
	}
}

func TestLedgerRequiresCompleteStepForNewCompletedTodos(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Add parser", Status: "in_progress"},
			{Content: "Already done", Status: "completed"},
		},
	})

	current := []TodoItem{
		{Content: "Add parser", Status: "completed"},
		{Content: "Already done", Status: "completed"},
	}
	missing, hasBaseline := ledger.UnverifiedCompletedTodos(current)
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 1 || missing[0].Content != "Add parser" {
		t.Fatalf("missing = %+v, want only Add parser", missing)
	}

	ledger.Record(Receipt{ToolName: "complete_step", Success: false, Step: "Add parser"})
	missing, hasBaseline = ledger.UnverifiedCompletedTodos(current)
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline after failed complete_step")
	}
	if len(missing) != 1 || missing[0].Content != "Add parser" {
		t.Fatalf("failed complete_step without proof-bearing recovery should not authorize completion, missing = %+v", missing)
	}

	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "Add parser"})
	missing, hasBaseline = ledger.UnverifiedCompletedTodos(current)
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline after successful complete_step")
	}
	if len(missing) != 0 {
		t.Fatalf("successful complete_step should authorize completion, missing = %+v", missing)
	}
}

func TestLedgerMatchesCompletionByActiveFormAndNumber(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Add parser", Status: "in_progress", ActiveForm: "Adding parser"},
			{Content: "Wire parser", Status: "in_progress"},
		},
	})

	current := []TodoItem{
		{Content: "Add parser", Status: "completed", ActiveForm: "Adding parser"},
		{Content: "Wire parser", Status: "in_progress"},
	}
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "Adding parser"})
	missing, hasBaseline := ledger.UnverifiedCompletedTodos(current)
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 0 {
		t.Fatalf("activeForm complete_step should authorize completion, missing = %+v", missing)
	}

	current = []TodoItem{
		{Content: "Add parser", Status: "completed", ActiveForm: "Adding parser"},
		{Content: "Wire parser", Status: "completed"},
	}
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "2"})
	missing, hasBaseline = ledger.UnverifiedCompletedTodos(current)
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 0 {
		t.Fatalf("numeric complete_step should authorize completion, missing = %+v", missing)
	}
}

func TestLedgerNumericCompleteStepDoesNotAuthorizeReplacedTodo(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []TodoItem{{Content: "Add parser", Status: "in_progress"}},
	})
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "1"})

	missing, hasBaseline := ledger.UnverifiedCompletedTodos([]TodoItem{
		{Content: "Ship parser", Status: "completed"},
	})
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 1 || missing[0].Content != "Ship parser" {
		t.Fatalf("numeric complete_step should not authorize a replaced todo, missing = %+v", missing)
	}
}

func TestLedgerNumericCompleteStepFollowsReorderedSignedTodo(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Add parser", Status: "in_progress"},
			{Content: "Write tests", Status: "pending"},
		},
	})
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "1"})

	missing, hasBaseline := ledger.UnverifiedCompletedTodos([]TodoItem{
		{Content: "Write tests", Status: "pending"},
		{Content: "Add parser", Status: "completed"},
	})
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 0 {
		t.Fatalf("numeric complete_step should follow the signed todo identity after reorder, missing = %+v", missing)
	}
}

func TestLedgerNoBaselineDoesNotConstrainCompletedTodos(t *testing.T) {
	ledger := NewLedger()
	missing, hasBaseline := ledger.UnverifiedCompletedTodos([]TodoItem{
		{Content: "Add parser", Status: "completed"},
	})

	if hasBaseline {
		t.Fatal("empty ledger should not report a prior todo_write baseline")
	}
	if len(missing) != 0 {
		t.Fatalf("no baseline should not report missing completions, got %+v", missing)
	}
}

func TestLedgerNumericCompleteStepAuthorizesRephrasedTodo(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos: []TodoItem{
			{Content: "Add parser", Status: "in_progress"},
			{Content: "Write tests", Status: "pending"},
		},
	})
	ledger.Record(Receipt{ToolName: "complete_step", Success: true, Step: "1"})

	// The model rephrased item 1 (added detail) but it's the same task.
	missing, hasBaseline := ledger.UnverifiedCompletedTodos([]TodoItem{
		{Content: "Add parser with streaming support", Status: "completed"},
		{Content: "Write tests", Status: "pending"},
	})
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline")
	}
	if len(missing) != 0 {
		t.Fatalf("rephrased todo at same index should be authorized by content overlap, missing = %+v", missing)
	}

	// The model also rephrased item 2; still ok because the new text contains the old.
	missing, hasBaseline = ledger.UnverifiedCompletedTodos([]TodoItem{
		{Content: "Add parser with streaming support", Status: "completed"},
		{Content: "Write tests and benchmarks", Status: "completed"},
	})
	if !hasBaseline {
		t.Fatal("expected prior todo_write baseline for second rephrase")
	}
	// Item 1 is already authorized; item 2 is also rephrased but lacks a
	// complete_step — so it should still be flagged.
	if len(missing) != 1 || missing[0].Content != "Write tests and benchmarks" {
		t.Fatalf("rephrased todo without complete_step should still be missing, got %+v", missing)
	}
}

func TestPendingArtifactDeliveryUsesLatestDecisionBoundary(t *testing.T) {
	beforeDecision := NewLedger()
	beforeDecision.Record(Receipt{ToolName: "task", Success: true, ArtifactMutation: true})
	if got, pending := beforeDecision.PendingArtifactDelivery(); !pending || len(got) != 0 {
		t.Fatalf("candidate-free mutation did not require a manifest: %v, %v", got, pending)
	}
	beforeDecision.Record(Receipt{ToolName: "deliver_files", Success: true, ArtifactDecision: true})
	if got, pending := beforeDecision.PendingArtifactDelivery(); pending || len(got) != 0 {
		t.Fatalf("explicit empty manifest did not clear candidate-free mutation: %v, %v", got, pending)
	}

	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "write_file", Success: true, ArtifactPaths: []string{"first.md", "first.md"}})
	ledger.Record(Receipt{ToolName: "deliver_files", Success: false, ArtifactDecision: true})
	if got, pending := ledger.PendingArtifactDelivery(); !pending || len(got) != 1 || got[0] != "first.md" {
		t.Fatalf("failed decision cleared pending paths: %v, %v", got, pending)
	}

	ledger.Record(Receipt{ToolName: "deliver_files", Success: true, ArtifactDecision: true})
	if got, pending := ledger.PendingArtifactDelivery(); pending || len(got) != 0 {
		t.Fatalf("successful empty decision left pending paths: %v, %v", got, pending)
	}

	ledger.Record(Receipt{ToolName: "bash", Success: true, ArtifactPaths: []string{"second.pdf", "third.docx"}})
	if got, pending := ledger.PendingArtifactDelivery(); !pending || len(got) != 2 || got[0] != "second.pdf" || got[1] != "third.docx" {
		t.Fatalf("paths after latest decision = %v, %v", got, pending)
	}

	ledger.Record(Receipt{ToolName: "deliver_files", Success: true, ArtifactDecision: true})
	if got, pending := ledger.PendingArtifactDelivery(); pending || len(got) != 0 {
		t.Fatalf("latest decision did not clear later observations: %v, %v", got, pending)
	}

	ledger.Record(Receipt{ToolName: "delete_file", Success: true, ArtifactMutation: true, Paths: []string{"second.pdf"}})
	if got, pending := ledger.PendingArtifactDelivery(); !pending || len(got) != 1 || got[0] != "second.pdf" {
		t.Fatalf("mutation after decision did not invalidate it: %v, %v", got, pending)
	}

	ledger.Record(Receipt{ToolName: "deliver_files", Success: true, ArtifactDecision: true})
	ledger.Record(Receipt{ToolName: "bash", Success: false, ArtifactMutation: true})
	if got, pending := ledger.PendingArtifactDelivery(); !pending || len(got) != 0 {
		t.Fatalf("failed workspace mutation did not invalidate decision: %v, %v", got, pending)
	}
}

func TestLatestArtifactDeliveryDecisionDistinguishesEmptyAndInvalidated(t *testing.T) {
	ledger := NewLedger()
	if paths, hasDecision, valid := ledger.LatestArtifactDeliveryDecision(); paths != nil || hasDecision || valid {
		t.Fatalf("initial decision = %v, %v, %v", paths, hasDecision, valid)
	}

	ledger.Record(Receipt{
		ToolName: "deliver_files", Success: true, ArtifactDecision: true,
		Paths: []string{"reports/final.md"},
	})
	paths, hasDecision, valid := ledger.LatestArtifactDeliveryDecision()
	if !hasDecision || !valid || len(paths) != 1 || paths[0] != filepath.Clean("reports/final.md") {
		t.Fatalf("verified decision = %v, %v, %v", paths, hasDecision, valid)
	}
	ledger.Record(Receipt{ToolName: "read_file", Success: true, Read: true})
	paths, hasDecision, valid = ledger.LatestArtifactDeliveryDecision()
	if !hasDecision || !valid || len(paths) != 1 {
		t.Fatalf("read-only continuation changed decision = %v, %v, %v", paths, hasDecision, valid)
	}

	ledger.Record(Receipt{ToolName: "bash", Success: false, ArtifactMutation: true})
	paths, hasDecision, valid = ledger.LatestArtifactDeliveryDecision()
	if !hasDecision || valid || len(paths) != 1 {
		t.Fatalf("invalidated decision = %v, %v, %v", paths, hasDecision, valid)
	}

	ledger.Record(Receipt{ToolName: "deliver_files", Success: true, ArtifactDecision: true})
	paths, hasDecision, valid = ledger.LatestArtifactDeliveryDecision()
	if !hasDecision || !valid || len(paths) != 0 {
		t.Fatalf("explicit empty decision = %v, %v, %v", paths, hasDecision, valid)
	}
}
