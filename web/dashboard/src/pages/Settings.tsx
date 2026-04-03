import { useEffect, useState } from 'react';
import { Card, Descriptions, Typography, Spin, message } from 'antd';
import { api } from '../api/client';
import type { User } from '../types';

const { Title } = Typography;

export default function Settings() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getProfile()
      .then(resp => setUser(resp.data))
      .catch((e: any) => message.error(e.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Spin />;

  return (
    <div>
      <Title level={4}>设置</Title>
      {user && (
        <Card title="账户信息">
          <Descriptions column={1}>
            <Descriptions.Item label="用户 ID">{user.id}</Descriptions.Item>
            <Descriptions.Item label="邮箱">{user.email}</Descriptions.Item>
            <Descriptions.Item label="显示名称">{user.display_name}</Descriptions.Item>
            <Descriptions.Item label="状态">{user.status}</Descriptions.Item>
            <Descriptions.Item label="注册时间">{new Date(user.created_at).toLocaleString()}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}
    </div>
  );
}
