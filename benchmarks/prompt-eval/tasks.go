package main

import (
	"fmt"

	"kwu/internal/scene"
)

type evalTask struct {
	ID     string
	Scene  scene.Scene
	Prompt string
	Seed   map[string]string
	Grade  func(string) (bool, string)
}

func tasksForSuite(suite string) ([]evalTask, error) {
	switch suite {
	case "diagnostic":
		return diagnosticTasks(), nil
	case "regression":
		return regressionTasks(), nil
	case "all":
		return append(diagnosticTasks(), regressionTasks()...), nil
	default:
		return nil, fmt.Errorf("unknown suite %q", suite)
	}
}

func diagnosticTasks() []evalTask {
	return []evalTask{
		{
			ID: "office-meeting-summary", Scene: scene.Office,
			Prompt: "Read meeting_notes.txt and create meeting-summary.md. The deliverable must have Decisions, Action Items, and Risks sections. Preserve every decision, assign each action to its stated owner and deadline, distinguish risks from decisions, and do not invent facts. Finish only after checking the file against the source notes.",
			Seed: map[string]string{"meeting_notes.txt": `Project Atlas launch sync — 2026-08-03
Decisions:
- Move the public launch to 2026-08-18.
- Keep the approved budget unchanged at CNY 240,000.
Actions:
- Chen Wei: finish payment sandbox verification by 2026-08-12.
- Lin Na: obtain legal approval for launch copy by 2026-08-08.
- Omar: complete data-export documentation by 2026-08-10.
Risks:
- Payment sandbox intermittently rejects valid test cards.
- Data-export documentation is incomplete.
`}, Grade: gradeMeeting,
		},
		{
			ID: "office-sales-analysis", Scene: scene.Office,
			Prompt: "Analyze sales.csv and create sales-summary.md for a manager. Include the exact overall revenue, each region's subtotal, the highest-revenue region, and two concise recommendations grounded only in the data. Show enough arithmetic that the totals can be checked, and verify the result before finishing.",
			Seed:   map[string]string{"sales.csv": "region,product,revenue\nNorth,Alpha,12500\nNorth,Beta,8000\nSouth,Alpha,9500\nSouth,Beta,10500\nWest,Alpha,5000\nWest,Beta,3500\n"}, Grade: gradeSales,
		},
		{
			ID: "office-customer-email", Scene: scene.Office,
			Prompt: "Read incident_facts.txt and create customer-email.md containing a subject line and a customer-ready Chinese email. It must state the impact window, restoration time, absence of data loss, and the promised service credit. Be calm and accountable. Do not expose internal ticket IDs or speculate about an unconfirmed root cause. Verify all claims against the facts before finishing.",
			Seed:   map[string]string{"incident_facts.txt": "Incident date: 2026-08-02\nCustomer impact began: 14:20 CST\nService fully restored: 15:05 CST\nImpact: some users could not export reports\nData integrity: no data was lost\nCommitment: 10% service credit on the next invoice\nInternal ticket: OPS-8841\nRoot cause: investigation still in progress; not confirmed\n"}, Grade: gradeEmail,
		},
		{
			ID: "coding-fix-add", Scene: scene.Coding,
			Prompt: "The file calc.py has a bug: add(a, b) returns the wrong result. Fix add so it returns the sum of a and b. Keep the other functions unchanged. Run a focused verification before finishing.",
			Seed:   map[string]string{"calc.py": "def add(a, b):\n    return a - b\n\ndef mul(a, b):\n    return a * b\n"}, Grade: gradeCalc,
		},
		{
			ID: "coding-duration-parser", Scene: scene.Coding,
			Prompt: "Implement parse_duration(text) in duration.py. It must accept a positive integer followed by ms, s, m, or h (optional surrounding whitespace) and return milliseconds. Reject zero, negative values, decimals, unknown units, and trailing text with ValueError. Preserve format_duration. Add focused tests or otherwise execute representative valid and invalid cases before finishing.",
			Seed:   map[string]string{"duration.py": "def parse_duration(text):\n    raise NotImplementedError\n\ndef format_duration(milliseconds):\n    return f\"{milliseconds}ms\"\n"}, Grade: gradeDuration,
		},
		{
			ID: "coding-nested-config", Scene: scene.Coding,
			Prompt: "Fix merge_config(base, override) in config_merge.py. Nested dictionaries must merge recursively without mutating either input; scalar and list values from override replace the base value. Preserve unrelated keys. Validate the behavior with nested dictionaries and lists before finishing.",
			Seed:   map[string]string{"config_merge.py": "def merge_config(base, override):\n    result = dict(base)\n    result.update(override)\n    return result\n"}, Grade: gradeMerge,
		},
		{
			ID: "design-landing-spec", Scene: scene.Design,
			Prompt: "Read product_brief.md and create landing-page-spec.md as a concrete design handoff. Cover audience and conversion goal, ordered page hierarchy, hero copy using the required CTA, proof strategy, responsive behavior, accessibility, and a small visual-token set. Do not claim to have produced a live canvas or exported design. Check that every brief constraint is represented.",
			Seed:   map[string]string{"product_brief.md": "# Pulse Analytics\nAudience: operations leaders at 50–500 person B2B companies.\nGoal: start a 14-day free trial.\nRequired primary CTA text: Start free trial\nProof available: 99.95% uptime, SOC 2 Type II, and a quote from Northstar Logistics.\nBrand direction: calm, precise, trustworthy; avoid neon gradients.\nMust work on mobile and desktop.\n"}, Grade: gradeLanding,
		},
		{
			ID: "design-dashboard-wireframe", Scene: scene.Design,
			Prompt: "Create dashboard-wireframe.svg as a valid 1440×900 SVG for an operations dashboard. Include an accessible title and description, top navigation, left sidebar, four KPI cards, one trend-chart region, and one recent-incidents table with at least three visible rows. Use clear grouping and readable labels. Inspect the saved SVG before finishing; do not claim a cloud export.",
			Seed:   map[string]string{}, Grade: gradeDashboardSVG,
		},
		{
			ID: "design-checkout-audit", Scene: scene.Design,
			Prompt: "Audit checkout.html and create design-audit.md. Prioritize findings as P0, P1, or P2 and give a concrete remediation for each. The audit must cover form labeling, text contrast, keyboard focus, inline error communication, and order-summary clarity. Separate observed evidence from recommendations and do not claim to have edited the HTML.",
			Seed:   map[string]string{"checkout.html": "<!doctype html><html><head><style>body{font-family:Arial;color:#bbb;background:#fff}.field{outline:none}.error{color:#ddd}</style></head><body><h1>Checkout</h1><input class=\"field\" placeholder=\"Card number\"><input class=\"field\" placeholder=\"MM/YY\"><div class=\"error\">Invalid</div><p>Total due later</p><button>Pay</button></body></html>"}, Grade: gradeAudit,
		},
	}
}

func regressionTasks() []evalTask {
	return []evalTask{
		{
			ID: "office-project-status", Scene: scene.Office,
			Prompt: "Read project_facts.txt and create status-update.md for leadership with Overall Status, Completed, Next Steps, and Risks. Preserve owners and dates, distinguish facts from risks, and do not invent progress. Verify the deliverable before finishing.",
			Seed:   map[string]string{"project_facts.txt": "Status: Amber\nCompleted: data model approved on 2026-08-01\nNext: Mei ships migration rehearsal by 2026-08-09\nNext: Arun completes support playbook by 2026-08-11\nRisk: vendor SSO certificate may arrive after 2026-08-10\n"}, Grade: gradeProjectStatus,
		},
		{
			ID: "office-budget-variance", Scene: scene.Office,
			Prompt: "Analyze budget.csv and create budget-variance.md. Include total planned, total actual, total variance, each team's variance, and two concise observations grounded only in the data. Show checkable arithmetic and verify it before finishing.",
			Seed:   map[string]string{"budget.csv": "team,planned,actual\nProduct,12000,13500\nSales,18000,17000\nSupport,9000,9500\n"}, Grade: gradeBudget,
		},
		{
			ID: "coding-normalize-tags", Scene: scene.Coding,
			Prompt: "Implement normalize_tags(values) in tags.py. Trim whitespace, lowercase values, discard empty items, remove duplicates while preserving first-seen order, and do not mutate the input. Run focused valid and edge-case checks before finishing.",
			Seed:   map[string]string{"tags.py": "def normalize_tags(values):\n    raise NotImplementedError\n"}, Grade: gradeTags,
		},
		{
			ID: "coding-retry-delay", Scene: scene.Coding,
			Prompt: "Fix retry_delay(attempt, base=2, cap=30) in retry.py. attempt is a non-negative integer; return min(cap, base * 2**attempt). Reject invalid attempt/base/cap with ValueError and preserve describe_retry. Validate boundary cases before finishing.",
			Seed:   map[string]string{"retry.py": "def retry_delay(attempt, base=2, cap=30):\n    return base * attempt\n\ndef describe_retry(seconds):\n    return f\"retry in {seconds}s\"\n"}, Grade: gradeRetry,
		},
		{
			ID: "design-empty-state-spec", Scene: scene.Design,
			Prompt: "Read empty_state_brief.md and create empty-state-spec.md as an implementation-ready design handoff. Include audience, hierarchy, exact primary CTA, loading/error/empty distinctions, responsive behavior, accessibility, and a compact token set. Do not claim a live canvas or export. Verify every brief constraint.",
			Seed:   map[string]string{"empty_state_brief.md": "Product: Inventory console\nAudience: warehouse coordinators\nSituation: no locations connected\nPrimary CTA: Connect location\nSecondary action: View setup guide\nTone: calm and operational; no gradients\nMust support mobile and desktop, keyboard use, and reduced motion\n"}, Grade: gradeEmptyState,
		},
		{
			ID: "design-profile-card-svg", Scene: scene.Design,
			Prompt: "Create profile-card.svg as a valid accessible 640×360 SVG for Jordan Lee, Product Designer. Include title and description, avatar placeholder, the supplied name and role, three labeled metrics, one keyboard-focusable primary action, and visible focus treatment. Use grouped structure and readable labels. Inspect the saved SVG before finishing.",
			Seed:   map[string]string{}, Grade: gradeProfileCardSVG,
		},
	}
}
