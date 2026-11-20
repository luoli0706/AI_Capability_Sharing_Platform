<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Link, Search, UploadFilled } from '@element-plus/icons-vue'
import {
  ElMessage,
  type FormInstance,
  type FormRules,
  type UploadRequestOptions
} from 'element-plus'
import {
  createAsset,
  getAssetDetail,
  getAssets,
  importAgentFromUrl,
  importSkillFromUrl,
  parseAgentFile,
  parseAgentText,
  parseMcpJson,
  parseSkillFile,
  searchMcpRegistry,
  updateAsset,
  uploadSkillPackage
} from '@/api/asset'
import type {
  AgentConfig,
  AgentImportResult,
  AssetItem,
  AssetType,
  AssetVisibility,
  CreateAssetPayload,
  DomainItem,
  KbConfig,
  McpConfig,
  McpImportItem,
  SkillConfig
} from '@/types/api'
import { normalizeList } from '@/utils/format'

const props = defineProps<{
  modelValue: boolean
  editAssetId: number | null
  domains: DomainItem[]
  fixedDomainId?: number | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
})

const isEdit = computed(() => Boolean(props.editAssetId))
const formRef = ref<FormInstance>()
const loadingDetail = ref(false)
const submitting = ref(false)
const tagInput = ref('')

interface AssetFormState {
  asset_code: string
  asset_type: AssetType
  name: string
  summary: string
  tags: string[]
  domain_id: number | undefined
  visibility: AssetVisibility
  mcp_config: McpConfig
  skill_config: SkillConfig
  agent_config: AgentConfig
  kb_config: KbConfig
}

function createInitialForm(): AssetFormState {
  return {
    asset_code: '',
    asset_type: 'mcp',
    name: '',
    summary: '',
    tags: [],
    domain_id: props.fixedDomainId || props.domains[0]?.id,
    visibility: 'domain',
    mcp_config: {
      transport_type: 'sse',
      endpoint_url: '',
      auth_type: 'none',
      source_url: '',
      package_command: ''
    },
    skill_config: {
      skill_type: 'prompt',
      content: '',
      runtime_env: 'python3.11',
      source_url: '',
      package_files: []
    },
    agent_config: {
      model_provider: 'openai',
      model_name: 'gpt-4o-mini',
      system_prompt: '',
      temperature: 0.7,
      max_tokens: 2048,
      bound_mcp_ids: [],
      bound_skill_ids: [],
      bound_kb_ids: [],
      source_url: ''
    },
    kb_config: {
      chunk_size: 500,
      chunk_overlap: 50
    }
  }
}

const domainMcps = ref<AssetItem[]>([])
const domainSkills = ref<AssetItem[]>([])
const domainKbs = ref<AssetItem[]>([])

async function loadBindableAssets(targetDomainId?: number) {
  const did = targetDomainId || form.domain_id || props.fixedDomainId
  if (!did) return
  try {
    const res = await getAssets({
      scope: 'marketplace',
      domain_id: did,
      page: 1,
      page_size: 100
    })
    const items = normalizeList(res).items
    domainMcps.value = items.filter((a) => a.asset_type === 'mcp')
    domainSkills.value = items.filter((a) => a.asset_type === 'skill')
    domainKbs.value = items.filter((a) => a.asset_type === 'kb')
  } catch {
    domainMcps.value = []
    domainSkills.value = []
    domainKbs.value = []
  }
}

const form = reactive<AssetFormState>(createInitialForm())

const rules: FormRules = {
  name: [{ required: true, message: '请输入资产名称', trigger: 'blur' }],
  asset_code: [
    { required: true, message: '请输入资产编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ],
  asset_type: [{ required: true, message: '请选择资产类型', trigger: 'change' }],
  domain_id: [{ required: true, message: '请选择所属业务域', trigger: 'change' }],
  visibility: [{ required: true, message: '请选择可见范围', trigger: 'change' }],
  'mcp_config.endpoint_url': [
    {
      validator: (_rule, val, cb) => {
        if (
          form.asset_type === 'mcp' &&
          form.mcp_config.transport_type !== 'stdio' &&
          !val?.trim()
        ) {
          cb(new Error('请输入 MCP 服务端点 URL'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ],
  'skill_config.content': [
    {
      validator: (_rule, val, cb) => {
        if (form.asset_type === 'skill' && !val?.trim()) {
          cb(new Error('请输入技能内容'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ],
  'agent_config.model_name': [
    {
      validator: (_rule, val, cb) => {
        if (form.asset_type === 'agent' && !val?.trim()) {
          cb(new Error('请输入模型名称'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ]
}

const skillsShUrl = ref('')
const importingUrl = ref(false)
const parsingFile = ref(false)
const pendingSkillFile = ref<File | null>(null)

function slugifyCode(raw: string): string {
  return raw
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
}

function applyParsedSkill(meta: import('@/types/api').ParsedSkillMeta, fallbackUrl?: string) {
  if (meta.name && !form.name) {
    form.name = meta.name
  } else if (meta.name) {
    form.name = meta.name
  }
  if (!isEdit.value) {
    const rawCode = meta.asset_code || (meta.name ? slugifyCode(meta.name) : '')
    if (rawCode) form.asset_code = rawCode
  }
  if (meta.summary) form.summary = meta.summary
  if (meta.skill_type) form.skill_config.skill_type = meta.skill_type
  if (meta.runtime_env) form.skill_config.runtime_env = meta.runtime_env
  if (meta.content !== undefined) form.skill_config.content = meta.content
  if (meta.source_url || fallbackUrl) {
    form.skill_config.source_url = meta.source_url || fallbackUrl || ''
  }
  if (Array.isArray(meta.package_files)) {
    form.skill_config.package_files = meta.package_files
  }
  if (Array.isArray(meta.tags) && meta.tags.length > 0) {
    form.tags = [...new Set([...form.tags, ...meta.tags])]
  }
}

async function handleImportSkillUrl() {
  const url = skillsShUrl.value.trim()
  if (!url) {
    ElMessage.warning('请输入 skills.sh 链接')
    return
  }
  importingUrl.value = true
  try {
    const parsed = await importSkillFromUrl(url)
    applyParsedSkill(parsed, url)
    ElMessage.success('已从 skills.sh 解析并填充技能信息')
  } catch {
    // Handled by interceptor
  } finally {
    importingUrl.value = false
  }
}

async function handleSkillFileRequest(options: UploadRequestOptions) {
  const file = options.file as File
  pendingSkillFile.value = file
  parsingFile.value = true
  try {
    const parsed = await parseSkillFile(file)
    applyParsedSkill(parsed)
    ElMessage.success(`已解析技能包「${file.name}」，提交保存时将自动上传`)
  } catch {
    // Handled by interceptor
  } finally {
    parsingFile.value = false
  }
}

const mcpRegistryQuery = ref('')
const searchingRegistry = ref(false)
const mcpRegistryResults = ref<McpImportItem[]>([])
const showMcpJsonImport = ref(false)
const mcpJsonContent = ref('')
const parsingMcpJson = ref(false)
const mcpParsedCandidates = ref<McpImportItem[]>([])
const mcpEndpointPlaceholder = ref('https://mcp.example.com/sse（Stdio 类型可留空）')

type McpPresetKey = 'streamable_http' | 'sse' | 'stdio_npx' | 'stdio_uvx'

function applyMcpPreset(preset: McpPresetKey) {
  if (preset === 'streamable_http') {
    form.mcp_config.transport_type = 'streamable_http'
    form.mcp_config.auth_type = 'none'
    form.mcp_config.endpoint_url = ''
    mcpEndpointPlaceholder.value = 'https://api.example.com/mcp'
    ElMessage.success('已应用「Streamable HTTP 服务」模板')
  } else if (preset === 'sse') {
    form.mcp_config.transport_type = 'sse'
    form.mcp_config.auth_type = 'bearer'
    form.mcp_config.endpoint_url = ''
    mcpEndpointPlaceholder.value = 'https://api.example.com/sse'
    ElMessage.success('已应用「SSE 流式服务」模板')
  } else if (preset === 'stdio_npx') {
    form.mcp_config.transport_type = 'stdio'
    form.mcp_config.package_command = 'npx -y @modelcontextprotocol/server-github'
    form.mcp_config.endpoint_url = 'stdio://server-github'
    form.mcp_config.auth_type = 'none'
    mcpEndpointPlaceholder.value = 'stdio://server-github'
    ElMessage.success('已应用「Stdio npx 命令」模板')
  } else if (preset === 'stdio_uvx') {
    form.mcp_config.transport_type = 'stdio'
    form.mcp_config.package_command = 'uvx mcp-server-fetch'
    form.mcp_config.endpoint_url = 'stdio://mcp-server-fetch'
    form.mcp_config.auth_type = 'none'
    mcpEndpointPlaceholder.value = 'stdio://mcp-server-fetch'
    ElMessage.success('已应用「Stdio uvx 命令」模板')
  }
}

async function handleSearchMcpRegistry() {
  const q = mcpRegistryQuery.value.trim()
  if (!q) {
    ElMessage.warning('请输入搜索关键词，例如 github、postgres、filesystem')
    return
  }
  searchingRegistry.value = true
  try {
    const res = await searchMcpRegistry(q, 10)
    mcpRegistryResults.value = normalizeList(res).items
    if (mcpRegistryResults.value.length === 0) {
      ElMessage.info('未在官方 MCP Registry 中找到匹配服务')
    }
  } catch {
    mcpRegistryResults.value = []
  } finally {
    searchingRegistry.value = false
  }
}

function applyMcpImportItem(item: McpImportItem) {
  if (item.name) form.name = item.name
  if (!isEdit.value) {
    const code = item.slug || (item.name ? slugifyCode(item.name) : '')
    if (code) form.asset_code = code
  }
  if (item.description) form.summary = item.description
  if (item.transport_type) form.mcp_config.transport_type = item.transport_type
  if (item.endpoint_url !== undefined) form.mcp_config.endpoint_url = item.endpoint_url || ''
  if (item.auth_type) form.mcp_config.auth_type = item.auth_type
  if (item.package_command !== undefined) {
    form.mcp_config.package_command = item.package_command || ''
  }
  if (item.source_url !== undefined) {
    form.mcp_config.source_url = item.source_url || ''
  }
  ElMessage.success(`已填入「${item.name || item.slug}」配置`)
}

async function handleParseMcpJson() {
  const content = mcpJsonContent.value.trim()
  if (!content) {
    ElMessage.warning('请先粘贴官方 Registry 条目、server.json、mcpServers 或 VS Code servers 配置')
    return
  }
  parsingMcpJson.value = true
  mcpParsedCandidates.value = []
  try {
    const res = await parseMcpJson(content)
    const items = Array.isArray(res) ? res : res ? [res] : []
    if (items.length === 0) {
      ElMessage.warning('未能解析出有效的 MCP 服务配置')
      return
    }
    if (items.length > 1) {
      mcpParsedCandidates.value = items
      applyMcpImportItem(items[0])
      ElMessage.info(`共解析出 ${items.length} 个服务，已默认填入第 1 个，可在下方切换选择`)
    } else {
      applyMcpImportItem(items[0])
      showMcpJsonImport.value = false
    }
  } catch {
    // Handled by interceptor
  } finally {
    parsingMcpJson.value = false
  }
}

const agentImportUrl = ref('')
const importingAgentUrl = ref(false)
const parsingAgentFile = ref(false)
const showAgentTextImport = ref(false)
const agentTextContent = ref('')
const parsingAgentText = ref(false)
const detectedAgentFormat = ref('')

function applyAgentImport(parsed: AgentImportResult, fallbackUrl?: string) {
  if (parsed.name?.trim()) form.name = parsed.name.trim()
  if (!isEdit.value) {
    const code = parsed.slug?.trim() || (parsed.name ? slugifyCode(parsed.name) : '')
    if (code) form.asset_code = code
  }
  if (parsed.description?.trim()) form.summary = parsed.description.trim()
  if (Array.isArray(parsed.tags) && parsed.tags.length > 0) {
    form.tags = [...new Set([...form.tags, ...parsed.tags])]
  }
  if (parsed.model_provider?.trim()) {
    form.agent_config.model_provider = parsed.model_provider.trim()
  }
  if (parsed.model_name?.trim()) {
    form.agent_config.model_name = parsed.model_name.trim()
  }
  if (parsed.system_prompt?.trim()) {
    form.agent_config.system_prompt = parsed.system_prompt
  }
  if (typeof parsed.temperature === 'number' && !Number.isNaN(parsed.temperature)) {
    form.agent_config.temperature = parsed.temperature
  }
  if (typeof parsed.max_tokens === 'number' && parsed.max_tokens > 0) {
    form.agent_config.max_tokens = parsed.max_tokens
  }
  if (parsed.source_url?.trim() || fallbackUrl) {
    form.agent_config.source_url = parsed.source_url?.trim() || fallbackUrl || ''
  }
  if (parsed.detected_format) {
    detectedAgentFormat.value = parsed.detected_format
  }
}

async function handleImportAgentUrl() {
  const url = agentImportUrl.value.trim()
  if (!url) {
    ElMessage.warning('请输入 Agent 定义文件或页面链接')
    return
  }
  importingAgentUrl.value = true
  try {
    const parsed = await importAgentFromUrl(url)
    applyAgentImport(parsed, url)
    ElMessage.success('已解析 URL 并填充 Agent 配置')
  } catch {
    // Handled by interceptor
  } finally {
    importingAgentUrl.value = false
  }
}

async function handleAgentFileRequest(options: UploadRequestOptions) {
  const file = options.file as File
  parsingAgentFile.value = true
  try {
    const parsed = await parseAgentFile(file)
    applyAgentImport(parsed)
    ElMessage.success(`已解析文件「${file.name}」`)
  } catch {
    // Handled by interceptor
  } finally {
    parsingAgentFile.value = false
  }
}

async function handleParseAgentText() {
  const content = agentTextContent.value.trim()
  if (!content) {
    ElMessage.warning('请先粘贴 Markdown / JSON / YAML 内容')
    return
  }
  parsingAgentText.value = true
  try {
    const parsed = await parseAgentText(content)
    applyAgentImport(parsed)
    showAgentTextImport.value = false
    ElMessage.success('已解析文本并填充 Agent 配置')
  } catch {
    // Handled by interceptor
  } finally {
    parsingAgentText.value = false
  }
}

function resetForm() {
  Object.assign(form, createInitialForm())
  tagInput.value = ''
  skillsShUrl.value = ''
  pendingSkillFile.value = null
  mcpRegistryQuery.value = ''
  mcpRegistryResults.value = []
  showMcpJsonImport.value = false
  mcpJsonContent.value = ''
  mcpParsedCandidates.value = []
  mcpEndpointPlaceholder.value = 'https://mcp.example.com/sse（Stdio 类型可留空）'
  agentImportUrl.value = ''
  showAgentTextImport.value = false
  agentTextContent.value = ''
  detectedAgentFormat.value = ''
  formRef.value?.clearValidate()
}

watch(
  () => [props.modelValue, props.editAssetId] as const,
  async ([isOpen, editId]) => {
    if (!isOpen) return
    resetForm()
    loadBindableAssets(props.fixedDomainId || undefined)
    if (!editId) return

    loadingDetail.value = true
    try {
      const detail = await getAssetDetail(editId)
      form.asset_code = detail.asset_code
      form.asset_type = detail.asset_type
      form.name = detail.name
      form.summary = detail.summary || ''
      form.tags = Array.isArray(detail.tags) ? [...detail.tags] : []
      form.domain_id = detail.domain_id
      form.visibility = detail.visibility || 'domain'
      if (detail.mcp_config) {
        form.mcp_config = { ...form.mcp_config, ...detail.mcp_config }
      }
      if (detail.skill_config) {
        form.skill_config = { ...form.skill_config, ...detail.skill_config }
      }
      if (detail.agent_config) {
        form.agent_config = { ...form.agent_config, ...detail.agent_config }
      }
      if (detail.kb_config) {
        form.kb_config = { ...form.kb_config, ...detail.kb_config }
      }
    } catch {
      visible.value = false
    } finally {
      loadingDetail.value = false
    }
  }
)

function addTag() {
  const val = tagInput.value.trim()
  if (!val) return
  if (!form.tags.includes(val)) {
    form.tags.push(val)
  }
  tagInput.value = ''
}

function removeTag(tag: string) {
  form.tags = form.tags.filter((t) => t !== tag)
}

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid || !form.domain_id) return

  const basePayload = {
    asset_code: form.asset_code.trim(),
    asset_type: form.asset_type,
    name: form.name.trim(),
    summary: form.summary.trim(),
    tags: form.tags,
    domain_id: form.domain_id,
    visibility: form.visibility
  }

  const payload: CreateAssetPayload = { ...basePayload }
  if (form.asset_type === 'mcp') {
    payload.mcp_config = { ...form.mcp_config, endpoint_url: form.mcp_config.endpoint_url.trim() }
  } else if (form.asset_type === 'skill') {
    payload.skill_config = { ...form.skill_config }
  } else if (form.asset_type === 'agent') {
    payload.agent_config = { ...form.agent_config }
  } else if (form.asset_type === 'kb') {
    payload.kb_config = { ...form.kb_config }
  }

  submitting.value = true
  try {
    let savedId: number | undefined
    if (isEdit.value && props.editAssetId) {
      await updateAsset(props.editAssetId, payload)
      savedId = props.editAssetId
      ElMessage.success('资产更新成功')
    } else {
      const created = await createAsset(payload)
      savedId = created?.id
      ElMessage.success('资产创建成功')
    }
    if (form.asset_type === 'skill' && pendingSkillFile.value && savedId) {
      await uploadSkillPackage(savedId, pendingSkillFile.value)
      ElMessage.success('技能包文件已上传')
      pendingSkillFile.value = null
    }
    visible.value = false
    emit('saved')
  } catch {
    // Error shown by interceptor
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-drawer
    v-model="visible"
    :title="isEdit ? '编辑 AI 资产' : '新建 AI 资产'"
    size="640px"
    destroy-on-close
  >
    <div v-if="loadingDetail" class="drawer-loading">
      <el-skeleton :rows="8" animated />
    </div>

    <el-form
      v-else
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      class="asset-form"
    >
      <div class="form-section-title">基础信息</div>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="资产名称" prop="name">
            <el-input v-model="form.name" placeholder="例如：订单查询 MCP 服务" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="资产唯一编码" prop="asset_code">
            <el-input
              v-model="form.asset_code"
              :disabled="isEdit"
              placeholder="例如：order-query-mcp"
              class="mono-text"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="8">
          <el-form-item label="资产类型" prop="asset_type">
            <el-select v-model="form.asset_type" :disabled="isEdit" style="width: 100%">
              <el-option label="MCP 服务" value="mcp" />
              <el-option label="Skills 技能" value="skill" />
              <el-option label="Agent 实例" value="agent" />
              <el-option label="私有知识库" value="kb" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="所属业务域" prop="domain_id">
            <el-select
              v-model="form.domain_id"
              :disabled="Boolean(fixedDomainId)"
              placeholder="选择业务域"
              style="width: 100%"
              @change="loadBindableAssets()"
            >
              <el-option
                v-for="d in domains"
                :key="d.id"
                :label="d.domain_name"
                :value="d.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="可见范围" prop="visibility">
            <el-select v-model="form.visibility" style="width: 100%">
              <el-option label="业务域内可见" value="domain" />
              <el-option label="全平台公开" value="public" />
              <el-option label="仅自己可见" value="private" />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="能力简介" prop="summary">
        <el-input
          v-model="form.summary"
          type="textarea"
          :rows="2"
          placeholder="简要说明该 AI 能力的功能、适用场景与输入输出特点"
        />
      </el-form-item>

      <el-form-item label="标签">
        <div class="tags-editor">
          <el-tag
            v-for="tag in form.tags"
            :key="tag"
            closable
            @close="removeTag(tag)"
          >
            {{ tag }}
          </el-tag>
          <el-input
            v-model="tagInput"
            size="small"
            placeholder="输入标签后按回车添加"
            style="width: 180px"
            @keyup.enter.prevent="addTag"
          />
        </div>
      </el-form-item>

      <el-divider />

      <!-- MCP Config -->
      <template v-if="form.asset_type === 'mcp'">
        <div class="form-section-title">MCP 服务配置</div>

        <div class="mcp-import-box">
          <div class="import-header-row">
            <span class="import-label" style="margin-bottom: 0">
              从官方 MCP Registry 导入
              <span class="registry-sub mono-text">(registry.modelcontextprotocol.io)</span>
            </span>
            <el-button link type="primary" size="small" @click="showMcpJsonImport = !showMcpJsonImport">
              {{ showMcpJsonImport ? '收起 JSON 导入' : '粘贴 mcpServers / server.json 导入' }}
            </el-button>
          </div>

          <div class="import-row" style="margin-top: 8px">
            <el-input
              v-model="mcpRegistryQuery"
              placeholder="搜索关键词（如 github、postgres、filesystem）..."
              :prefix-icon="Search"
              clearable
              @keyup.enter.prevent="handleSearchMcpRegistry"
            />
            <el-button
              type="primary"
              plain
              :loading="searchingRegistry"
              @click="handleSearchMcpRegistry"
            >
              搜索 Registry
            </el-button>
          </div>

          <div v-if="mcpRegistryResults.length > 0" class="registry-results">
            <div v-for="srv in mcpRegistryResults" :key="srv.registry_name || srv.slug" class="registry-item">
              <div class="registry-info">
                <div class="registry-title-row">
                  <span class="registry-name">{{ srv.name || srv.registry_name }}</span>
                  <span v-if="srv.version" class="registry-ver mono-text">{{ srv.version }}</span>
                  <el-tag size="small" type="info" effect="plain" class="mono-text">
                    {{ srv.transport_type }}
                  </el-tag>
                </div>
                <div class="registry-desc">{{ srv.description || '暂无描述' }}</div>
              </div>
              <el-button type="primary" size="small" plain @click="applyMcpImportItem(srv)">
                填入表单
              </el-button>
            </div>
          </div>

          <div v-if="showMcpJsonImport" class="json-import-panel">
            <div class="json-helper-text">
              支持格式：官方 MCP Registry 条目 (<code class="mono-text">{"server": {...}}</code>)、<code class="mono-text">server.json</code>、<code class="mono-text">mcpServers</code> 以及 VS Code <code class="mono-text">servers</code> 格式
            </div>
            <el-input
              v-model="mcpJsonContent"
              type="textarea"
              :rows="5"
              placeholder='支持官方 MCP Registry 条目 ({"server": {...}})、server.json、mcpServers 或 VS Code servers 格式...'
              class="mono-text"
            />
            <div class="json-import-actions">
              <el-button
                type="primary"
                size="small"
                :loading="parsingMcpJson"
                @click="handleParseMcpJson"
              >
                解析并填入
              </el-button>
            </div>
            <div v-if="mcpParsedCandidates.length > 1" class="registry-results">
              <div
                v-for="cand in mcpParsedCandidates"
                :key="cand.slug || cand.name"
                class="registry-item"
              >
                <div class="registry-info">
                  <div class="registry-title-row">
                    <span class="registry-name">{{ cand.name || cand.slug }}</span>
                    <el-tag size="small" type="info" effect="plain" class="mono-text">
                      {{ cand.transport_type }}
                    </el-tag>
                  </div>
                  <div class="registry-desc mono-text">
                    {{ cand.package_command || cand.endpoint_url || '-' }}
                  </div>
                </div>
                <el-button type="primary" size="small" plain @click="applyMcpImportItem(cand)">
                  填入表单
                </el-button>
              </div>
            </div>
          </div>
        </div>

        <div class="preset-bar">
          <span class="preset-label">快速配置模板：</span>
          <div class="preset-btns">
            <el-button size="small" plain @click="applyMcpPreset('streamable_http')">
              Streamable HTTP 服务
            </el-button>
            <el-button size="small" plain @click="applyMcpPreset('sse')">
              SSE 流式服务
            </el-button>
            <el-button size="small" plain @click="applyMcpPreset('stdio_npx')">
              Stdio npx 命令
            </el-button>
            <el-button size="small" plain @click="applyMcpPreset('stdio_uvx')">
              Stdio uvx 命令
            </el-button>
          </div>
        </div>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="传输协议 (Transport)" prop="mcp_config.transport_type">
              <el-select v-model="form.mcp_config.transport_type" style="width: 100%">
                <el-option label="SSE" value="sse" />
                <el-option label="Streamable HTTP" value="streamable_http" />
                <el-option label="Stdio" value="stdio" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="鉴权方式" prop="mcp_config.auth_type">
              <el-select v-model="form.mcp_config.auth_type" style="width: 100%">
                <el-option label="无鉴权 (none)" value="none" />
                <el-option label="Bearer Token" value="bearer" />
                <el-option label="自定义 Header" value="header" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="服务端点 URL" prop="mcp_config.endpoint_url">
          <el-input
            v-model="form.mcp_config.endpoint_url"
            :placeholder="mcpEndpointPlaceholder"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="启动命令 (Package Command，适用于 Stdio)">
          <el-input
            v-model="form.mcp_config.package_command"
            placeholder="例如：npx -y @modelcontextprotocol/server-github"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="来源链接 (Source URL)">
          <el-input
            v-model="form.mcp_config.source_url"
            placeholder="https://github.com/..."
            class="mono-text"
          />
        </el-form-item>
      </template>

      <!-- Skill Config -->
      <template v-else-if="form.asset_type === 'skill'">
        <div class="form-section-title">Skills 技能配置</div>

        <div class="skill-import-box">
          <div class="import-label">从 skills.sh 导入</div>
          <div class="import-row">
            <el-input
              v-model="skillsShUrl"
              placeholder="https://skills.sh/vercel-labs/skills/find-skills"
              :prefix-icon="Link"
              clearable
            />
            <el-button type="primary" plain :loading="importingUrl" @click="handleImportSkillUrl">
              导入解析
            </el-button>
          </div>
        </div>

        <div class="skill-upload-box">
          <el-upload
            drag
            action="#"
            :show-file-list="false"
            :http-request="handleSkillFileRequest"
            accept=".md,.zip"
            :disabled="parsingFile"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              拖拽或点击上传 <em>SKILL.md / .zip 技能包</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                <span v-if="pendingSkillFile">
                  已选待上传文件：<strong class="mono-text">{{ pendingSkillFile.name }}</strong>
                </span>
                <span v-else>
                  上传后将自动解析名称、简介与 SKILL.md 内容，并在保存资产时同步上传技能包
                </span>
              </div>
            </template>
          </el-upload>
        </div>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="技能类型" prop="skill_config.skill_type">
              <el-select v-model="form.skill_config.skill_type" style="width: 100%">
                <el-option label="Prompt 模板" value="prompt" />
                <el-option label="脚本 (Script)" value="script" />
                <el-option label="工作流 (Workflow)" value="workflow" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="运行环境" prop="skill_config.runtime_env">
              <el-input
                v-model="form.skill_config.runtime_env"
                placeholder="例如：python3.11 / nodejs20"
                class="mono-text"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="来源链接 (Source URL)">
          <el-input
            v-model="form.skill_config.source_url"
            placeholder="https://skills.sh/..."
            class="mono-text"
          />
        </el-form-item>

        <el-form-item v-if="form.skill_config.package_files?.length" label="技能包内文件">
          <div class="package-files-list">
            <el-tag
              v-for="f in form.skill_config.package_files"
              :key="f"
              size="small"
              type="info"
              effect="plain"
              class="mono-text"
            >
              {{ f }}
            </el-tag>
          </div>
        </el-form-item>

        <el-form-item label="技能内容 / Prompt 定义" prop="skill_config.content">
          <el-input
            v-model="form.skill_config.content"
            type="textarea"
            :rows="6"
            placeholder="请输入 Prompt 模板、Python/JS 脚本或工作流定义..."
            class="mono-text"
          />
        </el-form-item>
      </template>

      <!-- Agent Config -->
      <template v-else-if="form.asset_type === 'agent'">
        <div class="form-section-title">Agent 实例配置</div>

        <div class="mcp-import-box">
          <div class="import-header-row">
            <span class="import-label" style="margin-bottom: 0">
              智能导入 Agent
              <el-tag
                v-if="detectedAgentFormat"
                size="small"
                type="success"
                effect="light"
                class="mono-text"
                style="margin-left: 8px"
              >
                {{ detectedAgentFormat }}
              </el-tag>
            </span>
            <el-button
              link
              type="primary"
              size="small"
              @click="showAgentTextImport = !showAgentTextImport"
            >
              {{ showAgentTextImport ? '收起文本粘贴' : '粘贴 Markdown / JSON / YAML' }}
            </el-button>
          </div>

          <div class="import-row" style="margin-top: 8px">
            <el-input
              v-model="agentImportUrl"
              placeholder="输入 ModelScope / Agentverse / GitHub (.md/.json/.yaml) 链接..."
              :prefix-icon="Link"
              clearable
              @keyup.enter.prevent="handleImportAgentUrl"
            />
            <el-button
              type="primary"
              plain
              :loading="importingAgentUrl"
              @click="handleImportAgentUrl"
            >
              导入解析
            </el-button>
          </div>

          <div style="margin-top: 10px">
            <el-upload
              drag
              action="#"
              :show-file-list="false"
              :http-request="handleAgentFileRequest"
              accept=".md,.txt,.json,.yaml,.yml"
              :disabled="parsingAgentFile"
            >
              <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
              <div class="el-upload__text">
                拖拽或点击上传 <em>.md / .txt / .json / .yaml</em> Agent 配置文件
              </div>
              <template #tip>
                <div class="el-upload__tip">
                  兼容单 Markdown、Frontmatter .md、Dify DSL YAML 及通用 JSON 格式
                </div>
              </template>
            </el-upload>
          </div>

          <div v-if="showAgentTextImport" class="json-import-panel">
            <el-input
              v-model="agentTextContent"
              type="textarea"
              :rows="5"
              placeholder="粘贴 Markdown / Frontmatter / Dify YAML / JSON 配置..."
              class="mono-text"
            />
            <div class="json-import-actions">
              <el-button
                type="primary"
                size="small"
                :loading="parsingAgentText"
                @click="handleParseAgentText"
              >
                解析并填入
              </el-button>
            </div>
          </div>
        </div>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="模型厂商" prop="agent_config.model_provider">
              <el-input
                v-model="form.agent_config.model_provider"
                placeholder="例如：openai / deepseek / qwen"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="模型名称" prop="agent_config.model_name">
              <el-input
                v-model="form.agent_config.model_name"
                placeholder="例如：gpt-4o-mini / deepseek-chat"
                class="mono-text"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Temperature (0 ~ 2)">
              <el-input-number
                v-model="form.agent_config.temperature"
                :min="0"
                :max="2"
                :step="0.1"
                :precision="2"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Max Tokens">
              <el-input-number
                v-model="form.agent_config.max_tokens"
                :min="128"
                :max="128000"
                :step="256"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="System Prompt">
          <el-input
            v-model="form.agent_config.system_prompt"
            type="textarea"
            :rows="4"
            placeholder="设定 Agent 的角色定位、行为约束与输出规范..."
          />
        </el-form-item>
        <el-form-item label="来源链接 (Source URL)">
          <el-input
            v-model="form.agent_config.source_url"
            placeholder="https://github.com/... 或 ModelScope 链接"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="绑定域内 MCP 服务">
          <el-select
            v-model="form.agent_config.bound_mcp_ids"
            multiple
            clearable
            placeholder="选择已发布的 MCP 服务"
            style="width: 100%"
          >
            <el-option v-for="m in domainMcps" :key="m.id" :label="m.name" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定域内 Skills 技能">
          <el-select
            v-model="form.agent_config.bound_skill_ids"
            multiple
            clearable
            placeholder="选择已发布的 Skills 技能"
            style="width: 100%"
          >
            <el-option v-for="s in domainSkills" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定域内知识库">
          <el-select
            v-model="form.agent_config.bound_kb_ids"
            multiple
            clearable
            placeholder="选择已发布的私有知识库"
            style="width: 100%"
          >
            <el-option v-for="k in domainKbs" :key="k.id" :label="k.name" :value="k.id" />
          </el-select>
        </el-form-item>
      </template>

      <!-- KB Config -->
      <template v-else-if="form.asset_type === 'kb'">
        <div class="form-section-title">私有知识库配置</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="切片大小 (Chunk Size)">
              <el-input-number
                v-model="form.kb_config.chunk_size"
                :min="100"
                :max="4000"
                :step="50"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="切片重叠 (Chunk Overlap)">
              <el-input-number
                v-model="form.kb_config.chunk_overlap"
                :min="0"
                :max="500"
                :step="10"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </template>
    </el-form>

    <template #footer>
      <div class="drawer-footer">
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEdit ? '保存修改' : '确认创建' }}
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-loading {
  padding: 12px 0;
}

.form-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  margin-bottom: 14px;
  padding-left: 8px;
  border-left: 3px solid var(--ascp-primary);
}

.tags-editor {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.skill-import-box {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 14px;
}

.import-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  margin-bottom: 8px;
}

.import-row {
  display: flex;
  gap: 8px;
}

.skill-upload-box {
  margin-bottom: 16px;
}

.package-files-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mcp-import-box {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 16px;
}

.import-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.registry-sub {
  font-size: 11.5px;
  font-weight: 400;
  color: var(--ascp-text-muted);
}

.registry-results {
  margin-top: 10px;
  max-height: 220px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.registry-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  background: #ffffff;
  border: 1px solid var(--ascp-border);
  border-radius: 8px;
}

.registry-info {
  min-width: 0;
  flex: 1;
}

.registry-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.registry-name {
  font-weight: 600;
  font-size: 13.5px;
  color: var(--ascp-text-primary);
}

.registry-ver {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
}

.registry-desc {
  margin-top: 3px;
  font-size: 12.5px;
  color: var(--ascp-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.json-import-panel {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--ascp-border);
}

.json-import-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

.json-helper-text {
  font-size: 12px;
  color: var(--ascp-text-muted);
  margin-bottom: 8px;
  line-height: 1.5;
}

.preset-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
  padding: 10px 12px;
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 8px;
}

.preset-label {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ascp-text-secondary);
}

.preset-btns {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
