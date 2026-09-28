package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	whitespace      = regexp.MustCompile(`\s+`)
	avoidedGradient = regexp.MustCompile(`(?i)(avoid|no|without|避免|不使用|不要使用|禁止)[^.!?。\n]{0,80}((neon[^.!?。\n]{0,30}gradients?)|霓虹渐变)`)
	svgFocusBlock   = regexp.MustCompile(`(?is):focus(?:-visible)?[^\{]*\{([^}]*)\}`)
)

func gradeMeeting(dir string) (bool, string) {
	return gradeText(dir, "meeting-summary.md", [][]string{{"decisions", "决定"}, {"action items", "行动项"}, {"risks", "风险"}, {"2026-08-18"}, {"240000"}, {"chen wei"}, {"2026-08-12"}, {"lin na"}, {"2026-08-08"}, {"omar"}, {"2026-08-10"}, {"payment sandbox"}, {"data-export"}}, nil)
}
func gradeSales(dir string) (bool, string) {
	return gradeText(dir, "sales-summary.md", [][]string{{"49000"}, {"20500"}, {"20000"}, {"8500"}, {"north"}}, nil)
}
func gradeEmail(dir string) (bool, string) {
	return gradeText(dir, "customer-email.md", [][]string{{"subject", "主题"}, {"14:20"}, {"15:05"}, {"没有数据丢失", "无数据丢失", "未造成任何数据丢失", "no data was lost"}, {"10%"}}, []string{"ops-8841", "根本原因是", "root cause was"})
}
func gradeLanding(dir string) (bool, string) {
	ok, note := gradeText(dir, "landing-page-spec.md", [][]string{{"operations leaders", "运营负责人"}, {"start free trial"}, {"14-day", "14 天", "14天"}, {"99.95%"}, {"soc 2 type ii"}, {"northstar logistics"}, {"mobile", "移动端"}, {"accessibility", "可访问性", "无障碍"}, {"token", "设计变量"}}, []string{"exported the design", "created a live canvas", "produced a live canvas", "live canvas is ready", "已导出设计", "已创建实时画布"})
	if !ok {
		return false, note
	}
	b, _ := os.ReadFile(filepath.Join(dir, "landing-page-spec.md"))
	if !avoidedGradient.Match(b) {
		return false, "missing explicit avoidance of neon gradients"
	}
	return true, "all required concepts present"
}
func gradeAudit(dir string) (bool, string) {
	return gradeText(dir, "design-audit.md", [][]string{{"p0"}, {"p1"}, {"label", "标签"}, {"contrast", "对比度"}, {"focus", "焦点"}, {"error", "错误"}, {"order summary", "订单摘要"}, {"evidence", "证据"}, {"recommend", "建议"}}, nil)
}
func gradeProjectStatus(dir string) (bool, string) {
	return gradeText(dir, "status-update.md", [][]string{{"overall status", "总体状态"}, {"amber", "琥珀"}, {"completed", "已完成"}, {"2026-08-01"}, {"next steps", "下一步"}, {"mei"}, {"2026-08-09"}, {"arun"}, {"2026-08-11"}, {"risk", "风险"}, {"sso"}, {"2026-08-10"}}, nil)
}
func gradeBudget(dir string) (bool, string) {
	return gradeText(dir, "budget-variance.md", [][]string{{"39000"}, {"40000"}, {"1000"}, {"product"}, {"1500"}, {"sales"}, {"-1000", "1000 under", "节省 1000", "节省1000"}, {"support"}, {"500"}}, nil)
}
func gradeEmptyState(dir string) (bool, string) {
	return gradeText(dir, "empty-state-spec.md", [][]string{{"warehouse coordinators", "仓库协调"}, {"connect location"}, {"view setup guide"}, {"loading", "加载"}, {"error", "错误"}, {"empty", "空状态"}, {"mobile", "移动端"}, {"accessibility", "可访问"}, {"reduced motion", "reduced-motion", "prefers-reduced-motion", "减少动态"}, {"token", "设计变量"}, {"no gradient", "no gradients", "不使用渐变", "避免渐变"}}, []string{"created a live canvas", "produced a live canvas", "live canvas is ready", "exported the design", "已创建实时画布", "已导出设计"})
}

func gradeCalc(dir string) (bool, string) {
	return gradePython(dir, "import calc\nassert calc.add(2,3)==5\nassert calc.add(-2,3)==1\nassert calc.mul(2,3)==6\n")
}
func gradeDuration(dir string) (bool, string) {
	return gradePython(dir, `import duration
assert duration.parse_duration("15ms")==15
assert duration.parse_duration(" 2s ")==2000
assert duration.parse_duration("3m")==180000
assert duration.parse_duration("1h")==3600000
assert duration.format_duration(9)=="9ms"
for value in ["0s","-1s","1.5s","1d","1s extra","abc",""]:
 try: duration.parse_duration(value)
 except ValueError: pass
 else: raise AssertionError(value)
`)
}
func gradeMerge(dir string) (bool, string) {
	return gradePython(dir, `import copy,config_merge
base={"db":{"host":"a","port":5432},"features":["a"],"keep":1}
override={"db":{"port":6432},"features":["b"]}
b0=copy.deepcopy(base); o0=copy.deepcopy(override)
got=config_merge.merge_config(base,override)
assert got=={"db":{"host":"a","port":6432},"features":["b"],"keep":1}
assert base==b0 and override==o0
got["db"]["host"]="changed"
assert base["db"]["host"]=="a"
`)
}
func gradeTags(dir string) (bool, string) {
	return gradePython(dir, `import tags
values=[" Alpha ","beta","ALPHA"," ","Beta","gamma"]
original=list(values)
assert tags.normalize_tags(values)==["alpha","beta","gamma"]
assert values==original
assert tags.normalize_tags([])==[]
`)
}
func gradeRetry(dir string) (bool, string) {
	return gradePython(dir, `import retry
assert retry.retry_delay(0)==2
assert retry.retry_delay(1)==4
assert retry.retry_delay(4)==30
assert retry.retry_delay(3,base=1,cap=20)==8
assert retry.describe_retry(4)=="retry in 4s"
for args in [(-1,2,30),(1,0,30),(1,2,0),(1.5,2,30)]:
 try: retry.retry_delay(*args)
 except ValueError: pass
 else: raise AssertionError(args)
`)
}

func gradeText(dir, name string, required [][]string, forbidden []string) (bool, string) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false, "missing " + name
	}
	text := normalizeText(string(b))
	for _, alternatives := range required {
		matched := false
		for _, want := range alternatives {
			if strings.Contains(text, normalizeText(want)) {
				matched = true
				break
			}
		}
		if !matched {
			return false, "missing required concept: " + strings.Join(alternatives, " | ")
		}
	}
	for _, bad := range forbidden {
		if strings.Contains(text, normalizeText(bad)) {
			return false, "contains forbidden claim: " + bad
		}
	}
	return true, "all required concepts present"
}

func normalizeText(value string) string {
	value = strings.ToLower(html.UnescapeString(value))
	value = strings.NewReplacer(",", "", "，", "", "–", "-", "—", "-", "−", "-").Replace(value)
	return strings.TrimSpace(whitespace.ReplaceAllString(value, " "))
}

func gradePython(dir, script string) (bool, string) {
	cmd := exec.Command("python", "-c", script)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(b))
	}
	return true, "focused Python verification passed"
}

type svgFacts struct {
	Width, Height               float64
	ViewBoxWidth, ViewBoxHeight float64
	Title, Desc                 string
	Text                        []string
	Attrs                       []string
	Styles                      []string
	Elements                    map[string]int
	CardRects                   int
	FocusableControls           int
}

func parseSVG(path string) (svgFacts, error) {
	f, err := os.Open(path)
	if err != nil {
		return svgFacts{}, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	facts := svgFacts{Elements: map[string]int{}}
	var capture string
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return svgFacts{}, err
		}
		switch value := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(value.Name.Local)
			facts.Elements[name]++
			focusable, interactive := false, name == "a"
			for _, attr := range value.Attr {
				attrName := strings.ToLower(attr.Name.Local)
				switch attrName {
				case "id", "class", "role", "aria-label", "tabindex":
					if strings.TrimSpace(attr.Value) != "" {
						facts.Attrs = append(facts.Attrs, attr.Value)
					}
				}
				if attrName == "tabindex" && strings.TrimSpace(attr.Value) != "" && strings.TrimSpace(attr.Value) != "-1" {
					focusable = true
				}
				if (attrName == "role" && containsAny(strings.ToLower(attr.Value), []string{"button", "link"})) ||
					(attrName == "href" && strings.TrimSpace(attr.Value) != "") {
					interactive = true
				}
				if name == "a" && attrName == "href" && strings.TrimSpace(attr.Value) != "" {
					focusable = true
				}
			}
			if focusable && interactive {
				facts.FocusableControls++
			}
			if name == "svg" {
				for _, attr := range value.Attr {
					switch strings.ToLower(attr.Name.Local) {
					case "width":
						facts.Width = parseNumber(attr.Value)
					case "height":
						facts.Height = parseNumber(attr.Value)
					case "viewbox":
						facts.ViewBoxWidth, facts.ViewBoxHeight = parseViewBoxSize(attr.Value)
					}
				}
			}
			if name == "title" || name == "desc" || name == "text" || name == "style" {
				capture = name
				buf.Reset()
			}
			if name == "rect" {
				width, height := 0.0, 0.0
				for _, attr := range value.Attr {
					if attr.Name.Local == "width" {
						width = parseNumber(attr.Value)
					}
					if attr.Name.Local == "height" {
						height = parseNumber(attr.Value)
					}
				}
				if width >= 150 && width <= 360 && height >= 70 && height <= 190 {
					facts.CardRects++
				}
			}
		case xml.CharData:
			if capture != "" {
				buf.Write([]byte(value))
			}
		case xml.EndElement:
			name := strings.ToLower(value.Name.Local)
			if name == capture {
				text := strings.TrimSpace(buf.String())
				switch name {
				case "title":
					facts.Title = text
				case "desc":
					facts.Desc = text
				case "text":
					if text != "" {
						facts.Text = append(facts.Text, text)
					}
				case "style":
					if text != "" {
						facts.Styles = append(facts.Styles, text)
					}
				}
				capture = ""
			}
		}
	}
	return facts, nil
}

func parseNumber(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "px"))
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func parseViewBoxSize(value string) (float64, float64) {
	parts := strings.Fields(strings.ReplaceAll(value, ",", " "))
	if len(parts) != 4 {
		return 0, 0
	}
	width, widthErr := strconv.ParseFloat(parts[2], 64)
	height, heightErr := strconv.ParseFloat(parts[3], 64)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 ||
		math.IsNaN(width) || math.IsNaN(height) || math.IsInf(width, 0) || math.IsInf(height, 0) {
		return 0, 0
	}
	return width, height
}

func hasPositiveSVGCanvas(facts svgFacts) bool {
	return (facts.Width > 0 && facts.Height > 0) || (facts.ViewBoxWidth > 0 && facts.ViewBoxHeight > 0)
}

func gradeDashboardSVG(dir string) (bool, string) {
	facts, err := parseSVG(filepath.Join(dir, "dashboard-wireframe.svg"))
	if err != nil {
		return false, "invalid or missing dashboard-wireframe.svg: " + err.Error()
	}
	if facts.Width != 1440 || facts.Height != 900 {
		return false, fmt.Sprintf("canvas = %.0fx%.0f", facts.Width, facts.Height)
	}
	if facts.Title == "" || facts.Desc == "" {
		return false, "missing accessible title or description"
	}
	text := normalizeText(facts.Title + " " + facts.Desc + " " + strings.Join(facts.Text, " ") + " " + strings.Join(facts.Attrs, " "))
	for _, group := range [][]string{{"sidebar", "left sidebar", "左侧栏", "左侧边栏", "侧边栏"}, {"trend", "request volume", "趋势"}, {"recent incidents", "近期事件", "最近事件", "近期事故"}} {
		if !containsAny(text, group) {
			return false, "missing SVG concept: " + strings.Join(group, " | ")
		}
	}
	if facts.CardRects < 4 {
		return false, fmt.Sprintf("card-like rectangles = %d, want >= 4", facts.CardRects)
	}
	if facts.Elements["text"] < 12 || facts.Elements["line"] < 2 {
		return false, "insufficient visible table/card labels"
	}
	return true, "valid accessible dashboard SVG structure"
}

func gradeProfileCardSVG(dir string) (bool, string) {
	facts, err := parseSVG(filepath.Join(dir, "profile-card.svg"))
	if err != nil {
		return false, "invalid or missing profile-card.svg: " + err.Error()
	}
	if facts.Width != 640 || facts.Height != 360 {
		return false, fmt.Sprintf("canvas = %.0fx%.0f", facts.Width, facts.Height)
	}
	if facts.Title == "" || facts.Desc == "" {
		return false, "missing accessible title or description"
	}
	text := normalizeText(facts.Title + " " + facts.Desc + " " + strings.Join(facts.Text, " ") + " " + strings.Join(facts.Attrs, " "))
	for _, group := range [][]string{{"avatar"}, {"jordan lee"}, {"product designer"}, {"metric", "projects", "tasks"}, {"action", "view profile", "contact"}} {
		if !containsAny(text, group) {
			return false, "missing SVG concept: " + strings.Join(group, " | ")
		}
	}
	if facts.FocusableControls < 1 {
		return false, "missing keyboard-focusable primary action"
	}
	if !hasVisibleSVGFocusStyle(facts.Styles) {
		return false, "missing visible SVG focus style"
	}
	if facts.Elements["text"] < 7 || facts.Elements["g"] < 2 {
		return false, "insufficient grouped visible labels"
	}
	return true, "valid accessible profile card SVG"
}

func hasVisibleSVGFocusStyle(styles []string) bool {
	for _, style := range styles {
		for _, block := range svgFocusBlock.FindAllStringSubmatch(style, -1) {
			declarations := strings.ToLower(whitespace.ReplaceAllString(block[1], ""))
			for _, property := range []string{"stroke:", "outline:", "filter:", "box-shadow:", "fill:"} {
				if value, ok := cssDeclarationValue(declarations, property); ok && value != "none" && value != "transparent" {
					return true
				}
			}
		}
	}
	return false
}

func cssDeclarationValue(declarations, property string) (string, bool) {
	start := strings.Index(declarations, property)
	if start < 0 {
		return "", false
	}
	value := declarations[start+len(property):]
	if end := strings.IndexByte(value, ';'); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value), strings.TrimSpace(value) != ""
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
