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

export interface UserBrief {
  id: number
  username: string
  nickname: string
  avatar: string
  email_verified?: boolean
}

export interface PublicUserProfile {
  id: number
  username: string
  nickname: string
  avatar: string
  bio: string
  favorites_public: boolean
  email_verified?: boolean
  created_at: string
}

export interface UserInfo {
  id: number
  username: string
  nickname: string
  email?: string
  email_verified?: boolean
  avatar: string
  bio?: string
  favorites_public?: boolean
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
  email_code: string
  nickname?: string
}

export interface LoginResult {
  token: string
  expires_in?: number
  user: UserInfo
}

export interface UpdateProfilePayload {
  nickname: string
  avatar?: string
  bio?: string
  favorites_public?: boolean
}

export interface BindEmailPayload {
  email: string
  email_code: string
  current_password: string
}

export interface UpdatePasswordPayload {
  old_password: string
  new_password: string
}

export type DomainRole = 'owner' | 'admin' | 'member'
export type DomainVisibility = 'public' | 'private' | 'org_private'
export type DomainJoinMode = 'approval_only' | 'approval_or_quiz'

export interface DomainItem {
  id: number
  domain_uid: string
  domain_code: string
  domain_name: string
  description: string
  visibility: DomainVisibility
  join_mode?: DomainJoinMode
  join_question?: string
  has_join_answer?: boolean
  allow_member_kb_write?: boolean
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
  join_mode?: DomainJoinMode
  join_question?: string
  join_answer?: string
}

export interface UpdateDomainJoinSettingsPayload {
  join_mode: DomainJoinMode
  join_question?: string
  join_answer?: string
}

export interface UpdateDomainPayload {
  domain_name: string
  description: string
  visibility: DomainVisibility
  allow_member_kb_write?: boolean
}

export type OrgVisibility = 'public' | 'private'

export interface UpdateOrgPayload {
  org_name: string
  description: string
  visibility?: OrgVisibility
}

export type OrgRole = 'owner' | 'admin' | 'member'

export interface OrgItem {
  id: number
  org_code: string
  org_name: string
  description: string
  visibility?: OrgVisibility
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
  visibility?: OrgVisibility
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

export interface KbUserSummary {
  id: number
  username: string
  nickname: string
  avatar: string
}

export interface KbDocument {
  id: number
  kb_asset_id?: number
  file_name: string
  file_ext?: string
  file_size: number
  folder_path: string
  is_folder: boolean
  parse_status?: KbParseStatus
  error_msg?: string
  chunk_count?: number
  uploader_id?: number
  uploader?: UserBrief | KbUserSummary
  created_at?: string
  updated_at?: string
}

export interface KbDocumentListResponse {
  list: KbDocument[]
  items?: KbDocument[]
  users?: Record<number, KbUserSummary>
  total?: number
  has_more?: boolean
  next_offset?: number
}

export interface KbContentResult {
  id: number
  file_name: string
  folder_path: string
  file_size: number
  content: string
  is_binary?: boolean
  obfuscated?: boolean
  payload?: string
}

export interface KbBilibiliPreview {
  available: boolean
  status_code: number
  bvid: string
  title: string
  cover: string
  embed_url: string
  watch_url: string
  message: string
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

export type ApprovalTargetType =
  | 'asset_access'
  | 'asset_publish'
  | 'org_join'
  | 'domain_join'
  | 'cross_domain_ref'

export interface ApprovalRequest {
  id: number
  request_no: string
  applicant_id: number
  applicant_name?: string
  applicant_domain_id: number
  target_type: ApprovalTargetType | string
  asset_id: number
  asset_name?: string
  asset_code?: string
  asset_type?: AssetType
  domain_id?: number
  domain_name?: string
  org_id?: number
  org_name?: string
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

export interface AuditLogItem {
  id: number
  created_at: string
  operator_id: number
  operator_name: string
  domain_id: number
  domain_name: string
  asset_id?: number
  asset_code?: string
  asset_name?: string
  asset_type: AssetType | string
  action: string
  target_name?: string
  detail?: string
  client_ip?: string
}

export interface AuditLogQuery {
  asset_type?: AssetType | 'domain' | 'org' | ''
  action?: string
  keyword?: string
  domain_id?: number
  page?: number
  page_size?: number
}

export interface SevenOReleaseItem {
  file_name: string
  platform: string
  kind: string
  size: number
  sha256: string
  updated_at: string
}

export interface SevenOInspectResult {
  kind: string
  detail: string
}

export type CrossRefType = 'asset' | 'kb_file'
export type CrossRefStatus = 'pending' | 'active' | 'rejected'

export interface DomainCrossRefItem {
  id: number
  target_domain_id: number
  source_domain_id: number
  source_domain_name: string
  ref_type: CrossRefType
  source_asset_id: number
  source_doc_id: number
  title: string
  description: string
  created_by: number
  creator_name: string
  status: CrossRefStatus
  approval_request_id: number | null
  created_at: string
  file_ext?: string
  file_size?: number
  asset_type?: string
  kb_name?: string
  folder_path?: string
}

export interface CreateCrossRefPayload {
  source_domain_id: number
  ref_type: CrossRefType
  source_asset_id: number
  source_doc_id: number
  description: string
}

export interface CrossRefSourceKbFile {
  id: number
  kb_asset_id: number
  kb_name: string
  file_name: string
  file_ext: string
  file_size: number
  folder_path: string
}

export interface CrossRefSourceItemsResult {
  assets: AssetItem[]
  kb_files: CrossRefSourceKbFile[]
}

export interface UserFavoriteItem {
  id: number
  kb_asset_id: number
  doc_id: number
  domain_id: number
  domain_name: string
  kb_name: string
  file_name: string
  folder_path: string
  file_ext: string
  is_folder?: boolean
  created_at: string
}

export interface ContactSourceItem {
  type: 'domain' | 'org' | string
  id: number
  name: string
}

export interface MessageContactItem extends UserBrief {
  sources?: Array<ContactSourceItem | string>
}

export interface AssetCardPayload {
  asset_id: number
  asset_name: string
  asset_type: AssetType | string
  domain_id: number
  domain_name?: string
  doc_id?: number
  doc_name?: string
  is_folder?: boolean
  ref_domain_id?: number
  cross_ref_id?: number
  note?: string
  url?: string
}

export interface ShareAssetCardPayload {
  recipient_ids: number[]
  asset_id: number
  doc_id?: number
  ref_domain_id?: number
  cross_ref_id?: number
  note?: string
}

export interface AssetAccessCheckResult {
  accessible: boolean
  reason?: string
}

export interface ConversationItem {
  id: number
  peer: UserBrief
  last_message_preview: string
  last_message_at: string | null
  unread_count: number
  updated_at: string
}

export interface SystemNoticePayload {
  title?: string
  content?: string
  body?: string
  domain_id?: number
  domain_name?: string
  action_label?: string
  action_path?: string
  [key: string]: unknown
}

export interface UserMessageItem {
  id: number
  conversation_id: number
  sender_id: number
  receiver_id: number
  content: string
  msg_type?: 'text' | 'asset_card' | 'system_notice' | string
  payload?: AssetCardPayload | SystemNoticePayload | Record<string, unknown> | null
  is_read: boolean
  created_at: string
}

export interface SendMessagePayload {
  content?: string
  msg_type?: 'text' | 'asset_card' | 'system_notice' | string
  payload?: string | AssetCardPayload | SystemNoticePayload | Record<string, unknown>
}

export interface KbDocumentInteractions {
  like_count: number
  liked_by_me: boolean
  like_users: UserBrief[]
  favorited_by_me: boolean
  comment_count: number
}

export interface KbCommentItem {
  id: number
  kb_asset_id: number
  doc_id: number
  user_id: number
  author: UserBrief
  parent_id?: number | null
  quote_text: string
  content: string
  can_delete: boolean
  created_at: string
}

export interface KbReadingProgress {
  current_page: number
  total_pages: number
  updated_at: string | null
}

export interface CopyKbDocumentRequest {
  target_folder_path: string
  new_name?: string
  target_kb_id?: number
  ref_domain_id?: number
}

export interface EditableKbTarget {
  domain_id: number
  domain_name: string
  kb_id: number
  kb_name: string
}

