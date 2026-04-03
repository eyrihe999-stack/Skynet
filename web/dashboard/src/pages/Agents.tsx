import { useEffect, useState } from 'react';
import { Table, Tag, Select, Switch, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { Agent } from '../types';

const { Title } = Typography;

export default function Agents() {
  const navigate = useNavigate();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [mine, setMine] = useState(false);

  useEffect(() => { loadData(); }, [page, status, mine]);

  const loadData = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page), page_size: '20' });
      if (status) params.set('status', status);
      if (mine) params.set('mine', 'true');
      const resp = await api.listAgents(params.toString());
      setAgents(resp.data.items || []);
      setTotal(resp.data.total || 0);
    } catch (e) { /* ignore */ }
    finally { setLoading(false); }
  };

  const columns = [
    { title: 'Agent ID', dataIndex: 'agent_id', key: 'agent_id',
      render: (id: string) => <a onClick={() => navigate(`/agents/${id}`)}>{id}</a> },
    { title: '名称', dataIndex: 'display_name', key: 'display_name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Tag color={s === 'online' ? 'green' : s === 'offline' ? 'default' : 'red'}>{s}</Tag> },
    { title: 'Skill 数', key: 'skills', render: (_: any, r: Agent) => r.capabilities?.length || 0 },
    { title: '版本', dataIndex: 'version', key: 'version' },
  ];

  return (
    <div>
      <Title level={4}>Agent 管理</Title>
      <Space style={{ marginBottom: 16 }}>
        <Select value={status} onChange={setStatus} style={{ width: 120 }} placeholder="状态" allowClear
          options={[{ value: 'online', label: '在线' }, { value: 'offline', label: '离线' }]} />
        <Switch checked={mine} onChange={setMine} checkedChildren="我的" unCheckedChildren="全部" />
      </Space>
      <Table rowKey="id" columns={columns} dataSource={agents} loading={loading}
        pagination={{ current: page, total, pageSize: 20, onChange: setPage }} size="middle" />
    </div>
  );
}
