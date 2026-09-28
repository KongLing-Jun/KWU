package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var testFunctionPattern = regexp.MustCompile(`(?m)^\s*def\s+test_[A-Za-z0-9_]+\s*\(`)

func gradeDesktopOfficeProject(root string) []desktopCheck {
	checks := []desktopCheck{
		gradeConceptFile(root, "leadership-status.md", [][]string{
			{"project orion"}, {"liu mei"}, {"2026-09-30"}, {"480000"},
			{"migration rehearsal"}, {"chen wei"}, {"at risk"}, {"security review"},
			{"priya shah"}, {"on track"}, {"support training"}, {"omar haddad"},
			{"not started"}, {"2026-09-22"}, {"supersed", "replaced", "替代", "取代", "作废"},
			{"vendor export"}, {"no delay is confirmed", "delay is not confirmed", "尚未确认延期", "未确认延期"},
		}, nil),
	}
	records, err := readCSVRecords(filepath.Join(root, "action-items.csv"))
	if err != nil {
		return append(checks, desktopCheck{Name: "action-items.csv", Passed: false, Note: err.Error()})
	}
	want := []map[string]string{
		{"owner": "Chen Wei", "due_date": "2026-09-10", "status": "open", "action": "vendor export"},
		{"owner": "Priya Shah", "due_date": "2026-09-15", "status": "open", "action": "security"},
		{"owner": "Omar Haddad", "due_date": "2026-09-20", "status": "open", "action": "training"},
	}
	checks = append(checks, gradeMappedRows("action-items.csv", records, want))
	return checks
}

func gradeDesktopOfficeBudget(root string) []desktopCheck {
	checks := []desktopCheck{
		gradeConceptFile(root, "budget-analysis.md", [][]string{
			{"390000"}, {"402000"}, {"12000"}, {"product"}, {"120000"}, {"135000"},
			{"15000"}, {"sales"}, {"180000"}, {"172000"}, {"-8000", "8000 under", "节省8000", "低于计划8000"},
			{"support"}, {"90000"}, {"95000"}, {"5000"}, {"actual - planned", "实际减计划"},
		}, nil),
		gradeConceptFile(root, "decision-summary.md", [][]string{
			{"405000"}, {"product"}, {"contractor"}, {"sales"}, {"q4"},
			{"support"}, {"not confirmed", "unconfirmed", "未确认", "待确认"}, {"2026-09-08"},
		}, []string{"support overrun is caused by", "support overrun was caused by", "support超支原因是"}),
	}
	return checks
}

func gradeDesktopCodingRetry(root string) []desktopCheck {
	verification := `import retry_policy
assert retry_policy.retry_delay(0) == 2
assert retry_policy.retry_delay(1) == 4
assert retry_policy.retry_delay(4) == 30
assert retry_policy.retry_delay(3, base=1, cap=20) == 8
assert retry_policy.retry_delay(2, base=2.5, cap=20) == 10
assert retry_policy.describe_retry(7) == "retry in 7s"
invalid = [
    ((-1,), {}), ((True,), {}), ((1.5,), {}),
    ((1,), {"base": 0}), ((1,), {"base": -1}), ((1,), {"base": True}),
    ((1,), {"cap": 0}), ((1,), {"cap": -1}), ((1,), {"cap": True}),
]
for args, kwargs in invalid:
    try:
        retry_policy.retry_delay(*args, **kwargs)
    except (ValueError, TypeError):
        pass
    else:
        raise AssertionError((args, kwargs))
`
	passed, note := gradePython(root, verification)
	return []desktopCheck{
		{Name: "hidden-retry-semantics", Passed: passed, Note: note},
		gradePythonSuite(root),
		gradeMinimumPythonTests(root, "test_retry_policy.py", 4),
	}
}

func gradeDesktopCodingConfig(root string) []desktopCheck {
	verification := `import copy
from config_merge import merge_config
from settings import effective_settings, DEFAULTS
base = {"db": {"host": "a", "tls": {"enabled": False, "modes": ["old"]}}, "features": ["a"], "keep": {"x": 1}}
override = {"db": {"tls": {"enabled": True}}, "features": ["b"]}
b0 = copy.deepcopy(base); o0 = copy.deepcopy(override)
got = merge_config(base, override)
assert got == {"db": {"host": "a", "tls": {"enabled": True, "modes": ["old"]}}, "features": ["b"], "keep": {"x": 1}}
assert base == b0 and override == o0
got["db"]["tls"]["modes"].append("new")
got["features"].append("c")
got["keep"]["x"] = 2
assert base == b0 and override == o0
before = copy.deepcopy(DEFAULTS)
settings = effective_settings({"db": {"port": 6432}})
assert settings["db"] == {"host": "localhost", "port": 6432}
assert settings["features"] == ["audit"] and settings["keep"] is True
assert DEFAULTS == before
`
	passed, note := gradePython(root, verification)
	return []desktopCheck{
		{Name: "hidden-config-semantics", Passed: passed, Note: note},
		gradePythonSuite(root),
		gradeMinimumPythonTests(root, "test_config_merge.py", 4),
	}
}

func gradeDesktopDesignEmpty(root string) []desktopCheck {
	htmlBody, htmlErr := os.ReadFile(filepath.Join(root, "empty-state.html"))
	cssBody, cssErr := os.ReadFile(filepath.Join(root, "empty-state.css"))
	checks := []desktopCheck{
		gradeRead("empty-state.html", htmlErr),
		gradeRead("empty-state.css", cssErr),
	}
	if htmlErr != nil || cssErr != nil {
		return checks
	}
	htmlText := normalizeText(string(htmlBody))
	cssText := strings.ToLower(string(cssBody))
	checks = append(checks,
		checkContainsAll("empty-state-copy", htmlText, []string{"no locations connected", "connect a warehouse location to start tracking inventory", "connect location", "view setup guide"}),
		desktopCheck{Name: "semantic-empty-state", Passed: containsAny(htmlText, []string{"<main", "role=\"main\"", "role='main'"}) && strings.Contains(htmlText, "<h1"), Note: "requires main semantics and h1"},
		desktopCheck{Name: "stylesheet-linked", Passed: strings.Contains(strings.ToLower(string(htmlBody)), "empty-state.css"), Note: "HTML references empty-state.css"},
		gradeResponsiveCSS(cssText),
		gradeReducedMotionCSS(cssText),
		gradeVisibleFocusCSS(cssText),
		gradeColorSchemeCSS(cssText),
		gradeNoGradient(string(htmlBody)+"\n"+string(cssBody)),
	)
	return checks
}

func gradeDesktopDesignDashboard(root string) []desktopCheck {
	htmlBody, htmlErr := os.ReadFile(filepath.Join(root, "operations-dashboard.html"))
	cssBody, cssErr := os.ReadFile(filepath.Join(root, "dashboard.css"))
	checks := []desktopCheck{gradeRead("operations-dashboard.html", htmlErr), gradeRead("dashboard.css", cssErr)}
	if htmlErr == nil && cssErr == nil {
		htmlText := normalizeText(string(htmlBody))
		cssText := strings.ToLower(string(cssBody))
		checks = append(checks,
			checkContainsAll("dashboard-html", htmlText, []string{"dashboard-visual.svg", "review incidents"}),
			gradeResponsiveCSS(cssText), gradeReducedMotionCSS(cssText), gradeVisibleFocusCSS(cssText),
			gradeNoGradient(string(htmlBody)+"\n"+string(cssBody)),
		)
	}
	facts, err := parseSVG(filepath.Join(root, "dashboard-visual.svg"))
	if err != nil {
		checks = append(checks, desktopCheck{Name: "dashboard-visual.svg", Passed: false, Note: err.Error()})
		return checks
	}
	text := normalizeText(facts.Title + " " + facts.Desc + " " + strings.Join(facts.Text, " ") + " " + strings.Join(facts.Attrs, " "))
	required := []string{"on-time delivery", "open incidents", "orders today", "average delay", "trend", "inc-104", "inc-108", "inc-111"}
	checks = append(checks,
		desktopCheck{Name: "svg-canvas", Passed: hasPositiveSVGCanvas(facts), Note: fmt.Sprintf("size %.0fx%.0f; viewBox %.0fx%.0f", facts.Width, facts.Height, facts.ViewBoxWidth, facts.ViewBoxHeight)},
		desktopCheck{Name: "svg-accessible-name", Passed: facts.Title != "" && facts.Desc != "", Note: "requires title and desc"},
		checkContainsAll("svg-visible-content", text, required),
		desktopCheck{Name: "svg-kpi-cards", Passed: facts.CardRects >= 4, Note: fmt.Sprintf("card-like rectangles=%d", facts.CardRects)},
		desktopCheck{Name: "svg-trend-structure", Passed: facts.Elements["line"] >= 2 || facts.Elements["polyline"] >= 1 || facts.Elements["path"] >= 1, Note: "requires visible trend structure"},
	)
	return checks
}

func gradeConceptFile(root, name string, required [][]string, forbidden []string) desktopCheck {
	passed, note := gradeText(root, name, required, forbidden)
	return desktopCheck{Name: name, Passed: passed, Note: note}
}

func gradeMappedRows(name string, records, expected []map[string]string) desktopCheck {
	if len(records) != len(expected) {
		return desktopCheck{Name: name, Passed: false, Note: fmt.Sprintf("rows=%d want=%d", len(records), len(expected))}
	}
	matched := make([]bool, len(records))
	for _, want := range expected {
		found := false
		for index, record := range records {
			if matched[index] || normalizeText(record["owner"]) != normalizeText(want["owner"]) ||
				normalizeText(record["due_date"]) != normalizeText(want["due_date"]) ||
				normalizeText(record["status"]) != normalizeText(want["status"]) ||
				!strings.Contains(normalizeText(record["action"]), normalizeText(want["action"])) {
				continue
			}
			matched[index] = true
			found = true
			break
		}
		if !found {
			return desktopCheck{Name: name, Passed: false, Note: "missing row for " + want["owner"]}
		}
	}
	return desktopCheck{Name: name, Passed: true, Note: "all explicit actions present"}
}

func gradePythonSuite(root string) desktopCheck {
	passed, note := gradePython(root, "import unittest; suite=unittest.defaultTestLoader.discover('.'); result=unittest.TextTestRunner(verbosity=0).run(suite); assert result.wasSuccessful() and result.testsRun > 0")
	return desktopCheck{Name: "local-python-suite", Passed: passed, Note: note}
}

func gradeMinimumPythonTests(root, name string, minimum int) desktopCheck {
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return desktopCheck{Name: "added-focused-tests", Passed: false, Note: err.Error()}
	}
	count := len(testFunctionPattern.FindAll(body, -1))
	return desktopCheck{Name: "added-focused-tests", Passed: count >= minimum, Note: fmt.Sprintf("test functions=%d want>=%d", count, minimum)}
}

func gradeRead(name string, err error) desktopCheck {
	if err != nil {
		return desktopCheck{Name: name, Passed: false, Note: err.Error()}
	}
	return desktopCheck{Name: name, Passed: true, Note: "file readable"}
}

func checkContainsAll(name, text string, values []string) desktopCheck {
	missing := make([]string, 0)
	for _, value := range values {
		if !strings.Contains(text, normalizeText(value)) {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return desktopCheck{Name: name, Passed: len(missing) == 0, Note: "missing: " + strings.Join(missing, ", ")}
}

func gradeResponsiveCSS(css string) desktopCheck {
	passed := strings.Contains(css, "@media") && (strings.Contains(css, "max-width") || strings.Contains(css, "min-width"))
	return desktopCheck{Name: "responsive-css", Passed: passed, Note: "requires a width media query"}
}

func gradeReducedMotionCSS(css string) desktopCheck {
	passed := strings.Contains(css, "prefers-reduced-motion") && strings.Contains(css, "reduce")
	return desktopCheck{Name: "reduced-motion-css", Passed: passed, Note: "requires prefers-reduced-motion: reduce"}
}

func gradeVisibleFocusCSS(css string) desktopCheck {
	blocks := regexp.MustCompile(`(?is):focus-visible\s*\{([^}]+)\}`).FindAllStringSubmatch(css, -1)
	for _, block := range blocks {
		declarations := strings.ReplaceAll(block[1], " ", "")
		visibleOutline := strings.Contains(declarations, "outline:") &&
			containsAny(declarations, []string{"solid", "dashed", "double"})
		visibleShadow := strings.Contains(declarations, "box-shadow:") && !strings.Contains(declarations, "box-shadow:none")
		if visibleOutline || visibleShadow {
			return desktopCheck{Name: "visible-focus-css", Passed: true, Note: "visible :focus-visible treatment found"}
		}
	}
	return desktopCheck{Name: "visible-focus-css", Passed: false, Note: "missing visible :focus-visible outline or shadow"}
}

func gradeColorSchemeCSS(css string) desktopCheck {
	passed := strings.Contains(css, "prefers-color-scheme") || strings.Contains(css, "color-scheme:")
	return desktopCheck{Name: "color-scheme-css", Passed: passed, Note: "requires explicit light/dark support"}
}

func gradeNoGradient(body string) desktopCheck {
	passed := !regexp.MustCompile(`(?i)(linear|radial|conic)-gradient\s*\(`).MatchString(body)
	return desktopCheck{Name: "no-gradient", Passed: passed, Note: "no CSS gradient function"}
}
