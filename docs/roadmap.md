# Skynet 待实现模块清单

> 基于 architecture.md 设计文档与当前代码实现的对比整理。
> 按优先级分为 P0 ~ P3 四个等级，每个条目标注当前状态、缺失内容、涉及文件和影响范围。
> 最后更新：2026-04-03

---

## P0 — 骨架能跑通，必须先修

当前代码可以编译运行，但以下问题导致核心流程无法真正使用。

### 1. CLI `dev` / `up` 命令实际功能

| 维度 | 内容 |
|------|------|
| 现状 | `cmd_dev.go` 和 `cmd_up.go` 只打印提示信息，不真正启动 Agent |
| 缺什么 | `dev` 命令应加载 skynet.yaml、创建 Agent 实例、调用 `LocalServer.Start()`；`up` 命令应调用 `agent.Run()` 进入 prod 模式连接 Platform |
| 涉及文件 | `cmd/skynet/cmd_dev.go`、`cmd/skynet/cmd_up.go` |
| 影响 | 开发者安装 CLI 后第一件事就是 `skynet dev`，跑不起来等于框架不可用 |

### 2. Invoke 权限校验

| 维度 | 内容 |
|------|------|
| 现状 | `POST /api/v1/invoke` 直接转发到 Gateway，不检查调用方是否有权访问目标 Skill |
| 缺什么 | Gateway.Invoke() 执行前至少检查：Skill 的 `visibility`（public 放行、private 仅 owner、restricted 查白名单）；调用方身份与目标 Skill 的匹配关系 |
| 涉及文件 | `internal/gateway/service.go`、`internal/store/capability_repo.go` |
| 影响 | 任何认证用户可以调用任何 Agent 的任何 Skill，包括 private 和 restricted 的 |

### 3. Framework 输入校验 + panic recovery

| 维度 | 内容 |
|------|------|
| 现状 | Skill Handler 收到的 input 不做 schema 校验；Handler panic 会崩掉整个 WebSocket 连接 |
| 缺什么 | ① `handleInvoke` 中按 `input_schema` 校验必填字段和类型；② `handleInvoke` 外层加 `defer recover()` 捕获 panic，返回 failed 状态而非断连 |
| 涉及文件 | `pkg/framework/tunnel.go`、`pkg/framework/schema.go` |
| 影响 | 一个格式错误的请求或一个 Handler bug 就能断掉整个 Agent 连接，导致该 Agent 的所有 Skill 不可用 |

---

## P1 — 端到端功能完整，Phase 1/2 承诺的核心特性

### 4. CLI `register` 命令

| 维度 | 内容 |
|------|------|
| 现状 | 注册合并在 WebSocket tunnel 连接流程中，没有独立命令 |
| 缺什么 | `skynet register` 命令：读取 skynet.yaml，通过 HTTP API 或短暂 WebSocket 连接完成注册，验证配置正确后退出 |
| 涉及文件 | 新增 `cmd/skynet/cmd_register.go` |
| 影响 | 开发者无法在不启动 Agent 的情况下验证注册流程 |

### 5. CLI `add skill` 命令

| 维度 | 内容 |
|------|------|
| 现状 | 未实现 |
| 缺什么 | `skynet add skill <name>` 命令：在 `skills/` 目录生成 Skill 模板文件，更新 `skills/skills.go` 注册入口 |
| 涉及文件 | 新增 `cmd/skynet/cmd_add_skill.go`；需要新增嵌入模板 |
| 影响 | 开发者只能手动创建 Skill 文件，脚手架体验不完整 |

### 6. Framework 优雅关闭

| 维度 | 内容 |
|------|------|
| 现状 | `skynetd` 有 graceful shutdown（监听 SIGINT/SIGTERM），但 Agent Framework Runtime 没有 |
| 缺什么 | `Agent.Run()` 中捕获系统信号，等待进行中的 invoke Handler 完成，再关闭 WebSocket 连接和本地 HTTP 服务 |
| 涉及文件 | `pkg/framework/agent.go`、`pkg/framework/tunnel.go`、`pkg/framework/local.go` |
| 影响 | Agent 进程被 kill 时，正在执行的调用会丢失结果，Gateway 侧会超时 |

### 7. 调用历史查询 API

| 维度 | 内容 |
|------|------|
| 现状 | `invocations` 表有写入逻辑，`InvocationRepo` 有 `List()` 方法，但没有 HTTP API 暴露 |
| 缺什么 | `GET /api/v1/invocations` 端点，支持按 `caller_agent_id`、`target_agent_id`、`caller_user_id` 过滤，分页 |
| 涉及文件 | `internal/api/router.go`、新增 `internal/api/handler/invocation_handler.go` |
| 影响 | Dashboard 和 CLI 无法查看调用日志，缺少可观测性 |

### 8. 数据库字段补齐

| 维度 | 内容 |
|------|------|
| 现状 | 多张表缺少架构文档定义的字段 |
| 缺什么 | 新增 migration 补齐以下字段 |
| 涉及文件 | 新增 `migrations/002_add_missing_columns.up.sql` / `down.sql`；更新对应 model |

缺失字段明细：

| 表 | 缺失字段 | 用途 |
|----|----------|------|
| `users` | `avatar_url VARCHAR(512)` | 用户头像（Dashboard、OAuth2） |
| `users` | `auth_provider VARCHAR(50) DEFAULT 'local'` | 认证来源（local/google/github） |
| `agents` | `avatar_url VARCHAR(512)` | Agent 头像（Dashboard、Agent Card） |
| `invocations` | `input_ref VARCHAR(512)` | 调用输入的存储引用（大 payload 分流） |
| `invocations` | `output_ref VARCHAR(512)` | 调用输出的存储引用 |
| `invocations` | `call_chain JSON` | 级联调用链路追踪 |
| `invocations` status 枚举 | 增加 `'input_required'` | 多轮对话中等待用户输入的状态 |

---

## P2 — 产品差异化能力，Phase 2/3 核心特性

### 9. 异步任务 + 多轮对话

| 维度 | 内容 |
|------|------|
| 现状 | 协议只有 invoke/result 两种调用消息；`Skill.MultiTurn` 字段存在但无执行逻辑；`Context` 没有 `NeedInput()` 方法 |
| 缺什么 | 见下方分层说明 |
| 影响 | 这是架构文档的核心卖点之一，缺了它只能做单轮同步调用 |

需要实现的内容：

**协议层** (`pkg/protocol/`)：
- 新增消息类型常量：`need_input`、`reply`、`progress`
- 新增载荷结构体：`NeedInputPayload`（question 字段）、`ReplyPayload`（input 字段）、`ProgressPayload`（progress + message 字段）

**Framework 层** (`pkg/framework/`)：
- `Context` 新增 `NeedInput(question)` 方法，返回特殊结果使框架发送 `need_input` 消息
- `tunnel.go` 的 `readLoop` 处理 `TypeReply` 消息，将追问回复路由到挂起的 Handler
- `handleInvoke` 支持多轮状态管理

**Gateway 层** (`internal/gateway/`)：
- `AgentConn` 支持接收 `need_input` 消息并转发给等待方
- 任务状态管理：submitted → assigned → working → input_required → working → completed/failed
- SSE 推流支持（`GET /api/v1/tasks/:id/stream`）

**API 层** (`internal/api/`)：
- `POST /api/v1/tasks` — 创建异步任务
- `GET /api/v1/tasks/:id` — 查询任务状态
- `POST /api/v1/tasks/:id/reply` — 回复追问
- `GET /api/v1/tasks/:id/stream` — SSE 实时状态流
- `POST /api/v1/tasks/:id/cancel` — 取消任务

**数据库**：
- 新增 `task_messages` 表（多轮对话消息记录）

### 10. 细粒度权限规则

| 维度 | 内容 |
|------|------|
| 现状 | 无 `permission_rules` 表，无权限引擎。`visibility` 和 `approval_mode` 字段存在但无执行逻辑 |
| 缺什么 | 见下方说明 |
| 影响 | `restricted` 可见性无法落地，无法实现白名单控制 |

需要实现的内容：

**数据库**：
- 新增 `permission_rules` 表（agent_id, skill_name, caller_type, caller_id, action, rate_limit 等字段）

**服务层**：
- 新增 `internal/authz/permission.go`：权限规则匹配引擎
  - 输入：caller 身份 + target agent + skill
  - 匹配逻辑：按 priority 排序，逐条匹配 caller_type/caller_id → 返回 allow/deny + approval_mode
  - 无匹配规则时 fallback 到 Skill 的默认 visibility

**API 层**：
- `GET /api/v1/agents/:id/permissions` — 查看权限规则
- `POST /api/v1/agents/:id/permissions` — 创建权限规则
- `PUT /api/v1/agents/:id/permissions/:rule_id` — 更新权限规则
- `DELETE /api/v1/agents/:id/permissions/:rule_id` — 删除权限规则

### 11. 审批流程 (require_approval)

| 维度 | 内容 |
|------|------|
| 现状 | Skill 有 `approval_mode` 字段，但无任何执行逻辑 |
| 缺什么 | 见下方说明 |
| 影响 | 敏感 Skill 无法做人工审批卡点 |

需要实现的内容：

**数据库**：
- 新增 `approval_queue` 表（invocation_id, owner_id, status, decided_at, expires_at）

**Gateway 层**：
- `Invoke()` 识别到 `approval_mode = "manual"` 时：
  - 创建 invocation 记录（status = submitted）
  - 插入 approval_queue 记录（status = pending）
  - 通知 Agent Owner（WebSocket 推送或 Webhook）
  - 返回 `202 Accepted`（task_id + status = pending）
- Owner 审批通过后触发实际调用

**API 层**：
- `GET /api/v1/approvals` — 查看待审批列表（按 owner 过滤）
- `POST /api/v1/approvals/:id` — 审批操作（approve / deny）

### 12. 限流

| 维度 | 内容 |
|------|------|
| 现状 | `skynet.yaml` 有 `rate_limit` 配置（max + window），`permission_rules` 设计中也有限流字段，但 Gateway 不做任何限流检查 |
| 缺什么 | Gateway 调用前按 agent/skill/caller 维度做频率限制 |
| 涉及文件 | `internal/gateway/service.go`，新增 `internal/gateway/rate_limiter.go` |
| 影响 | 单个调用方可以无限打满某个 Agent |

实现方案：
- MVP 阶段：内存滑动窗口计数器（`sync.Map` + 时间窗口）
- 后期：迁移到 Redis 实现分布式限流

### 13. 级联调用 + call_chain 风控

| 维度 | 内容 |
|------|------|
| 现状 | `InvokePayload` 没有 `call_chain` 字段；Gateway 不做深度/环路检查 |
| 缺什么 | 见下方说明 |
| 影响 | Agent A 调 Agent B 调 Agent C 的场景没有安全保障，可能出现无限递归 |

需要实现的内容：

**协议层**：
- `InvokePayload` 新增 `CallChain []string` 字段

**Gateway 层** (`internal/gateway/service.go`)：
- 在 `Invoke()` 中检查：
  1. 链长度 ≤ 3（可配置）→ 超限拒绝
  2. 无环路（call_chain 中无重复 agent_id）→ 有环拒绝
  3. 每一跳独立鉴权（A 能调 B 不代表 B 能调 C）
- call_chain 写入 invocations 表

**Framework 层**：
- 当 Agent 在 Skill Handler 中调用其他 Agent 时，自动将当前 agent_id 追加到 call_chain

---

## P3 — 增强特性，Phase 3/4

### 14. OAuth2 登录 (Google/GitHub)

| 维度 | 内容 |
|------|------|
| 现状 | 只有 email/password 注册登录 |
| 缺什么 | OAuth2 授权码流程、回调端点、用户关联/创建逻辑 |
| 涉及文件 | `internal/authz/service.go`、`internal/authz/handler.go`、`internal/api/router.go` |
| 前置条件 | 数据库 `users` 表补齐 `auth_provider` 字段（P1-8） |
| 影响 | Dashboard 用户体验差，每次都要输密码 |

### 15. 实时事件流

| 维度 | 内容 |
|------|------|
| 现状 | 无 |
| 缺什么 | `WS /api/v1/events` 端点，向已连接的 Dashboard 客户端广播事件：Agent 上线/下线、Agent Card 更新、调用完成等 |
| 涉及文件 | 新增 `internal/api/handler/events_handler.go`；Registry 和 Gateway 中触发事件 |
| 影响 | Dashboard 无法实时刷新 Agent 状态，只能轮询 |

### 16. Agent 评分系统

| 维度 | 内容 |
|------|------|
| 现状 | 无 |
| 缺什么 | `agent_ratings` 表；评分 CRUD API；评分聚合展示（平均分、评分数）；与 invocation 关联（只有真正调用过的用户才能评分） |
| 涉及文件 | 新增 migration、model、repo、handler |
| 影响 | Skill 市场没有质量信号，用户无法判断 Agent 好坏 |

### 17. 语义搜索

| 维度 | 内容 |
|------|------|
| 现状 | 只有 MySQL FULLTEXT 关键词搜索 |
| 缺什么 | `capability_embeddings` 表；Embedding 生成（调用外部 API 如 OpenAI）；向量相似度计算；与关键词搜索结果合并排序 |
| 涉及文件 | 新增 migration、model、`internal/registry/embedding.go` |
| 影响 | 搜索质量受限于关键词匹配，"帮我整理会议纪要"搜不到 "transcribe_and_summarize" |

### 18. Dashboard SPA

| 维度 | 内容 |
|------|------|
| 现状 | 完全未实现 |
| 缺什么 | 完整的前端 SPA |
| 技术选型 | React + TypeScript + Ant Design / shadcn/ui（架构文档已确定）；D3.js 网络拓扑图 |

页面清单：

| 页面 | 功能 |
|------|------|
| 网络概览 | 拓扑图、在线 Agent 数、总 Skill 数、今日调用数 |
| Skill 市场 | 按分类/标签浏览、搜索、查看 Agent Card 详情 |
| Agent 详情 | Skill 列表、在线试用调用（表单输入 → 结果展示）、调用统计 |
| 我的 Agent | 管理自己的 Agent、查看 Skill 状态、配置权限 |
| 调用历史 | 我发起的调用 + 别人对我 Agent 的调用、耗时、结果 |
| 审批队列 | 待审批调用请求（require_approval 模式） |
| 设置 | 账户信息、API Key 管理、通知偏好 |

### 19. CLI 剩余命令

| 命令 | 功能 | 复杂度 |
|------|------|--------|
| `skynet test` | 运行 Skill 单元测试（加载 Agent、逐个调用 Skill、校验输出） | 中 |
| `skynet down` | 从网络下线（调用 API 将 Agent 标记为 offline） | 低 |
| `skynet logs` | 查看调用日志（调用 `GET /api/v1/invocations`） | 低，依赖 P1-7 |
| `skynet export-mcp` | 导出为标准 MCP Server（将 Skill 转为 MCP tool） | 高 |

### 20. Python SDK

| 维度 | 内容 |
|------|------|
| 现状 | 只有 Go Framework |
| 缺什么 | Python 版本的 Agent Framework：配置加载、Skill 定义（装饰器风格）、本地开发服务器、WebSocket 反向隧道 |
| 影响 | AI 开发者群体（大量 Python 用户）无法使用 Skynet 框架 |

---

## 依赖关系

```
P0-1 (dev/up 可用)
 │
 ├─► P0-3 (panic recovery)      ── 独立
 ├─► P1-4 (register 命令)       ── 独立
 ├─► P1-5 (add skill 命令)      ── 独立
 └─► P1-6 (优雅关闭)            ── 独立

P1-8 (DB 字段补齐)
 ├─► P2-9 (异步/多轮)           ── 需要 invocations.input_required 状态
 ├─► P2-13 (级联调用)           ── 需要 invocations.call_chain 字段
 ├─► P3-14 (OAuth2)             ── 需要 users.auth_provider 字段
 └─► P3-18 (Dashboard)          ── 需要 users/agents.avatar_url 字段

P0-2 (invoke 权限校验)
 └─► P2-10 (细粒度权限规则)     ── P0-2 是简化版，P2-10 是完整版
      └─► P2-11 (审批流程)      ── 需要权限引擎判断 approval_mode
           └─► P3-18 (Dashboard 审批页面)

P2-9 (异步/多轮)
 └─► P3-18 (Dashboard 在线试用多轮对话)

P1-7 (调用历史 API)
 └─► P3-19 (skynet logs 命令)
```

---

## 进度跟踪

| 编号 | 模块 | 优先级 | 状态 |
|:----:|------|:------:|:----:|
| 1 | CLI dev/up 实际功能 | P0 | 未开始 |
| 2 | Invoke 权限校验 | P0 | 未开始 |
| 3 | Framework 输入校验 + panic recovery | P0 | 未开始 |
| 4 | CLI register 命令 | P1 | 未开始 |
| 5 | CLI add skill 命令 | P1 | 未开始 |
| 6 | Framework 优雅关闭 | P1 | 未开始 |
| 7 | 调用历史查询 API | P1 | 未开始 |
| 8 | 数据库字段补齐 | P1 | 未开始 |
| 9 | 异步任务 + 多轮对话 | P2 | 未开始 |
| 10 | 细粒度权限规则 | P2 | 未开始 |
| 11 | 审批流程 | P2 | 未开始 |
| 12 | 限流 | P2 | 未开始 |
| 13 | 级联调用 + call_chain | P2 | 未开始 |
| 14 | OAuth2 登录 | P3 | 未开始 |
| 15 | 实时事件流 | P3 | 未开始 |
| 16 | Agent 评分系统 | P3 | 未开始 |
| 17 | 语义搜索 | P3 | 未开始 |
| 18 | Dashboard SPA | P3 | 未开始 |
| 19 | CLI 剩余命令 | P3 | 未开始 |
| 20 | Python SDK | P3 | 未开始 |
