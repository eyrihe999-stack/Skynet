import { useEffect, useState } from 'react';
import { Card, Typography, Steps, Alert, Button, message } from 'antd';
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

      <Card>
        <Title level={5}>作为开发者 — 构建自己的 Agent</Title>

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
                  {user?.api_key && (
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
