import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Descriptions, Tag, Table, Button, Input, Typography, message, Space, Modal, Divider } from 'antd';
import { PlayCircleOutlined, InfoCircleOutlined, CheckCircleFilled, CloseCircleFilled, QuestionCircleFilled } from '@ant-design/icons';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { api } from '../api/client';
import type { Agent, Capability, InvokeResponse } from '../types';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

export default function AgentDetail() {
  const { agentId } = useParams<{ agentId: string }>();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedSkill, setSelectedSkill] = useState<Capability | null>(null);
  const [inputJSON, setInputJSON] = useState('{}');
  const [invoking, setInvoking] = useState(false);
  const [result, setResult] = useState<InvokeResponse | null>(null);
  const [resultModalOpen, setResultModalOpen] = useState(false);

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
      setResultModalOpen(true);
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
      setResultModalOpen(true);
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setInvoking(false);
    }
  };

  const generateExample = (schema: any): any => {
    if (!schema || !schema.properties) return {};
    const example: Record<string, any> = {};
    for (const [key, prop] of Object.entries(schema.properties) as any) {
      switch (prop.type) {
        case 'string':
          if (prop.enum) example[key] = prop.enum[0];
          else example[key] = prop.description || key;
          break;
        case 'integer': example[key] = 0; break;
        case 'number': example[key] = 0.0; break;
        case 'boolean': example[key] = false; break;
        case 'array':
          example[key] = prop.items?.type === 'string' ? ['示例'] : [];
          break;
        case 'object': example[key] = {}; break;
        default: example[key] = null;
      }
    }
    return example;
  };

  const fillExample = () => {
    if (!selectedSkill?.input_schema) return;
    setInputJSON(JSON.stringify(generateExample(selectedSkill.input_schema), null, 2));
  };

  const renderSchemaFields = (schema: any, label: string) => {
    if (!schema?.properties) return null;
    const required = schema.required || [];
    return (
      <div style={{ marginBottom: 16 }}>
        <Text strong>{label}：</Text>
        <Table
          size="small"
          pagination={false}
          dataSource={Object.entries(schema.properties).map(([key, prop]: any) => ({
            key,
            name: key,
            type: prop.type || '-',
            required: required.includes(key),
            description: prop.description || '-',
            enum: prop.enum,
          }))}
          columns={[
            { title: '字段名', dataIndex: 'name', width: 120,
              render: (v: string, r: any) => <Text code>{v}{r.required ? ' *' : ''}</Text> },
            { title: '类型', dataIndex: 'type', width: 80, render: (v: string) => <Tag>{v}</Tag> },
            { title: '说明', dataIndex: 'description' },
            { title: '可选值', dataIndex: 'enum', render: (v: string[]) => v ? v.map(e => <Tag key={e} color="blue">{e}</Tag>) : '-' },
          ]}
          style={{ marginTop: 8 }}
        />
      </div>
    );
  };

  const successRate = (s: Capability) => s.call_count > 0 ? `${((s.success_count / s.call_count) * 100).toFixed(0)}%` : '-';

  if (loading) return <div>加载中...</div>;
  if (!agent) return <div>Agent 不存在</div>;

  const skillColumns = [
    { title: '名称', dataIndex: 'display_name', key: 'display_name' },
    { title: 'Skill ID', dataIndex: 'name', key: 'name', render: (v: string) => <Text code>{v}</Text> },
    { title: '分类', dataIndex: 'category', key: 'category', render: (v: string) => <Tag>{v}</Tag> },
    { title: '可见性', dataIndex: 'visibility', key: 'visibility',
      render: (v: string) => <Tag color={v === 'public' ? 'green' : v === 'private' ? 'red' : 'orange'}>{v}</Tag> },
    { title: '调用/成功率', key: 'stats',
      render: (_: any, r: Capability) => `${r.call_count} 次 / ${successRate(r)}` },
    { title: '操作', key: 'action',
      render: (_: any, r: Capability) => (
        <Button type="primary" size="small" icon={<PlayCircleOutlined />}
          onClick={() => { setSelectedSkill(r); setResult(null); setInputJSON(JSON.stringify(generateExample(r.input_schema), null, 2)); }}>
          试用
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={4}>{agent.display_name}</Title>
      <Descriptions bordered size="small" column={2} style={{ marginBottom: 24 }}>
        <Descriptions.Item label="Agent ID"><Text code>{agent.agent_id}</Text></Descriptions.Item>
        <Descriptions.Item label="状态"><Tag color={agent.status === 'online' ? 'green' : 'default'}>{agent.status}</Tag></Descriptions.Item>
        <Descriptions.Item label="描述" span={2}>{agent.description}</Descriptions.Item>
        <Descriptions.Item label="版本">{agent.version}</Descriptions.Item>
        <Descriptions.Item label="连接模式">{agent.connection_mode}</Descriptions.Item>
      </Descriptions>

      <Title level={5}>Skill 列表</Title>
      <Table rowKey="id" columns={skillColumns} dataSource={agent.capabilities || []} size="small" pagination={false} style={{ marginBottom: 24 }} />

      {selectedSkill && (
        <Card
          title={<Space><InfoCircleOutlined />{selectedSkill.display_name}</Space>}
          style={{ marginTop: 16 }}
        >
          {/* Skill 详细说明 */}
          <Descriptions size="small" column={2} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="Skill ID"><Text code>{selectedSkill.name}</Text></Descriptions.Item>
            <Descriptions.Item label="分类"><Tag>{selectedSkill.category}</Tag></Descriptions.Item>
            <Descriptions.Item label="功能描述" span={2}>{selectedSkill.description || '暂无描述'}</Descriptions.Item>
            <Descriptions.Item label="审批模式">{selectedSkill.approval_mode === 'auto' ? '自动' : '需人工审批'}</Descriptions.Item>
            <Descriptions.Item label="多轮对话">{selectedSkill.multi_turn ? '是' : '否'}</Descriptions.Item>
            {selectedSkill.tags && selectedSkill.tags.length > 0 && (
              <Descriptions.Item label="标签" span={2}>
                {selectedSkill.tags.map(t => <Tag key={t} color="blue">{t}</Tag>)}
              </Descriptions.Item>
            )}
          </Descriptions>

          <Divider style={{ margin: '12px 0' }} />

          {/* 输入参数表格 */}
          {renderSchemaFields(selectedSkill.input_schema, '输入参数')}

          {/* 输出参数表格 */}
          {renderSchemaFields(selectedSkill.output_schema, '输出格式')}

          <Divider style={{ margin: '12px 0' }} />

          {/* 调用区 */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
            <Text strong>输入 JSON</Text>
            <Button type="link" size="small" onClick={fillExample} style={{ padding: 0 }}>重置为示例</Button>
          </div>
          <TextArea rows={5} value={inputJSON} onChange={e => setInputJSON(e.target.value)}
            placeholder="JSON 输入参数" style={{ marginBottom: 12, fontFamily: 'monospace', fontSize: 13 }} />
          <Space>
            {result?.status === 'input_required' ? (
              <Button type="primary" loading={invoking} onClick={handleReply} icon={<PlayCircleOutlined />}>回复追问</Button>
            ) : (
              <Button type="primary" loading={invoking} onClick={handleInvoke} icon={<PlayCircleOutlined />}>执行调用</Button>
            )}
          </Space>

          {/* 追问提示（内联显示） */}
          {result?.status === 'input_required' && result.question && (
            <div style={{ marginTop: 16, padding: 16, background: '#e6f7ff', borderRadius: 6, border: '1px solid #91d5ff' }}>
              <Text strong><QuestionCircleFilled style={{ color: '#1890ff', marginRight: 8 }} />Agent 追问：</Text>
              <Paragraph style={{ margin: '8px 0 0' }}>{result.question.prompt}</Paragraph>
              {result.question.options && (
                <Space style={{ marginTop: 8 }}>
                  {result.question.options.map(opt => (
                    <Button key={opt} size="small" onClick={() => {
                      // 点击选项自动填入
                      const field = result.question!.field;
                      try {
                        const current = JSON.parse(inputJSON);
                        current[field] = opt;
                        setInputJSON(JSON.stringify(current, null, 2));
                      } catch {
                        setInputJSON(JSON.stringify({ [field]: opt }, null, 2));
                      }
                    }}>{opt}</Button>
                  ))}
                </Space>
              )}
            </div>
          )}
        </Card>
      )}

      {/* 结果弹窗 */}
      <Modal
        open={resultModalOpen}
        onCancel={() => setResultModalOpen(false)}
        footer={<Button type="primary" onClick={() => setResultModalOpen(false)}>确定</Button>}
        width={720}
        title={
          result?.status === 'completed' ? (
            <Space><CheckCircleFilled style={{ color: '#52c41a' }} />调用成功</Space>
          ) : result?.status === 'failed' ? (
            <Space><CloseCircleFilled style={{ color: '#ff4d4f' }} />调用失败</Space>
          ) : result?.status === 'input_required' ? (
            <Space><QuestionCircleFilled style={{ color: '#1890ff' }} />需要更多输入</Space>
          ) : '调用结果'
        }
      >
        {result?.status === 'completed' && (
          <div>
            <Text type="secondary" style={{ fontSize: 12 }}>Task ID: {result.task_id}</Text>
            <div style={{ marginTop: 12, maxHeight: 500, overflow: 'auto' }}>
              {(() => {
                const output = result.output;
                if (!output || typeof output !== 'object') {
                  return <pre style={{ margin: 0, fontSize: 13, background: '#f5f5f5', padding: 16, borderRadius: 6 }}>{JSON.stringify(output, null, 2)}</pre>;
                }
                const entries = Object.entries(output as Record<string, any>);
                // 如果只有一个字符串字段，直接全幅 Markdown 渲染
                if (entries.length === 1 && typeof entries[0][1] === 'string') {
                  return <div className="markdown-output"><Markdown remarkPlugins={[remarkGfm]}>{entries[0][1]}</Markdown></div>;
                }
                // 多字段：逐个渲染
                const hasRichContent = entries.some(([, v]) => typeof v === 'string' && (v.includes('|') || v.includes('#') || v.includes('**') || v.includes('\n')));
                if (hasRichContent) {
                  return entries.map(([k, v]) => (
                    <div key={k} style={{ marginBottom: 16 }}>
                      {entries.length > 1 && <Text strong>{k}</Text>}
                      {typeof v === 'string' ? (
                        <div className="markdown-output"><Markdown remarkPlugins={[remarkGfm]}>{v}</Markdown></div>
                      ) : (
                        <pre style={{ margin: '4px 0 0', fontSize: 13, background: '#f5f5f5', padding: 12, borderRadius: 6 }}>{JSON.stringify(v, null, 2)}</pre>
                      )}
                    </div>
                  ));
                }
                return <pre style={{ margin: 0, fontSize: 13, background: '#f5f5f5', padding: 16, borderRadius: 6 }}>{JSON.stringify(output, null, 2)}</pre>;
              })()}
            </div>
          </div>
        )}
        {result?.status === 'failed' && (
          <div>
            <Text type="secondary">Task ID: {result.task_id}</Text>
            <div style={{ marginTop: 12, padding: 16, background: '#fff2f0', borderRadius: 6, border: '1px solid #ffccc7' }}>
              <Text type="danger">{result.error}</Text>
            </div>
          </div>
        )}
        {result?.status === 'input_required' && result.question && (
          <div>
            <Text type="secondary">Task ID: {result.task_id}</Text>
            <div style={{ marginTop: 12, padding: 16, background: '#e6f7ff', borderRadius: 6, border: '1px solid #91d5ff' }}>
              <Paragraph strong>{result.question.prompt}</Paragraph>
              {result.question.options && (
                <div>可选项：{result.question.options.map(o => <Tag key={o} color="blue">{o}</Tag>)}</div>
              )}
              <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
                请在输入框中填写回答后点击"回复追问"
              </Paragraph>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
