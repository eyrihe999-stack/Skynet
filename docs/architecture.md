# Skynet — Agent Network Architecture

> Agent 开发框架 + 互联网络。
> 提供标准化的 Agent 开发脚手架，让开发者快速构建自己的 Agent 并一键接入网络。
> 网络中的所有参与者可以发现、浏览并调用其他 Agent 的能力。

---

## 1. 系统全景

```
  开发者本地                                Skynet Platform
  ─────────                                ────────────────
                                  ┌──────────────────────────────────────────────┐
  ┌─────────────┐                 │                                              │
  │ skynet CLI  │   skynet init   │                                              │
  │             │   skynet dev    │                                              │
  │ 创建项目     │   skynet test   │                                              │
  │ 开发 Skill  │                 │                                              │
  └──────┬──────┘                 │                                              │
         │                        │                                              │
         │ skynet register        │                                              │
         ▼                        │                                              │
  ┌─────────────┐   WebSocket     │  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
  │ Agent       │   反向通道       │  │ Registry │  │ Gateway  │  │   AuthZ   │  │
  │ (Framework  │ ◄══════════════►│  │ Service  │  │ Service  │  │  Service  │  │
  │  Runtime)   │                 │  └────┬─────┘  └────┬─────┘  └─────┬─────┘  │
  └─────────────┘                 │       │             │              │         │
                                  │       │        ┌────▼──────┐       │         │
  ┌─────────────┐   WebSocket     │       │        │  Routing  │◄──────┘         │
  │ Agent B     │ ◄══════════════►│       │        │  Engine   │                 │
  │ (另一个开发者)│                 │       │        └────┬──────┘                 │
  └─────────────┘                 │       │             │                        │
                                  │  ┌────▼─────────────▼──────┐                │
  ┌─────────────┐                 │  │         MySQL            │                │
  │   Browser   │ ────────────►   │  │  (cards, perms, logs)    │                │
  │   (User)    │ ◄────────────   │  └──────────┬───────────────┘                │
  └─────────────┘                 │             │                                │
                                  │  ┌──────────▼───────────────┐                │
                                  │  │  Object Storage (可选)    │                │
                                  │  │  大 payload 存储          │                │
                                  │  └──────────────────────────┘                │
                                  │                                              │
                                  │  ┌──────────────────────────┐                │
                                  │  │      Dashboard (SPA)     │                │
                                  │  └──────────────────────────┘                │
                                  └──────────────────────────────────────────────┘
```

关键设计变更（相比纯 SDK 方案）：

| 维度 | 旧方案 (SDK) | 新方案 (Framework) |
|------|-------------|-------------------|
| 接入方式 | 用户已有 Agent，用 SDK 包装 | 用户用 CLI 创建项目，基于框架开发 |
| 网络连接 | Agent 需要公网 endpoint | Agent 主动通过 WebSocket 反向通道连接 Platform |
| 协议 | Gateway 需要适配 MCP/REST/gRPC | 统一使用 Skynet 内部协议，Gateway 大幅简化 |
| Agent Card | 手动填写 | 从 `skynet.yaml` + Skill 定义自动生成 |

---

## 2. 核心模块拆解

### 2.1 模块总览

| 模块 | 职责 | 对外接口 |
|------|------|----------|
| **Agent Framework** | Agent 开发脚手架 + 运行时 | Go / Python 包 |
| **Skynet CLI** | 项目创建、本地开发、注册上线 | 命令行工具 |
| **Registry Service** | Agent 注册、注销、心跳、能力索引、发现 | REST + WebSocket |
| **Gateway Service** | 调用路由、反向通道管理、限流、审计 | REST + SSE + WebSocket |
| **AuthZ Service** | 身份认证、权限策略、Token 签发 | REST (内部) |
| **Dashboard** | 可视化界面：Agent 浏览、能力搜索、调用触发、管理 | SPA |
| **MySQL** | 持久化存储 | 内部 |

---

### 2.2 Agent Framework — 开发框架

#### 2.2.1 CLI 命令集

```
skynet init <name>              创建新 Agent 项目（生成脚手架代码）
skynet add skill <name>         添加一个 Skill 模板文件
skynet dev                      本地开发模式（不连网络，本地 HTTP 测试）
skynet test                     运行 Skill 的单元测试
skynet register                 注册到网络（首次需输入网络地址 + API Key）
skynet up                       启动 Agent 并连接网络（注册 + 心跳 + 监听调用）
skynet down                     从网络下线
skynet status                   查看当前 Agent 在网络上的状态
skynet invoke <agent> <skill>   从命令行调用网络上其他 Agent
skynet logs                     查看调用日志
skynet export-mcp               导出为标准 MCP Server（兼容其他生态）
```

#### 2.2.2 项目结构

```
skynet init my-agent 生成：

my-agent/
├── skynet.yaml              # Agent 配置
├── skills/
│   ├── skills.go            # Skill 注册入口
│   └── hello.go             # 示例 Skill
├── main.go                  # 入口
├── go.mod
└── go.sum
```

#### 2.2.3 skynet.yaml 配置

```yaml
agent:
  id: alice-legal-assistant         # 全局唯一标识
  display_name: Alice 的法律助手
  description: 擅长合同审查、法规查询
  version: 1.0.0
  avatar_url: ""                    # 可选

network:
  registry: https://skynet.example.com
  api_key: ${SKYNET_API_KEY}        # 从环境变量读取

server:
  port: 9100                        # 本地监听端口（dev 模式用）

defaults:
  visibility: public                # Skill 默认可见性
  approval_mode: auto               # 默认审批模式
  rate_limit:
    max: 100
    window: 1h

data_policy:
  store_input: false                # 是否允许平台存储调用输入
  store_output: false               # 是否允许平台存储调用输出
  retention: 0                      # 数据保留时长（小时），0 = 不保留
```

#### 2.2.4 Skill 定义方式

```go
// skills/review_contract.go
package skills

import "github.com/user/skynet"

var ReviewContract = skynet.Skill{
    Name:        "review_contract",
    DisplayName: "合同审查",
    Description: "审查合同条款，标注风险点并给出修改建议",
    Category:    "legal",
    Tags:        []string{"合同", "法律", "风险"},

    // 声明式 Schema，框架自动转为 JSON Schema
    Input: skynet.Schema{
        "contract_text": skynet.String("合同全文").Required(),
        "focus_areas":   skynet.StringArray("重点关注领域"),
    },
    Output: skynet.Schema{
        "risk_level":  skynet.Enum("风险等级", "low", "medium", "high"),
        "issues":      skynet.Array("发现的问题"),
        "suggestions": skynet.Array("修改建议"),
    },

    // 可选：覆盖全局配置
    Visibility:   skynet.Public,
    ApprovalMode: skynet.AutoApprove,

    // 处理函数
    Handler: func(ctx skynet.Context, input skynet.Input) (any, error) {
        text := input.String("contract_text")
        areas := input.StringArray("focus_areas")

        result := analyzeContract(text, areas)

        return map[string]any{
            "risk_level":  result.RiskLevel,
            "issues":      result.Issues,
            "suggestions": result.Suggestions,
        }, nil
    },
}
```

#### 2.2.5 多轮对话 Skill

对于需要追问的复杂场景，Skill 可以声明为多轮模式：

```go
var DeepAnalysis = skynet.Skill{
    Name:        "deep_analysis",
    DisplayName: "深度分析",
    MultiTurn:   true,          // 标记为多轮 Skill

    Handler: func(ctx skynet.Context, input skynet.Input) (any, error) {
        text := input.String("document")

        // 需要追问 → 返回 NeedInput
        if !input.Has("jurisdiction") {
            return ctx.NeedInput(skynet.Question{
                Field:       "jurisdiction",
                Prompt:      "这份文件适用哪个法域？",
                Options:     []string{"中国大陆", "美国", "欧盟", "其他"},
            }), nil
        }

        // 所有信息已收集，执行分析
        jurisdiction := input.String("jurisdiction")
        return doAnalysis(text, jurisdiction), nil
    },
}
```

#### 2.2.6 main.go 入口

```go
package main

import (
    "my-agent/skills"
    "github.com/user/skynet"
)

func main() {
    agent := skynet.New()           // 读取 skynet.yaml

    agent.Register(skills.ReviewContract)
    agent.Register(skills.DeepAnalysis)

    agent.Run()
    // 内部流程：
    // 1. 从 Skill 定义自动生成 Agent Card
    // 2. WebSocket 连接 Gateway（反向通道）
    // 3. 向 Registry 注册 Agent Card
    // 4. 维持心跳
    // 5. 通过反向通道接收调用请求
    // 6. 路由到对应 Skill Handler 执行
    // 7. 返回结果
}
```

#### 2.2.7 框架自动处理的事情

| 职责 | 说明 |
|------|------|
| Agent Card 生成 | 从 `skynet.yaml` + 所有 Skill 定义自动组装 |
| Schema 转换 | `skynet.Schema` → JSON Schema（兼容 MCP） |
| 输入校验 | 按 Schema 自动校验必填字段、类型 |
| 输出校验 | 按 Schema 自动校验返回值结构 |
| 网络连接 | WebSocket 反向通道建立与重连 |
| 心跳 | 自动每 30s 上报 |
| 请求签名验证 | 确认调用来自 Gateway，防伪造 |
| 超时控制 | 默认 30s，可配置 |
| 错误包装 | 将 panic / error 统一包装为标准错误响应 |
| 调用计量 | 本地统计调用次数、延迟 |
| 优雅关闭 | SIGTERM 时完成进行中的调用再退出 |

---

### 2.3 Registry Service — 注册中心

#### 2.3.1 职责

- 接收 Agent 注册，存储 Agent Card（由框架自动生成并提交）
- 维护 Agent 在线状态（心跳 / WebSocket 长连接）
- 提供能力搜索与 Agent 发现
- 向已连接的客户端广播 Agent 上下线事件

#### 2.3.2 Agent Card 数据模型

Agent Card 由框架从 `skynet.yaml` + Skill 定义自动生成，开发者不需要手动编写：

```json
{
  "agent_id": "alice-legal-assistant",
  "owner_id": "user_alice_001",
  "display_name": "Alice 的法律助手",
  "description": "擅长合同审查、法规查询、合规分析",
  "avatar_url": "https://...",
  "version": "1.0.0",
  "connection_mode": "tunnel",
  "capabilities": [
    {
      "name": "review_contract",
      "display_name": "合同审查",
      "description": "审查合同条款，标注风险点并给出修改建议",
      "category": "legal",
      "tags": ["合同", "风险", "法律"],
      "multi_turn": false,
      "input_schema": {
        "type": "object",
        "properties": {
          "contract_text": { "type": "string", "description": "合同全文" },
          "focus_areas": {
            "type": "array",
            "items": { "type": "string" },
            "description": "重点关注领域"
          }
        },
        "required": ["contract_text"]
      },
      "output_schema": {
        "type": "object",
        "properties": {
          "risk_level": { "type": "string", "enum": ["low", "medium", "high"] },
          "issues": { "type": "array" },
          "suggestions": { "type": "array" }
        }
      },
      "estimated_latency_ms": 5000,
      "visibility": "public",
      "approval_mode": "auto"
    }
  ],
  "data_policy": {
    "store_input": false,
    "store_output": false,
    "retention_hours": 0
  },
  "metadata": {
    "framework_version": "0.1.0",
    "created_at": "2026-04-01T10:00:00Z",
    "updated_at": "2026-04-02T08:30:00Z"
  }
}
```

#### 2.3.3 API 设计

```
POST   /api/v1/agents                        注册 Agent（提交 Agent Card）
PUT    /api/v1/agents/{agent_id}             更新 Agent Card
DELETE /api/v1/agents/{agent_id}             注销 Agent
GET    /api/v1/agents                        列出 Agent（支持过滤、分页）
GET    /api/v1/agents/{agent_id}             获取 Agent 详情（含 Card）
POST   /api/v1/agents/{agent_id}/heartbeat   心跳上报

GET    /api/v1/capabilities                  按能力搜索（关键词 + 语义）
GET    /api/v1/categories                    获取能力分类列表

WS     /api/v1/events                        实时事件流（上下线、Card 更新）
```

#### 2.3.4 能力发现：关键词 + 语义搜索

```
用户搜索: "帮我把会议录音整理成纪要"

Step 1: MySQL FULLTEXT 关键词匹配
  → 匹配 display_name / description 中含"会议""录音""纪要"的 Skill

Step 2: 语义向量匹配（补充关键词搜不到的结果）
  → 将查询文本做 embedding
  → 与预计算的 capability embedding 做余弦相似度
  → 匹配到 "transcribe_and_summarize" 等语义相关的 Skill

Step 3: 合并排序
  → 关键词匹配优先，语义匹配补充
  → 按相关性 + Agent 在线状态 + 调用成功率排序

实现方式：
  - Embedding: 调用外部 API (OpenAI / 本地模型)
  - 向量存储: 单独的 capability_embeddings 表 (BLOB 存向量)
  - 检索: 应用层计算余弦相似度（MVP 阶段数据量小，够用）
  - 后期: 迁移到专用向量索引 (Faiss / pgvector)
```

#### 2.3.5 Agent 状态机

```
                 register
    ┌───────┐ ──────────► ┌────────┐
    │ UNKNOWN│             │ ONLINE │◄──── heartbeat (每 30s)
    └───────┘             └───┬────┘      或 WebSocket 保活
                              │
                    miss 3 heartbeats (90s)
                    或 WebSocket 断开
                              │
                          ┌───▼─────┐       reconnect
                          │ OFFLINE │ ─────────────────► ONLINE
                          └───┬─────┘
                              │
                         unregister
                              │
                          ┌───▼─────┐
                          │ REMOVED │
                          └─────────┘
```

---

### 2.4 Gateway Service — 调用网关

#### 2.4.1 职责

- 管理 Agent 的 WebSocket 反向通道
- 接收调用请求，校验权限
- 通过反向通道将请求路由到目标 Agent
- 同步/异步/多轮调用支持
- 调用链追踪（tracing）
- 审计日志
- 限流与熔断

#### 2.4.2 反向通道（Tunnel）

核心设计：Agent 不需要公网 IP，主动通过 WebSocket 连接 Gateway。

```
Agent (NAT 后面)                          Gateway
     │                                       │
     │  1. WebSocket 连接                     │
     │  wss://gateway/api/v1/tunnel           │
     │  Headers: Agent-ID + HMAC 签名         │
     │──────────────────────────────────────►  │
     │                                        │
     │  2. 连接建立，Gateway 记录：             │
     │     agent_id → ws_conn 映射             │
     │                                        │
     │  3. 有调用到达时，Gateway 通过 WS 推送   │
     │  ◄──────────────────────────────────── │
     │  { "type": "invoke",                   │
     │    "request_id": "xxx",                │
     │    "skill": "review_contract",         │
     │    "input": { ... } }                  │
     │                                        │
     │  4. Agent 执行完毕，通过 WS 返回结果    │
     │  ──────────────────────────────────►    │
     │  { "type": "result",                   │
     │    "request_id": "xxx",                │
     │    "output": { ... } }                 │
     │                                        │
     │  5. 心跳 (每 30s 双向 ping/pong)        │
     │  ◄────────────────────────────────────►│
```

断线重连策略：
- 断开后立即重连，前 3 次间隔 1s
- 之后指数退避：2s → 4s → 8s → 最大 60s
- 重连后自动重新注册 Agent Card

#### 2.4.3 Skynet 内部协议

由于所有 Agent 都使用框架，Gateway 与 Agent 之间使用统一的 Skynet 协议，不需要多协议适配器。

```
WebSocket 消息格式 (JSON):

// 调用请求 (Gateway → Agent)
{
  "type": "invoke",
  "request_id": "req_xxxx",
  "skill": "review_contract",
  "input": { "contract_text": "..." },
  "caller": {
    "agent_id": "bob-research-agent",       // 可选，agent 调用时
    "user_id": "user_bob_001",
    "display_name": "Bob"
  },
  "call_chain": ["bob-research-agent"],      // 级联调用链
  "timeout_ms": 30000
}

// 调用结果 (Agent → Gateway)
{
  "type": "result",
  "request_id": "req_xxxx",
  "status": "completed",                    // completed / failed
  "output": { "risk_level": "high", ... },
  "error": null
}

// 多轮追问 (Agent → Gateway)
{
  "type": "need_input",
  "request_id": "req_xxxx",
  "task_id": "task_xxxx",
  "question": {
    "field": "jurisdiction",
    "prompt": "这份文件适用哪个法域？",
    "options": ["中国大陆", "美国", "欧盟"]
  }
}

// 追问回复 (Gateway → Agent)
{
  "type": "reply",
  "request_id": "req_xxxx",
  "task_id": "task_xxxx",
  "input": { "jurisdiction": "中国大陆" }
}

// 进度更新 (Agent → Gateway)
{
  "type": "progress",
  "request_id": "req_xxxx",
  "progress": 0.6,
  "message": "正在分析第三章..."
}

// 心跳
{ "type": "ping" }
{ "type": "pong" }
```

#### 2.4.4 对外 API 设计

```
# 同步调用
POST /api/v1/invoke
{
  "target_agent": "alice-legal-assistant",
  "skill": "review_contract",
  "input": { "contract_text": "..." },
  "timeout_ms": 30000
}
→ 200 { "output": { "risk_level": "high", ... } }

# 异步调用（长任务 / 多轮）
POST /api/v1/tasks
{
  "target_agent": "alice-legal-assistant",
  "skill": "deep_analysis",
  "input": { "document": "..." },
  "mode": "async"
}
→ 202 { "task_id": "task_xxxx", "status": "submitted" }

# 查询任务状态
GET /api/v1/tasks/{task_id}
→ 200 {
    "task_id": "task_xxxx",
    "status": "input_required",
    "question": { "field": "jurisdiction", "prompt": "..." }
  }

# 回复追问
POST /api/v1/tasks/{task_id}/reply
{
  "input": { "jurisdiction": "中国大陆" }
}
→ 200 { "task_id": "task_xxxx", "status": "working" }

# 任务状态 SSE 流
GET /api/v1/tasks/{task_id}/stream
→ SSE: { "status": "working", "progress": 0.6 }
→ SSE: { "status": "input_required", "question": { ... } }
→ SSE: { "status": "completed", "output": { ... } }

# 取消任务
POST /api/v1/tasks/{task_id}/cancel

# 调用历史
GET /api/v1/invocations?caller={agent_id}&target={agent_id}&limit=50

# Agent 反向通道
WS  /api/v1/tunnel   Agent 框架运行时连接此端点
```

#### 2.4.5 任务状态机（扩展自 A2A Task Model，增加多轮支持）

```
  ┌───────────┐  Gateway 收到   ┌──────────┐  Agent 处理中   ┌──────────┐
  │ submitted │ ──────────────► │ assigned │ ──────────────► │ working  │
  └───────────┘                 └──────────┘                 └────┬─────┘
                                                                  │
                                               ┌──────────────────┼─────────────────┐
                                               │                  │                 │
                                          ┌────▼──────────┐  ┌────▼─────┐     ┌─────▼────┐
                                          │input_required │  │completed │     │  failed  │
                                          │(Agent 追问)    │  └──────────┘     └──────────┘
                                          └────┬──────────┘
                                               │
                                          caller 回复
                                               │
                                          ┌────▼─────┐
                                          │ working  │ (继续处理)
                                          └──────────┘
```

#### 2.4.6 级联调用风控

```
每次调用携带 call_chain：

Agent A 调用 Agent B:
  call_chain: ["agent-a"]

Agent B 内部又调用 Agent C:
  call_chain: ["agent-a", "agent-b"]

Gateway 在每一跳检查：
  1. 链长度 ≤ 3（可配置）           → 超限则拒绝
  2. 无环路（chain 中无重复 ID）    → 有环则拒绝
  3. 每一跳独立鉴权                 → A 能调 B 不代表 A→B→C 中 B 能调 C
  4. call_chain 记入审计日志        → 可追溯完整调用路径
```

---

### 2.5 AuthZ Service — 认证授权

#### 2.5.1 身份体系

```
┌──────────┐  1:N  ┌──────────┐
│   User   │ ─────►│  Agent   │
│ (Owner)  │       │(注册的)   │
└──────────┘       └──────────┘

User 认证方式:
  - OAuth2 (Google / GitHub) — Dashboard 登录
  - API Key — CLI / 程序化访问

Agent 认证方式:
  - agent_id + agent_secret — 反向通道建立时 HMAC 签名
  - Gateway 签发的短期 JWT — 级联调用时传递身份
```

#### 2.5.2 权限模型

三层权限控制：

```
┌──────────────────────────────────────────────────────────────────┐
│                       Permission Layers                          │
│                                                                  │
│  Layer 1: Network Membership                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ 谁能加入网络                                                │  │
│  │ - 开放注册 / 邀请制 / 审批制                                │  │
│  │ - 网络级黑名单                                              │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Layer 2: Skill Visibility                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Agent Owner 定义每个 Skill 的可见性                         │  │
│  │ - public: 所有网络成员可见可调用                             │  │
│  │ - restricted: 仅白名单可见可调用                             │  │
│  │ - private: 仅 Owner 自己可见                                │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Layer 3: Invocation Control                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ 调用时的实时控制                                            │  │
│  │ - auto_approve: 满足 Layer 2 即放行                         │  │
│  │ - require_approval: 每次调用需 Owner 人工审批               │  │
│  │ - rate_limit: 调用频率限制                                  │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

#### 2.5.3 权限规则数据结构

```json
{
  "agent_id": "alice-legal-assistant",
  "rules": [
    {
      "skill": "review_contract",
      "visibility": "public",
      "invocation_policy": "auto_approve",
      "rate_limit": { "max_calls": 100, "window": "1h" }
    },
    {
      "skill": "internal_case_search",
      "visibility": "restricted",
      "allowed_callers": ["bob-research-agent", "carol-compliance-agent"],
      "invocation_policy": "require_approval",
      "rate_limit": { "max_calls": 10, "window": "1d" }
    }
  ]
}
```

#### 2.5.4 审批流程（require_approval 模式）

```
  Caller Agent           Gateway              Owner (Alice)
       │                    │                       │
       │  POST /invoke      │                       │
       │───────────────────►│                       │
       │                    │   Push notification    │
       │                    │  (WebSocket/邮件/Webhook)
       │                    │──────────────────────►│
       │                    │                       │
       │  202 pending       │                       │
       │◄───────────────────│                       │
       │                    │   approve / deny      │
       │                    │◄──────────────────────│
       │                    │                       │
       │                    │──► 执行调用 / 拒绝     │
       │  result / denied   │                       │
       │◄───────────────────│                       │
```

---

### 2.6 Dashboard — 可视化界面

#### 2.6.1 页面结构

```
┌──────────────────────────────────────────────────────────────────────┐
│  Skynet Dashboard                              [Alice] [Settings]    │
├───────────┬──────────────────────────────────────────────────────────┤
│           │                                                          │
│  网络概览  │   Agent Network Overview                                │
│           │   ┌──────────────────────────────────────────────────┐   │
│  Skill    │   │          网络拓扑可视化 (Force Graph)              │   │
│  市场     │   │     ○ Alice ──── ○ Bob                           │   │
│           │   │       \          /                                │   │
│  我的     │   │        ○ Carol ○                                 │   │
│  Agent    │   │          \   /                                   │   │
│           │   │           ○ Dave                                  │   │
│  调用     │   └──────────────────────────────────────────────────┘   │
│  历史     │                                                          │
│           │   在线 Agent: 42    总 Skill: 186    今日调用: 1,204     │
│  审批     │                                                          │
│  队列     │   ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│           │   │ 法律     │ │ 数据分析 │ │ 设计     │ │ 开发     │   │
│  设置     │   │ 12 agents│ │ 8 agents │ │ 5 agents │ │ 15 agents│   │
│           │   └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
│           │                                                          │
└───────────┴──────────────────────────────────────────────────────────┘
```

#### 2.6.2 核心页面

| 页面 | 功能 |
|------|------|
| **网络概览** | 拓扑图、统计指标、最近活跃 Agent |
| **Skill 市场** | 按分类/标签浏览、语义搜索、查看 Agent Card 详情、数据隐私策略展示 |
| **Agent 详情** | Skill 列表、输入表单、在线试用调用（含多轮对话）、调用统计、评分 |
| **我的 Agent** | 管理自己注册的 Agent、查看 Skill 状态、配置权限规则 |
| **调用历史** | 我发起的调用 + 别人对我 Agent 的调用、耗时、结果、调用链路 |
| **审批队列** | 待审批的调用请求（require_approval 模式下） |
| **设置** | 账户、API Key 管理、通知偏好、数据隐私偏好 |

---

## 3. 数据库设计 (MySQL)

### 3.1 ER 图

```
┌──────────────┐     ┌──────────────────┐     ┌───────────────────┐
│    users     │     │     agents       │     │   capabilities    │
├──────────────┤     ├──────────────────┤     ├───────────────────┤
│ id (PK)      │◄───┐│ id (PK)          │◄───┐│ id (PK)           │
│ email        │    ││ owner_id (FK)    │    ││ agent_id (FK)     │
│ display_name │    ││ agent_id (UK)    │    ││ name              │
│ avatar_url   │    ││ display_name     │    ││ display_name      │
│ auth_provider│    ││ description      │    ││ description       │
│ api_key_hash │    ││ avatar_url       │    ││ category          │
│ status       │    ││ connection_mode  │    ││ tags (JSON)       │
│ created_at   │    ││ agent_secret_hash│    ││ input_schema (JSON)│
│ updated_at   │    ││ data_policy(JSON)│    ││ output_schema(JSON)│
└──────────────┘    ││ status           │    ││ visibility        │
                    ││ last_heartbeat   │    ││ approval_mode     │
                    ││ framework_version│    ││ multi_turn        │
                    ││ version          │    ││ estimated_latency │
                    ││ created_at       │    ││ call_count        │
                    ││ updated_at       │    ││ success_rate      │
                    │└──────────────────┘    ││ avg_latency_ms    │
                    │                        ││ created_at        │
                    │                        ││ updated_at        │
                    │                        │└───────────────────┘
                    │                        │
                    │┌──────────────────┐    │  ┌────────────────────────┐
                    ││ permission_rules │    │  │ capability_embeddings  │
                    │├──────────────────┤    │  ├────────────────────────┤
                    ││ id (PK)          │    │  │ capability_id (FK, PK) │
                    ││ agent_id (FK)    │────┘  │ embedding (BLOB)       │
                    ││ skill_name       │       │ model_version          │
                    ││ caller_type      │       │ updated_at             │
                    ││ caller_id        │       └────────────────────────┘
                    ││ action           │
                    ││ approval_mode    │
                    ││ rate_limit_max   │
                    ││ rate_limit_window│
                    ││ priority         │
                    ││ created_at       │
                    │└──────────────────┘
                    │
                    │┌──────────────────┐
                    ││  invocations     │ (调用审计日志，按月分区)
                    │├──────────────────┤
                    ││ id (PK)          │
                    ││ task_id (UK)     │
                    ││ caller_agent_id  │
                    ││ caller_user_id   │
                    ││ target_agent_id  │
                    ││ skill_name       │
                    ││ input_ref        │  ← 大 payload 存对象存储，此处存引用
                    ││ output_ref       │  ← 同上
                    ││ call_chain (JSON)│  ← 级联调用链路
                    ││ status           │
                    ││ mode             │
                    ││ error_message    │
                    ││ latency_ms       │
                    ││ created_at       │
                    ││ completed_at     │
                    │└──────────────────┘
                    │
                    │┌──────────────────┐
                    ││  task_messages   │ (多轮对话消息)
                    │├──────────────────┤
                    ││ id (PK)          │
                    ││ task_id (FK)     │
                    ││ direction        │  (to_agent / from_agent)
                    ││ message_type     │  (input / output / question / reply / progress)
                    ││ payload_ref      │  ← 大 payload 存对象存储
                    ││ created_at       │
                    │└──────────────────┘
                    │
                    │┌──────────────────┐
                    ││ approval_queue   │
                    │├──────────────────┤
                    ││ id (PK)          │
                    ││ invocation_id(FK)│
                    ││ owner_id (FK)    │
                    ││ status           │
                    ││ decided_at       │
                    ││ expires_at       │
                    ││ created_at       │
                    │└──────────────────┘
                    │
                    │┌──────────────────┐
                    ││ agent_ratings    │ (信誉系统)
                    │├──────────────────┤
                    ││ id (PK)          │
                    ││ agent_id (FK)    │
                    ││ skill_name       │
                    ││ rater_user_id(FK)│
                    ││ score            │  (1-5)
                    ││ comment          │
                    ││ invocation_id    │  ← 关联到具体调用
                    ││ created_at       │
                    │└──────────────────┘
```

### 3.2 核心建表 SQL

```sql
-- ============================================================
-- 用户表
-- ============================================================
CREATE TABLE users (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    display_name    VARCHAR(100) NOT NULL,
    avatar_url      VARCHAR(512) DEFAULT NULL,
    auth_provider   VARCHAR(50) NOT NULL DEFAULT 'local',
    password_hash   VARCHAR(255) DEFAULT NULL,
    api_key_hash    VARCHAR(255) DEFAULT NULL,
    status          ENUM('active', 'suspended', 'deleted') NOT NULL DEFAULT 'active',
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- Agent 表
-- ============================================================
CREATE TABLE agents (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL UNIQUE,
    owner_id            BIGINT UNSIGNED NOT NULL,
    display_name        VARCHAR(200) NOT NULL,
    description         TEXT,
    avatar_url          VARCHAR(512) DEFAULT NULL,
    connection_mode     ENUM('tunnel', 'direct') NOT NULL DEFAULT 'tunnel',
    endpoint_url        VARCHAR(512) DEFAULT NULL,          -- direct 模式下使用
    agent_secret_hash   VARCHAR(255) NOT NULL,
    data_policy         JSON DEFAULT NULL,                  -- {"store_input":false,"store_output":false,"retention_hours":0}
    status              ENUM('online', 'offline', 'removed') NOT NULL DEFAULT 'offline',
    last_heartbeat_at   DATETIME(3) DEFAULT NULL,
    framework_version   VARCHAR(50) DEFAULT NULL,
    version             VARCHAR(50) DEFAULT '1.0.0',
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    FOREIGN KEY (owner_id) REFERENCES users(id),
    INDEX idx_owner (owner_id),
    INDEX idx_status (status),
    INDEX idx_heartbeat (last_heartbeat_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 能力（Skill）表
-- ============================================================
CREATE TABLE capabilities (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL,
    name                VARCHAR(100) NOT NULL,
    display_name        VARCHAR(200) NOT NULL,
    description         TEXT,
    category            VARCHAR(50) DEFAULT 'general',
    tags                JSON DEFAULT NULL,
    input_schema        JSON NOT NULL,
    output_schema       JSON DEFAULT NULL,
    visibility          ENUM('public', 'restricted', 'private') NOT NULL DEFAULT 'public',
    approval_mode       ENUM('auto', 'manual') NOT NULL DEFAULT 'auto',
    multi_turn          TINYINT(1) NOT NULL DEFAULT 0,
    estimated_latency_ms INT UNSIGNED DEFAULT NULL,
    call_count          BIGINT UNSIGNED NOT NULL DEFAULT 0,
    success_count       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    total_latency_ms    BIGINT UNSIGNED NOT NULL DEFAULT 0,  -- 用于计算平均延迟
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_agent_skill (agent_id, name),
    FOREIGN KEY (agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
    INDEX idx_category (category),
    INDEX idx_visibility (visibility),
    FULLTEXT INDEX ft_search (display_name, description)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 能力向量表（语义搜索）
-- ============================================================
CREATE TABLE capability_embeddings (
    capability_id   BIGINT UNSIGNED PRIMARY KEY,
    embedding       BLOB NOT NULL,                          -- 序列化的 float32 向量
    model_version   VARCHAR(50) NOT NULL,                   -- embedding 模型版本
    updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    FOREIGN KEY (capability_id) REFERENCES capabilities(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 权限规则表
-- ============================================================
CREATE TABLE permission_rules (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL,
    skill_name          VARCHAR(100) DEFAULT NULL,
    caller_type         ENUM('user', 'agent', 'any') NOT NULL DEFAULT 'any',
    caller_id           VARCHAR(100) DEFAULT NULL,
    action              ENUM('allow', 'deny') NOT NULL DEFAULT 'allow',
    approval_mode       ENUM('auto', 'manual') NOT NULL DEFAULT 'auto',
    rate_limit_max      INT UNSIGNED DEFAULT NULL,
    rate_limit_window   VARCHAR(10) DEFAULT NULL,
    priority            INT NOT NULL DEFAULT 0,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    FOREIGN KEY (agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
    INDEX idx_agent_skill (agent_id, skill_name),
    INDEX idx_caller (caller_type, caller_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 调用记录表（按月分区）
-- ============================================================
CREATE TABLE invocations (
    id                  BIGINT UNSIGNED AUTO_INCREMENT,
    task_id             VARCHAR(36) NOT NULL,
    caller_agent_id     VARCHAR(100) DEFAULT NULL,
    caller_user_id      BIGINT UNSIGNED DEFAULT NULL,
    target_agent_id     VARCHAR(100) NOT NULL,
    skill_name          VARCHAR(100) NOT NULL,
    input_ref           VARCHAR(512) DEFAULT NULL,          -- 对象存储路径或内联 (小 payload 直接存)
    output_ref          VARCHAR(512) DEFAULT NULL,
    call_chain          JSON DEFAULT NULL,                  -- ["agent-a", "agent-b"]
    status              ENUM('submitted', 'assigned', 'working', 'input_required', 'completed', 'failed', 'cancelled')
                        NOT NULL DEFAULT 'submitted',
    mode                ENUM('sync', 'async') NOT NULL DEFAULT 'sync',
    error_message       TEXT DEFAULT NULL,
    latency_ms          INT UNSIGNED DEFAULT NULL,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    completed_at        DATETIME(3) DEFAULT NULL,
    PRIMARY KEY (id, created_at),
    UNIQUE KEY uk_task (task_id, created_at),
    INDEX idx_caller_agent (caller_agent_id, created_at),
    INDEX idx_caller_user (caller_user_id, created_at),
    INDEX idx_target (target_agent_id, created_at),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
PARTITION BY RANGE (TO_DAYS(created_at)) (
    PARTITION p202604 VALUES LESS THAN (TO_DAYS('2026-05-01')),
    PARTITION p202605 VALUES LESS THAN (TO_DAYS('2026-06-01')),
    PARTITION p202606 VALUES LESS THAN (TO_DAYS('2026-07-01')),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);

-- ============================================================
-- 多轮对话消息表
-- ============================================================
CREATE TABLE task_messages (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id         VARCHAR(36) NOT NULL,
    direction       ENUM('to_agent', 'from_agent') NOT NULL,
    message_type    ENUM('input', 'output', 'question', 'reply', 'progress') NOT NULL,
    payload_ref     VARCHAR(512) DEFAULT NULL,
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_task (task_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 审批队列表
-- ============================================================
CREATE TABLE approval_queue (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    invocation_id       BIGINT UNSIGNED NOT NULL,
    owner_id            BIGINT UNSIGNED NOT NULL,
    status              ENUM('pending', 'approved', 'denied', 'expired') NOT NULL DEFAULT 'pending',
    decided_at          DATETIME(3) DEFAULT NULL,
    expires_at          DATETIME(3) NOT NULL,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_owner_status (owner_id, status),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- Agent 评分表
-- ============================================================
CREATE TABLE agent_ratings (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id        VARCHAR(100) NOT NULL,
    skill_name      VARCHAR(100) DEFAULT NULL,
    rater_user_id   BIGINT UNSIGNED NOT NULL,
    score           TINYINT UNSIGNED NOT NULL,              -- 1-5
    comment         TEXT DEFAULT NULL,
    invocation_id   BIGINT UNSIGNED DEFAULT NULL,
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    FOREIGN KEY (agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
    FOREIGN KEY (rater_user_id) REFERENCES users(id),
    INDEX idx_agent (agent_id, skill_name),
    INDEX idx_rater (rater_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 3.3 大 Payload 存储策略

```
调用的 input / output 可能很大（合同全文、分析报告等）。
不直接存 MySQL JSON 列，而是按大小分流：

  payload ≤ 4KB  →  inline 存入 input_ref / output_ref 列（加 "inline:" 前缀）
  payload > 4KB  →  写入对象存储，input_ref 存路径（如 "s3://skynet/invocations/xxx/input.json"）

对于声明了 data_policy.store_input = false 的 Agent：
  - input 不持久化，input_ref = NULL
  - 仅在调用期间内存中存在

对于声明了 data_policy.retention_hours > 0 的 Agent：
  - 定时任务在 retention 过期后清除 payload

MVP 阶段对象存储可以用本地文件系统，后期迁移到 S3/MinIO。
```

---

## 4. 模块间交互时序

### 4.1 Agent 启动 & 注册流程

```
  Framework Runtime         Gateway               Registry           MySQL
     │                        │                       │                 │
     │  1. WebSocket 连接      │                       │                 │
     │  wss://gateway/tunnel  │                       │                 │
     │  + HMAC 签名            │                       │                 │
     │───────────────────────►│                       │                 │
     │                        │  2. 验证签名           │                 │
     │                        │  记录 agent→conn 映射  │                 │
     │                        │                       │                 │
     │  3. 发送 Agent Card    │                       │                 │
     │  (从 yaml+skill 生成)  │                       │                 │
     │───────────────────────►│  4. 转发注册           │                 │
     │                        │──────────────────────►│                 │
     │                        │                       │  INSERT/UPDATE  │
     │                        │                       │────────────────►│
     │                        │                       │                 │
     │                        │                       │  broadcast:     │
     │                        │                       │  agent_online   │
     │                        │                       │  (→ Dashboard)  │
     │                        │                       │                 │
     │  5. 注册成功            │                       │                 │
     │◄───────────────────────│                       │                 │
     │                        │                       │                 │
     │  6. 保持 WebSocket     │                       │                 │
     │  ping/pong 心跳 (30s)  │                       │                 │
     │◄──────────────────────►│  7. 更新心跳时间       │                 │
     │                        │──────────────────────►│                 │
```

### 4.2 同步调用流程（通过反向通道）

```
  Caller           Gateway            AuthZ          Target Agent
    │                 │                  │            (via Tunnel WS)
    │ POST /invoke    │                  │                 │
    │────────────────►│                  │                 │
    │                 │ check permission │                 │
    │                 │─────────────────►│                 │
    │                 │ ◄── allowed ─────│                 │
    │                 │                  │                 │
    │                 │ 通过反向通道推送调用请求              │
    │                 │ WS: {"type":"invoke", ...}         │
    │                 │──────────────────────────────────►│
    │                 │                                    │
    │                 │   Framework 路由到 Skill Handler    │
    │                 │   执行业务逻辑                      │
    │                 │                                    │
    │                 │ WS: {"type":"result", ...}         │
    │                 │◄──────────────────────────────────│
    │                 │                  │                 │
    │                 │ write audit log  │                 │
    │ ◄── result ─────│                  │                 │
```

### 4.3 多轮对话流程

```
  Caller           Gateway                           Target Agent
    │                 │                                    │
    │ POST /tasks     │                                    │
    │ mode: async     │                                    │
    │────────────────►│  WS: invoke                        │
    │                 │───────────────────────────────────►│
    │ ◄─ 202 task_id  │                                    │
    │                 │                                    │
    │                 │  WS: need_input                    │
    │                 │  "这份文件适用哪个法域？"             │
    │                 │◄───────────────────────────────────│
    │                 │                                    │
    │ SSE/GET: status │                                    │
    │  input_required │                                    │
    │  + question     │                                    │
    │◄────────────────│                                    │
    │                 │                                    │
    │ POST /tasks/    │                                    │
    │  {id}/reply     │                                    │
    │ {"jurisdiction" │  WS: reply                         │
    │  : "中国大陆"}   │───────────────────────────────────►│
    │────────────────►│                                    │
    │                 │                                    │
    │                 │  WS: progress (0.5)                │
    │                 │◄───────────────────────────────────│
    │ SSE: progress   │                                    │
    │◄────────────────│                                    │
    │                 │                                    │
    │                 │  WS: result                        │
    │                 │◄───────────────────────────────────│
    │ SSE: completed  │                                    │
    │◄────────────────│                                    │
```

### 4.4 审批流程

```
  Caller           Gateway              Owner (via Dashboard/Push)
    │                 │                       │
    │ POST /invoke    │                       │
    │────────────────►│                       │
    │                 │  policy = manual      │
    │                 │  创建审批记录          │
    │                 │  推送通知              │
    │                 │──────────────────────►│
    │                 │                       │
    │ ◄─ 202 pending  │                       │
    │                 │                       │
    │                 │  POST /approvals/{id} │
    │                 │  action: approve      │
    │                 │◄──────────────────────│
    │                 │                       │
    │                 │  通过反向通道执行调用   │
    │                 │──────────────────────►│ (Target Agent)
    │                 │                       │
    │ ◄─ result       │                       │
```

---

## 5. 安全设计

### 5.1 通信安全

```
所有通信链路：

Agent Framework ──WSS (TLS)──► Skynet Platform

- Agent 到 Platform 的 WebSocket 强制 TLS
- Dashboard 强制 HTTPS
- 对外 API 强制 HTTPS
- direct 模式下 Agent endpoint 也必须 HTTPS
```

### 5.2 认证机制

```
┌──────────────────────────────────────────────────────────────┐
│                      Authentication                           │
│                                                               │
│  User → Dashboard:                                            │
│    OAuth2 (Google / GitHub) 或 email + password               │
│    签发 JWT (access_token 15min + refresh_token 7d)          │
│                                                               │
│  Agent Framework → Gateway:                                   │
│    WebSocket 建立时: agent_id + timestamp + HMAC(secret)      │
│    后续通信在已认证的 WebSocket 连接上，不需要重复认证          │
│                                                               │
│  级联调用 (Agent A → Agent B):                                │
│    Gateway 为每次调用签发短期 JWT:                             │
│    - caller_agent_id                                         │
│    - target_agent_id                                         │
│    - skill_name                                              │
│    - call_chain                                              │
│    - expiry (调用 timeout + 缓冲)                             │
│    附在 invoke 消息中，Target Agent 框架自动验签              │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### 5.3 数据隐私

```
┌──────────────────────────────────────────────────────────────┐
│                      Data Privacy                             │
│                                                               │
│  1. Agent 声明数据策略 (skynet.yaml → data_policy)            │
│     - store_input: false   → 平台不持久化调用输入             │
│     - store_output: false  → 平台不持久化调用输出             │
│     - retention: 24        → 24 小时后自动清除                │
│                                                               │
│  2. 调用者可见对方的 data_policy                              │
│     Dashboard 在 Agent 详情页展示数据策略                     │
│     CLI invoke 时提示目标 Agent 的数据策略                    │
│                                                               │
│  3. 平台侧执行                                               │
│     Gateway 根据 data_policy 决定是否存储 payload             │
│     定时任务按 retention 清除过期数据                         │
│     审计日志中只记录调用元数据（谁调了谁、耗时、状态）         │
│     不记录 payload 内容                                      │
│                                                               │
│  4. 后期增强                                                  │
│     - 端到端加密选项（Gateway 无法解密 payload）              │
│     - 数据脱敏（自动检测 PII 并 mask）                       │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### 5.4 防 Prompt Injection

```
Agent 间调用存在间接 Prompt Injection 风险：

Agent A 调用 Agent B 的 Skill
→ Agent B 的输出中嵌入恶意指令
→ Agent A 的 LLM 误执行恶意指令

缓解措施：
1. 框架默认将跨 Agent 调用结果标记为 "untrusted external data"
2. 输出必须符合 Skill 声明的 output_schema，不合规则框架拒绝返回
3. Dashboard 展示调用结果时做 sanitize
4. 后期: Gateway 对 output 做 injection pattern 扫描
```

### 5.5 级联调用安全

```
call_chain 机制保障：

1. 深度限制: 默认最大 3 跳，网络级可配置
2. 环路检测: call_chain 中不允许重复 agent_id
3. 逐跳鉴权: A→B→C 中，B 调用 C 时以 B 的身份鉴权
              不会因为 A 有权限就自动传递
4. 审计追踪: 完整 call_chain 写入 invocations 表
```

---

## 6. 部署架构

### 6.1 MVP 单机部署

```
┌──────────────────────────────────────────┐
│            Single Server                  │
│                                           │
│  ┌───────────────────────────────────┐   │
│  │  Skynet Binary (Go)               │   │
│  │                                    │   │
│  │  ┌──────────┐  ┌──────────┐       │   │
│  │  │ Registry │  │ Gateway  │       │   │
│  │  │ Module   │  │ Module   │       │   │
│  │  └──────────┘  └──────────┘       │   │
│  │  ┌──────────┐  ┌──────────┐       │   │
│  │  │  AuthZ   │  │   API    │       │   │
│  │  │ Module   │  │ Handler  │       │   │
│  │  └──────────┘  └──────────┘       │   │
│  └───────────────────────────────────┘   │
│                                           │
│  ┌──────────┐  ┌──────────┐              │
│  │  MySQL   │  │ Local FS │              │
│  │  8.0+    │  │ (payload)│              │
│  └──────────┘  └──────────┘              │
│                                           │
│  ┌───────────────────────────────────┐   │
│  │  Dashboard (SPA, embedded / Nginx) │   │
│  └───────────────────────────────────┘   │
│                                           │
└──────────────────────────────────────────┘

MVP 阶段：
- 所有模块编译成一个 Go binary
- 前端 SPA 通过 go:embed 打包进 binary
- 大 payload 存本地文件系统
- 单进程服务
```

### 6.2 生产环境拆分

```
                           ┌──────────┐
                           │  Nginx   │
                           │  / LB    │
                           └────┬─────┘
               ┌────────────────┼────────────────┐
               │                │                │
        ┌──────▼──┐      ┌─────▼────┐     ┌─────▼─────┐
        │Registry │      │ Gateway  │     │ Dashboard │
        │Service  │      │ Service  │     │  (CDN)    │
        │ x2      │      │ x3       │     │           │
        └────┬────┘      └────┬─────┘     └───────────┘
             │                │
        ┌────▼────────────────▼──────┐
        │          MySQL             │
        │    (Primary-Replica)       │
        └────────────┬───────────────┘
                     │
        ┌────────────▼───────────────┐
        │    Object Storage (S3)     │
        └────────────────────────────┘

注意：
  Gateway 是有状态的（维护 WebSocket 连接池），
  水平扩展时需要：
  - Agent 通过 consistent hashing 分配到固定 Gateway 实例
  - 或 Gateway 实例间通过 Redis pub/sub 转发调用请求
```

---

## 7. 技术选型汇总

| 组件 | 选型 | 理由 |
|------|------|------|
| Platform 后端 | Go | 高性能、并发友好、单 binary 部署、WebSocket 支持好 |
| Web 框架 | gin 或 echo | 轻量成熟 |
| WebSocket | gorilla/websocket 或 nhooyr/websocket | 生产级 WebSocket 库 |
| 数据库 | MySQL 8.0+ | JSON 类型、全文索引、分区表、成熟运维 |
| 对象存储 | 本地 FS → S3/MinIO | MVP 用本地，后期迁移 |
| 前端 | React + TypeScript | 生态好，组件丰富 |
| UI 库 | Ant Design 或 shadcn/ui | 快速搭建管理界面 |
| 图可视化 | D3.js force graph | 网络拓扑展示 |
| 实时通信 | WebSocket (tunnel + Dashboard) + SSE (task stream) | |
| 认证 | JWT + OAuth2 | 标准方案 |
| Agent Framework | Go (主) + Python (后期) | Go 先行，Python 覆盖 AI 开发者群体 |
| 语义搜索 | 外部 Embedding API + 应用层计算 | MVP 够用，后期加 Faiss |

---

## 8. MVP 里程碑规划

```
Phase 1: 框架 + 基础骨架                        ← 核心
├── skynet CLI: init / add skill / dev / test
├── Agent Framework Runtime: Skill 注册、本地 HTTP 测试
├── Platform: 用户注册登录 (OAuth2)
├── Registry: Agent 注册/注销/列表
├── Gateway: WebSocket 反向通道
├── Dashboard: Agent 列表 + Skill 浏览
└── 端到端跑通: init → 开发 → register → 在 Dashboard 可见

Phase 2: 调用通路
├── Gateway: 同步调用转发 (通过反向通道)
├── 简单权限: public / private
├── 调用审计日志
├── Dashboard: 在线试用调用（表单输入 → 看结果）
├── CLI: skynet invoke 命令
└── 数据隐私策略: data_policy 声明 + 执行

Phase 3: 高级特性
├── 异步任务 + 多轮对话
├── 细粒度权限规则 + 审批流程
├── 限流
├── 级联调用 + call_chain 风控
├── Agent 评分系统
└── Dashboard: 审批队列 + 权限管理

Phase 4: 体验与规模
├── 语义搜索
├── 网络拓扑可视化
├── 调用统计分析
├── Python SDK
├── Gateway 水平扩展
├── skynet export-mcp (MCP 生态兼容)
└── 文档 + 示例项目
```

---

## 9. 开放问题

| 问题 | 选项 | 当前决策 |
|------|------|---------|
| Agent 是否支持不用框架的裸接入？ | A. 仅框架 / B. 框架优先但支持裸 REST 接入 | MVP 先 A，Phase 4 考虑 B |
| 计费模型 | A. 免费 / B. 调用方付费 / C. 平台抽成 | MVP 先 A，验证价值后再加 |
| 多租户/团队 | A. 单一网络 / B. 多个独立网络（类似 Slack workspace） | MVP 先 A，后期 B |
| Gateway 水平扩展方案 | A. Consistent hashing / B. Redis pub/sub 转发 | 后期根据实际规模选择 |
| 语义搜索 Embedding 来源 | A. 外部 API / B. 本地模型 | MVP 用 A，降低部署复杂度 |
| Python SDK 优先级 | A. Phase 2 / B. Phase 4 | B，先跑通 Go 全链路 |
