import { useEffect, useState } from 'react';
import { Card, Button, Typography, message, Alert, Modal, Space, Spin } from 'antd';
import { KeyOutlined, CopyOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import { api } from '../api/client';
import type { User } from '../types';

const { Title, Text } = Typography;

export default function ApiKeys() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getProfile()
      .then(resp => setUser(resp.data))
      .catch((e: any) => message.error(e.message))
      .finally(() => setLoading(false));
  }, []);

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    message.success('已复制到剪贴板');
  };

  const handleRegenerate = () => {
    Modal.confirm({
      title: '确认重新生成 API Key？',
      icon: <ExclamationCircleOutlined />,
      content: '当前 API Key 将立即失效，所有使用旧 Key 的 Agent 和 CLI 都需要更新。',
      okText: '确认生成',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const resp = await api.regenerateKey();
          setUser(prev => prev ? { ...prev, api_key: resp.data.api_key } : prev);
          message.success('API Key 已重新生成');
        } catch (err: any) {
          message.error(err.message || '生成失败');
        }
      },
    });
  };

  if (loading) return <Spin />;

  return (
    <div>
      <Title level={4}>API Key 管理</Title>

      <Card style={{ marginBottom: 24 }}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong>当前 API Key</Text>
          </div>

          <div style={{
            background: '#f5f5f5', padding: 12, borderRadius: 6,
            wordBreak: 'break-all', fontFamily: 'monospace', fontSize: 14,
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          }}>
            <span>{user?.api_key || '无'}</span>
            {user?.api_key && (
              <Button type="text" icon={<CopyOutlined />} onClick={() => copyKey((user as any).api_key)} />
            )}
          </div>

          <Alert
            type="info"
            message="使用方式"
            description={
              <div style={{ fontFamily: 'monospace', fontSize: 13 }}>
                <div>CLI：SKYNET_API_KEY=sk-xxx skynet invoke ...</div>
                <div>Agent：SKYNET_API_KEY=sk-xxx go run .</div>
                <div>HTTP：curl -H &quot;X-API-Key: sk-xxx&quot; ...</div>
              </div>
            }
          />

          <Button
            type="primary"
            danger
            icon={<KeyOutlined />}
            onClick={handleRegenerate}
          >
            重新生成 API Key
          </Button>
        </Space>
      </Card>
    </div>
  );
}
