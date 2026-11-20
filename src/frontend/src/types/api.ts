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
export type DomainVisibility = 'public' | 'private'

export interface DomainItem {
  id: number
  domain_uid: string
  domain_code: string
  domain_name: string
  description: string
  visibility: DomainVisibility
  org_id: number
  org_name: string
  owner_id: number
  status?: number
  member_count?: number
  my_role?: DomainRole | string
  can_manage: boolean
  has_org_access: boolean
  created_at?: string
  updated_at?: string
}

export interface CreateDomainPayload {
  domain_code: string
  domain_name: string
  description: string
  visibility: DomainVisibility
  org_id?: number
}

export interface UpdateDomainPayload {
  domain_name: string
  description: string
  visibility: DomainVisibility
}

export type OrgRole = 'owner' | 'admin' | 'member'

export interface OrgItem {
  id: number
  org_code: string
  org_name: string
  description: string
  owner_id: number
  status?: number
  member_count?: number
  domain_count?: number
  public_domain_count?: number
  my_role?: OrgRole | string
  created_at?: string
  updated_at?: string
}

export interface OrgMember {
  id?: number
  org_id?: number
  user_id: number
  username: string
  nickname: string
  email?: string
  org_role: OrgRole | string
  joined_at?: string
}

export interface OrgDetail extends OrgItem {
  members?: OrgMember[]
  public_domains?: DomainItem[]
}

export interface CreateOrgPayload {
  org_code: string
  org_name: string
  description: string
}

export interface AddOrgMemberPayload {
  username: string
  org_role: 'admin' | 'member'
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
  source_url?: string
  package_command?: string
}

export interface McpImportItem {
  slug: string
  registry_name: string
  name: string
  description: string
  version: string
  transport_type: 'sse' | 'streamable_http' | 'stdio'
  endpoint_url: string
  auth_type: 'none' | 'bearer' | 'header'
  package_command: string
  source_url: string
}

export interface SkillConfig {
  skill_type: 'prompt' | 'script' | 'workflow'
  content: string
  runtime_env: string
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  dependencies?: string[] | Record<string, unknown>
  source_url?: string
  package_path?: string
  package_files?: string[]
}

export interface ParsedSkillMeta {
  name?: string
  asset_code?: string
  summary?: string
  skill_type?: 'prompt' | 'script' | 'workflow'
  content?: string
  runtime_env?: string
  source_url?: string
  package_files?: string[]
  tags?: string[]
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
  source_url?: string
}

export interface AgentImportResult {
  slug?: string
  name?: string
  description?: string
  model_provider?: string
  model_name?: string
  system_prompt?: string
  temperature?: number
  max_tokens?: number
  tags?: string[]
  source_url?: string
  detected_format?: string
}

export interface KbConfig {
  chunk_size: number
  chunk_overlap: number
  embedding_model?: string
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

export interface PublishAssetPayload {
  version?: string
  changelog?: string
}

export interface AssetVersion {
  id: number
  asset_id: number
  version: string
  changelog: string
  snapshot_json?: unknown
  publisher_id: number
  publisher_name?: string
  is_active: number | boolean
  created_at: string
}

export type KbParseStatus = 'pending' | 'processing' | 'completed' | 'failed'

export interface KbDocument {
  id: number
  kb_asset_id: number
  file_name: string
  file_ext: string
  file_size: number
  folder_path: string
  is_folder: boolean
  parse_status: KbParseStatus
  error_msg?: string
  chunk_count: number
  uploader_id: number
  created_at: string
  updated_at?: string
}

export interface KbContentResult {
  id: number
  file_name: string
  folder_path: string
  file_size: number
  content: string
}

export interface KbDownloadPreview {
  scope: 'file' | 'folder' | 'all'
  target: string
  file_count: number
  total_bytes: number
  files?: Array<{
    id: number
    file_name: string
    folder_path: string
    file_size: number
  }>
}

export interface KbChunk {
  id: number
  kb_asset_id: number
  doc_id: number
  chunk_index: number
  content: string
  token_count: number
  metadata?: Record<string, unknown>
  score?: number
  file_name?: string
}

export interface ApprovalRecord {
  id: number
  request_id: number
  step_no: number
  step_name: string
  approver_id: number
  approver_name?: string
  action: 'pending' | 'approve' | 'reject'
  comment: string
  handled_at?: string
  created_at?: string
}

export interface ApprovalRequest {
  id: number
  request_no: string
  applicant_id: number
  applicant_name?: string
  applicant_domain_id: number
  target_type: string
  asset_id: number
  asset_name?: string
  asset_code?: string
  asset_type?: AssetType
  requested_perm: string
  duration_days: number
  reason: string
  current_step: number
  total_steps: number
  status: 'pending' | 'approved' | 'rejected' | 'cancelled'
  records?: ApprovalRecord[]
  created_at: string
  updated_at?: string
}

export interface CreateApprovalPayload {
  asset_id: number
  requested_perm?: 'invoke' | 'edit' | 'admin'
  duration_days: number
  reason: string
  applicant_domain_id?: number
}

export interface HandleApprovalPayload {
  action: 'approve' | 'reject'
  comment?: string
}

export interface UserApiKey {
  id: number
  user_id?: number
  key_name: string
  key_prefix: string
  expires_at?: string | null
  status: number
  last_used_at?: string | null
  created_at: string
}

export interface CreateApiKeyPayload {
  key_name: string
  expires_days?: number
}

export interface CreateApiKeyResult {
  api_key: string | UserApiKey
  key_info?: UserApiKey
  plain_key?: string
  key?: string
}

export interface InvokeLogItem {
  id: number
  trace_id: string
  asset_id: number
  asset_name?: string
  asset_code?: string
  asset_type: AssetType
  asset_version: string
  caller_user_id: number
  auth_mode: 'jwt' | 'api_key'
  status_code: number
  is_success: number | boolean
  latency_ms: number
  error_message?: string
  client_ip?: string
  created_at: string
}

