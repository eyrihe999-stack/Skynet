import { useEffect, useState } from 'react';
import { Table, Tag, Input, Typography, Space } from 'antd';
import { api } from '../api/client';
import type { Invocation } from '../types';

const { Title } = Typography;

const statusColors: Record<string, string> = {
  completed: 'green', failed: 'red', submitted: 'blue',
  working: 'processing', input_required: 'orange', cancelled: 'default',
};

export default function Invocations() {
  const [invocations, setInvocations] = useState<Invocation[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [targetAgent, setTargetAgent] = useState('');

  useEffect(() => { loadData(); }, [page, targetAgent]);

  const loadData = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page), page_size: '20' });
      if (targetAgent) params.set('target_agent_id', targetAgent);
      const resp = await api.listInvocations(params.toString());
      setInvocations(resp.data.items || []);
      setTotal(resp.data.total || 0);
    } catch (e) { /* ignore */ }
    finally { setLoading(false); }
  };

  const columns = [
    { title: 'Task ID', dataIndex: 'task_id', key: 'task_id', ellipsis: true, width: 280 },
    { title: '目标 Agent', dataIndex: 'target_agent_id', key: 'target_agent_id' },
    { title: 'Skill', dataIndex: 'skill_name', key: 'skill_name' },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Tag color={statusColors[s] || 'default'}>{s}</Tag> },
    { title: '延迟', dataIndex: 'latency_ms', key: 'latency_ms',
      render: (v: number | null) => v != null ? `${v}ms` : '-' },
    { title: '时间', dataIndex: 'created_at', key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString() },
  ];

  return (
    <div>
      <Title level={4}>调用历史</Title>
      <Space style={{ marginBottom: 16 }}>
        <Input.Search placeholder="按 Agent ID 筛选" allowClear style={{ width: 300 }}
          onSearch={v => { setTargetAgent(v); setPage(1); }} enterButton="筛选" />
      </Space>
      <Table rowKey="id" columns={columns} dataSource={invocations} loading={loading} size="middle"
        pagination={{ current: page, total, pageSize: 20, onChange: setPage }} />
    </div>
  );
}
