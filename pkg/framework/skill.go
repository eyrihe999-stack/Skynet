package framework

// Skill 可见性常量，用于控制 Skill 在 Skynet 网络中的访问权限。
const (
	// Public 表示 Skill 对所有网络中的 Agent 和用户公开可见。
	Public Visibility = "public"
	// Restricted 表示 Skill 仅对经过授权的调用者可见。
	Restricted Visibility = "restricted"
	// Private 表示 Skill 仅对 Agent 自身可见，不对外暴露。
	Private Visibility = "private"
)

// Skill 审批模式常量，用于控制 Skill 调用时是否需要人工审批。
const (
	// AutoApprove 表示 Skill 调用自动通过，无需人工审批。
	AutoApprove ApprovalMode = "auto"
	// ManualApprove 表示 Skill 调用需要人工审批后才能执行。
	ManualApprove ApprovalMode = "manual"
)

// Visibility 是 Skill 可见性的类型别名，取值为 Public、Restricted 或 Private。
// 它决定了 Skill 在 Skynet 网络中的访问控制策略。
type Visibility string

// ApprovalMode 是 Skill 审批模式的类型别名，取值为 AutoApprove 或 ManualApprove。
// 它决定了 Skill 被调用时是否需要经过人工审批流程。
type ApprovalMode string

// Skill 定义了 Agent 对外暴露的一个能力（技能）单元。
//
// Skill 是 Skynet Agent 框架的核心概念，每个 Skill 代表 Agent 的一项具体能力。
// 开发者通过定义 Skill 结构体并实现 Handler 函数来构建 Agent 的业务逻辑。
// Skill 注册到 Agent 后，会自动生成对应的 Agent Card 条目，并通过网络或本地服务器对外提供服务。
//
// 字段说明：
//   - Name: Skill 的唯一标识名称，用于路由和调用（必填）
//   - DisplayName: Skill 的显示名称，用于 UI 展示
//   - Description: Skill 的功能描述，说明该技能的用途
//   - Category: Skill 的分类标签，用于归类和检索
//   - Tags: Skill 的标签列表，用于搜索和过滤
//   - Input: Skill 的输入参数 Schema 定义，描述输入数据的结构
//   - Output: Skill 的输出结果 Schema 定义，描述输出数据的结构
//   - Visibility: Skill 的可见性级别（public/restricted/private）
//   - ApprovalMode: Skill 的审批模式（auto/manual）
//   - EstimatedLatencyMs: Skill 的预估执行延迟（毫秒），用于调用方做超时和调度决策
//   - Handler: Skill 的处理函数，接收上下文和输入参数，返回执行结果（必填）
type Skill struct {
	Name               string
	DisplayName        string
	Description        string
	Category           string
	Tags               []string
	Input              Schema
	Output             Schema
	Visibility         Visibility
	ApprovalMode       ApprovalMode
	EstimatedLatencyMs uint
	Handler            HandlerFunc
}

// HandlerFunc 是 Skill 处理函数的签名类型。
//
// 每个 Skill 都必须实现一个符合该签名的处理函数。当 Skill 被调用时，
// 框架会构造 Context（执行上下文）和 Input（输入参数），传入 Handler 执行业务逻辑。
//
// 参数：
//   - ctx: Skill 执行上下文，包含请求 ID、调用者信息等元数据
//   - input: 经过解析的输入参数访问器，提供类型安全的字段访问方法
//
// 返回值：
//   - any: Skill 的执行结果，将被序列化为 JSON 返回给调用方
//   - error: 执行过程中的错误，非 nil 时表示执行失败
type HandlerFunc func(ctx Context, input Input) (any, error)
