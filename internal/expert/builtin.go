package expert

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed builtin.json
var builtinJSON []byte

// builtinDefinitions mirrors the expert catalog that used to live as static
// frontend data. It is the seed data for the SQLite experts table.
var builtinDefinitions = []Definition{
	{
		ID:          "architect",
		Mode:        ModeExpert,
		Name:        "架构顾问",
		Role:        "技术工程",
		Description: "拆解复杂系统设计，给出模块边界、接口关系和演进路径。",
		Categories:  []string{"技术工程", "项目管理"},
		Tags:        []string{"架构设计", "技术方案", "边界梳理"},
		IconKey:     "code2",
		Tone:        "blue",
		AvatarKey:   "architect",
		Prompts: []string{
			"请帮我拆解这个功能的技术方案，说明模块边界、核心流程和风险点。",
			"请评估当前架构是否适合继续扩展，并给出最小改动建议。",
			"请把这个需求拆成前端、后端和数据三部分的交付清单。",
		},
		SortOrder: 10,
	},
	{
		ID:          "code-review",
		Mode:        ModeExpert,
		Name:        "代码审查专家",
		Role:        "代码质量",
		Description: "检查变更中的正确性风险、可维护性问题和必要测试。",
		Categories:  []string{"技术工程"},
		Tags:        []string{"Bug风险", "代码质量", "回归点"},
		IconKey:     "file-check-2",
		Tone:        "violet",
		AvatarKey:   "code-review",
		Prompts: []string{
			"请从代码审查角度检查这次改动，优先指出可能导致线上问题的风险。",
			"请帮我找出这个实现里最值得先修的三个问题。",
			"请评估这段代码是否需要补测试，并说明测试重点。",
		},
		SortOrder: 20,
	},
	{
		ID:          "security-review",
		Mode:        ModeExpert,
		Name:        "安全审查专家",
		Role:        "安全质量",
		Description: "聚焦注入、权限、密钥、外部输入和敏感操作风险。",
		Categories:  []string{"技术工程"},
		Tags:        []string{"权限", "注入", "密钥"},
		IconKey:     "shield-check",
		Tone:        "green",
		AvatarKey:   "security-review",
		Prompts: []string{
			"请帮我检查这个方案里的安全风险，重点看权限、输入和密钥处理。",
			"请评估这个接口是否存在越权或注入风险。",
			"请给我一份上线前安全检查清单。",
		},
		SortOrder: 30,
	},
	{
		ID:          "automation-flow",
		Mode:        ModeExpert,
		Name:        "自动化流程专家",
		Role:        "办公自动化",
		Description: "把重复任务整理成可执行步骤，适合定时、批量和跨工具流程。",
		Categories:  []string{"办公自动化", "项目管理"},
		Tags:        []string{"流程", "定时", "执行"},
		IconKey:     "workflow",
		Tone:        "teal",
		AvatarKey:   "automation-flow",
		Prompts: []string{
			"请把这个重复任务设计成自动化流程，列出触发条件、步骤和异常处理。",
			"请帮我把这件事拆成可以每天自动执行的任务清单。",
			"请设计一个适合团队复用的办公自动化流程。",
		},
		SortOrder: 40,
	},
	{
		ID:          "data-report",
		Mode:        ModeExpert,
		Name:        "数据分析报告师",
		Role:        "数据智能",
		Description: "把数据和日志整理成指标诊断、趋势判断和决策报告。",
		Categories:  []string{"数据分析", "报告生成", "财务分析"},
		Tags:        []string{"指标", "诊断", "报告"},
		IconKey:     "bar-chart-3",
		Tone:        "amber",
		AvatarKey:   "data-report",
		Prompts: []string{
			"请帮我设计一套数据分析报告框架，包含核心指标、诊断路径和结论结构。",
			"请把这些数据整理成一份面向管理层的分析报告。",
			"请帮我分析指标变化原因，并给出可执行建议。",
		},
		SortOrder: 50,
	},
	{
		ID:          "doc-report",
		Mode:        ModeExpert,
		Name:        "文档报告专家",
		Role:        "报告生成",
		Description: "把讨论、代码和资料整理成结构清晰的交付说明。",
		Categories:  []string{"报告生成", "办公自动化"},
		Tags:        []string{"文档", "摘要", "交付"},
		IconKey:     "file-text",
		Tone:        "stone",
		AvatarKey:   "doc-report",
		Prompts: []string{
			"请把当前内容整理成一份结构清晰的交付说明。",
			"请帮我生成一份项目进展报告，突出结论、风险和下一步。",
			"请把这些零散信息整理成可读性强的文档。",
		},
		SortOrder: 60,
	},
	{
		ID:          "research",
		Mode:        ModeExpert,
		Name:        "探索专家",
		Role:        "深度研究",
		Description: "快速阅读代码库和资料，整理关键路径、证据和结论摘要。",
		Categories:  []string{"技术工程", "数据分析"},
		Tags:        []string{"代码探索", "调用链", "结论摘要"},
		IconKey:     "compass",
		Tone:        "violet",
		AvatarKey:   "research",
		Prompts: []string{
			"请帮我快速探索这个代码库，找出和当前需求最相关的文件和调用链。",
			"请先调研再给我结论，列出关键证据和不确定点。",
			"请帮我梳理这个模块的职责和关键路径。",
		},
		SortOrder: 70,
	},
	{
		ID:          "ops-growth",
		Mode:        ModeExpert,
		Name:        "运营增长顾问",
		Role:        "运营增长",
		Description: "围绕增长目标拆解人群、触点、内容和转化动作。",
		Categories:  []string{"运营增长", "客户服务", "数据分析"},
		Tags:        []string{"增长", "转化", "复盘"},
		IconKey:     "briefcase-business",
		Tone:        "orange",
		AvatarKey:   "ops-growth",
		Prompts: []string{
			"请帮我设计一个运营增长方案，包含目标人群、触点和复盘指标。",
			"请分析这次活动转化偏低的可能原因，并给出优化动作。",
			"请帮我把客户反馈整理成可执行的运营改进计划。",
		},
		SortOrder: 80,
	},
	{
		ID:          "code-delivery-team",
		Mode:        ModeTeam,
		Name:        "代码交付专家团",
		Role:        "技术工程",
		Description: "围绕功能交付协同分析架构、代码质量、测试覆盖和上线风险。",
		Categories:  []string{"技术工程", "项目管理"},
		Tags:        []string{"方案", "质量", "测试"},
		IconKey:     "users",
		Tone:        "blue",
		AvatarKey:   "code-delivery-team",
		Members: []Member{
			{Name: "架构顾问", Role: "系统设计", Task: "拆解模块边界、数据流和接口契约，明确最小可交付架构。", Output: "技术方案与依赖关系", Tone: "blue"},
			{Name: "代码审查专家", Role: "实现质量", Task: "检查关键路径、异常分支、可维护性和可能引入的回归点。", Output: "风险清单与修复优先级", Tone: "violet"},
			{Name: "测试策略专家", Role: "验证覆盖", Task: "设计单测、集成验证和手工验收路径，保证改动可验证。", Output: "测试矩阵与验收步骤", Tone: "green"},
			{Name: "发布风险顾问", Role: "上线控制", Task: "评估配置、兼容性、回滚方案和上线观察指标。", Output: "发布检查表", Tone: "stone"},
		},
		Prompts: []string{
			"请帮我拆解这个功能的交付方案，包括架构、代码质量、测试和风险点。",
			"请从专家团视角评估这个实现能不能进入开发。",
			"请把这个需求拆成可交付的开发任务，并标出优先级。",
		},
		SortOrder: 90,
	},
	{
		ID:          "office-automation-team",
		Mode:        ModeTeam,
		Name:        "办公自动化专家团",
		Role:        "办公自动化",
		Description: "把表格、文档、消息和定时任务串成可复用的办公流程。",
		Categories:  []string{"办公自动化", "表格处理", "报告生成"},
		Tags:        []string{"表格", "文档", "流程"},
		IconKey:     "bot",
		Tone:        "teal",
		AvatarKey:   "office-automation-team",
		Members: []Member{
			{Name: "流程编排专家", Role: "任务流设计", Task: "定义触发条件、执行步骤、人工确认点和失败重试策略。", Output: "自动化流程图", Tone: "teal"},
			{Name: "表格建模专家", Role: "表格处理", Task: "整理字段、公式、校验规则和跨表引用关系。", Output: "表格模板与字段说明", Tone: "amber"},
			{Name: "文档自动化专家", Role: "报告生成", Task: "把数据结果转成固定模板，生成日报、周报和交付说明。", Output: "报告模板结构", Tone: "stone"},
			{Name: "通知集成专家", Role: "消息联动", Task: "设计消息提醒、审批节点和团队协作通知。", Output: "通知规则与异常提醒", Tone: "blue"},
		},
		Prompts: []string{
			"请把这个办公任务设计成自动化流程，包含表格处理、文档生成和消息提醒。",
			"请帮我把日报流程改造成半自动化方案。",
			"请设计一个从数据收集到报告输出的办公自动化流程。",
		},
		SortOrder: 100,
	},
	{
		ID:          "data-analysis-team",
		Mode:        ModeTeam,
		Name:        "数据分析专家团",
		Role:        "数据分析",
		Description: "从指标定义、数据清洗、诊断分析到报告输出形成闭环。",
		Categories:  []string{"数据分析", "表格处理", "财务分析", "报告生成"},
		Tags:        []string{"指标", "清洗", "洞察"},
		IconKey:     "bar-chart-3",
		Tone:        "amber",
		AvatarKey:   "data-analysis-team",
		Members: []Member{
			{Name: "指标体系专家", Role: "指标口径", Task: "定义核心指标、派生指标、维度拆分和口径边界。", Output: "指标字典", Tone: "amber"},
			{Name: "数据质量专家", Role: "数据校验", Task: "检查缺失、异常、重复、分布偏移和口径冲突。", Output: "数据质量报告", Tone: "green"},
			{Name: "分析建模专家", Role: "原因诊断", Task: "拆解趋势、分群、漏斗和贡献度，定位变化原因。", Output: "诊断路径与证据", Tone: "violet"},
			{Name: "报告叙事专家", Role: "结论表达", Task: "把分析结果整理成面向业务决策的结论和行动建议。", Output: "决策报告大纲", Tone: "stone"},
		},
		Prompts: []string{
			"请用专家团方式帮我完成数据分析，先定义指标，再诊断原因，最后输出报告。",
			"请帮我检查这批数据能不能支撑结论，并指出质量问题。",
			"请把这些表格整理成一份业务分析报告。",
		},
		SortOrder: 110,
	},
	{
		ID:          "customer-ops-team",
		Mode:        ModeTeam,
		Name:        "客户运营专家团",
		Role:        "客户服务",
		Description: "把客户问题、服务流程和增长动作整理成可执行方案。",
		Categories:  []string{"客户服务", "运营增长", "报告生成"},
		Tags:        []string{"客户", "服务", "复盘"},
		IconKey:     "users",
		Tone:        "orange",
		AvatarKey:   "customer-ops-team",
		Members: []Member{
			{Name: "客户洞察专家", Role: "需求归因", Task: "归纳客户诉求、高频问题、情绪变化和关键阻塞点。", Output: "客户问题分层", Tone: "orange"},
			{Name: "服务流程专家", Role: "响应效率", Task: "梳理响应路径、升级规则、交付标准和 SLA 风险。", Output: "服务流程优化表", Tone: "teal"},
			{Name: "运营增长顾问", Role: "转化留存", Task: "设计触达策略、转化动作、留存路径和复盘指标。", Output: "运营动作清单", Tone: "amber"},
			{Name: "复盘报告专家", Role: "管理汇报", Task: "沉淀客户问题、改进动作、责任人和下一轮跟踪计划。", Output: "复盘报告", Tone: "stone"},
		},
		Prompts: []string{
			"请帮我把客户反馈整理成服务改进和运营增长方案。",
			"请分析这些客户问题背后的流程短板。",
			"请给我一个客户服务复盘报告框架。",
		},
		SortOrder: 120,
	},
	{
		ID:          "project-delivery-team",
		Mode:        ModeTeam,
		Name:        "项目交付专家团",
		Role:        "项目管理",
		Description: "围绕目标、排期、资源、风险和交付物推进项目落地。",
		Categories:  []string{"项目管理", "报告生成", "办公自动化"},
		Tags:        []string{"计划", "风险", "交付"},
		IconKey:     "flask-conical",
		Tone:        "green",
		AvatarKey:   "project-delivery-team",
		Members: []Member{
			{Name: "项目拆解专家", Role: "范围拆分", Task: "拆分目标、里程碑、任务颗粒度和关键依赖关系。", Output: "WBS 与里程碑", Tone: "green"},
			{Name: "资源排期专家", Role: "计划管理", Task: "评估人力、节奏、并行度和关键路径上的排期风险。", Output: "排期与资源表", Tone: "blue"},
			{Name: "风险控制顾问", Role: "风险治理", Task: "识别延期、质量、沟通和外部依赖风险，并给出缓解动作。", Output: "风险台账", Tone: "orange"},
			{Name: "交付报告专家", Role: "进展汇报", Task: "整理项目状态、阻塞事项、决策请求和下一步计划。", Output: "周报与汇报材料", Tone: "stone"},
		},
		Prompts: []string{
			"请帮我制定这个项目的交付计划，包含里程碑、风险和每周检查点。",
			"请从专家团角度评估这个项目计划是否可执行。",
			"请把当前项目状态整理成一份周报。",
		},
		SortOrder: 130,
	},
	{
		ID:           "finance-expert-team",
		Mode:         ModeTeam,
		Name:         "财务专家团",
		Role:         "财务分析",
		Description:  "汇聚进销存、科目余额和报销数据，穿透分析利润变化、成本异常、发票合规与税务筹划，输出季度经营诊断与决策方案。",
		SystemPrompt: "Finance deliverable rule: after completing a finance expert-team conversation or analysis, you must generate an Excel workbook file (.xlsx) and include the generated file path in the final response. The workbook must contain sheets for diagnosis summary, source-data working papers, audit evidence, tax calculations, option comparison, and action plan. If source data is incomplete, still create the workbook with assumptions, data gaps, and required follow-up fields.",
		Categories:   []string{"财务分析", "数据分析", "报告生成"},
		Tags:         []string{"进销存", "科目余额", "报销审计", "税务筹划"},
		IconKey:      "bar-chart-3",
		Tone:         "amber",
		AvatarKey:    "data-analysis-team",
		Members: []Member{
			{Kind: MemberKindLead, Name: "会计专家", Role: "全维度数据采集", Task: "调取本季度进销存流水、科目余额表和员工报销记录，汇聚跨部门经营数据并统一账务口径。", Output: "经营数据底稿与科目口径表", Tone: "amber", AvatarKey: "data-report"},
			{Kind: MemberKindSub, Name: "审计专家", Role: "穿透核查与风险识别", Task: "对照企业历史数据逐条扫描利润、成本、费用和发票链路，定位利润下滑原因并排查发票合规风险。", Output: "异常清单与审计证据", Tone: "violet", AvatarKey: "research"},
			{Kind: MemberKindSub, Name: "税务专家", Role: "政策匹配与税务筹划", Task: "对照最新国家财税政策评估研发加计扣除、高新技术企业优惠和采购发票结构等合规方案。", Output: "税务优惠测算与合规建议", Tone: "green", AvatarKey: "security-review"},
			{Kind: MemberKindSub, Name: "经营决策专家", Role: "诊断报告与方案推演", Task: "将分析结论整理成季度经营诊断与决策方案，比较方案收益、风险和执行优先级。", Output: "方案对比决策底稿", Tone: "blue", AvatarKey: "doc-report"},
		},
		Prompts: []string{
			"请调取本季度进销存流水、科目余额表和员工报销记录，诊断利润下滑原因并输出季度经营诊断与决策方案，生成Excel给我。",
			"请从审计和税务视角穿透扫描经营数据，排查发票合规风险并给出合规节税方案。",
			"请比较研发加计扣除和采购发票结构调整两套方案，形成对比决策底稿。",
		},
		SortOrder: 140,
	},
}

func Builtin() ([]Expert, error) {
	return expertsFromDefinitions(BuiltinDefinitions())
}

func legacyBuiltinExperts() ([]Expert, error) {
	var experts []Expert
	if err := json.Unmarshal(builtinJSON, &experts); err != nil {
		return nil, fmt.Errorf("parse builtin experts: %w", err)
	}
	return experts, nil
}

func BuiltinCatalog() (*Catalog, error) {
	experts, err := Builtin()
	if err != nil {
		return nil, err
	}
	return NewCatalog(experts)
}

func BuiltinDefinitions() []Definition {
	defs := cloneDefinitions(builtinDefinitions)
	for i := range defs {
		defs[i].Source = SourceOfficial
	}
	experts, err := legacyBuiltinExperts()
	if err != nil {
		return defs
	}
	for i, ex := range experts {
		if hasDefinition(defs, ex.ID) {
			continue
		}
		categories := []string{}
		if ex.Category != "" {
			categories = append(categories, ex.Category)
		}
		mode := ModeExpert
		iconKey := "bot"
		systemPrompt := ex.Prompt
		if ex.ID == "quarterly-business-decision-team" {
			mode = ModeTeam
			iconKey = "users"
			systemPrompt = "协调会计、审计、税务、成本管控和经营分析成员，基于可用数据形成可追溯的季度经营诊断、方案比较与行动计划。"
		}
		defs = append(defs, Definition{
			ID:           ex.ID,
			Source:       SourceOfficial,
			Mode:         mode,
			Name:         ex.Name,
			Role:         ex.Category,
			Description:  ex.Description,
			SystemPrompt: systemPrompt,
			Categories:   categories,
			IconKey:      iconKey,
			Tone:         "stone",
			Members:      legacyBuiltinMembers(ex.ID),
			Prompts:      legacyBuiltinPrompts(ex.ID, ex.Name),
			SortOrder:    len(defs)*10 + (i+1)*10,
		})
	}
	return cloneDefinitions(defs)
}

func legacyBuiltinMembers(id string) []Member {
	if id != "quarterly-business-decision-team" {
		return nil
	}
	return []Member{
		{
			Kind: MemberKindLead, Name: "会计核算专家", Role: "财务数据统筹",
			Task:   "汇总进销存、科目余额和报销记录，统一核算口径并定位账务差异。",
			Output: "季度财务底稿与差异清单", Tone: "amber", AvatarKey: "data-report",
		},
		{
			Kind: MemberKindSub, Name: "审计风控专家", Role: "审计与内控",
			Task:   "核查异常交易、关键流程和证据链，识别审计与内控风险。",
			Output: "审计证据与风险清单", Tone: "violet", AvatarKey: "research",
		},
		{
			Kind: MemberKindSub, Name: "税务合规专家", Role: "税务合规",
			Task:   "评估发票、申报和税务筹划方案的适用条件与合规边界。",
			Output: "税务测算与合规建议", Tone: "green", AvatarKey: "security-review",
		},
		{
			Kind: MemberKindSub, Name: "成本管控专家", Role: "成本优化",
			Task:   "拆解成本结构和异常增长来源，提出不影响核心交付的优化措施。",
			Output: "成本驱动分析与优化清单", Tone: "orange", AvatarKey: "ops-growth",
		},
		{
			Kind: MemberKindSub, Name: "经营分析顾问", Role: "经营决策",
			Task:   "综合各成员结论，比较方案收益、风险和执行条件并形成行动计划。",
			Output: "方案对比与季度行动计划", Tone: "blue", AvatarKey: "doc-report",
		},
	}
}

func IsBuiltinID(id string) bool {
	for _, def := range BuiltinDefinitions() {
		if def.ID == id {
			return true
		}
	}
	return false
}

var legacyBuiltinPresetPrompts = map[string][]string{
	"architecture-review": {
		"请评审当前系统架构，指出关键边界、依赖关系和主要风险。",
		"请比较两种架构方案的复杂度、扩展性和迁移成本。",
		"请给出一份可落地的架构改进清单，并标明优先级。",
	},
	"testing": {
		"请为当前改动设计覆盖核心路径和异常分支的测试方案。",
		"请检查现有测试缺口，并列出最值得补充的用例。",
		"请给出一份从单元测试到验收测试的验证清单。",
	},
	"product": {
		"请把当前想法整理成目标用户、核心价值和功能范围。",
		"请评估这个需求的优先级，并说明收益、成本和风险。",
		"请把需求拆成可验证的最小版本和后续迭代计划。",
	},
	"documentation": {
		"请把当前材料整理成结构清晰、可直接交付的文档。",
		"请生成一份突出结论、风险和下一步的项目报告。",
		"请检查现有文档的信息缺口，并给出补充建议。",
	},
	"accounting-expert": {
		"请核对当前账务处理口径，并指出需要补充的原始凭证。",
		"请分析科目余额和业务流水之间可能存在的差异。",
		"请给出本期结账前需要完成的会计检查清单。",
	},
	"audit-expert": {
		"请从审计视角检查当前流程中的内控薄弱点和证据缺口。",
		"请梳理这组业务数据中的异常线索和核查路径。",
		"请生成一份按风险等级排序的审计检查清单。",
	},
	"tax-compliance-expert": {
		"请检查当前业务安排涉及的主要税务合规风险。",
		"请比较可选税务方案的适用条件、收益和风险。",
		"请列出申报前需要准备的资料和复核事项。",
	},
	"cost-control-expert": {
		"请分析当前成本结构，找出异常增长和主要驱动因素。",
		"请提出不影响核心交付的成本优化措施。",
		"请设计一套可持续跟踪的成本指标和复盘机制。",
	},
	"business-analysis-advisor": {
		"请分析当前经营指标变化的原因并给出行动建议。",
		"请把现有数据整理成管理层可读的经营分析结论。",
		"请比较不同经营方案的收益、风险和执行优先级。",
	},
	"quarterly-business-decision-team": {
		"请综合会计、审计、税务和经营视角诊断本季度经营问题。",
		"请比较本季度可选经营方案的收益、风险和执行条件。",
		"请输出季度经营决策建议、责任分工和下一步行动计划。",
	},
}

func legacyBuiltinPrompts(id, name string) []string {
	if prompts := legacyBuiltinPresetPrompts[id]; len(prompts) > 0 {
		return append([]string(nil), prompts...)
	}
	return []string{
		fmt.Sprintf("请以%s的视角分析当前问题。", name),
		fmt.Sprintf("请以%s的视角指出主要风险和改进建议。", name),
		fmt.Sprintf("请以%s的视角给出可执行的下一步计划。", name),
	}
}

func hasDefinition(defs []Definition, id string) bool {
	for _, def := range defs {
		if def.ID == id {
			return true
		}
	}
	return false
}

func cloneDefinitions(defs []Definition) []Definition {
	out := make([]Definition, len(defs))
	for i, def := range defs {
		out[i] = def
		out[i].Categories = append([]string(nil), def.Categories...)
		out[i].Tags = append([]string(nil), def.Tags...)
		out[i].Members = append([]Member(nil), def.Members...)
		out[i].Prompts = append([]string(nil), def.Prompts...)
	}
	return out
}
