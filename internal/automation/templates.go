package automation

import (
	"strconv"
	"strings"
)

const (
	TemplateKindTask                = "task"
	TemplateKindBusinessDiagnostics = "business_diagnostics"
	TemplateSourceBuiltin           = "builtin"
	TemplateSourceUser              = "user"
)

// TaskTemplate is a preset that can seed the normal task editor.
type TaskTemplate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Interval    string `json:"interval,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Source      string `json:"source,omitempty"`
}

var defaultTaskTemplates = []TaskTemplate{
	{
		ID:          "daily-office-software-tip",
		Title:       "每日办公软件技巧",
		Description: "每个工作日介绍一个常用办公软件技巧，包含操作方法和简短示例。",
		Prompt:      "请每个工作日介绍一个实用的办公软件技巧，轮换覆盖文字、表格、演示、PDF、打印、审阅和协作。注明适用软件与平台，并给出适用场景、简要步骤、示例和注意事项；不确定的菜单路径、快捷键或版本差异不要编写，避免与近期内容重复。",
		Interval:    "168h|weekly:mon,tue,wed,thu,fri@08:45",
		Icon:        "question",
	},
	{
		ID:          "meeting-prep",
		Title:       "会议前准备",
		Description: "会议开始前整理议题、目标和待确认问题。",
		Prompt:      "请在会议前帮我整理准备事项：会议目标、必须讨论的议题、需要确认的问题、相关材料清单和会后行动项模板。输出要简洁，适合会前快速扫读。",
		Interval:    "24h|daily@09:00",
		Icon:        "checklist",
	},
	{
		ID:          "weekly-work-report",
		Title:       "每周工作周报",
		Description: "每周五汇总仓库 PR 与 Issue 进展，形成简洁周报。",
		Prompt:      "请每周五汇总本周工作进展，重点整理已完成事项、进行中事项、阻塞风险、下周计划，以及仓库 PR 和 Issue 的关键变化。输出适合直接发送给团队的中文周报。",
		Interval:    "168h|weekly:fri@17:30",
		Icon:        "clipboard",
	},
	{
		ID:          "workplace-communication-example",
		Title:       "沟通表达范例",
		Description: "围绕常见工作沟通场景，提供一段可直接修改使用的表达范例。",
		Prompt:      "请介绍一个常见工作沟通场景，轮换覆盖请示、汇报、协同、催办、反馈和异议处理。给出沟通目标、简短表达范例和使用注意；不虚构具体单位、人员、事项或制度，避免与近期内容重复。",
		Interval:    "168h|weekly:mon,tue,wed,thu,fri@10:00",
		Icon:        "message",
	},
	{
		ID:          "information-retrieval-method",
		Title:       "资料检索方法",
		Description: "讲解一个资料查找和来源核验方法，帮助更快找到可靠信息。",
		Prompt:      "请介绍一个实用的资料检索方法，轮换覆盖关键词拆分、范围限定、来源判断、交叉核验和信息记录。给出适用场景、操作步骤和简短示例；不声称已经检索用户资料或获得最新结果，不确定的检索语法不要编写。",
		Interval:    "168h|weekly:mon,wed,fri@14:00",
		Icon:        "question",
	},
	{
		ID:          "chart-reading-practice",
		Title:       "图表读数练习",
		Description: "通过一个简短示例练习理解比例、变化和常见图表。",
		Prompt:      "请给出一道简短的图表读数练习，轮换覆盖表格、柱状图、折线图、饼图和同比环比。先列出少量明确的模拟数据和问题，再给出答案与解读要点；明确标注数据为示例，不将其描述为真实业务数据。只在聊天中回复，不要创建或保存文件。",
		Interval:    "168h|weekly:tue,thu@14:00",
		Icon:        "business",
	},
	{
		ID:          "weekly-business-diagnostics",
		Title:       "经营诊断",
		Description: "每周汇总真实经营数据，识别销售、库存和采购风险。",
		Prompt:      "只使用 businessDiagnostics 中配置的真实经营数据源执行周度经营诊断：采集销售、采集库存、对比历史、识别风险并生成可审计方案。",
		Interval:    "168h|weekly:mon@03:30",
		Icon:        "business",
		Kind:        "business_diagnostics",
	},
	{
		ID:          "daily-ai-news",
		Title:       "每日 AI 新闻推送",
		Description: "关注当天 AI 领域的重要动态，侧重产品、模型和工具进展。",
		Prompt:      "请每天汇总 AI 领域的重要动态，优先覆盖模型发布、产品更新、开发工具、开源项目和行业事件。输出 5 条以内高价值新闻，每条包含一句摘要、影响判断和可继续关注的关键词。",
		Interval:    "24h|daily@09:00",
		Icon:        "news",
	},
	{
		ID:          "office-security-basics",
		Title:       "信息安全常识",
		Description: "结合办公场景介绍一个信息安全注意事项和自查方法。",
		Prompt:      "请介绍一个常见办公信息安全场景，轮换覆盖钓鱼邮件、密码、文件分享、移动存储、公共网络、会议和打印材料。给出风险信号、处理步骤和简短自查清单；不虚构单位制度，不提供绕过安全控制的方法，涉及事件处置时提醒遵循本单位规定。",
		Interval:    "168h|weekly:wed@09:30",
		Icon:        "checklist",
	},
	{
		ID:          "daily-english-words",
		Title:       "每日 5 个英语单词",
		Description: "每天推荐 5 个高频实用英语单词，包含例句和记忆提示。",
		Prompt:      "请每天推荐 5 个高频实用英语单词。每个单词包含中文释义、英文例句、常见搭配和一个简短记忆提示。最后用 3 道小题帮助复习。",
		Interval:    "24h|daily@08:30",
		Icon:        "language",
	},
	{
		ID:          "daily-why",
		Title:       "每日一个为什么",
		Description: "每天抛出一个有趣问题，先提问再解释。",
		Prompt:      "请每天提出一个有趣的“为什么”问题，先给出问题本身，再用通俗语言解释答案。解释要适合普通读者，包含一个生活中的例子和一个可继续探索的小问题。",
		Interval:    "24h|daily@12:00",
		Icon:        "question",
	},
	{
		ID:          "today-in-history",
		Title:       "历史上的今天",
		Description: "从科技、电影、音乐等领域挑选一个今天发生的事件。",
		Prompt:      "请围绕今天这个日期，挑选一个历史上的重要事件。优先从科技、电影、音乐、商业或文化领域选择，说明事件经过、当时背景和它对今天的影响。",
		Interval:    "24h|daily@08:00",
		Icon:        "calendar",
	},
}

// DefaultTaskTemplates returns the built-in automation templates.
func DefaultTaskTemplates() []TaskTemplate {
	out := make([]TaskTemplate, len(defaultTaskTemplates))
	copy(out, defaultTaskTemplates)
	for i := range out {
		out[i].Source = TemplateSourceBuiltin
		if out[i].Kind == "" {
			out[i].Kind = TemplateKindTask
		}
	}
	return out
}

func TaskTemplateFromTask(task Task) TaskTemplate {
	kind := TemplateKindTask
	icon := "task"
	if task.BusinessDiagnostics != nil {
		kind = TemplateKindBusinessDiagnostics
		icon = "business"
	}
	return TaskTemplate{
		ID:          "user-" + strings.TrimSpace(task.ID),
		Title:       task.Name,
		Description: "来自手动创建的自动化任务。",
		Prompt:      task.Prompt.Prompt,
		Interval:    taskTemplateInterval(task.Schedule),
		Icon:        icon,
		Kind:        kind,
		Source:      TemplateSourceUser,
	}
}

func normalizeTaskTemplate(template TaskTemplate, source string) (TaskTemplate, error) {
	template.ID = strings.TrimSpace(template.ID)
	template.Title = strings.TrimSpace(template.Title)
	template.Description = strings.TrimSpace(template.Description)
	template.Prompt = strings.TrimSpace(template.Prompt)
	template.Interval = strings.TrimSpace(template.Interval)
	template.Icon = strings.TrimSpace(template.Icon)
	template.Kind = strings.TrimSpace(template.Kind)
	template.Source = strings.TrimSpace(template.Source)
	if source != "" {
		template.Source = source
	}
	if template.Source == "" {
		template.Source = TemplateSourceUser
	}
	if template.Kind == "" {
		template.Kind = TemplateKindTask
	}
	if template.Interval == "" {
		template.Interval = "30m"
	}
	if template.ID == "" {
		return TaskTemplate{}, ErrInvalidTask
	}
	if template.Title == "" {
		return TaskTemplate{}, ErrInvalidTask
	}
	if template.Prompt == "" {
		return TaskTemplate{}, ErrInvalidTask
	}
	return template, nil
}

func taskTemplateInterval(schedule Schedule) string {
	switch normalizeScheduleKind(schedule.Kind) {
	case ScheduleKindOnce:
		if !schedule.RunAt.IsZero() {
			return "once@" + schedule.RunAt.UTC().Format("2006-01-02T15:04:05Z")
		}
	case ScheduleKindCalendar:
		cal := schedule.Calendar
		timeOfDay := strings.TrimSpace(cal.TimeOfDay)
		if timeOfDay == "" {
			timeOfDay = "09:00"
		}
		switch strings.ToLower(strings.TrimSpace(cal.Frequency)) {
		case "weekly", "biweekly":
			days := strings.Join(cal.Weekdays, ",")
			if days == "" {
				days = "mon"
			}
			return "168h|weekly:" + days + "@" + timeOfDay
		case "monthly":
			days := intsCSV(cal.MonthDays, []int{1})
			return "720h|monthly:" + days + "@" + timeOfDay
		case "yearly":
			months := intsCSV(cal.Months, []int{1})
			days := intsCSV(cal.MonthDays, []int{1})
			return "8760h|yearly:" + strings.Split(months, ",")[0] + "-" + strings.Split(days, ",")[0] + "@" + timeOfDay
		default:
			return "24h|daily@" + timeOfDay
		}
	case ScheduleKindInterval:
		if every := strings.TrimSpace(schedule.Every); every != "" {
			return every
		}
	}
	return "30m"
}

func intsCSV(values []int, fallback []int) string {
	if len(values) == 0 {
		values = fallback
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}
