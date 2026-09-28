package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/skill"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

func reviewCommand(args []string) int {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	base := fs.String("base", "", "base branch/commit to diff against (defaults to HEAD — reviews uncommitted working-tree changes)")
	commit := fs.String("commit", "", "review a specific commit (shows changes introduced by that commit)")
	model := fs.String("model", "", "provider name override (default: config default_model)")
	instructions := fs.String("instructions", "", "extra review instructions appended to the prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// 1. Get the diff.
	diff, err := getReviewDiff(*base, *commit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if diff == "" {
		fmt.Println("No changes to review.")
		return 0
	}

	// 2. Load config and resolve model.
	cfg, err := loadReviewConfig(defaultCLIActorContext())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to load config:", err)
		return 1
	}
	modelName := *model
	if modelName == "" {
		modelName = cfg.DefaultModel
	}
	entry, ok := cfg.ResolveModel(modelName)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown model %q — check your config\n", modelName)
		return 1
	}
	if err := cfg.Validate(modelName); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// 3. Create provider.
	prov, err := boot.NewProviderWithProxy(entry, cfg.OutboundProxySpec())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to create provider:", err)
		return 1
	}

	// 4. Get the built-in review skill.
	root, _ := os.Getwd()
	reviewSk, err := loadReviewSkill(root, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// 5. Build a review-scoped sub-agent registry.
	reg := buildReviewSubagentRegistry(reviewSk)

	// 6. Prepare the review prompt.
	task, err := buildReviewTask(diff, *instructions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// 7. Run the review subagent.
	ctx := context.Background()
	result, err := agent.RunSubAgentWithSession(ctx, prov, reg, agent.NewSession(reviewSk.Body), task, agent.Options{
		MaxSteps:      12,
		Temperature:   cfg.Agent.Temperature,
		Pricing:       entry.Price,
		ContextWindow: entry.ContextWindow,
	}, event.Discard)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: review failed:", err)
		return 1
	}

	fmt.Print(result)
	return 0
}

func loadReviewConfig(actor userctx.ActorContext) (*config.Config, error) {
	return loadCLIConfigForActor(actor)
}

func newReviewSkillStore(root string, cfg *config.Config) *skill.Store {
	opts := skill.Options{ProjectRoot: root, Stderr: os.Stderr}
	if cfg != nil {
		opts.CustomPaths = cfg.SkillCustomPaths()
		opts.ExcludedPaths = cfg.SkillExcludedPaths()
		opts.MaxDepth = cfg.SkillMaxDepth()
	}
	return skill.New(opts)
}

func loadReviewSkill(root string, cfg *config.Config) (skill.Skill, error) {
	skillStore := newReviewSkillStore(root, cfg)
	reviewSk, ok := skillStore.Read("review")
	if !ok {
		return skill.Skill{}, fmt.Errorf("built-in review skill not found")
	}
	if reviewSk.RunAs != skill.RunSubagent {
		return skill.Skill{}, fmt.Errorf("review skill is not a subagent skill")
	}
	return reviewSk, nil
}

func buildReviewSubagentRegistry(reviewSk skill.Skill) *tool.Registry {
	// The shared helper strips subagent-unavailable background capabilities while
	// preserving foreground bash. This direct CLI path does not go through boot,
	// so it first builds the small parent set from the review skill allow-list.
	parentReg := tool.NewRegistry()
	for _, name := range reviewSk.AllowedTools {
		if tl, ok := tool.LookupBuiltin(name); ok {
			parentReg.Add(tl)
		}
	}
	return agent.SubagentToolRegistry(parentReg, reviewSk.AllowedTools)
}

// getReviewDiff runs the appropriate git diff command and returns its output.
// - commit="abc": shows diff of abc^..abc
// - base="main": shows diff of main...HEAD
// - neither: shows diff of uncommitted working-tree changes
func getReviewDiff(base, commit string) (string, error) {
	cwd, _ := os.Getwd()
	ctx := context.Background()
	switch {
	case commit != "":
		return runGit(ctx, cwd, "diff", commit+"^.."+commit)
	case base != "":
		return runGit(ctx, cwd, "diff", base+"...HEAD")
	default:
		// Working tree changes: staged + unstaged.
		out, err := runGit(ctx, cwd, "diff", "HEAD")
		if err != nil {
			return "", err
		}
		if out == "" {
			// No working-tree changes; check for staged-only.
			out, err = runGit(ctx, cwd, "diff", "--cached")
		}
		return out, err
	}
}

const reviewDiffMaxLen = 16000

func buildReviewTask(diff string, extra string) (string, error) {
	if len(diff) > reviewDiffMaxLen {
		return "", fmt.Errorf("review diff too large (%d chars > %d); use --base/--commit or split the review into smaller changes", len(diff), reviewDiffMaxLen)
	}
	var b strings.Builder
	b.WriteString("Review the following changes. ")
	if extra != "" {
		b.WriteString(extra)
		b.WriteString(" ")
	}
	b.WriteString("The diff is:\n\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```")
	return b.String(), nil
}
