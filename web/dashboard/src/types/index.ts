export interface User {
  id: number;
  email: string;
  display_name: string;
  avatar_url?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Agent {
  id: number;
  agent_id: string;
  owner_id: number;
  display_name: string;
  description: string;
  avatar_url?: string;
  connection_mode: string;
  endpoint_url?: string;
  status: 'online' | 'offline' | 'removed';
  last_heartbeat_at?: string;
  framework_version?: string;
  version: string;
  data_policy?: Record<string, any>;
  capabilities?: Capability[];
  created_at: string;
  updated_at: string;
}

export interface Capability {
  id: number;
  agent_id: string;
  name: string;
  display_name: string;
  description: string;
  category: string;
  tags?: string[];
  input_schema: any;
  output_schema?: any;
  visibility: 'public' | 'restricted' | 'private';
  approval_mode: 'auto' | 'manual';
  multi_turn: boolean;
  estimated_latency_ms?: number;
  call_count: number;
  success_count: number;
  created_at: string;
  updated_at: string;
}

export interface Invocation {
  id: number;
  task_id: string;
  caller_agent_id?: string;
  caller_user_id?: number;
  target_agent_id: string;
  skill_name: string;
  status: string;
  mode: string;
  error_message?: string;
  latency_ms?: number;
  created_at: string;
  completed_at?: string;
}

export interface Approval {
  id: number;
  invocation_id: number;
  owner_id: number;
  status: 'pending' | 'approved' | 'denied' | 'expired';
  decided_at?: string;
  expires_at: string;
  created_at: string;
}

export interface InvokeResponse {
  task_id: string;
  status: string;
  output?: any;
  error?: string;
  question?: {
    field: string;
    prompt: string;
    options?: string[];
  };
}

export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface PaginatedData<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}
