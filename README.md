# Skynet

Agent 开发框架 + 互联网络。开发者可以用 Skynet 快速构建自己的 Agent，一键接入网络，发现和调用网络上其他 Agent 的能力。

## 快速开始

### 1. 启动 Platform

**前置条件**：MySQL 8.0+

```bash
# 创建数据库
mysql -u root -e "CREATE DATABASE IF NOT EXISTS skynet DEFAULT CHARACTER SET utf8mb4;"

# 启动（首次运行会自动建表）
cd /path/to/Skynet
go run ./cmd/skynetd/

# 自定义配置（环境变量）
LISTEN_ADDR=:9090 \
DB_HOST=127.0.0.1 \
DB_PORT=3306 \
DB_USER=root \
DB_PASSWORD= \
DB_NAME=skynet \
JWT_SECRET=your-secret \
LOG_LEVEL=debug \
go run ./cmd/skynetd/
```

也可以创建 `.env` 文件（参考 `.env.example`），程序启动时会自动加载。

启动成功后可以验证：

```bash
curl http://localhost:9090/health
# {"status":"ok"}
```

### 2. 注册账号

```bash
curl -X POST http://localhost:9090/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "123456",
    "display_name": "Alice"
  }'
```

返回结果中包含 `api_key`，**请妥善保存，只显示一次**：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "user": { "id": 1, "email": "alice@example.com", "display_name": "Alice" },
    "api_key": "sk-xxxxxxxxxxxx"
  }
}
```

### 3. 创建 Agent 项目

```bash
# 安装 CLI（二选一）
go install github.com/skynetplatform/skynet/cmd/skynet@latest
# 或者从源码运行
go run ./cmd/skynet/ init my-agent
```

生成的项目结构：

```
my-agent/
├── skynet.yaml          # Agent 配置
├── main.go              # 入口
├── skills/
│   ├── skills.go        # Skill 注册
│   └── hello.go         # 示例 Skill
└── go.mod
```

### 4. 配置 Agent

编辑 `skynet.yaml`：

```yaml
agent:
  id: alice-assistant          # 全局唯一 ID
  display_name: "Alice 的助手"
  description: "擅长问候和数据处理"
  version: 1.0.0

network:
  registry: http://localhost:9090    # Platform 地址
  api_key: ${SKYNET_API_KEY}         # 从环境变量读取

server:
  port: 9100                         # 本地 dev 模式端口

defaults:
  visibility: public
  approval_mode: auto
```

### 5. 编写 Skill

每个 Skill 就是一个函数，在 `skills/` 目录下创建文件：

```go
// skills/greet.go
package skills

import (
    "fmt"
    "github.com/skynetplatform/skynet/pkg/framework"
)

var Greet = framework.Skill{
    Name:        "greet",
    DisplayName: "问候",
    Description: "用指定语言问候",
    Category:    "general",
    Tags:        []string{"demo"},

    Input: framework.Schema{
        "name":     framework.String("姓名").Required(),
        "language": framework.Enum("语言", "zh", "en", "ja"),
    },
    Output: framework.Schema{
        "message": framework.String("问候语"),
    },

    Handler: func(ctx framework.Context, input framework.Input) (any, error) {
        name := input.String("name")
        lang := input.String("language")

        var msg string
        switch lang {
        case "en":
            msg = fmt.Sprintf("Hello, %s!", name)
        case "ja":
            msg = fmt.Sprintf("こんにちは、%sさん！", name)
        default:
            msg = fmt.Sprintf("你好，%s！", name)
        }

        return map[string]any{"message": msg}, nil
    },
}
```

在 `skills/skills.go` 中注册：

```go
package skills

import "github.com/skynetplatform/skynet/pkg/framework"

func RegisterAll(agent *framework.Agent) {
    agent.Register(Hello)
    agent.Register(Greet)
}
```

### 6. 本地测试

```bash
cd my-agent
SKYNET_MODE=dev go run .
```

启动后可以直接调用：

```bash
# 查看 Agent Card
curl http://localhost:9100/agent-card

# 列出所有 Skill
curl http://localhost:9100/skills

# 调用 Skill
curl -X POST http://localhost:9100/skills/greet \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "language": "zh"}'
# {"output": {"message": "你好，Alice！"}}
```

### 7. 连接网络

```bash
cd my-agent
export SKYNET_API_KEY="sk-xxxxxxxxxxxx"
go run .
```

输出 `Agent 'alice-assistant' registered and connected` 表示成功。

Agent 会通过 WebSocket 反向通道连接 Platform，**不需要公网 IP**。

### 8. 发现和调用

```bash
API_KEY="sk-xxxxxxxxxxxx"

# 查看所有在线 Agent
curl http://localhost:9090/api/v1/agents \
  -H "X-API-Key: $API_KEY"

# 查看某个 Agent 的详情
curl http://localhost:9090/api/v1/agents/alice-assistant \
  -H "X-API-Key: $API_KEY"

# 按关键词搜索能力
curl "http://localhost:9090/api/v1/capabilities?q=问候" \
  -H "X-API-Key: $API_KEY"

# 调用其他 Agent 的 Skill
curl -X POST http://localhost:9090/api/v1/invoke \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "target_agent": "alice-assistant",
    "skill": "greet",
    "input": {"name": "Bob", "language": "en"},
    "timeout_ms": 30000
  }'
# {"code":0,"data":{"task_id":"xxx","status":"completed","output":{"message":"Hello, Bob!"}}}
```

也可以用 CLI 调用：

```bash
go run ./cmd/skynet/ invoke alice-assistant greet \
  -i '{"name":"Bob","language":"en"}' \
  -s http://localhost:9090 \
  -k "$API_KEY"
```

---

## Schema DSL

定义 Skill 的输入输出 Schema：

```go
framework.Schema{
    "name":     framework.String("描述").Required(),   // 必填字符串
    "age":      framework.Int("描述"),                  // 整数
    "score":    framework.Number("描述"),               // 浮点数
    "active":   framework.Bool("描述"),                 // 布尔
    "role":     framework.Enum("描述", "admin", "user"),// 枚举
    "tags":     framework.StringArray("描述"),          // 字符串数组
    "items":    framework.Array("描述"),                // 通用数组
    "metadata": framework.Object("描述"),               // 对象
}
```

框架会自动将其转换为 JSON Schema，用于输入校验和 Agent Card 展示。

---

## API 参考

所有需要认证的接口支持两种方式：
- `X-API-Key: sk-xxx` 请求头
- `Authorization: Bearer <jwt>` 请求头（通过 `/api/v1/auth/login` 获取 JWT）

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 注册账号，返回 API Key |
| POST | `/api/v1/auth/login` | 登录，返回 JWT |
| GET | `/api/v1/auth/profile` | 查看当前用户信息 |

### Agent 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/agents` | 列出所有 Agent（支持 `?status=online&page=1&page_size=20&mine=true`） |
| GET | `/api/v1/agents/:agent_id` | 获取 Agent 详情（含能力列表） |
| DELETE | `/api/v1/agents/:agent_id` | 删除自己的 Agent |
| POST | `/api/v1/agents/:agent_id/heartbeat` | 心跳上报 |

### 能力搜索

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/capabilities` | 搜索能力（`?q=关键词&category=general&page=1`） |

### 调用

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/invoke` | 同步调用 Agent Skill |

调用请求体：

```json
{
  "target_agent": "agent-id",
  "skill": "skill-name",
  "input": { ... },
  "timeout_ms": 30000
}
```

### WebSocket

| 路径 | 说明 |
|------|------|
| `/api/v1/tunnel` | Agent 反向通道（框架自动管理） |

---

## 项目结构

```
Skynet/
├── cmd/
│   ├── skynetd/                 # Platform 服务端
│   └── skynet/                  # CLI 工具
│       └── templates/           # 项目脚手架模板
├── internal/
│   ├── api/handler/             # HTTP + WebSocket Handler
│   ├── authz/                   # 认证授权
│   ├── config/                  # 配置
│   ├── gateway/                 # 网关（反向通道 + 调用路由）
│   ├── model/                   # 数据模型
│   ├── registry/                # 注册中心
│   └── store/                   # 数据库 Repo
├── pkg/
│   ├── database/                # MySQL 连接
│   ├── framework/               # Agent 开发框架（用户 import）
│   ├── logger/                  # 日志
│   ├── protocol/                # WebSocket 通信协议
│   └── response/                # HTTP 响应工具
├── migrations/                  # SQL 建表脚本
└── docs/
    └── architecture.md          # 架构设计文档
```

---

## 工作原理

```
开发者                    Platform                  调用者
                                                    
  go run .               ┌──────────┐               
     │                   │ Gateway  │               
     │  WebSocket 连接    │          │               
     ├──────────────────►│ 反向通道  │               
     │                   │          │  POST /invoke  
     │  注册 Agent Card  │ Registry │◄──────────────
     ├──────────────────►│          │               
     │                   │          │  通过反向通道   
     │  收到调用请求      │          │  路由到 Agent  
     │◄──────────────────┤          ├──────────────►
     │                   │          │               
     │  执行 Skill       │          │               
     │  返回结果          │          │               
     ├──────────────────►│          │──────────────►
     │                   └──────────┘    返回结果    
```

关键特点：
- **Agent 不需要公网 IP** — 通过 WebSocket 反向通道，Agent 主动连接 Platform
- **断线自动重连** — 指数退避重连策略
- **自动注册** — Agent Card 从 `skynet.yaml` + Skill 定义自动生成
- **同步调用** — 通过反向通道转发请求，等待结果返回
