import { useState } from 'react';
import { Card, Button, Typography, message, Alert, Modal, Space } from 'antd';
import { KeyOutlined, CopyOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import { api } from '../api/client';

const { Title, Text } = Typography;

export default function ApiKeys() {
  const [newKey, setNewKey] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleRegenerate = () => {
    Modal.confirm({
      title: '确认重新生成 API Key？',
      icon: <ExclamationCircleOutlined />,
      content: '当前 API Key 将立即失效，所有使用旧 Key 的 Agent 和 CLI 都需要更新。',
      okText: '确认生成',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setLoading(true);
        try {
          const resp = await api.regenerateKey();
          setNewKey(resp.data.api_key);
          message.success('API Key 已重新生成');
        } catch (err: any) {
          message.error(err.message || '生成失败');
        } finally {
          setLoading(false);
        }
      },
    });
  };

  const copyKey = () => {
    if (newKey) {
      navigator.clipboard.writeText(newKey);
      message.success('已复制到剪贴板');
    }
  };

  return (
    <div>
      <Title level={4}>API Key 管理</Title>

      <Card style={{ marginBottom: 24 }}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong>你的 API Key</Text>
            <br />
            <Text type="secondary">
              API Key 用于 CLI 工具和 Agent 连接平台。出于安全原因，Key 以哈希形式存储，无法查看当前 Key 的明文。
              如果忘记了 Key，可以重新生成。
            </Text>
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
            loading={loading}
          >
            重新生成 API Key
          </Button>
        </Space>
      </Card>

      {newKey && (
        <Card title="新的 API Key（仅显示一次）">
          <Alert
            type="warning"
            message="请立即复制并妥善保存，关闭页面后无法再查看！"
            style={{ marginBottom: 16 }}
          />
          <div style={{
            background: '#f5f5f5', padding: 12, borderRadius: 6,
            wordBreak: 'break-all', fontFamily: 'monospace', fontSize: 14,
          }}>
            {newKey}
          </div>
          <Button type="link" icon={<CopyOutlined />} onClick={copyKey} style={{ marginTop: 8, padding: 0 }}>
            复制 Key
          </Button>
        </Card>
      )}
    </div>
  );
}
