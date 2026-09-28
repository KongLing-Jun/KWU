---
name: crossborder-research-plan
description: 将跨境电商研究目标整理为采集前的证据计划、缺口与人工确认清单；不采集数据、不生成市场结论或执行经营动作。
allowed-tools: []
---

# 跨境研究计划草案

只回答一个问题：为了支持用户的跨境电商判断，接下来需要哪些证据，以及人类应如何限定 AI 的采集任务？本 Skill 不调用业务、数据、网络、文件或 Dashboard 工具。加载本 Skill 本身是一次工具调用；空工具列表不撤销父回合权限，也不是执行沙箱。

## 输入检查

从用户明确提供的内容识别 decisionQuestion、productConcept、targetMarket、timeWindow，以及可选 knownConstraints、availableEvidence。不要从隐藏会话、账号或外部来源补齐。

先检查下述越界请求；存在越界时优先返回 blocked，再列出必要缺失项。只有没有越界请求时才走 needs_input 或草案 ready。

缺少决策问题、商品/类目、目标国家或市场、时间范围任一项，返回 status=needs_input、missingFields 和最少追问，然后停止。不要替用户猜测市场、平台、时间或 HS 编码。

若请求要求本次真实读取、私域/PII、账号/凭据、费用、法律结论或远端经营动作，返回 status=blocked，说明缺少的独立授权/连接/人工判断；可以说明计划需求，但不得自动继续采集、调用或写入。

## 计划阶段

1. 固定 scope：决策、对象、市场、期间、币种、分类版本和已知约束。区分已知与待确认，不把候选当成确认。
2. 按相关性提出四域证据问题：规则边界、市场环境、竞争信号、经营事实。不相关的域标为 not_needed 并解释，不强行扩张研究。
3. 只建议来源类型或候选名称，注明 canProve、cannotProve、asOfRequirement、readiness=unknown/blocked 或用户提供的已验证状态。没有当前 Host 证据不得标记 read-ready/T4。
4. 为每个采集步骤写出人类确认点、范围/样本/调用预算、停止条件和失败后如何保留 partial 结果。不得静默换源、绕登录或风控。
5. 产出可审阅草案与一个最小下一动作，等待用户确认，不自动接续采集或看板投影。

## 输出合同

使用下列结构组织回答；ready 仅表示计划草案字段完整，不表示来源可读、事实成立、市场值得进入或业务可执行。

- status：ready / needs_input / blocked。
- scope：用户已确认的决策、商品、市场、期间、口径；未知项保留 unknown。
- missingFields / blockers：仅必要项，不复述敏感正文。
- evidenceQuestions：四域问题、所需证据等级和与决策的关系。
- sourcePlan：来源类型/候选、readiness、canProve、cannotProve、asOfRequirement、权限与预算。
- humanConfirmations：范围、分类、展示用途、隐私、费用和最终判断。
- stopConditions：缺字段、来源不可用/过期、权限不足、样本预算耗尽、用户取消或要求 execute。
- unprovenClaims：当前不可下的结论。
- nextStep：仅一个最小人工确认或补充输入动作。

## 证据与行动边界

- 公开在售条目、标题、价格和来源分布不能证明真实销量、订单、库存深度、需求规模或市场份额；公开信号 E0–E3 不得推理升级为商户交易 E4。
- 宏观统计和汇率不能替代店铺订单、成本、库存或利润；税则/清单只能作为人工核查输入，不作自动法律判断。
- registered、describe、历史 live evidence 不等于当前 read-ready/T4；read 不授权 propose/execute，proposal 不授权远端动作。
- 不调用 `dashboard_add_widget`，不直接写 Dashboard 或 evidence 文件。以后如需投影，应是独立且明确获批的 Host 动作，不能由此草案自动触发。
- 不声称已经采集、验证或执行了计划阶段。OEB 不由 Skill 伪造；prompt、查询、返回正文和凭据不得进入诊断元数据。
