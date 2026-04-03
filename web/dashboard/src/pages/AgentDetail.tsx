import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Descriptions, Tag, Table, Button, Input, Typography, message, Space, Alert } from 'antd';
import { api } from '../api/client';
import type { Agent, Capability, InvokeResponse } from '../types';

const { Title, Text } = Typography;
const { TextArea } = Input;

export default function AgentDetail() {
  const { agentId } = useParams<{ agentId: string }>();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedSkill, setSelectedSkill] = useState<Capability | null>(null);
  const [inputJSON, setInputJSON] = useState('{}');
  const [invoking, setInvoking] = useState(false);
  const [result, setResult] = useState<InvokeResponse | null>(null);

  useEffect(() => {
    if (agentId) loadAgent();
  }, [agentId]);

  const loadAgent = async () => {
    try {
      const resp = await api.getAgent(agentId!);
      setAgent(resp.data);
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  const handleInvoke = async () => {
    if (!selectedSkill || !agentId) return;
    setInvoking(true);
    setResult(null);
    try {
      const input = JSON.parse(inputJSON);
      const resp = await api.invoke(agentId, selectedSkill.name, input);
      setResult(resp.data);
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setInvoking(false);
    }
  };

  const handleReply = async () => {
    if (!result?.task_id) return;
    setInvoking(true);
    try {
      const input = JSON.parse(inputJSON);
      const resp = await api.replyTask(result.task_id, input);
      setResult(resp.data);
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setInvoking(false);
    }
  };

  if (loading) return <div>加载中...</div>;
  if (!agent) return <div>Agent 不存在</div>;

  const skillColumns = [
    { title: '名称', dataIndex: 'display_name', key: 'display_name' },
    { title: 'Skill ID', dataIndex: 'name', key: 'name' },
    { title: '分类', dataIndex: 'category', key: 'category' },
    { title: '可见性', dataIndex: 'visibility', key: 'visibility',
      render: (v: string) => <Tag>{v}</Tag> },
    { title: '调用次数', dataIndex: 'call_count', key: 'call_count' },
    { title: '操作', key: 'action',
      render: (_: any, r: Capability) => (
        <Button type="link" onClick={() => { setSelectedSkill(r); setResult(null); setInputJSON('{}'); }}>
          试用
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={4}>{agent.display_name}</Title>
      <Descriptions bordered size="small" column={2} style={{ marginBottom: 24 }}>
        <Descriptions.Item label="Agent ID">{agent.agent_id}</Descriptions.Item>
        <Descriptions.Item label="状态"><Tag color={agent.status === 'online' ? 'green' : 'default'}>{agent.status}</Tag></Descriptions.Item>
        <Descriptions.Item label="描述" span={2}>{agent.description}</Descriptions.Item>
        <Descriptions.Item label="版本">{agent.version}</Descriptions.Item>
        <Descriptions.Item label="连接模式">{agent.connection_mode}</Descriptions.Item>
      </Descriptions>

      <Title level={5}>Skill 列表</Title>
      <Table rowKey="id" columns={skillColumns} dataSource={agent.capabilities || []} size="small" pagination={false} style={{ marginBottom: 24 }} />

      {selectedSkill && (
        <Card title={`试用: ${selectedSkill.display_name}`} style={{ marginTop: 16 }}>
          {selectedSkill.input_schema && (
            <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
              Input Schema: {JSON.stringify(selectedSkill.input_schema)}
            </Text>
          )}
          <TextArea rows={4} value={inputJSON} onChange={e => setInputJSON(e.target.value)}
            placeholder="JSON 输入参数" style={{ marginBottom: 12, fontFamily: 'monospace' }} />
          <Space>
            {result?.status === 'input_required' ? (
              <Button type="primary" loading={invoking} onClick={handleReply}>回复追问</Button>
            ) : (
              <Button type="primary" loading={invoking} onClick={handleInvoke}>调用</Button>
            )}
          </Space>

          {result && (
            <div style={{ marginTop: 16 }}>
              {result.status === 'input_required' && result.question && (
                <Alert type="info" message={`追问: ${result.question.prompt}`}
                  description={result.question.options?.join(', ')} style={{ marginBottom: 12 }} />
              )}
              {result.status === 'completed' && (
                <Alert type="success" message="调用成功"
                  description={<pre style={{ margin: 0 }}>{JSON.stringify(result.output, null, 2)}</pre>} />
              )}
              {result.status === 'failed' && (
                <Alert type="error" message="调用失败" description={result.error} />
              )}
            </div>
          )}
        </Card>
      )}
    </div>
  );
}
