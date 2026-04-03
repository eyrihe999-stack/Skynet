const API_BASE = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('token');
}

export function setToken(token: string) {
  localStorage.setItem('token', token);
}

export function clearToken() {
  localStorage.removeItem('token');
}

export function isAuthenticated(): boolean {
  return !!getToken();
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (resp.status === 401) {
    clearToken();
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  const body = await resp.json();

  if (!resp.ok && body.code !== 0) {
    throw new Error(body.message || `Request failed with status ${resp.status}`);
  }

  return body;
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    request<any>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  register: (email: string, password: string, display_name: string) =>
    request<any>('/auth/register', { method: 'POST', body: JSON.stringify({ email, password, display_name }) }),
  getProfile: () => request<any>('/auth/profile'),
  regenerateKey: () => request<any>('/auth/regenerate-key', { method: 'POST' }),

  // Agents
  listAgents: (params?: string) => request<any>(`/agents${params ? '?' + params : ''}`),
  getAgent: (agentId: string) => request<any>(`/agents/${agentId}`),
  deleteAgent: (agentId: string) => request<any>(`/agents/${agentId}`, { method: 'DELETE' }),

  // Capabilities
  searchCapabilities: (params?: string) => request<any>(`/capabilities${params ? '?' + params : ''}`),

  // Invoke
  invoke: (targetAgent: string, skill: string, input: any, timeoutMs = 30000) =>
    request<any>('/invoke', {
      method: 'POST',
      body: JSON.stringify({ target_agent: targetAgent, skill, input, timeout_ms: timeoutMs }),
    }),

  // Tasks
  getTask: (taskId: string) => request<any>(`/tasks/${taskId}`),
  replyTask: (taskId: string, input: any) =>
    request<any>(`/tasks/${taskId}/reply`, { method: 'POST', body: JSON.stringify({ input }) }),
  cancelTask: (taskId: string) =>
    request<any>(`/tasks/${taskId}/cancel`, { method: 'POST' }),

  // Invocations
  listInvocations: (params?: string) => request<any>(`/invocations${params ? '?' + params : ''}`),

  // Approvals
  listApprovals: (params?: string) => request<any>(`/approvals${params ? '?' + params : ''}`),
  decideApproval: (id: number, action: 'approve' | 'deny') =>
    request<any>(`/approvals/${id}`, { method: 'POST', body: JSON.stringify({ action }) }),
};
