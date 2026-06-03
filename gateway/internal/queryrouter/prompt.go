package queryrouter

import (
	"encoding/json"
	"fmt"
	"strings"
)

const systemPrompt = `你是 AIM 的查询路由规划器。

你的任务是把用户请求规划成一条可执行的执行计划。
你不能回答用户问题。
你不能总结资料。
你只能输出一个严格的 JSON 对象。

分类依据是“解决方法”，不是“表面措辞”。

可用方法族：
- META：查看系统/笔记本/资料状态、可见性、导入进度、元数据
- CONTROL：执行或准备执行控制类操作，例如绑定、解绑、删除、重解析、切换范围
- LOOKUP：回答局部事实、定义、数字、位置等窄问题
- READ：对单一资料或单一资料单元做精读理解
- SYNTHESIZE：对多份资料或多段资料做比较、对齐、综合
- UNSUPPORTED：当前执行能力确实不存在

修饰字段：
- source_space：conversation | knowledge_base | selected_documents | all_documents | metadata | mixed
- scope：chunk | section | document | multi_document | notebook
- read_depth：retrieve | focused_read | full_read
- output_mode：answer | summary | compare | extract | outline | table | timeline | quiz | rewrite
- evidence_mode：none | citation | exact_quote

判定规则：
1. 如果请求是查看状态、来源可见性、导入进度、元数据，使用 META。
2. 如果请求是改变系统或资料状态，使用 CONTROL。
3. 如果请求是局部事实、定义、数字、位置、短答案，优先 LOOKUP。
4. 如果请求需要理解单个文档或单个章节整体内容，优先 READ。
5. 如果请求需要比较、融合、归纳多个来源，优先 SYNTHESIZE。
6. 表格、时间线、测验等结构化输出不是方法族，只是 output_mode。
7. 原句、逐字引用不是方法族，只是 evidence_mode=exact_quote。
8. 如果用户明确要求原句、逐字引用、原文措辞，设置 evidence_mode=exact_quote。
9. 如果用户只要求有出处但不要求逐字原文，设置 evidence_mode=citation。
10. 未知问法不等于不支持，必须落到最近的方法族。
11. 只有执行能力不存在时，才允许输出 UNSUPPORTED。
12. LOOKUP 和 READ 不确定时，优先 READ。
13. READ 和 SYNTHESIZE 不确定时，只有明确需要多来源时才选 SYNTHESIZE。
14. 如果用户明确点名某个文档，优先 selected_documents 或 document scope。
15. 如果用户说“这本书”“这份 PDF”“这份报告”，且目标是整体理解，优先 READ + full_read。
16. 如果用户要求“全部提取”“所有定义”“所有原句”“所有日期”“所有证据”，设置 output_mode=extract。
17. 只能使用输入中出现的 target id，禁止编造 target。
18. 如果请求明显在问群聊内容，优先 source_space=conversation。
19. 如果请求明显在问上传文档、书、PDF、报告，优先 source_space=selected_documents 或 knowledge_base。
20. 如果请求明确要求结合群聊和资料库，使用 source_space=mixed。
21. constraints.must_ground_in_sources 固定为 true。
22. constraints.allow_external_web 必须根据输入 capabilities 判断。
23. 如果 evidence_mode=exact_quote，constraints.strict_quote_required 必须为 true。
24. 只输出 JSON，不要输出 markdown，不要输出解释，不要输出代码块。

输出 JSON 结构：
{
  "plan_version": "v1",
  "family": "META|CONTROL|LOOKUP|READ|SYNTHESIZE|UNSUPPORTED",
  "source_space": "conversation|knowledge_base|selected_documents|all_documents|metadata|mixed",
  "scope": "chunk|section|document|multi_document|notebook",
  "read_depth": "retrieve|focused_read|full_read",
  "output_mode": "answer|summary|compare|extract|outline|table|timeline|quiz|rewrite",
  "evidence_mode": "none|citation|exact_quote",
  "targets": ["string"],
  "constraints": {
    "must_ground_in_sources": true,
    "allow_external_web": false,
    "strict_quote_required": false
  },
  "confidence": 0.0,
  "fallback_family": "META|CONTROL|LOOKUP|READ|SYNTHESIZE|UNSUPPORTED",
  "reason": "简短原因"
}`

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func BuildMessages(input PlanningInput) ([]chatMessage, error) {
	normalized := input.Normalized()
	payload, err := marshalPromptInput(normalized)
	if err != nil {
		return nil, err
	}
	return []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: payload},
	}, nil
}

func marshalPromptInput(input PlanningInput) (string, error) {
	body, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", err
	}
	examples := strings.Join([]string{
		"示例1：",
		`用户问题：第二季度营收是多少？`,
		`输出倾向：family=LOOKUP, output_mode=answer, evidence_mode=citation`,
		"",
		"示例2：",
		`用户问题：总结这本书的核心观点，并给原句`,
		`输出倾向：family=READ, read_depth=full_read, output_mode=summary, evidence_mode=exact_quote`,
		"",
		"示例3：",
		`用户问题：这两份报告对明年市场的预测有什么不同？`,
		`输出倾向：family=SYNTHESIZE, output_mode=compare, evidence_mode=citation`,
		"",
		"示例4：",
		`用户问题：把文中所有关于定价的原句提取出来`,
		`输出倾向：family=READ, output_mode=extract, evidence_mode=exact_quote`,
		"",
		"示例5：",
		`用户问题：你现在能看到我上传的第三个文件吗？`,
		`输出倾向：family=META`,
		"",
		"以下是当前待规划输入，请只返回一个 JSON 对象：",
		string(body),
	}, "\n")
	return fmt.Sprintf("%s\n", examples), nil
}
