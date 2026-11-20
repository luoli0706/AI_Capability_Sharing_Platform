export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  request_id?: string
}

export interface PaginatedData<T> {
  items?: T[]
  list?: T[]
  total: number
  page?: number
  page_size?: number
}

export interface UserInfo {
  id: number
  username: string
  nickname: string
  email: string
  avatar: string
  roles: string[]
  status?: number
  created_at?: string
}

export interface LoginPayload {
  username: string
  password: string
}

export interface RegisterPayload {
  username: string
  password: string
  email: string
  nickname?: string
}

export interface LoginResult {
  token: string
  expires_in?: number
  user: UserInfo
}

export interface UpdateProfilePayload {
  nickname: string
  email: string
  avatar?: string
}

export interface UpdatePasswordPayload {
  old_password: string
  new_password: string
}

export type DomainRole = 'owner' | 'admin' | 'member'

export interface DomainItem {
  id: number
  domain_code: string
  domain_name: string
  description: string
  owner_id: number
  status?: number
  member_count?: number
  my_role?: DomainRole | string
  created_at?: string
  updated_at?: string
}

export interface CreateDomainPayload {
  domain_code: string
  domain_name: string
  description: string
}

export interface DomainMember {
  id?: number
  domain_id?: number
  user_id: number
  username: string
  nickname: string
  email?: string
  avatar?: string
  domain_role: DomainRole | string
  joined_at?: string
}

export interface AddDomainMemberPayload {
  username: string
  domain_role: 'admin' | 'member'
}

export type AssetType = 'mcp' | 'skill' | 'agent' | 'kb'
export type AssetVisibility = 'private' | 'domain' | 'public'
export type AssetStatus = 'draft' | 'pending' | 'published' | 'deprecated'

export interface McpConfig {
  transport_type: 'sse' | 'streamable_http' | 'stdio'
  endpoint_url: string
  auth_type: 'none' | 'bearer' | 'header'
  auth_config_enc?: string
  tools_schema?: unknown[]
  health_status?: string
  last_checked_at?: string
}

export interface SkillConfig {
  skill_type: 'prompt' | 'script' | 'workflow'
  content: string
  runtime_env: string
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  dependencies?: string[] | Record<string, unknown>
}

export interface AgentConfig {
  model_provider: string
  model_name: string
  system_prompt: string
  temperature: number
  max_tokens: number
  bound_mcp_ids?: number[]
  bound_skill_ids?: number[]
  bound_kb_ids?: number[]
}

export interface KbConfig {
  chunk_size: number
  chunk_overlap: number
  embedding_model: string
  doc_count?: number
  total_chunks?: number
}

export interface AssetItem {
  id: number
  asset_code: string
  asset_type: AssetType
  name: string
  summary: string
  tags: string[]
  domain_id: number
  domain_name?: string
  owner_id: number
  owner_name?: string
  owner?: {
    id: number
    username: string
    nickname: string
  }
  visibility: AssetVisibility
  current_version: string
  status: AssetStatus | string
  invoke_count: number
  created_at?: string
  updated_at?: string
  mcp_config?: McpConfig
  skill_config?: SkillConfig
  agent_config?: AgentConfig
  kb_config?: KbConfig
}

export interface AssetQueryParams {
  scope?: 'marketplace' | 'workspace'
  asset_type?: AssetType | ''
  domain_id?: number | ''
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface CreateAssetPayload {
  asset_code: string
  asset_type: AssetType
  name: string
  summary: string
  tags: string[]
  domain_id: number
  visibility: AssetVisibility
  mcp_config?: McpConfig
  skill_config?: SkillConfig
  agent_config?: AgentConfig
  kb_config?: KbConfig
}

export type UpdateAssetPayload = Omit<CreateAssetPayload, 'asset_type' | 'asset_code'> & {
  asset_code?: string
}
