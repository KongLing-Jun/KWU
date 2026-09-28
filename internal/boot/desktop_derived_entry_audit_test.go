package boot

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDesktopStrictDerivedModelEntryAuditHasNoUnknownStreamBypass(t *testing.T) {
	root := auditRepositoryRoot(t)
	// Every production Stream selector under desktop/internal is classified.
	// The first group is Desktop-reachable only through an injected seam; the
	// second is the model-billing implementation behind that seam; serve remains
	// an explicit non-Desktop B2-TMP-012 exclusion.
	want := mergeAuditCounts(
		map[string]int{
			"internal/agent/stream_dispatcher.go|streamer":            1,
			"internal/agent/stream_dispatcher.go|a.prov":              1,
			"internal/agent/stream_dispatcher.go|a.derivedStreamer":   1,
			"internal/agent/stream_dispatcher.go|a.derivedProvider()": 1,
			"internal/agent/coordinator.go|c.plannerStreamer":         1,
			"internal/agent/coordinator.go|c.planner":                 1,
			"internal/control/auto_plan_classifier.go|c.prov":         1,
			"internal/tasktree/planner.go|p.model":                    1,
		},
		map[string]int{
			"desktop/model_billing.go|gateway":                                1,
			"internal/airclaude/gateway.go|model":                             1,
			"internal/modelbilling/fixture_gateway_runtime.go|fixtureGateway": 1,
			"internal/modelbilling/main_executor_dispatcher.go|d.Gateway":     1,
			"internal/modelbilling/main_executor_dispatcher.go|byok":          1,
		},
		map[string]int{
			"internal/serve/serve.go|s.titleProv": 1,
		},
	)
	found := map[string]int{}
	for _, dir := range []string{"desktop", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "Stream" {
					found[rel+"|"+renderAuditNode(t, fset, selector.X)]++
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	assertAuditCounts(t, "production Stream selectors", found, want)
}

func TestDesktopStrictDerivedEntryCompositionKeepsEveryRoleOnBillingSeams(t *testing.T) {
	root := auditRepositoryRoot(t)
	bootFiles := parseAuditFiles(t, root, "internal/boot/boot.go", "internal/boot/model_billing_derived.go")

	wantRoleCalls := map[string]int{
		"roleBoundProvider|modelbilling.CallRoleClassifier":         1,
		"roleBoundProvider|modelbilling.CallRoleTasktreePlanner":    2,
		"roleBoundStreamDispatcher|modelbilling.CallRoleExecutor":   1,
		"roleBoundStreamDispatcher|modelbilling.CallRolePlanner":    1,
		"roleBoundStreamDispatcher|modelbilling.CallRoleSubagent":   1,
		"roleBoundStreamDispatcher|modelbilling.CallRoleCompaction": 5,
		"roleBoundStreamDispatcher|modelbilling.CallRoleGuardian":   2,
	}
	roleCalls := map[string]int{}
	guardianConstructors := 0
	resolverBindings := 0
	classifierConstructors := 0
	taskTreePlannerBindings := 0
	for _, parsed := range bootFiles {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := renderAuditNode(t, parsed.fset, call.Fun)
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && (selector.Sel.Name == "roleBoundProvider" || selector.Sel.Name == "roleBoundStreamDispatcher") {
				roleArg := 0
				if selector.Sel.Name == "roleBoundProvider" {
					roleArg = 1
				}
				if len(call.Args) <= roleArg {
					t.Fatalf("%s call has no role argument", selector.Sel.Name)
				}
				key := selector.Sel.Name + "|" + renderAuditNode(t, parsed.fset, call.Args[roleArg])
				roleCalls[key]++
			}
			switch fun {
			case "guardian.NewSessionWithDispatchers":
				guardianConstructors++
				if len(call.Args) < 2 || renderAuditNode(t, parsed.fset, call.Args[len(call.Args)-2]) != "guardianDispatcher" || renderAuditNode(t, parsed.fset, call.Args[len(call.Args)-1]) != "guardianCompactionDispatcher" {
					t.Fatal("guardian constructor no longer binds guardian and compaction dispatchers")
				}
			case "control.NewBillableProviderAutoPlanClassifier":
				classifierConstructors++
				if len(call.Args) == 0 || renderAuditNode(t, parsed.fset, call.Args[0]) != "classifierProv" {
					t.Fatal("classifier no longer consumes the role-bound provider")
				}
			case "tasktree.NewLLMPlanner":
				taskTreePlannerBindings += compositeFieldValueCount(t, parsed.fset, call.Args, "Provider", "taskTreePlannerProvider")
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "WithStreamDispatcherResolver" {
				if len(call.Args) == 1 && renderAuditNode(t, parsed.fset, call.Args[0]) == "resolveSubagentDispatchers" {
					resolverBindings++
				}
			}
			return true
		})
	}
	assertAuditCounts(t, "role-bound boot calls", roleCalls, wantRoleCalls)
	if guardianConstructors != 2 || resolverBindings != 1 || classifierConstructors != 1 || taskTreePlannerBindings != 1 {
		t.Fatalf("guardian/resolver/classifier/tasktree bindings = %d/%d/%d/%d, want 2/1/1/1", guardianConstructors, resolverBindings, classifierConstructors, taskTreePlannerBindings)
	}

	wantFields := map[string]int{
		"StreamDispatcher|billing.dispatcher":                  1,
		"StreamDispatcher|plannerDispatcher":                   1,
		"StreamDispatcher|dispatchers.Main":                    2,
		"DerivedStreamDispatcher|executorCompactionDispatcher": 1,
		"DerivedStreamDispatcher|plannerCompactionDispatcher":  1,
		"DerivedStreamDispatcher|dispatchers.Compaction":       2,
		"ExecutorStreamDispatcher|executorSyntheticDispatcher": 1,
	}
	foundFields := map[string]int{}
	for _, parsed := range bootFiles {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			field, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				return true
			}
			switch key.Name {
			case "StreamDispatcher", "DerivedStreamDispatcher", "ExecutorStreamDispatcher":
				foundFields[key.Name+"|"+renderAuditNode(t, parsed.fset, field.Value)]++
			}
			return true
		})
	}
	assertAuditCounts(t, "dispatcher field bindings", foundFields, wantFields)

	turnSet, turnFile := parseAuditFile(t, root, "internal/control/turn_orchestrator.go")
	syntheticBindings := 0
	ast.Inspect(turnFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || renderAuditNode(t, turnSet, call.Fun) != "agent.WithStreamDispatcher" || len(call.Args) != 3 {
			return true
		}
		if renderAuditNode(t, turnSet, call.Args[0]) == "ctx" && renderAuditNode(t, turnSet, call.Args[1]) == "c.executor" && renderAuditNode(t, turnSet, call.Args[2]) == "c.executorStreamDispatcher" {
			syntheticBindings++
		}
		return true
	})
	if syntheticBindings != 1 {
		t.Fatalf("plan synthetic dispatcher bindings = %d, want 1", syntheticBindings)
	}

	taskSource, err := os.ReadFile(filepath.Join(root, "internal", "agent", "task.go"))
	if err != nil {
		t.Fatal(err)
	}
	policy := bytes.Index(taskSource, []byte("t.validateExecution(ctx, p.Prompt, legacyBackground)"))
	transcript := bytes.Index(taskSource, []byte("t.prepareTranscriptRun("))
	providerResolution := bytes.Index(taskSource, []byte("t.resolveSubSessionRuntime("))
	if policy < 0 || transcript < 0 || providerResolution < 0 || policy > transcript || policy > providerResolution {
		t.Fatalf("background policy order = policy:%d transcript:%d provider:%d", policy, transcript, providerResolution)
	}
}

func TestDesktopStrictDerivedConsumerAuditSensitivity(t *testing.T) {
	const valid = `package boot
func buildBoundConsumer() {
	_ = agent.New(execProv, reg, sess, agent.Options{
		StreamDispatcher: billing.dispatcher,
		DerivedStreamDispatcher: executorCompactionDispatcher,
	}, sink)
}`
	const withUnboundConsumer = `package boot
func buildBoundConsumer() {
	_ = agent.New(execProv, reg, sess, agent.Options{
		StreamDispatcher: billing.dispatcher,
		DerivedStreamDispatcher: executorCompactionDispatcher,
	}, sink)
	_ = agent.New(rawProv, reg, sess, agent.Options{}, sink)
}`

	validConsumers, validViolations := auditDesktopConsumerSource(t, "valid.go", valid)
	unboundConsumers, unboundViolations := auditDesktopConsumerSource(t, "unbound.go", withUnboundConsumer)
	if len(validConsumers) != 1 || len(validViolations) != 0 {
		t.Fatalf("valid consumer audit = consumers:%#v violations:%#v", validConsumers, validViolations)
	}
	if len(unboundConsumers) != 2 || len(unboundViolations) != 1 {
		t.Fatalf("unbound consumer audit = consumers:%#v violations:%#v", unboundConsumers, unboundViolations)
	}
	if auditStreamSelectorCount(t, "valid.go", valid) != auditStreamSelectorCount(t, "unbound.go", withUnboundConsumer) {
		t.Fatal("sensitivity fixture unexpectedly changed the Stream selector count")
	}
}

func TestDesktopStrictDerivedDormantBuildAssemblyBoundarySensitivity(t *testing.T) {
	const dormantMethod = `package boot
func (b *buildAssembly) initializeExecutor() {
	_ = agent.New(rawProv, reg, sess, agent.Options{}, sink)
}`
	const enabledFactory = `package boot
func Build(ctx context.Context, opts Options) {
	_ = newBuildAssembly(ctx, opts)
}`
	const enabledFactoryAlias = `package boot
func Build(ctx context.Context, opts Options) {
	makeAssembly := newBuildAssembly
	_ = makeAssembly(ctx, opts)
}`
	const directAssembly = `package boot
func Build(ctx context.Context, opts Options) {
	_ = &buildAssembly{ctx: ctx, opts: opts}
}`
	const ordinaryFunction = `package boot
func initializeExecutor() {
	_ = agent.New(rawProv, reg, sess, agent.Options{}, sink)
}`

	dormant := map[string]int{"fixture.go|initializeExecutor|agent.New": 1}
	if violations := auditDormantBuildAssemblySource(t, "fixture.go", dormantMethod, dormant); len(violations) != 0 {
		t.Fatalf("registered dormant method violations = %#v", violations)
	}
	if violations := auditDormantBuildAssemblySource(t, "fixture.go", enabledFactory, nil); len(violations) != 1 {
		t.Fatalf("enabled buildAssembly factory violations = %#v, want one", violations)
	}
	if violations := auditDormantBuildAssemblySource(t, "fixture.go", enabledFactoryAlias, nil); len(violations) != 1 {
		t.Fatalf("aliased buildAssembly factory violations = %#v, want one", violations)
	}
	if violations := auditDormantBuildAssemblySource(t, "fixture.go", directAssembly, nil); len(violations) != 1 {
		t.Fatalf("direct buildAssembly construction violations = %#v, want one", violations)
	}
	if violations := auditDormantBuildAssemblySource(t, "fixture.go", ordinaryFunction, dormant); len(violations) != 1 {
		t.Fatalf("ordinary dormant impostor violations = %#v, want one", violations)
	}
}

func TestDesktopStrictDerivedConsumerInventoryAndBindings(t *testing.T) {
	root := auditRepositoryRoot(t)
	bound := map[string]int{
		"internal/boot/boot.go|Build|agent.New":                                                                            1,
		"internal/boot/boot.go|Build|agent.NewTaskTool":                                                                    1,
		"internal/boot/boot.go|Build|agent.RunSubAgentWithSession":                                                         2,
		"internal/boot/boot.go|Build|guardian.NewSessionWithDispatchers":                                                   2,
		"internal/boot/boot.go|Build|tasktree.NewLLMPlanner":                                                               1,
		"internal/boot/model_billing_derived.go|buildBootAutoPlanClassifier|control.NewBillableProviderAutoPlanClassifier": 1,
		"internal/boot/model_billing_derived.go|buildBootPlanner|agent.NewCoordinator":                                     1,
	}
	// These consumers belong to a dormant legacy builder. They may remain
	// unbound only while every entry stays on a *buildAssembly method and no
	// production entry constructs that builder. This is not B2-TMP-012.
	dormantBuildAssembly := map[string]int{
		"internal/boot/build_agent_tools.go|newTaskTool|agent.NewTaskTool":                                        1,
		"internal/boot/build_agent_tools.go|readOnlySkillRunner|agent.RunSubAgentWithSession":                     1,
		"internal/boot/build_agent_tools.go|skillRunner|agent.RunSubAgentWithSession":                             1,
		"internal/boot/build_runner.go|configureAutoPlanClassifier|control.NewBillableProviderAutoPlanClassifier": 1,
		"internal/boot/build_runner.go|configureGuardian|guardian.NewSession":                                     1,
		"internal/boot/build_runner.go|configurePlannerRunner|agent.NewCoordinator":                               1,
		"internal/boot/build_runner.go|initializeExecutor|agent.New":                                              1,
		"internal/boot/build_runner.go|wrapAgentModeRunner|tasktree.NewLLMPlanner":                                1,
	}
	found := map[string]int{}
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, "internal", "boot"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fset, file := parseAuditFile(t, root, filepath.ToSlash(rel))
		consumers, fileViolations := auditDesktopConsumersInFile(t, fset, file, filepath.ToSlash(rel), func(key string) bool {
			return bound[key] > 0
		})
		for _, consumer := range consumers {
			found[consumer]++
		}
		violations = append(violations, fileViolations...)
		violations = append(violations, auditDormantBuildAssemblyFile(t, fset, file, filepath.ToSlash(rel), dormantBuildAssembly)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertAuditCounts(t, "model consumer inventory", found, mergeAuditCounts(bound, dormantBuildAssembly))
	if len(violations) != 0 {
		t.Fatalf("Desktop model consumer binding violations = %#v", violations)
	}
}

func auditDesktopConsumerSource(t *testing.T, name, source string) ([]string, []string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, 0)
	if err != nil {
		t.Fatal(err)
	}
	return auditDesktopConsumersInFile(t, fset, file, name, func(string) bool { return true })
}

func auditDormantBuildAssemblySource(t *testing.T, name, source string, dormant map[string]int) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, 0)
	if err != nil {
		t.Fatal(err)
	}
	return auditDormantBuildAssemblyFile(t, fset, file, name, dormant)
}

func auditDormantBuildAssemblyFile(t *testing.T, fset *token.FileSet, file *ast.File, rel string, dormant map[string]int) []string {
	t.Helper()
	var violations []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			violations = append(violations, auditDormantBuildAssemblyNode(t, fset, declaration, rel, "", "", false, dormant)...)
			continue
		}
		if function.Body == nil {
			continue
		}
		receiver := ""
		if function.Recv != nil && len(function.Recv.List) == 1 {
			receiver = renderAuditNode(t, fset, function.Recv.List[0].Type)
		}
		factoryDefinition := rel == "internal/boot/build_state.go" && function.Name.Name == "newBuildAssembly" && receiver == ""
		violations = append(violations, auditDormantBuildAssemblyNode(t, fset, function.Body, rel, function.Name.Name, receiver, factoryDefinition, dormant)...)
	}
	return violations
}

func auditDormantBuildAssemblyNode(t *testing.T, fset *token.FileSet, node ast.Node, rel, functionName, receiver string, factoryDefinition bool, dormant map[string]int) []string {
	t.Helper()
	var violations []string
	ast.Inspect(node, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			fun := renderAuditNode(t, fset, value.Fun)
			if fun == "new" && len(value.Args) == 1 && renderAuditNode(t, fset, value.Args[0]) == "buildAssembly" && !factoryDefinition {
				violations = append(violations, rel+"|"+functionName+" directly constructs dormant buildAssembly")
			}
			if isDesktopModelConsumerConstructor(fun) {
				key := rel + "|" + functionName + "|" + fun
				if dormant[key] > 0 && receiver != "*buildAssembly" {
					violations = append(violations, key+" is not on a *buildAssembly method")
				}
			}
		case *ast.CompositeLit:
			if renderAuditNode(t, fset, value.Type) == "buildAssembly" && !factoryDefinition {
				violations = append(violations, rel+"|"+functionName+" directly constructs dormant buildAssembly")
			}
		case *ast.ValueSpec:
			if value.Type != nil && renderAuditNode(t, fset, value.Type) == "buildAssembly" {
				violations = append(violations, rel+"|"+functionName+" directly declares dormant buildAssembly")
			}
		case *ast.Ident:
			if value.Name == "newBuildAssembly" {
				violations = append(violations, rel+"|"+functionName+" enables dormant buildAssembly through newBuildAssembly")
			}
		}
		return true
	})
	return violations
}

func auditDesktopConsumersInFile(t *testing.T, fset *token.FileSet, file *ast.File, rel string, validate func(string) bool) ([]string, []string) {
	t.Helper()
	var consumers []string
	var violations []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		functionName := function.Name.Name
		taskConstructors := 0
		taskResolverBindings := 0
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := renderAuditNode(t, fset, call.Fun)
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "WithStreamDispatcherResolver" && len(call.Args) == 1 && renderAuditNode(t, fset, call.Args[0]) == "resolveSubagentDispatchers" {
				taskResolverBindings++
			}
			if !isDesktopModelConsumerConstructor(fun) {
				return true
			}
			key := rel + "|" + functionName + "|" + fun
			consumers = append(consumers, key)
			if fun == "agent.NewTaskTool" {
				taskConstructors++
				return true
			}
			if validate(key) && !desktopModelConsumerCallBound(t, fset, fun, call) {
				violations = append(violations, key+" is missing its dispatcher or role-bound provider")
			}
			return true
		})
		taskKey := rel + "|" + functionName + "|agent.NewTaskTool"
		if validate(taskKey) && taskConstructors != taskResolverBindings {
			violations = append(violations, fmt.Sprintf("%s resolver bindings = %d, want %d", taskKey, taskResolverBindings, taskConstructors))
		}
	}
	return consumers, violations
}

func isDesktopModelConsumerConstructor(fun string) bool {
	switch fun {
	case "agent.New", "agent.NewCoordinator", "agent.NewTaskTool", "agent.RunSubAgentWithSession",
		"control.NewBillableProviderAutoPlanClassifier", "guardian.NewSession", "guardian.NewSessionWithDispatchers",
		"tasktree.NewLLMPlanner":
		return true
	default:
		return false
	}
}

func desktopModelConsumerCallBound(t *testing.T, fset *token.FileSet, fun string, call *ast.CallExpr) bool {
	t.Helper()
	switch fun {
	case "agent.New":
		return auditCompositeFieldEquals(t, fset, call.Args, 3, "StreamDispatcher", "billing.dispatcher") &&
			auditCompositeFieldEquals(t, fset, call.Args, 3, "DerivedStreamDispatcher", "executorCompactionDispatcher")
	case "agent.NewCoordinator":
		return auditCompositeFieldEquals(t, fset, call.Args, 4, "StreamDispatcher", "plannerDispatcher") &&
			auditCompositeFieldEquals(t, fset, call.Args, 4, "DerivedStreamDispatcher", "plannerCompactionDispatcher")
	case "agent.RunSubAgentWithSession":
		return auditCompositeFieldEquals(t, fset, call.Args, 5, "StreamDispatcher", "dispatchers.Main") &&
			auditCompositeFieldEquals(t, fset, call.Args, 5, "DerivedStreamDispatcher", "dispatchers.Compaction")
	case "control.NewBillableProviderAutoPlanClassifier":
		return len(call.Args) > 0 && renderAuditNode(t, fset, call.Args[0]) == "classifierProv"
	case "guardian.NewSessionWithDispatchers":
		return len(call.Args) >= 2 && renderAuditNode(t, fset, call.Args[len(call.Args)-2]) == "guardianDispatcher" &&
			renderAuditNode(t, fset, call.Args[len(call.Args)-1]) == "guardianCompactionDispatcher"
	case "tasktree.NewLLMPlanner":
		return auditCompositeFieldEquals(t, fset, call.Args, 0, "Provider", "taskTreePlannerProvider")
	default:
		return false
	}
}

func auditCompositeFieldEquals(t *testing.T, fset *token.FileSet, args []ast.Expr, argIndex int, fieldName, value string) bool {
	t.Helper()
	if len(args) <= argIndex {
		return false
	}
	literal, ok := args[argIndex].(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if ok && key.Name == fieldName && renderAuditNode(t, fset, field.Value) == value {
			return true
		}
	}
	return false
}

func auditStreamSelectorCount(t *testing.T, name, source string) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "Stream" {
			count++
		}
		return true
	})
	return count
}

type parsedAuditFile struct {
	fset *token.FileSet
	file *ast.File
}

func parseAuditFiles(t *testing.T, root string, rels ...string) []parsedAuditFile {
	t.Helper()
	files := make([]parsedAuditFile, 0, len(rels))
	for _, rel := range rels {
		fset, file := parseAuditFile(t, root, rel)
		files = append(files, parsedAuditFile{fset: fset, file: file})
	}
	return files
}

func auditRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve audit test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
}

func parseAuditFile(t *testing.T, root, rel string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return fset, file
}

func renderAuditNode(t *testing.T, fset *token.FileSet, node any) string {
	t.Helper()
	var out bytes.Buffer
	if err := printer.Fprint(&out, fset, node); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func mergeAuditCounts(groups ...map[string]int) map[string]int {
	merged := map[string]int{}
	for _, group := range groups {
		for key, count := range group {
			merged[key] += count
		}
	}
	return merged
}

func assertAuditCounts(t *testing.T, label string, got, want map[string]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
	for key, count := range want {
		if got[key] != count {
			t.Fatalf("%s %q count = %d, want %d; all=%#v", label, key, got[key], count, got)
		}
	}
}

func compositeFieldValueCount(t *testing.T, fset *token.FileSet, args []ast.Expr, fieldName, value string) int {
	t.Helper()
	count := 0
	for _, arg := range args {
		literal, ok := arg.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if ok && key.Name == fieldName && renderAuditNode(t, fset, field.Value) == value {
				count++
			}
		}
	}
	return count
}
