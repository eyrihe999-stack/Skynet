import { useEffect, useState } from 'react';
import { Row, Col, Card, Statistic, Table, Tag, Typography } from 'antd';
import { RobotOutlined, AppstoreOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { Agent } from '../types';

const { Title } = Typography;

export default function Overview() {
  const navigate = useNavigate();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const resp = await api.listAgents('page=1&page_size=10');
      setAgents(resp.data.items || []);
      setTotal(resp.data.total || 0);
    } catch (e) {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  const onlineCount = agents.filter(a => a.status === 'online').length;
  const skillCount = agents.reduce((sum, a) => sum + (a.capabilities?.length || 0), 0);

  const columns = [
    { title: 'Agent ID', dataIndex: 'agent_id', key: 'agent_id',
      render: (id: string) => <a onClick={() => navigate(`/agents/${id}`)}>{id}</a> },
    { title: '名称', dataIndex: 'display_name', key: 'display_name' },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Tag color={s === 'online' ? 'green' : 'default'}>{s}</Tag> },
    { title: 'Skill 数', key: 'skills',
      render: (_: any, r: Agent) => r.capabilities?.length || 0 },
    { title: '版本', dataIndex: 'version', key: 'version' },
  ];

  return (
    <div>
      <Title level={4}>网络概览</Title>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card><Statistic title="在线 Agent" value={onlineCount} prefix={<RobotOutlined />} /></Card>
        </Col>
        <Col span={8}>
          <Card><Statistic title="总 Skill 数" value={skillCount} prefix={<AppstoreOutlined />} /></Card>
        </Col>
        <Col span={8}>
          <Card><Statistic title="注册 Agent 总数" value={total} prefix={<ThunderboltOutlined />} /></Card>
        </Col>
      </Row>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={agents}
        loading={loading}
        pagination={false}
        size="middle"
      />
    </div>
  );
}
