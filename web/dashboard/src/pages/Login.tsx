import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Form, Input, Button, Typography, message, Tabs } from 'antd';
import { UserOutlined, LockOutlined, MailOutlined } from '@ant-design/icons';
import { api, setToken } from '../api/client';

const { Title } = Typography;

export default function Login() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);

  const handleLogin = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      const resp = await api.login(values.email, values.password);
      setToken(resp.data.token);
      message.success('登录成功');
      navigate('/');
    } catch (err: any) {
      message.error(err.message || '登录失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (values: { email: string; password: string; display_name: string }) => {
    setLoading(true);
    try {
      await api.register(values.email, values.password, values.display_name);
      // 注册成功后自动登录
      const loginResp = await api.login(values.email, values.password);
      setToken(loginResp.data.token);
      message.success('注册成功');
      navigate('/');
    } catch (err: any) {
      message.error(err.message || '注册失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh',
      background: 'url(/bg.jpeg) center/cover no-repeat fixed',
    }}>
      <Card style={{ width: 420, backdropFilter: 'blur(12px)', background: 'rgba(255,255,255,0.85)', borderRadius: 12, border: 'none', boxShadow: '0 8px 32px rgba(0,0,0,0.3)' }}>
        <Title level={3} style={{ textAlign: 'center', marginBottom: 24 }}>Skynet Platform</Title>
        <Tabs
          centered
          items={[
            {
              key: 'login',
              label: '登录',
              children: (
                <Form onFinish={handleLogin}>
                  <Form.Item name="email" rules={[{ required: true, message: '请输入邮箱' }]}>
                    <Input prefix={<MailOutlined />} placeholder="邮箱" />
                  </Form.Item>
                  <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
                    <Input.Password prefix={<LockOutlined />} placeholder="密码" />
                  </Form.Item>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" loading={loading} block>登录</Button>
                  </Form.Item>
                </Form>
              ),
            },
            {
              key: 'register',
              label: '注册',
              children: (
                <Form onFinish={handleRegister}>
                  <Form.Item name="display_name" rules={[{ required: true, message: '请输入名称' }]}>
                    <Input prefix={<UserOutlined />} placeholder="显示名称" />
                  </Form.Item>
                  <Form.Item name="email" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
                    <Input prefix={<MailOutlined />} placeholder="邮箱" />
                  </Form.Item>
                  <Form.Item name="password" rules={[{ required: true, min: 6, message: '密码至少6位' }]}>
                    <Input.Password prefix={<LockOutlined />} placeholder="密码" />
                  </Form.Item>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" loading={loading} block>注册</Button>
                  </Form.Item>
                </Form>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}
