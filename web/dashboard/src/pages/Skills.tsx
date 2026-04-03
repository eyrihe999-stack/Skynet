import { useEffect, useState } from 'react';
import { Table, Input, Select, Space, Tag, Typography } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { Capability } from '../types';

const { Title } = Typography;

export default function Skills() {
  const navigate = useNavigate();
  const [skills, setSkills] = useState<Capability[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('');

  useEffect(() => { loadData(); }, [page, query, category]);

  const loadData = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page), page_size: '20' });
      if (query) params.set('q', query);
      if (category) params.set('category', category);
      const resp = await api.searchCapabilities(params.toString());
      setSkills(resp.data.items || []);
      setTotal(resp.data.total || 0);
    } catch (e) { /* ignore */ }
    finally { setLoading(false); }
  };

  const handleSearch = (value: string) => {
    setQuery(value);
    setPage(1);
  };

  const columns = [
    { title: 'Skill 名称', dataIndex: 'display_name', key: 'display_name' },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id',
      render: (id: string) => <a onClick={() => navigate(`/agents/${id}`)}>{id}</a> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true, width: 300 },
    { title: '分类', dataIndex: 'category', key: 'category', render: (c: string) => <Tag>{c}</Tag> },
    { title: '标签', dataIndex: 'tags', key: 'tags',
      render: (tags: string[]) => tags?.map(t => <Tag key={t} color="blue">{t}</Tag>) },
    { title: '调用次数', dataIndex: 'call_count', key: 'call_count', sorter: (a: Capability, b: Capability) => a.call_count - b.call_count },
    { title: '成功率', key: 'success_rate',
      render: (_: any, r: Capability) => r.call_count > 0 ? `${((r.success_count / r.call_count) * 100).toFixed(1)}%` : '-' },
  ];

  return (
    <div>
      <Title level={4}>Skill 市场</Title>
      <Space style={{ marginBottom: 16 }}>
        <Input.Search placeholder="搜索 Skill" onSearch={handleSearch} allowClear style={{ width: 300 }}
          prefix={<SearchOutlined />} enterButton />
        <Select value={category} onChange={v => { setCategory(v); setPage(1); }} style={{ width: 120 }}
          placeholder="分类" allowClear
          options={[
            { value: 'general', label: '通用' }, { value: 'legal', label: '法律' },
            { value: 'data', label: '数据' }, { value: 'design', label: '设计' },
          ]} />
      </Space>
      <Table rowKey="id" columns={columns} dataSource={skills} loading={loading} size="middle"
        pagination={{ current: page, total, pageSize: 20, onChange: setPage }} />
    </div>
  );
}
