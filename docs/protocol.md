# Skynet 通信协议文档

> 本文档描述 Skynet 系统中各角色之间的通信协议，包括 HTTP REST API、WebSocket 消息协议、认证流程等。
> 基于当前代码实现整理，不含架构文档中规划但尚未实现的部分。

---

## 1. 系统角色与通信概览

```
  ┌────────────┐      HTTP REST        ┌──────────────────────────────────────┐
  │  skynet    │  ──────────────────►  │                                      │
  │  CLI       │  ◄──────────────────  │          Skynet Platform              │
  └────────────┘                       │                                      │
                                       │  ┌──────────┐  ┌──────────────────┐ │
  ┌────────────┐      HTTP REST        │  │   API    │  │   AuthZ Service  │ │
  │ Dashboard  │  ──────────────────►  │  │  Router  │  │  (认证授权)       │ │
  │ (Browser)  │  ◄──────────────────  │  └────┬─────┘  └──────────────────┘ │
  └────────────┘                       │       │                              │
                                       │  ┌────▼─────┐  ┌──────────────────┐ │
  ┌────────────┐      WebSocket        │  │ Registry │  │     Gateway      │ │
  │   Agent    │  ◄═══════════════►   │  │ Service  │  │    Service       │ │
  │ (Framework │      反向隧道         │  └──────────┘  └──────────────────┘ │
  │  Runtime)  │                       │                                      │
  └────────────┘                       └──────────────────────────────────────┘
```

### 通信方式总览

| 通信路径 | 协议 | 说明 |
|----------|------|------|
| CLI → Platform | HTTP REST | skynet invoke、skynet status 等命令 |
| Dashboard → Platform | HTTP REST | 用户注册登录、Agent 管理、能力搜索、技能调用 |
| Agent → Platform | WebSocket | 反向隧道：注册、心跳、接收调用请求、返回结果 |
| Platform → Agent | WebSocket | 通过反向隧道推送 invoke 请求和 ping |

---

## 2. 认证机制

### 2.1 认证方式

Platform 支持两种认证方式，在 HTTP 请求中通过 Header 携带：

| 方式 | Header | 格式 | 适用场景 |
|------|--------|------|----------|
| API Key | `X-API-Key` | `sk-{64位十六进制字符}` | CLI 访问、Agent 注册 |
| JWT Token | `Authorization` | `Bearer {JWT}` | Dashboard 登录后访问 |

中间件优先检查 `X-API-Key`，其次检查 `Authorization: Bearer`。两者都无则返回 401。

### 2.2 API Key

- **格式**：`sk-` 前缀 + 64 位十六进制字符（32 字节随机数编码），共 67 个字符
- **生成时机**：用户注册时生成，仅在注册响应中返回一次明文
- **存储方式**：数据库中存储 bcrypt 哈希值
- **用途**：CLI 命令行访问、Agent Framework 连接隧道时在 Agent Card 中携带

### 2.3 JWT Token

- **算法**：HS256（HMAC-SHA256）
- **有效期**：24 小时
- **签名密钥**：服务端配置的 `jwt.secret`

**JWT Claims 结构**：

```json
{
  "user_id": 123,
  "email": "user@example.com",
  "exp": 1712188800,
  "iat": 1712102400
}
```

---

## 3. HTTP REST API

所有 API 以 `/api/v1` 为前缀。

### 3.1 统一响应格式

所有成功响应和部分错误响应均使用统一的 JSON 结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | 成功时为 `0`，失败时为 HTTP 状态码 |
| `message` | string | 成功时为 `"ok"` 或 `"created"`，失败时为错误描述 |
| `data` | object/null | 响应数据，错误时为 null |

**分页响应格式**（用于列表接口）：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [ ... ],
    "total": 100,
    "page": 1,
    "page_size": 20,
    "total_pages": 5
  }
}
```

### 3.2 认证接口（公开，无需认证）

#### POST /api/v1/auth/register — 用户注册

**请求**：

```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "mypassword",
  "display_name": "Alice"
}
```

| 字段 | 类型 | 必填 | 校验规则 |
|------|------|:----:|----------|
| `email` | string | 是 | 合法邮箱格式，全局唯一 |
| `password` | string | 是 | 最少 6 个字符 |
| `display_name` | string | 是 | 非空 |

**响应 201 Created**：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "user": {
      "id": 1,
      "email": "alice@example.com",
      "display_name": "Alice",
      "status": "active",
      "created_at": "2026-04-03T10:00:00Z",
      "updated_at": "2026-04-03T10:00:00Z"
    },
    "api_key": "sk-a1b2c3d4e5f6...（64位十六进制）"
  }
}
```

> **注意**：`api_key` 仅在此响应中返回一次明文，之后无法再查看。

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 参数校验失败、邮箱已注册 |
| 500 | 服务端内部错误 |

---

#### POST /api/v1/auth/login — 用户登录

**请求**：

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "mypassword"
}
```

**响应 200 OK**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "email": "alice@example.com",
      "display_name": "Alice",
      "status": "active",
      "created_at": "2026-04-03T10:00:00Z",
      "updated_at": "2026-04-03T10:00:00Z"
    }
  }
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 401 | 邮箱不存在或密码错误（统一提示 "invalid email or password"，不泄露具体原因） |

---

#### GET /api/v1/health — 健康检查

**响应 200 OK**：

```json
{
  "status": "ok"
}
```

---

### 3.3 认证接口（需认证）

#### GET /api/v1/auth/profile — 获取当前用户信息

**请求**：

```http
GET /api/v1/auth/profile
Authorization: Bearer {jwt_token}
```

**响应 200 OK**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 1,
    "email": "alice@example.com",
    "display_name": "Alice",
    "status": "active",
    "created_at": "2026-04-03T10:00:00Z",
    "updated_at": "2026-04-03T10:00:00Z"
  }
}
```

---

### 3.4 Agent 管理接口（需认证）

#### GET /api/v1/agents — 获取 Agent 列表

**请求**：

```http
GET /api/v1/agents?page=1&page_size=20&status=online&mine=true
Authorization: Bearer {jwt_token}
```

| 查询参数 | 类型 | 默认值 | 说明 |
|----------|------|--------|------|
| `page` | int | 1 | 页码 |
| `page_size` | int | 20 | 每页条数 |
| `status` | string | — | 可选过滤：`online`、`offline`、`removed` |
| `mine` | string | — | 设为 `"true"` 时只返回当前用户的 Agent |

**响应 200 OK**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "id": 1,
        "agent_id": "alice-legal-assistant",
        "owner_id": 1,
        "display_name": "Alice 的法律助手",
        "description": "擅长合同审查、法规查询",
        "connection_mode": "tunnel",
        "endpoint_url": "",
        "status": "online",
        "last_heartbeat_at": "2026-04-03T10:05:00Z",
        "framework_version": "1.0.0",
        "version": "1.0.0",
        "created_at": "2026-04-03T10:00:00Z",
        "updated_at": "2026-04-03T10:05:00Z",
        "capabilities": [
          {
            "id": 1,
            "agent_id": "alice-legal-assistant",
            "name": "review_contract",
            "display_name": "合同审查",
            "description": "审查合同条款，标注风险点",
            "category": "legal",
            "tags": ["合同", "法律"],
            "input_schema": { ... },
            "output_schema": { ... },
            "visibility": "public",
            "approval_mode": "auto",
            "multi_turn": false,
            "estimated_latency_ms": 5000,
            "call_count": 120,
            "success_count": 115,
            "total_latency_ms": 360000,
            "created_at": "2026-04-03T10:00:00Z",
            "updated_at": "2026-04-03T10:05:00Z"
          }
        ]
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20,
    "total_pages": 1
  }
}
```

---

#### GET /api/v1/agents/:agent_id — 获取 Agent 详情

**请求**：

```http
GET /api/v1/agents/alice-legal-assistant
X-API-Key: sk-a1b2c3d4...
```

**响应 200 OK**：返回单个 Agent 对象（结构同列表中的 item）。

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 404 | Agent 不存在 |

---

#### DELETE /api/v1/agents/:agent_id — 删除 Agent

**请求**：

```http
DELETE /api/v1/agents/alice-legal-assistant
Authorization: Bearer {jwt_token}
```

> 仅 Agent 所有者可删除自己的 Agent。

**响应 200 OK**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "agent_id": "alice-legal-assistant",
    "status": "removed"
  }
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 403 | 当前用户不是该 Agent 的所有者 |
| 404 | Agent 不存在 |

---

#### POST /api/v1/agents/:agent_id/heartbeat — 心跳上报

**请求**：

```http
POST /api/v1/agents/alice-legal-assistant/heartbeat
X-API-Key: sk-a1b2c3d4...
```

请求体为空或 `{}`。

**响应 200 OK**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "agent_id": "alice-legal-assistant",
    "status": "ok"
  }
}
```

---

### 3.5 能力搜索接口（需认证）

#### GET /api/v1/capabilities — 搜索能力/Skill

**请求**：

```http
GET /api/v1/capabilities?q=合同审查&category=legal&page=1&page_size=20
Authorization: Bearer {jwt_token}
```

| 查询参数 | 类型 | 默认值 | 说明 |
|----------|------|--------|------|
| `q` | string | — | 搜索关键词，使用 MySQL FULLTEXT 布尔模式匹配 `display_name` 和 `description` |
| `category` | string | — | 按分类过滤 |
| `page` | int | 1 | 页码 |
| `page_size` | int | 20 | 每页条数 |

> 搜索范围限定为：`visibility = "public"` 且 Agent `status = "online"` 的能力。

**响应 200 OK**：分页格式，items 为 Capability 对象数组。

---

### 3.6 技能调用接口（需认证）

#### POST /api/v1/invoke — 调用技能

**请求**：

```http
POST /api/v1/invoke
Content-Type: application/json
Authorization: Bearer {jwt_token}

{
  "target_agent": "alice-legal-assistant",
  "skill": "review_contract",
  "input": {
    "contract_text": "甲方与乙方约定..."
  },
  "timeout_ms": 30000
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `target_agent` | string | 是 | 目标 Agent ID |
| `skill` | string | 是 | 要调用的 Skill 名称 |
| `input` | object | 否 | Skill 输入参数（原始 JSON，透传给 Agent） |
| `timeout_ms` | int | 否 | 超时毫秒数，默认 30000（30 秒） |

**响应 200 OK（调用成功）**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "completed",
    "output": {
      "risk_level": "high",
      "issues": ["第三条存在违约金过高的问题"],
      "suggestions": ["建议将违约金比例调整为合同总额的 10%"]
    },
    "error": ""
  }
}
```

**响应 200 OK（调用失败）**：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "failed",
    "output": null,
    "error": "skill execution timed out"
  }
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 请求参数缺失或格式错误 |
| 500 | 目标 Agent 不在线、内部路由失败 |

### 3.7 调用处理内部流程

```
  POST /api/v1/invoke
         │
         ▼
  ┌─────────────┐     InvokeRequest     ┌──────────────┐
  │InvokeHandler│ ──────────────────►  │   Gateway     │
  │(HTTP 层)     │                      │   Service     │
  └─────────────┘                      └──────┬───────┘
                                              │
                               TunnelManager.GetConn(agent_id)
                                              │
                                       ┌──────▼───────┐
                                       │  AgentConn   │
                                       │  .SendInvoke │
                                       └──────┬───────┘
                                              │
                                  WebSocket: {"type":"invoke",...}
                                              │
                                       ┌──────▼───────┐
                                       │    Agent     │
                                       │  (Framework) │
                                       └──────┬───────┘
                                              │
                                  WebSocket: {"type":"result",...}
                                              │
                                       ┌──────▼───────┐
                                       │  AgentConn   │  ── request_id 匹配 ──► 返回 ResultPayload
                                       └──────────────┘
```

---

## 4. WebSocket 隧道协议

### 4.1 连接建立

**端点**：`GET /api/v1/tunnel`

Agent Framework 通过标准 WebSocket 握手连接到此端点：

```
GET /api/v1/tunnel HTTP/1.1
Host: skynet.example.com
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: ...
Sec-WebSocket-Version: 13
```

连接成功后进入消息交互阶段。

### 4.2 消息信封格式

所有 WebSocket 消息使用统一的 JSON 信封结构：

```json
{
  "type": "string",
  "request_id": "string",
  "payload": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 消息类型，决定 payload 的结构 |
| `request_id` | string | 请求标识符。调用类消息使用 UUID 关联请求与响应；心跳/注册类消息为空字符串 |
| `payload` | object | 消息载荷，结构由 type 决定 |

### 4.3 消息类型一览

| 类型 | 方向 | 说明 |
|------|------|------|
| `register` | Agent → Gateway | Agent 发送注册请求，携带 Agent Card |
| `registered` | Gateway → Agent | Gateway 返回注册结果 |
| `invoke` | Gateway → Agent | Gateway 向 Agent 转发技能调用请求 |
| `result` | Agent → Gateway | Agent 返回技能执行结果 |
| `ping` | Agent → Gateway | Agent 发送心跳 |
| `pong` | Gateway → Agent | Gateway 回复心跳 |
| `error` | 双向 | 通用错误消息 |

### 4.4 register — 注册请求

**方向**：Agent → Gateway

Agent 在 WebSocket 连接建立后立即发送此消息，携带完整的 Agent Card 信息。

```json
{
  "type": "register",
  "request_id": "",
  "payload": {
    "card": {
      "agent_id": "alice-legal-assistant",
      "owner_api_key": "sk-a1b2c3d4...",
      "display_name": "Alice 的法律助手",
      "description": "擅长合同审查、法规查询",
      "version": "1.0.0",
      "framework_version": "1.0.0",
      "connection_mode": "tunnel",
      "capabilities": [
        {
          "name": "review_contract",
          "display_name": "合同审查",
          "description": "审查合同条款，标注风险点",
          "category": "legal",
          "tags": ["合同", "法律"],
          "input_schema": {
            "type": "object",
            "properties": {
              "contract_text": {
                "type": "string",
                "description": "合同全文"
              }
            },
            "required": ["contract_text"]
          },
          "output_schema": {
            "type": "object",
            "properties": {
              "risk_level": { "type": "string" },
              "issues": { "type": "array" }
            }
          },
          "visibility": "public",
          "approval_mode": "auto",
          "multi_turn": false,
          "estimated_latency_ms": 5000
        }
      ],
      "data_policy": {
        "store_input": false,
        "store_output": false,
        "retention_hours": 0
      }
    }
  }
}
```

**Agent Card 字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `agent_id` | string | Agent 全局唯一标识，来自 skynet.yaml |
| `owner_api_key` | string | 所有者的 API Key，用于身份验证 |
| `display_name` | string | Agent 显示名称 |
| `description` | string | Agent 描述 |
| `version` | string | Agent 版本号 |
| `framework_version` | string | 框架版本号（当前为 "1.0.0"） |
| `connection_mode` | string | 连接模式，当前固定为 `"tunnel"` |
| `capabilities` | array | Skill 能力列表 |
| `data_policy` | object | 数据处理策略（可选） |

**Capability 字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | Skill 唯一标识名 |
| `display_name` | string | 显示名称 |
| `description` | string | 功能描述 |
| `category` | string | 分类（默认 "general"） |
| `tags` | string[] | 标签列表 |
| `input_schema` | object | 输入参数的 JSON Schema |
| `output_schema` | object | 输出结果的 JSON Schema |
| `visibility` | string | 可见性：`public`、`restricted`、`private` |
| `approval_mode` | string | 审批模式：`auto`、`manual` |
| `multi_turn` | bool | 是否为多轮对话 Skill |
| `estimated_latency_ms` | int | 预估延迟（毫秒） |

### 4.5 registered — 注册响应

**方向**：Gateway → Agent

```json
{
  "type": "registered",
  "request_id": "",
  "payload": {
    "success": true,
    "agent_secret": "as-7f3e2a1b...",
    "error": ""
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | bool | 注册是否成功 |
| `agent_secret` | string | 首次注册时返回的 Agent 密钥（`as-` 前缀 + 64 位十六进制）。重新注册时为空 |
| `error` | string | 失败时的错误信息 |

**注册失败示例**：

```json
{
  "type": "registered",
  "request_id": "",
  "payload": {
    "success": false,
    "agent_secret": "",
    "error": "invalid API key"
  }
}
```

### 4.6 invoke — 技能调用请求

**方向**：Gateway → Agent

Gateway 收到 HTTP invoke 请求后，通过 WebSocket 反向隧道将调用转发给目标 Agent。

```json
{
  "type": "invoke",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "payload": {
    "skill": "review_contract",
    "input": {
      "contract_text": "甲方与乙方约定..."
    },
    "caller": {
      "agent_id": "",
      "user_id": 1,
      "display_name": "Alice"
    },
    "timeout_ms": 30000
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `skill` | string | 要调用的 Skill 名称 |
| `input` | object | Skill 输入参数（原始 JSON） |
| `caller` | object | 调用方信息 |
| `caller.agent_id` | string | 调用方 Agent ID（Agent 间调用时填写） |
| `caller.user_id` | uint | 调用方用户 ID（用户发起调用时填写） |
| `caller.display_name` | string | 调用方显示名称 |
| `timeout_ms` | int | 超时毫秒数 |

### 4.7 result — 技能执行结果

**方向**：Agent → Gateway

Agent 执行完 Skill Handler 后，通过此消息返回结果。`request_id` 必须与 invoke 消息的 `request_id` 一致，用于 Gateway 关联请求和响应。

**执行成功**：

```json
{
  "type": "result",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "payload": {
    "status": "completed",
    "output": {
      "risk_level": "high",
      "issues": ["第三条违约金过高"],
      "suggestions": ["调整为 10%"]
    },
    "error": ""
  }
}
```

**执行失败**：

```json
{
  "type": "result",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "payload": {
    "status": "failed",
    "output": null,
    "error": "skill handler returned error: timeout"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | `"completed"` 成功，`"failed"` 失败 |
| `output` | object | 执行结果（成功时有值，失败时为 null） |
| `error` | string | 错误信息（失败时有值，成功时为空） |

### 4.8 ping / pong — 心跳

**Agent → Gateway**：

```json
{
  "type": "ping",
  "request_id": "",
  "payload": null
}
```

**Gateway → Agent**：

```json
{
  "type": "pong",
  "request_id": "",
  "payload": null
}
```

- Agent Framework 每 **30 秒** 发送一次 ping
- Gateway 收到 ping 后立即回复 pong
- 如果 Gateway 向 Agent 发送 ping，Agent 也会回复 pong
- Platform 的心跳监控器每 **30 秒** 检查一次，将超过 **90 秒** 未心跳的 Agent 标记为 offline

### 4.9 error — 通用错误

**双向**（通常 Gateway → Agent）：

```json
{
  "type": "error",
  "request_id": "",
  "payload": {
    "error": "expected register message"
  }
}
```

用于协议握手阶段的错误通知（如 Agent 发送的第一条消息不是 register 类型）。

---

## 5. WebSocket 连接生命周期

### 5.1 完整时序

```
  Agent (Framework)                           Gateway (Platform)
       │                                           │
       │  ① WebSocket 握手                          │
       │  GET /api/v1/tunnel                       │
       │──────────────────────────────────────────►│
       │◄──────────────────────────────────────────│
       │  101 Switching Protocols                   │
       │                                           │
       │  ② 发送注册消息 (register)                  │
       │  携带 Agent Card + API Key                │
       │──────────────────────────────────────────►│
       │                                           │  验证 API Key (AuthZ)
       │                                           │  注册 Agent (Registry)
       │                                           │  生成 Agent Secret
       │  ③ 接收注册响应 (registered)               │
       │◄──────────────────────────────────────────│
       │                                           │  注册到 TunnelManager
       │                                           │
       │  ④ 心跳循环（每 30 秒）                     │
       │  ping ──────────────────────────────────► │
       │  ◄────────────────────────────────── pong │
       │                                           │
       │  ⑤ 接收调用请求（按需）                     │
       │  ◄───────────────────────── invoke        │
       │         执行 Skill Handler                 │
       │  result ────────────────────────────────► │
       │                                           │
       │  ... 重复 ④ 和 ⑤ ...                      │
       │                                           │
       │  ⑥ 连接关闭                                │
       │  ──── WebSocket Close ────────────────►   │
       │                                           │  从 TunnelManager 注销
       │                                           │  在 Registry 中标记 offline
```

### 5.2 断线重连策略

Agent Framework 的 `ConnectWithRetry` 实现了指数退避重连：

| 参数 | 值 |
|------|-----|
| 初始间隔 | 1 秒 |
| 退避策略 | 每次翻倍（1s → 2s → 4s → 8s → ...） |
| 最大间隔 | 60 秒 |
| 重试次数 | 无限制，直到成功 |

重连成功后会重新发送 register 消息，重新注册 Agent Card。

### 5.3 invoke 请求-响应匹配

Gateway 通过 `request_id`（UUID）匹配 invoke 请求和 result 响应：

1. Gateway 为每个调用生成唯一的 `request_id`
2. 发送 invoke 消息时携带 `request_id`，同时在 `AgentConn.pending` map 中注册等待通道
3. Agent 执行完毕后返回的 result 消息必须携带相同的 `request_id`
4. `AgentConn.readLoop` 收到 result 后，根据 `request_id` 查找对应的等待通道并写入结果
5. 如果超时（默认 30 秒），等待通道自动取消

---

## 6. 本地开发模式 API

Agent 在本地开发模式下（`framework.IsDevMode() == true`）启动一个独立的 HTTP 服务器，用于不连接 Platform 的本地测试。

**默认端口**：`skynet.yaml` 中的 `server.port`（默认 9100）。

### GET /agent-card — 查看 Agent Card

**响应 200 OK**：返回完整的 Agent Card JSON（结构同 WebSocket register 消息中的 `card` 字段）。

### GET /skills — 列出所有 Skill

**响应 200 OK**：

```json
{
  "skills": ["review_contract", "deep_analysis"]
}
```

### POST /skills/:name — 调用 Skill

**请求**：

```http
POST /skills/review_contract
Content-Type: application/json

{
  "contract_text": "甲方与乙方约定..."
}
```

请求体直接作为 Skill 的 input 参数。空请求体默认为 `{}`。

**响应 200 OK**：

```json
{
  "output": {
    "risk_level": "high",
    "issues": ["..."],
    "suggestions": ["..."]
  }
}
```

**错误响应**：

| 状态码 | 响应体 | 场景 |
|--------|--------|------|
| 404 | `{"error": "skill 'xxx' not found"}` | Skill 不存在 |
| 500 | `{"error": "错误信息"}` | Skill 执行出错 |

---

## 7. CLI 通信详情

### 7.1 skynet invoke

通过 HTTP POST 调用远程 Agent 的 Skill。

**配置优先级**（服务器地址和 API Key）：
1. 命令行参数 `--server` / `--key`
2. 环境变量 `SKYNET_REGISTRY` / `SKYNET_API_KEY`
3. `skynet.yaml` 中的 `network.registry` / `network.api_key`

```bash
skynet invoke alice-legal-assistant review_contract \
  --input '{"contract_text": "..."}' \
  --server https://skynet.example.com \
  --key sk-a1b2c3d4...
```

等价于：

```http
POST https://skynet.example.com/api/v1/invoke
Content-Type: application/json
X-API-Key: sk-a1b2c3d4...

{
  "target_agent": "alice-legal-assistant",
  "skill": "review_contract",
  "input": {"contract_text": "..."},
  "timeout_ms": 30000
}
```

### 7.2 skynet status

查询 Agent 在 Platform 上的状态。

```bash
skynet status alice-legal-assistant --server https://skynet.example.com
```

等价于：

```http
GET https://skynet.example.com/api/v1/agents/alice-legal-assistant
X-API-Key: sk-a1b2c3d4...
```

如果不指定 agent_id，从当前目录的 `skynet.yaml` 中读取 `agent.id`。

---

## 8. 数据模型参考

### 8.1 User

```json
{
  "id": 1,
  "email": "alice@example.com",
  "display_name": "Alice",
  "status": "active",
  "created_at": "2026-04-03T10:00:00Z",
  "updated_at": "2026-04-03T10:00:00Z"
}
```

> `password_hash` 和 `api_key_hash` 不会出现在 API 响应中（json:"-"）。

### 8.2 Agent

```json
{
  "id": 1,
  "agent_id": "alice-legal-assistant",
  "owner_id": 1,
  "display_name": "Alice 的法律助手",
  "description": "擅长合同审查、法规查询",
  "connection_mode": "tunnel",
  "endpoint_url": "",
  "data_policy": {
    "store_input": false,
    "store_output": false,
    "retention_hours": 0
  },
  "status": "online",
  "last_heartbeat_at": "2026-04-03T10:05:00Z",
  "framework_version": "1.0.0",
  "version": "1.0.0",
  "created_at": "2026-04-03T10:00:00Z",
  "updated_at": "2026-04-03T10:05:00Z",
  "capabilities": [ ... ]
}
```

> `agent_secret_hash` 不会出现在 API 响应中（json:"-"）。

### 8.3 Capability

```json
{
  "id": 1,
  "agent_id": "alice-legal-assistant",
  "name": "review_contract",
  "display_name": "合同审查",
  "description": "审查合同条款，标注风险点",
  "category": "legal",
  "tags": ["合同", "法律"],
  "input_schema": { "type": "object", "properties": { ... } },
  "output_schema": { "type": "object", "properties": { ... } },
  "visibility": "public",
  "approval_mode": "auto",
  "multi_turn": false,
  "estimated_latency_ms": 5000,
  "call_count": 120,
  "success_count": 115,
  "total_latency_ms": 360000,
  "created_at": "2026-04-03T10:00:00Z",
  "updated_at": "2026-04-03T10:05:00Z"
}
```

### 8.4 Invocation

```json
{
  "id": 1,
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "caller_agent_id": "",
  "caller_user_id": 1,
  "target_agent_id": "alice-legal-assistant",
  "skill_name": "review_contract",
  "status": "completed",
  "mode": "sync",
  "error_message": "",
  "latency_ms": 2500,
  "created_at": "2026-04-03T10:10:00Z",
  "completed_at": "2026-04-03T10:10:02.5Z"
}
```

**Invocation 状态枚举**：

| 状态 | 说明 |
|------|------|
| `submitted` | 调用已提交，等待路由 |
| `assigned` | 已分配给目标 Agent |
| `working` | Agent 正在处理中 |
| `completed` | 执行成功 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |
