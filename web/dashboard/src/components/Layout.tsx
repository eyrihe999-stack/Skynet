import { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout as AntLayout, Menu, Button, theme } from 'antd';
import {
  DashboardOutlined,
  RobotOutlined,
  AppstoreOutlined,
  HistoryOutlined,
  AuditOutlined,
  SettingOutlined,
  LogoutOutlined,
  KeyOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import { clearToken } from '../api/client';

const { Sider, Content, Header } = AntLayout;

const menuItems = [
  { key: '/', icon: <DashboardOutlined />, label: '网络概览' },
  { key: '/agents', icon: <RobotOutlined />, label: 'Agent' },
  { key: '/skills', icon: <AppstoreOutlined />, label: 'Skill 市场' },
  { key: '/invocations', icon: <HistoryOutlined />, label: '调用历史' },
  { key: '/approvals', icon: <AuditOutlined />, label: '审批队列' },
  { key: '/api-keys', icon: <KeyOutlined />, label: 'API Key' },
  { key: '/guide', icon: <RocketOutlined />, label: '快速开始' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const { token: { colorBgContainer, borderRadiusLG } } = theme.useToken();

  const handleLogout = () => {
    clearToken();
    navigate('/login');
  };

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed}>
        <div style={{ height: 32, margin: 16, color: '#fff', fontSize: collapsed ? 14 : 18, fontWeight: 'bold', textAlign: 'center', lineHeight: '32px' }}>
          {collapsed ? 'S' : 'Skynet'}
        </div>
        <Menu
          theme="dark"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <AntLayout>
        <Header style={{ padding: '0 24px', background: colorBgContainer, display: 'flex', justifyContent: 'flex-end', alignItems: 'center' }}>
          <Button type="text" icon={<LogoutOutlined />} onClick={handleLogout}>
            退出
          </Button>
        </Header>
        <Content style={{ margin: 24 }}>
          <div style={{ padding: 24, background: colorBgContainer, borderRadius: borderRadiusLG, minHeight: 360 }}>
            {children}
          </div>
        </Content>
      </AntLayout>
    </AntLayout>
  );
}
