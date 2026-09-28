package automation

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultTaskTemplatesAreValidTaskSeeds(t *testing.T) {
	templates := DefaultTaskTemplates()
	wantTemplateIDs := []string{
		"daily-office-software-tip",
		"meeting-prep",
		"weekly-work-report",
		"workplace-communication-example",
		"information-retrieval-method",
		"chart-reading-practice",
		"weekly-business-diagnostics",
		"daily-ai-news",
		"office-security-basics",
		"daily-english-words",
		"daily-why",
		"today-in-history",
	}
	if len(templates) != len(wantTemplateIDs) {
		t.Fatalf("templates len = %d, want %d", len(templates), len(wantTemplateIDs))
	}

	seen := map[string]bool{}
	templatesByID := make(map[string]TaskTemplate, len(templates))
	var businessTemplate TaskTemplate
	var officeSoftwareTemplate TaskTemplate
	for i, template := range templates {
		if template.ID != wantTemplateIDs[i] {
			t.Fatalf("template %d id = %q, want %q", i, template.ID, wantTemplateIDs[i])
		}
		if strings.TrimSpace(template.ID) == "" || strings.TrimSpace(template.Title) == "" || strings.TrimSpace(template.Prompt) == "" {
			t.Fatalf("template has required blank field: %+v", template)
		}
		if strings.TrimSpace(template.Interval) == "" {
			t.Fatalf("template %q has blank interval", template.ID)
		}
		every := strings.Split(template.Interval, "|")[0]
		if _, err := time.ParseDuration(every); err != nil {
			t.Fatalf("template %q interval %q has invalid duration prefix: %v", template.ID, template.Interval, err)
		}
		if seen[template.ID] {
			t.Fatalf("duplicate template id %q", template.ID)
		}
		if template.ID == "pet-wallpaper" {
			t.Fatal("pet wallpaper template should not be built in")
		}
		if template.ID == "bedtime-story" {
			t.Fatal("bedtime story template should not be built in")
		}
		seen[template.ID] = true
		templatesByID[template.ID] = template

		_, err := NormalizeTask(Task{
			ID:       "task-" + template.ID,
			Name:     template.Title,
			Prompt:   PromptConfig{Prompt: template.Prompt},
			Schedule: Schedule{Kind: ScheduleKindInterval, Every: every},
			Execution: ExecutionPolicy{
				ApprovalMode: ApprovalModeAuto,
			},
		})
		if err != nil {
			t.Fatalf("template %q cannot seed task: %v", template.ID, err)
		}
		if template.Kind == "business_diagnostics" {
			businessTemplate = template
		}
		if template.ID == "daily-office-software-tip" {
			officeSoftwareTemplate = template
		}
	}
	wantOfficeSoftwareTemplate := TaskTemplate{
		ID:          "daily-office-software-tip",
		Title:       "每日办公软件技巧",
		Description: "每个工作日介绍一个常用办公软件技巧，包含操作方法和简短示例。",
		Prompt:      "请每个工作日介绍一个实用的办公软件技巧，轮换覆盖文字、表格、演示、PDF、打印、审阅和协作。注明适用软件与平台，并给出适用场景、简要步骤、示例和注意事项；不确定的菜单路径、快捷键或版本差异不要编写，避免与近期内容重复。",
		Interval:    "168h|weekly:mon,tue,wed,thu,fri@08:45",
		Icon:        "question",
		Kind:        TemplateKindTask,
		Source:      TemplateSourceBuiltin,
	}
	if officeSoftwareTemplate != wantOfficeSoftwareTemplate {
		t.Fatalf("office software template = %+v, want %+v", officeSoftwareTemplate, wantOfficeSoftwareTemplate)
	}
	wantNewTemplates := map[string]TaskTemplate{
		"workplace-communication-example": {
			ID:          "workplace-communication-example",
			Title:       "沟通表达范例",
			Description: "围绕常见工作沟通场景，提供一段可直接修改使用的表达范例。",
			Prompt:      "请介绍一个常见工作沟通场景，轮换覆盖请示、汇报、协同、催办、反馈和异议处理。给出沟通目标、简短表达范例和使用注意；不虚构具体单位、人员、事项或制度，避免与近期内容重复。",
			Interval:    "168h|weekly:mon,tue,wed,thu,fri@10:00",
			Icon:        "message",
			Kind:        TemplateKindTask,
			Source:      TemplateSourceBuiltin,
		},
		"information-retrieval-method": {
			ID:          "information-retrieval-method",
			Title:       "资料检索方法",
			Description: "讲解一个资料查找和来源核验方法，帮助更快找到可靠信息。",
			Prompt:      "请介绍一个实用的资料检索方法，轮换覆盖关键词拆分、范围限定、来源判断、交叉核验和信息记录。给出适用场景、操作步骤和简短示例；不声称已经检索用户资料或获得最新结果，不确定的检索语法不要编写。",
			Interval:    "168h|weekly:mon,wed,fri@14:00",
			Icon:        "question",
			Kind:        TemplateKindTask,
			Source:      TemplateSourceBuiltin,
		},
		"chart-reading-practice": {
			ID:          "chart-reading-practice",
			Title:       "图表读数练习",
			Description: "通过一个简短示例练习理解比例、变化和常见图表。",
			Prompt:      "请给出一道简短的图表读数练习，轮换覆盖表格、柱状图、折线图、饼图和同比环比。先列出少量明确的模拟数据和问题，再给出答案与解读要点；明确标注数据为示例，不将其描述为真实业务数据。只在聊天中回复，不要创建或保存文件。",
			Interval:    "168h|weekly:tue,thu@14:00",
			Icon:        "business",
			Kind:        TemplateKindTask,
			Source:      TemplateSourceBuiltin,
		},
		"office-security-basics": {
			ID:          "office-security-basics",
			Title:       "信息安全常识",
			Description: "结合办公场景介绍一个信息安全注意事项和自查方法。",
			Prompt:      "请介绍一个常见办公信息安全场景，轮换覆盖钓鱼邮件、密码、文件分享、移动存储、公共网络、会议和打印材料。给出风险信号、处理步骤和简短自查清单；不虚构单位制度，不提供绕过安全控制的方法，涉及事件处置时提醒遵循本单位规定。",
			Interval:    "168h|weekly:wed@09:30",
			Icon:        "checklist",
			Kind:        TemplateKindTask,
			Source:      TemplateSourceBuiltin,
		},
	}
	for id, want := range wantNewTemplates {
		if got := templatesByID[id]; got != want {
			t.Fatalf("template %q = %+v, want %+v", id, got, want)
		}
	}
	if businessTemplate.ID == "" {
		t.Fatal("business diagnostics template not found")
	}
	if businessTemplate.ID != "weekly-business-diagnostics" || businessTemplate.Interval != "168h|weekly:mon@03:30" || businessTemplate.Icon != "business" {
		t.Fatalf("business diagnostics template = %+v, want stable weekly business template", businessTemplate)
	}
	if !strings.Contains(businessTemplate.Prompt, "businessDiagnostics") {
		t.Fatalf("business diagnostics prompt should point to configured data sources: %q", businessTemplate.Prompt)
	}

	templates[0].Title = "mutated"
	if DefaultTaskTemplates()[0].Title == "mutated" {
		t.Fatal("DefaultTaskTemplates returned shared backing storage")
	}
}
