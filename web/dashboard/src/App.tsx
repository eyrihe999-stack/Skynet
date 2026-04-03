import { Routes, Route, Navigate } from 'react-router-dom';
import { isAuthenticated } from './api/client';
import Layout from './components/Layout';
import Login from './pages/Login';
import Overview from './pages/Overview';
import Agents from './pages/Agents';
import AgentDetail from './pages/AgentDetail';
import Skills from './pages/Skills';
import Invocations from './pages/Invocations';
import Approvals from './pages/Approvals';
import Settings from './pages/Settings';
import ApiKeys from './pages/ApiKeys';
import Guide from './pages/Guide';
import Tasks from './pages/Tasks';

function PrivateRoute({ children }: { children: React.ReactNode }) {
  return isAuthenticated() ? <>{children}</> : <Navigate to="/login" />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/*"
        element={
          <PrivateRoute>
            <Layout>
              <Routes>
                <Route path="/" element={<Overview />} />
                <Route path="/agents" element={<Agents />} />
                <Route path="/agents/:agentId" element={<AgentDetail />} />
                <Route path="/skills" element={<Skills />} />
                <Route path="/tasks" element={<Tasks />} />
                <Route path="/invocations" element={<Invocations />} />
                <Route path="/approvals" element={<Approvals />} />
                <Route path="/api-keys" element={<ApiKeys />} />
                <Route path="/guide" element={<Guide />} />
                <Route path="/settings" element={<Settings />} />
              </Routes>
            </Layout>
          </PrivateRoute>
        }
      />
    </Routes>
  );
}
