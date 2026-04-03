import { useEffect, useState, useCallback } from 'react';
import { Card, Table, Tag, Typography, Space, Button, Badge, Empty, Spin, Descriptions, message } from 'antd';
import { ReloadOutlined, CheckCircleOutlined, CloseCircleOutlined, SyncOutlined, ClockCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import { api } from '../api/client';
import { useEvents } from '../hooks/useEvents';
import type { Invocation } from '../types';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Title, Text } = Typography;

const statusConfig: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  submitted: { color: 'blue', icon: <ClockCircleOutlined />, label: '已提交' },
  working: { color: 'processing', icon: <SyncOutlined spin />, label: '执行中' },
  input_required: { color: 'orange', icon: <LoadingOutlined />, label: '等待输入' },
  completed: { color: 'green', icon: <CheckCircleOutlined />, label: '已完成' },
  failed: { color: 'red', icon: <CloseCircleOutlined />, label: '失败' },
  cancelled: { color: 'default', icon: <CloseCircleOutlined />, label: '已取消' },
  pending: { color: 'gold', icon: <ClockCircleOutlined />, label: '待审批' },
};

function StatusTag({ status }: { status: string }) {
  const cfg = statusConfig[status] || { color: 'default', icon: null, label: status };
  return <Tag color={cfg.color} icon={cfg.icon}>{cfg.label}</Tag>;
}

function timeSince(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return new Date(dateStr).toLocaleString();
}

export default function Tasks() {
  const [activeTasks, setActiveTasks] = useState<Invocation[]>([]);
  const [recentTasks, setRecentTasks] = useState<Invocation[]>([]);
  const [loading, setLoading] = useState(false);
  const [expandedResult, setExpandedResult] = useState<string | null>(null);
  const [taskResult, setTaskResult] = useState<any>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      // 加载进行中的任务
      const [activeResp, recentResp] = await Promise.all([
        api.listInvocations('mine=true&page_size=50'),
        api.listInvocations('mine=true&page_size=20'),
      ]);

      const all: Invocation[] = activeResp.data.items || [];
      const activeStatuses = new Set(['submitted', 'working', 'input_required', 'pending']);

      setActiveTasks(all.filter(t => activeStatuses.has(t.status)));

      const recent: Invocation[] = recentResp.data.items || [];
      setRecentTasks(recent.filter(t => !activeStatuses.has(t.status)).slice(0, 20));
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  // SSE 实时刷新
  useEvents(useCallback((event) => {
    if (event.type === 'invoke_status') {
      loadData();
      const { status, skill, target_agent } = event.data;
      if (status === 'completed') {
        message.success(`${target_agent}.${skill} 执行完成`);
      } else if (status === 'failed') {
        message.error(`${target_agent}.${skill} 执行失败`);
      }
    }
  }, [loadData]));

  const viewResult = async (taskId: string) => {
    if (expandedResult === taskId) {
      setExpandedResult(null);
      setTaskResult(null);
      return;
    }
    try {
      const resp = await api.getTask(taskId);
      setExpandedResult(taskId);
      setTaskResult(resp.data);
    } catch {
      message.error('获取任务详情失败');
    }
  };

  const activeColumns = [
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 120,
      render: (s: string) => <StatusTag status={s} />,
    },
    { title: '目标 Agent', dataIndex: 'target_agent_id', key: 'target_agent_id' },
    { title: 'Skill', dataIndex: 'skill_name', key: 'skill_name' },
    {
      title: '发起时间', dataIndex: 'created_at', key: 'created_at',
      render: (v: string) => <Text type="secondary">{timeSince(v)}</Text>,
    },
    {
      title: 'Task ID', dataIndex: 'task_id', key: 'task_id', ellipsis: true, width: 280,
      render: (v: string) => <Text copyable={{ text: v }} style={{ fontSize: 12 }}>{v.slice(0, 8)}...</Text>,
    },
  ];

  const recentColumns = [
    ...activeColumns,
    {
      title: '延迟', dataIndex: 'latency_ms', key: 'latency_ms', width: 80,
      render: (v: number | null) => v != null ? <Text type="secondary">{v}ms</Text> : '-',
    },
    {
      title: '', key: 'action', width: 80,
      render: (_: any, record: Invocation) => (
        record.status === 'completed' || record.status === 'failed' ? (
          <Button type="link" size="small" onClick={() => viewResult(record.task_id)}>
            {expandedResult === record.task_id ? '收起' : '详情'}
          </Button>
        ) : null
      ),
    },
  ];

  const activeCount = activeTasks.length;

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between', display: 'flex' }}>
        <Title level={4} style={{ margin: 0 }}>
          任务看板
          {activeCount > 0 && <Badge count={activeCount} style={{ marginLeft: 8 }} />}
        </Title>
        <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>刷新</Button>
      </Space>

      {/* 进行中的任务 */}
      <Card
        title={
          <Space>
            <SyncOutlined spin={activeCount > 0} />
            <span>进行中</span>
            <Tag>{activeCount}</Tag>
          </Space>
        }
        style={{ marginBottom: 24 }}
        styles={{ body: activeCount === 0 ? { padding: '12px 24px' } : undefined }}
      >
        {loading && activeTasks.length === 0 ? (
          <Spin />
        ) : activeCount === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无进行中的任务" />
        ) : (
          <Table
            rowKey="id"
            columns={activeColumns}
            dataSource={activeTasks}
            pagination={false}
            size="middle"
          />
        )}
      </Card>

      {/* 最近完成的任务 */}
      <Card
        title={<Space><CheckCircleOutlined /><span>最近完成</span></Space>}
      >
        <Table
          rowKey="id"
          columns={recentColumns}
          dataSource={recentTasks}
          pagination={false}
          size="middle"
          expandable={{
            expandedRowKeys: expandedResult ? [recentTasks.find(t => t.task_id === expandedResult)?.id || 0] : [],
            expandedRowRender: () => taskResult ? (
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="状态"><StatusTag status={taskResult.status} /></Descriptions.Item>
                {taskResult.output && (
                  <Descriptions.Item label="输出">
                    <div style={{ maxHeight: 400, overflow: 'auto' }}>
                      {typeof taskResult.output === 'string' ? (
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{taskResult.output}</ReactMarkdown>
                      ) : (
                        <pre style={{ margin: 0, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                          {JSON.stringify(taskResult.output, null, 2)}
                        </pre>
                      )}
                    </div>
                  </Descriptions.Item>
                )}
                {taskResult.error && (
                  <Descriptions.Item label="错误">
                    <Text type="danger">{taskResult.error}</Text>
                  </Descriptions.Item>
                )}
              </Descriptions>
            ) : <Spin />,
            showExpandColumn: false,
          }}
        />
      </Card>
    </div>
  );
}
