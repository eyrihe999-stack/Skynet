import { useEffect, useState } from 'react';
import { Card, Typography, Steps, Alert, Button, message, Tabs, Tag } from 'antd';
import { CopyOutlined } from '@ant-design/icons';
import { api } from '../api/client';
import type { User } from '../types';

const { Title, Text, Paragraph } = Typography;

const CodeBlock = ({ children }: { children: string }) => {
  const copy = () => {
    navigator.clipboard.writeText(children);
    message.success('已复制');
  };
  return (
    <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: '12px 16px', borderRadius: 6, fontFamily: 'monospace', fontSize: 13, position: 'relative', marginBottom: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
      <Button type="text" icon={<CopyOutlined />} size="small" onClick={copy}
        style={{ position: 'absolute', top: 8, right: 8, color: '#888' }} />
      {children}
    </div>
  );
};

export default function Guide() {
  const [user, setUser] = useState<User | null>(null);
  const platformURL = window.location.origin.replace(':3001', ':9090');
  const wsURL = platformURL.replace('http://', 'ws://').replace('https://', 'wss://');

  useEffect(() => {
    api.getProfile().then(resp => setUser(resp.data)).catch(() => {});
  }, []);

  const apiKey = user?.api_key || 'your-api-key';

  return (
    <div>
      <Title level={4}>快速开始</Title>

      <Card style={{ marginBottom: 24 }}>
        <Title level={5}>作为消费者 — 使用现有 Skill</Title>
        <Paragraph>
          无需安装任何东西。在左侧菜单点击<Text strong>「Skill 市场」</Text>搜索你需要的能力，
          点进 Agent 详情页即可在线试用。
        </Paragraph>
      </Card>

      <Card style={{ marginBottom: 24 }}>
        <Title level={5}>作为开发者 — 接入 Agent</Title>
        <Paragraph>
          Skynet 支持<Text strong>三种</Text>接入方式，选择最适合你的场景：
        </Paragraph>

        <Tabs items={[
          {
            key: 'go-sdk',
            label: <span><Tag color="blue">推荐</Tag>Go SDK</span>,
            children: <GoSDKGuide platformURL={platformURL} apiKey={apiKey} hasApiKey={!!user?.api_key} />,
          },
          {
            key: 'websocket',
            label: <span><Tag color="green">任意语言</Tag>WebSocket 协议</span>,
            children: <WebSocketGuide platformURL={platformURL} wsURL={wsURL} apiKey={apiKey} />,
          },
          {
            key: 'webhook',
            label: <span><Tag color="orange">Serverless</Tag>Webhook 模式</span>,
            children: <WebhookGuide platformURL={platformURL} apiKey={apiKey} />,
          },
        ]} />
      </Card>

      <Card style={{ marginTop: 24 }}>
        <Title level={5}>调用其他 Agent 的 Skill</Title>
        <Paragraph>
          在你的 Skill Handler 中，可以通过 <Text code>ctx.Invoke()</Text> 直接调用网络上其他 Agent 的 Skill，
          实现 Agent 间协作。框架会自动处理认证和调用链追踪。
        </Paragraph>
        <CodeBlock>{`var Summarize = framework.Skill{
    Name: "summarize",
    Handler: func(ctx framework.Context, input framework.Input) (any, error) {
        doc := input.String("doc")

        // 调用另一个 Agent 的翻译 Skill
        result, err := ctx.Invoke("translator-agent", "translate", map[string]any{
            "text":        doc,
            "target_lang": "en",
        })
        if err != nil {
            return nil, err
        }

        // 使用翻译结果继续处理
        translated := result.Output["text"]
        return map[string]any{"summary": translated}, nil
    },
}`}</CodeBlock>
        <Alert type="info" message="调用链保护" description={
          <span>
            平台自动追踪 Agent 间的调用链（call_chain），防止循环调用和过深嵌套（最大深度 3 层）。
            例如 A 调 B 调 C 是允许的，但 A 调 B 调 A 会被拒绝。
          </span>
        } />
      </Card>

      <Card style={{ marginTop: 24 }}>
        <Title level={5}>在代码中发现 Skill</Title>
        <Paragraph>
          在 Handler 中可以通过 <Text code>ctx.SearchSkills()</Text> 搜索网络上的 Skill，
          通过 <Text code>ctx.GetAgent()</Text> 查看某个 Agent 的详细信息和调用方式。
        </Paragraph>
        <CodeBlock>{`// 搜索网络上的 Skill（支持语义搜索）
skills, err := ctx.SearchSkills("合同审查")
for _, s := range skills {
    fmt.Printf("Agent: %s, Skill: %s\\n", s.AgentID, s.Name)
    fmt.Printf("  描述: %s\\n", s.Description)
    fmt.Printf("  输入: %v\\n", s.InputSchema)
}`}</CodeBlock>
        <CodeBlock>{`// 查看某个 Agent 的详情和 Skill 列表
agent, err := ctx.GetAgent("legal-bot")
fmt.Printf("Agent: %s (%s)\\n", agent.DisplayName, agent.Status)
for _, skill := range agent.Capabilities {
    fmt.Printf("  %s: %s\\n", skill.Name, skill.Description)
    fmt.Printf("  输入参数: %v\\n", skill.InputSchema)
}`}</CodeBlock>
        <Paragraph>
          完整的 Agent 间协作流程：<Text strong>发现</Text>（SearchSkills）→ <Text strong>了解</Text>（GetAgent）→ <Text strong>调用</Text>（Invoke）
        </Paragraph>
      </Card>
    </div>
  );
}

// ============ Go SDK 接入指南 ============
function GoSDKGuide({ platformURL, apiKey, hasApiKey }: { platformURL: string; apiKey: string; hasApiKey: boolean }) {
  return (
    <div>
      <Alert type="info" message="使用 Go SDK 可获得最完整的功能支持，包括多轮对话、Schema 验证、自动重连等。" style={{ marginBottom: 16 }} />
      <Steps
        direction="vertical"
        current={-1}
        items={[
          {
            title: '安装 CLI 工具',
            description: (
              <div>
                <Paragraph>需要 Go 1.25+</Paragraph>
                <CodeBlock>go install github.com/eyrihe999-stack/Skynet-client/cmd/skynet@latest</CodeBlock>
              </div>
            ),
          },
          {
            title: '创建 Agent 项目',
            description: (
              <div>
                <CodeBlock>{`skynet init my-agent\ncd my-agent\ngo mod tidy`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '本地开发测试',
            description: (
              <div>
                <Paragraph>启动本地开发服务器（不连网络）：</Paragraph>
                <CodeBlock>ENV=dev go run .</CodeBlock>
                <Paragraph>测试 Skill：</Paragraph>
                <CodeBlock>skynet test</CodeBlock>
              </div>
            ),
          },
          {
            title: '配置连接信息',
            description: (
              <div>
                <Paragraph>编辑 <Text code>skynet.yaml</Text>，填入 Platform 地址和你的 API Key：</Paragraph>
                <CodeBlock>{`network:\n  registry: ${platformURL}\n  api_key: ${apiKey}`}</CodeBlock>
                {hasApiKey && (
                  <Alert type="success" message="上面已自动填入你的 API Key，直接复制即可" style={{ marginBottom: 8 }} />
                )}
              </div>
            ),
          },
          {
            title: '连接网络',
            description: (
              <div>
                <CodeBlock>go run .</CodeBlock>
                <Paragraph>
                  看到 <Text code>Agent 'my-agent' registered and connected</Text> 表示连接成功。
                  回到 Dashboard 的 Agent 页面就能看到你的 Agent 已上线。
                </Paragraph>
              </div>
            ),
          },
          {
            title: '添加更多 Skill',
            description: (
              <div>
                <CodeBlock>skynet add skill review_contract</CodeBlock>
                <Paragraph>
                  编辑生成的 <Text code>skills/review_contract.go</Text>，实现 Handler 逻辑，
                  然后在 <Text code>skills/skills.go</Text> 中注册。重启 Agent 即可生效。
                </Paragraph>
              </div>
            ),
          },
        ]}
      />
    </div>
  );
}

// ============ WebSocket 协议接入指南 ============
function WebSocketGuide({ platformURL, wsURL, apiKey }: { platformURL: string; wsURL: string; apiKey: string }) {
  return (
    <div>
      <Alert type="info" message="只要能建立 WebSocket 连接，任何语言都能接入。以下是完整的协议规范和示例。" style={{ marginBottom: 16 }} />

      <Title level={5} style={{ marginTop: 16 }}>协议概览</Title>
      <Paragraph>
        Agent 通过 WebSocket 连接到 <Text code>{wsURL}/api/v1/tunnel</Text>，
        发送 JSON 消息完成注册、接收调用、返回结果。所有消息共享同一信封格式：
      </Paragraph>
      <CodeBlock>{`{
  "type": "register | registered | invoke | result | ping | pong",
  "request_id": "uuid（用于匹配请求和响应）",
  "payload": { ... }
}`}</CodeBlock>

      <Title level={5} style={{ marginTop: 24 }}>完整流程</Title>

      <Steps
        direction="vertical"
        current={-1}
        items={[
          {
            title: '1. 建立连接',
            description: (
              <div>
                <Paragraph>WebSocket 连接到隧道端点：</Paragraph>
                <CodeBlock>{`${wsURL}/api/v1/tunnel`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '2. 发送注册消息',
            description: (
              <div>
                <Paragraph>连接后立即发送 <Text code>register</Text> 消息：</Paragraph>
                <CodeBlock>{`{
  "type": "register",
  "payload": {
    "card": {
      "agent_id": "my-python-agent",
      "owner_api_key": "${apiKey}",
      "display_name": "My Python Agent",
      "description": "A demo agent in Python",
      "version": "1.0.0",
      "framework_version": "custom",
      "connection_mode": "tunnel",
      "capabilities": [
        {
          "name": "greet",
          "display_name": "Greet",
          "description": "Say hello in any language",
          "category": "general",
          "input_schema": {
            "type": "object",
            "properties": {
              "name": { "type": "string", "description": "Name to greet" },
              "language": { "type": "string", "enum": ["en", "zh", "ja"] }
            },
            "required": ["name"]
          },
          "visibility": "public",
          "approval_mode": "auto"
        }
      ]
    }
  }
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '3. 接收注册响应',
            description: (
              <div>
                <Paragraph>Platform 回复 <Text code>registered</Text> 消息：</Paragraph>
                <CodeBlock>{`{
  "type": "registered",
  "payload": {
    "success": true,
    "agent_secret": "as-xxxx..."  // 仅首次注册返回，妥善保存
  }
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '4. 监听 invoke 消息',
            description: (
              <div>
                <Paragraph>当有人调用你的 Skill 时，收到 <Text code>invoke</Text> 消息：</Paragraph>
                <CodeBlock>{`{
  "type": "invoke",
  "request_id": "abc-123",
  "payload": {
    "skill": "greet",
    "input": { "name": "Alice", "language": "zh" },
    "caller": { "user_id": 1, "display_name": "Bob" },
    "timeout_ms": 30000
  }
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '5. 返回 result 消息',
            description: (
              <div>
                <Paragraph>处理完成后，发送 <Text code>result</Text> 消息（request_id 必须匹配）：</Paragraph>
                <CodeBlock>{`{
  "type": "result",
  "request_id": "abc-123",
  "payload": {
    "status": "completed",
    "output": { "greeting": "你好，Alice！" }
  }
}`}</CodeBlock>
                <Paragraph>失败时：</Paragraph>
                <CodeBlock>{`{
  "type": "result",
  "request_id": "abc-123",
  "payload": {
    "status": "failed",
    "error": "unsupported language"
  }
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '6. 心跳保活',
            description: (
              <div>
                <Paragraph>每 30 秒发送 <Text code>ping</Text>，Platform 回复 <Text code>pong</Text>。90 秒未收到心跳会被判定离线。</Paragraph>
                <CodeBlock>{`{"type": "ping"}   →   {"type": "pong"}`}</CodeBlock>
              </div>
            ),
          },
        ]}
      />

      <Title level={5} style={{ marginTop: 24 }}>Python 完整示例</Title>
      <CodeBlock>{`import websocket
import json
import threading
import time

WS_URL = "${wsURL}/api/v1/tunnel"
API_KEY = "${apiKey}"

def on_message(ws, raw):
    msg = json.loads(raw)

    if msg["type"] == "registered":
        print("Registered!", msg["payload"])
    elif msg["type"] == "invoke":
        # 处理调用
        skill = msg["payload"]["skill"]
        inp = msg["payload"]["input"]
        name = inp.get("name", "World")
        lang = inp.get("language", "en")
        greetings = {"en": f"Hello, {name}!", "zh": f"你好，{name}！", "ja": f"こんにちは、{name}！"}
        result = {"greeting": greetings.get(lang, greetings["en"])}
        ws.send(json.dumps({
            "type": "result",
            "request_id": msg["request_id"],
            "payload": {"status": "completed", "output": result}
        }))
    elif msg["type"] == "ping":
        ws.send(json.dumps({"type": "pong"}))

def heartbeat(ws):
    while True:
        time.sleep(30)
        try:
            ws.send(json.dumps({"type": "ping"}))
        except:
            break

ws = websocket.WebSocketApp(WS_URL, on_message=on_message)
ws.on_open = lambda ws: (
    ws.send(json.dumps({
        "type": "register",
        "payload": {"card": {
            "agent_id": "python-greeter",
            "owner_api_key": API_KEY,
            "display_name": "Python Greeter",
            "description": "Greets in multiple languages",
            "version": "1.0.0",
            "framework_version": "custom",
            "connection_mode": "tunnel",
            "capabilities": [{
                "name": "greet",
                "display_name": "Greet",
                "description": "Say hello",
                "category": "general",
                "input_schema": {"type": "object", "properties": {
                    "name": {"type": "string"},
                    "language": {"type": "string", "enum": ["en", "zh", "ja"]}
                }, "required": ["name"]},
                "visibility": "public",
                "approval_mode": "auto"
            }]
        }}
    })),
    threading.Thread(target=heartbeat, args=(ws,), daemon=True).start()
)
ws.run_forever()`}</CodeBlock>

      <Title level={5} style={{ marginTop: 24 }}>Node.js 完整示例</Title>
      <CodeBlock>{`const WebSocket = require("ws");

const WS_URL = "${wsURL}/api/v1/tunnel";
const API_KEY = "${apiKey}";

const ws = new WebSocket(WS_URL);

ws.on("open", () => {
  ws.send(JSON.stringify({
    type: "register",
    payload: { card: {
      agent_id: "node-greeter",
      owner_api_key: API_KEY,
      display_name: "Node Greeter",
      description: "Greets in multiple languages",
      version: "1.0.0",
      framework_version: "custom",
      connection_mode: "tunnel",
      capabilities: [{
        name: "greet",
        display_name: "Greet",
        description: "Say hello",
        category: "general",
        input_schema: { type: "object", properties: {
          name: { type: "string" },
          language: { type: "string", enum: ["en", "zh", "ja"] }
        }, required: ["name"] },
        visibility: "public",
        approval_mode: "auto"
      }]
    }}
  }));

  // 心跳
  setInterval(() => ws.send(JSON.stringify({ type: "ping" })), 30000);
});

ws.on("message", (raw) => {
  const msg = JSON.parse(raw);
  if (msg.type === "registered") {
    console.log("Registered!", msg.payload);
  } else if (msg.type === "invoke") {
    const { skill, input } = msg.payload;
    const name = input.name || "World";
    const greetings = { en: \`Hello, \${name}!\`, zh: \`你好，\${name}！\`, ja: \`こんにちは、\${name}！\` };
    ws.send(JSON.stringify({
      type: "result",
      request_id: msg.request_id,
      payload: { status: "completed", output: { greeting: greetings[input.language] || greetings.en } }
    }));
  } else if (msg.type === "ping") {
    ws.send(JSON.stringify({ type: "pong" }));
  }
});`}</CodeBlock>
    </div>
  );
}

// ============ Webhook/Direct 模式接入指南 ============
function WebhookGuide({ platformURL, apiKey }: { platformURL: string; apiKey: string }) {
  return (
    <div>
      <Alert type="info" message="Webhook 模式适合无法维持长连接的场景：Serverless 函数、Claude Code Skill、定时启动的服务等。Agent 只需提供一个 HTTP 端点，Platform 会主动调用。" style={{ marginBottom: 16 }} />

      <Title level={5} style={{ marginTop: 16 }}>工作原理</Title>
      <Paragraph>
        与 WebSocket Tunnel 模式相反：Agent 注册时提供 <Text code>endpoint_url</Text>，
        当有人调用 Skill 时，Platform 向该 URL 发送 HTTP POST 请求，Agent 返回结果即可。
      </Paragraph>

      <Title level={5} style={{ marginTop: 24 }}>协议规范</Title>
      <Steps
        direction="vertical"
        current={-1}
        items={[
          {
            title: '1. 注册 Agent（REST API）',
            description: (
              <div>
                <Paragraph>通过 POST 请求注册，无需 WebSocket：</Paragraph>
                <CodeBlock>{`POST ${platformURL}/api/v1/agents/register
Authorization: Bearer <your-jwt-token>

{
  "agent_id": "my-webhook-agent",
  "display_name": "My Webhook Agent",
  "description": "A serverless agent",
  "version": "1.0.0",
  "endpoint_url": "https://your-server.com/skynet/invoke",
  "capabilities": [
    {
      "name": "translate",
      "display_name": "Translate",
      "description": "Translate text between languages",
      "category": "general",
      "input_schema": {
        "type": "object",
        "properties": {
          "text": { "type": "string" },
          "target_lang": { "type": "string" }
        },
        "required": ["text", "target_lang"]
      },
      "visibility": "public"
    }
  ]
}`}</CodeBlock>
                <Paragraph>响应：</Paragraph>
                <CodeBlock>{`{
  "code": 0,
  "data": {
    "agent_id": "my-webhook-agent",
    "status": "registered",
    "agent_secret": "as-xxxx..."  // 仅首次返回，用于验证 Platform 回调签名
  }
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '2. 接收 Skill 调用（HTTP POST）',
            description: (
              <div>
                <Paragraph>Platform 调用 Skill 时，会 POST 到你的 <Text code>endpoint_url</Text>：</Paragraph>
                <CodeBlock>{`POST https://your-server.com/skynet/invoke
Content-Type: application/json
X-Skynet-Request-ID: abc-123
X-Skynet-Agent-ID: my-webhook-agent
X-Skynet-Signature: sha256=<HMAC-SHA256 of body using agent_secret>

{
  "request_id": "abc-123",
  "skill": "translate",
  "input": { "text": "Hello world", "target_lang": "zh" },
  "caller": { "user_id": 1, "display_name": "Alice" },
  "timeout_ms": 30000
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '3. 返回结果',
            description: (
              <div>
                <Paragraph>你的 HTTP 端点返回 JSON 响应：</Paragraph>
                <CodeBlock>{`HTTP 200 OK
Content-Type: application/json

{
  "status": "completed",
  "output": { "translated": "你好世界" }
}`}</CodeBlock>
                <Paragraph>失败时：</Paragraph>
                <CodeBlock>{`{
  "status": "failed",
  "error": "unsupported language"
}`}</CodeBlock>
              </div>
            ),
          },
          {
            title: '4. 心跳保活（推荐）',
            description: (
              <div>
                <Paragraph>Webhook Agent 也需要定期心跳以保持在线状态（90 秒超时）：</Paragraph>
                <CodeBlock>{`POST ${platformURL}/api/v1/agents/my-webhook-agent/heartbeat
Authorization: Bearer <your-jwt-token>`}</CodeBlock>
                <Paragraph>或重新调用注册接口（会自动刷新心跳）。</Paragraph>
              </div>
            ),
          },
        ]}
      />

      <Title level={5} style={{ marginTop: 24 }}>验证签名（推荐）</Title>
      <Paragraph>
        Platform 用 <Text code>agent_secret</Text> 对请求体做 HMAC-SHA256 签名，
        放在 <Text code>X-Skynet-Signature</Text> 头中。你可以验证来源：
      </Paragraph>
      <CodeBlock>{`import hmac, hashlib

def verify_signature(body: bytes, secret: str, signature: str) -> bool:
    expected = "sha256=" + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)`}</CodeBlock>

      <Title level={5} style={{ marginTop: 24 }}>Python (Flask) 完整示例</Title>
      <CodeBlock>{`from flask import Flask, request, jsonify
import requests
import hmac
import hashlib
import threading
import time

app = Flask(__name__)

PLATFORM = "${platformURL}"
TOKEN = "your-jwt-token"  # 从登录接口获取
AGENT_SECRET = None  # 首次注册后保存

def register():
    global AGENT_SECRET
    resp = requests.post(f"{PLATFORM}/api/v1/agents/register",
        headers={"Authorization": f"Bearer {TOKEN}"},
        json={
            "agent_id": "flask-translator",
            "display_name": "Flask Translator",
            "description": "Translates text using an LLM",
            "version": "1.0.0",
            "endpoint_url": "https://your-server.com/invoke",
            "capabilities": [{
                "name": "translate",
                "display_name": "Translate",
                "description": "Translate text to target language",
                "category": "general",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "text": {"type": "string"},
                        "target_lang": {"type": "string"}
                    },
                    "required": ["text", "target_lang"]
                },
                "visibility": "public"
            }]
        })
    data = resp.json()["data"]
    if "agent_secret" in data:
        AGENT_SECRET = data["agent_secret"]
        print(f"Registered! Secret: {AGENT_SECRET}")

@app.route("/invoke", methods=["POST"])
def invoke():
    data = request.json
    skill = data["skill"]
    inp = data["input"]

    if skill == "translate":
        # 这里可以调用 LLM、翻译 API 等
        result = {"translated": f"[{inp['target_lang']}] {inp['text']}"}
        return jsonify({"status": "completed", "output": result})

    return jsonify({"status": "failed", "error": f"unknown skill: {skill}"})

def heartbeat_loop():
    while True:
        time.sleep(60)
        requests.post(
            f"{PLATFORM}/api/v1/agents/flask-translator/heartbeat",
            headers={"Authorization": f"Bearer {TOKEN}"}
        )

register()
threading.Thread(target=heartbeat_loop, daemon=True).start()
app.run(host="0.0.0.0", port=8080)`}</CodeBlock>

      <Title level={5} style={{ marginTop: 24 }}>Claude Code Skill 桥接示例</Title>
      <Alert type="success" message="你可以把 Claude Code 的能力作为 Skynet Skill 暴露给整个网络" style={{ marginBottom: 12 }} />
      <CodeBlock>{`#!/usr/bin/env python3
"""将 Claude Code CLI 包装为 Skynet Webhook Agent"""
import subprocess, json
from flask import Flask, request, jsonify
import requests

app = Flask(__name__)

PLATFORM = "${platformURL}"
TOKEN = "your-jwt-token"

# 注册
requests.post(f"{PLATFORM}/api/v1/agents/register",
    headers={"Authorization": f"Bearer {TOKEN}"},
    json={
        "agent_id": "claude-code-bridge",
        "display_name": "Claude Code Bridge",
        "description": "Run Claude Code prompts as Skynet Skills",
        "version": "1.0.0",
        "endpoint_url": "https://your-server.com/invoke",
        "capabilities": [{
            "name": "code-review",
            "display_name": "Code Review",
            "description": "Review code for bugs and improvements",
            "category": "development",
            "input_schema": {
                "type": "object",
                "properties": {
                    "code": {"type": "string", "description": "Code to review"},
                    "language": {"type": "string", "description": "Programming language"}
                },
                "required": ["code"]
            },
            "visibility": "public"
        }]
    })

@app.route("/invoke", methods=["POST"])
def invoke():
    data = request.json
    inp = data["input"]

    if data["skill"] == "code-review":
        lang = inp.get("language", "")
        prompt = f"Review this {lang} code for bugs and improvements:\\n\\n{inp['code']}"

        result = subprocess.run(
            ["claude", "-p", prompt],
            capture_output=True, text=True, timeout=120
        )
        return jsonify({
            "status": "completed",
            "output": {"review": result.stdout}
        })

    return jsonify({"status": "failed", "error": "unknown skill"})

app.run(port=8080)`}</CodeBlock>

      <Title level={5} style={{ marginTop: 24 }}>用 curl 快速测试</Title>
      <CodeBlock>{`# 1. 注册 webhook agent（用你的 JWT token）
curl -X POST ${platformURL}/api/v1/agents/register \\
  -H "Authorization: Bearer YOUR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "agent_id": "curl-test-agent",
    "display_name": "Curl Test",
    "endpoint_url": "https://httpbin.org/post",
    "capabilities": [{
      "name": "echo",
      "display_name": "Echo",
      "description": "Echoes input back",
      "input_schema": {"type": "object"},
      "visibility": "public"
    }]
  }'`}</CodeBlock>
    </div>
  );
}
