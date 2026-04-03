import { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Typography, message, Popconfirm } from 'antd';
import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { api } from '../api/client';
import type { Approval } from '../types';

const { Title } = Typography;

export default function Approvals() {
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => { loadData(); }, [page]);

  const loadData = async () => {
    setLoading(true);
    try {
      const resp = await api.listApprovals(`page=${page}&page_size=20`);
      setApprovals(resp.data.items || []);
      setTotal(resp.data.total || 0);
    } catch (e) { /* ignore */ }
    finally { setLoading(false); }
  };

  const handleDecide = async (id: number, action: 'approve' | 'deny') => {
    try {
      await api.decideApproval(id, action);
      message.success(action === 'approve' ? '已批准' : '已拒绝');
      loadData();
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '调用 ID', dataIndex: 'invocation_id', key: 'invocation_id' },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Tag color={s === 'pending' ? 'orange' : s === 'approved' ? 'green' : 'red'}>{s}</Tag> },
    { title: '过期时间', dataIndex: 'expires_at', key: 'expires_at',
      render: (v: string) => new Date(v).toLocaleString() },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString() },
    { title: '操作', key: 'action',
      render: (_: any, r: Approval) => r.status === 'pending' ? (
        <Space>
          <Popconfirm title="确认批准？" onConfirm={() => handleDecide(r.id, 'approve')}>
            <Button type="primary" size="small" icon={<CheckOutlined />}>批准</Button>
          </Popconfirm>
          <Popconfirm title="确认拒绝？" onConfirm={() => handleDecide(r.id, 'deny')}>
            <Button danger size="small" icon={<CloseOutlined />}>拒绝</Button>
          </Popconfirm>
        </Space>
      ) : '-',
    },
  ];

  return (
    <div>
      <Title level={4}>审批队列</Title>
      <Table rowKey="id" columns={columns} dataSource={approvals} loading={loading} size="middle"
        pagination={{ current: page, total, pageSize: 20, onChange: setPage }} />
    </div>
  );
}
