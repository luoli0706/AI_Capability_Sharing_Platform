<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  ArrowLeft,
  ArrowRight,
  CopyDocument,
  Document,
  Download,
  Folder,
  Key,
  Picture,
  Promotion,
  VideoPlay
} from '@element-plus/icons-vue'
import { downloadAgentConfig, downloadSkillPackage, getAssetDetail, subscribeAsset } from '@/api/asset'
import { createApproval } from '@/api/approval'
import { invokeAgent, invokeKBQuery, invokeMCP, invokeSkill } from '@/api/invoke'
import { getDocumentContent, getDocumentRawBlob, getDocuments } from '@/api/kb'
import type { AssetItem, KbContentResult, KbDocument } from '@/types/api'
import {
  formatDateTime,
  formatFileSize,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getVisibilityLabel,
  normalizeList
} from '@/utils/format'
import {
  buildImmediateKbTreeNodes,
  confirmAndDownloadKb,
  extractImmediateFiles,
  isImageFile,
  isMarkdownFile,
  isTextFile,
  type KbTreeNode
} from '@/utils/kbDownload'
import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{
  modelValue: boolean
  assetId: number | null
  showActions?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const loading = ref(false)
const detail = ref<AssetItem | null>(null)
const activeTab = ref('overview')

const subscribing = ref(false)
const approvalModalOpen = ref(false)
const approvalFormRef = ref<FormInstance>()
const submittingApproval = ref(false)
const approvalForm = reactive<{
  requested_perm: 'invoke' | 'edit'
  duration_days: number
  reason: string
}>({
  requested_perm: 'invoke',
  duration_days: 30,
  reason: ''
})

const approvalRules: FormRules = {
  duration_days: [{ required: true, message: '请填写申请天数', trigger: 'blur' }],
  reason: [{ required: true, message: '请说明申请用途', trigger: 'blur' }]
}

// Online invoke test state
const invoking = ref(false)
const invokePayloadText = ref('{\n  "input": "hello"\n}')
const kbQueryText = ref('')
const kbTopK = ref(5)
const invokeResultText = ref('')
const invokeError = ref('')

const visible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
})

const typeMeta = computed(() =>
  detail.value ? getAssetTypeMeta(detail.value.asset_type) : null
)
const statusMeta = computed(() =>
  detail.value ? getAssetStatusMeta(detail.value.status) : null
)
const ownerDisplay = computed(() => {
  if (!detail.value) return '-'
  return (
    detail.value.owner?.nickname ||
    detail.value.owner?.username ||
    detail.value.owner_name ||
    `用户 #${detail.value.owner_id}`
  )
})

function initDefaultPayload(asset: AssetItem) {
  invokeResultText.value = ''
  invokeError.value = ''
  if (asset.asset_type === 'mcp') {
    invokePayloadText.value = JSON.stringify({ tool: 'ping', arguments: {} }, null, 2)
  } else if (asset.asset_type === 'skill') {
    invokePayloadText.value = JSON.stringify({ input: '测试输入' }, null, 2)
  } else if (asset.asset_type === 'agent') {
    invokePayloadText.value = JSON.stringify({ message: '你好，请介绍一下你的能力' }, null, 2)
  } else {
    kbQueryText.value = ''
    kbTopK.value = 5
  }
}

const loadingKbTree = ref(false)
const kbTreeRefreshSeed = ref(0)
const kbTreeKey = computed(() => `${detail.value?.id || 0}-${kbTreeRefreshSeed.value}`)
const kbTreeProps = {
  label: 'label',
  children: 'children',
  isLeaf: 'isLeaf'
}
const folderFilesCache = ref<Record<string, KbDocument[]>>({})
const kbRootEmpty = ref(false)
const kbPreviewDoc = ref<KbContentResult | null>(null)
const kbImageUrlMap = ref<Record<number, string>>({})
const sameFolderImages = ref<KbDocument[]>([])
const loadingKbImageBlob = ref(false)
const kbImageDoc = ref<KbDocument | null>(null)
const kbBinaryDoc = ref<KbDocument | null>(null)
const kbPreviewLoading = ref(false)
const kbPreviewMode = ref<'rendered' | 'raw'>('rendered')

const kbImagePreviewUrl = computed(() =>
  kbImageDoc.value ? kbImageUrlMap.value[kbImageDoc.value.id] || '' : ''
)
const currentKbImageIndex = computed(() => {
  if (!kbImageDoc.value) return 0
  const idx = sameFolderImages.value.findIndex((d) => d.id === kbImageDoc.value?.id)
  return idx >= 0 ? idx : 0
})
const folderImageUrls = computed(() =>
  sameFolderImages.value.map((d) => kbImageUrlMap.value[d.id] || '')
)

const kbRenderedHtml = computed(() =>
  kbPreviewDoc.value ? renderMarkdown(kbPreviewDoc.value.content) : ''
)

function revokeAllKbImageUrls() {
  for (const url of Object.values(kbImageUrlMap.value)) {
    if (url) URL.revokeObjectURL(url)
  }
  kbImageUrlMap.value = {}
}

function resetKbPreviewState() {
  revokeAllKbImageUrls()
  sameFolderImages.value = []
  kbPreviewDoc.value = null
  kbImageDoc.value = null
  kbBinaryDoc.value = null
}

function resetKbTreeState() {
  folderFilesCache.value = {}
  kbRootEmpty.value = false
  kbTreeRefreshSeed.value++
  resetKbPreviewState()
}

async function loadKbTreeNode(
  node: { level: number; data?: KbTreeNode },
  resolve: (data: KbTreeNode[]) => void
) {
  if (!detail.value || detail.value.asset_type !== 'kb') {
    resolve([])
    return
  }
  const kbId = detail.value.id
  const folderPath = node.level === 0 ? '' : (node.data?.folderPath || '')
  if (node.level === 0) {
    loadingKbTree.value = true
    kbRootEmpty.value = false
  }
  try {
    const res = await getDocuments(kbId, { lazy: true, folder_path: folderPath })
    const items = normalizeList(res).items
    const files = extractImmediateFiles(items, folderPath)
    folderFilesCache.value = { ...folderFilesCache.value, [folderPath]: files }
    const nodes = buildImmediateKbTreeNodes(items, folderPath)
    if (node.level === 0) {
      kbRootEmpty.value = nodes.length === 0
    }
    resolve(nodes)
  } catch {
    if (node.level === 0) {
      kbRootEmpty.value = true
    }
    resolve([])
  } finally {
    if (node.level === 0) {
      loadingKbTree.value = false
    }
  }
}

async function ensureKbImageUrl(doc: KbDocument) {
  if (!detail.value) return
  if (kbImageUrlMap.value[doc.id]) return
  loadingKbImageBlob.value = true
  try {
    const blob = await getDocumentRawBlob(detail.value.id, doc.id, doc.file_name)
    const url = URL.createObjectURL(blob)
    kbImageUrlMap.value = { ...kbImageUrlMap.value, [doc.id]: url }
  } finally {
    loadingKbImageBlob.value = false
  }
}

async function selectKbImageAtIndex(idx: number) {
  const target = sameFolderImages.value[idx]
  if (!target) return
  kbImageDoc.value = target
  await ensureKbImageUrl(target)
}

function handleKbLightboxSwitch(index: number) {
  selectKbImageAtIndex(index)
}

async function handleKbNodeClick(node: KbTreeNode) {
  if (node.isFolder || !node.doc || !detail.value) return
  const doc = node.doc
  resetKbPreviewState()
  if (isImageFile(doc.file_name)) {
    const cleanFolder = (doc.folder_path || '').replace(/^\/+|\/+$/g, '')
    let folderDocs = folderFilesCache.value[cleanFolder]
    if (!folderDocs) {
      try {
        const res = await getDocuments(detail.value.id, { lazy: true, folder_path: cleanFolder })
        folderDocs = extractImmediateFiles(normalizeList(res).items, cleanFolder)
        folderFilesCache.value = { ...folderFilesCache.value, [cleanFolder]: folderDocs }
      } catch {
        folderDocs = [doc]
      }
    }
    const imgs = folderDocs
      .filter((d) => !d.is_folder && isImageFile(d.file_name))
      .sort((a, b) => a.file_name.localeCompare(b.file_name))
    sameFolderImages.value = imgs.length > 0 ? imgs : [doc]
    kbImageDoc.value = doc
    try {
      await ensureKbImageUrl(doc)
    } catch {
      resetKbPreviewState()
    }
    return
  }

  kbPreviewLoading.value = true
  try {
    if (isTextFile(doc.file_name)) {
      const contentRes = await getDocumentContent(detail.value.id, doc.id)
      if (contentRes.is_binary) {
        kbBinaryDoc.value = doc
      } else {
        kbPreviewDoc.value = contentRes
        kbPreviewMode.value = isMarkdownFile(doc.file_name) ? 'rendered' : 'raw'
      }
    } else {
      kbBinaryDoc.value = doc
    }
  } catch {
    resetKbPreviewState()
  } finally {
    kbPreviewLoading.value = false
  }
}

onBeforeUnmount(() => {
  revokeAllKbImageUrls()
})

async function handleDownloadKbAll() {
  if (!detail.value) return
  await confirmAndDownloadKb(
    detail.value.id,
    { scope: 'all' },
    `${detail.value.asset_code || 'knowledge-base'}.zip`
  )
}

async function handleDownloadKbFolder(node: KbTreeNode) {
  if (!detail.value) return
  const folderName = node.label || 'folder'
  await confirmAndDownloadKb(
    detail.value.id,
    { scope: 'folder', folder_path: node.folderPath },
    `${folderName}.zip`
  )
}

async function handleDownloadKbFile(docId: number, fileName: string) {
  if (!detail.value) return
  await confirmAndDownloadKb(detail.value.id, { scope: 'file', doc_id: docId }, fileName)
}

watch(
  () => [props.modelValue, props.assetId] as const,
  async ([isOpen, id]) => {
    if (!isOpen || !id) {
      if (!isOpen) resetKbPreviewState()
      return
    }
    activeTab.value = 'overview'
    loading.value = true
    detail.value = null
    resetKbTreeState()
    try {
      const res = await getAssetDetail(id)
      detail.value = res
      initDefaultPayload(res)
      if (res.asset_type === 'kb') {
        kbTreeRefreshSeed.value++
      }
    } catch {
      detail.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)

async function handleSubscribe() {
  if (!detail.value) return
  subscribing.value = true
  try {
    await subscribeAsset(detail.value.id)
    ElMessage.success('订阅成功，您已获得调用授权')
  } catch {
    // Handled by interceptor
  } finally {
    subscribing.value = false
  }
}

function openApprovalModal() {
  approvalForm.requested_perm = 'invoke'
  approvalForm.duration_days = 30
  approvalForm.reason = ''
  approvalFormRef.value?.clearValidate()
  approvalModalOpen.value = true
}

async function handleSubmitApproval() {
  if (!approvalFormRef.value || !detail.value) return
  const valid = await approvalFormRef.value.validate().catch(() => false)
  if (!valid) return

  submittingApproval.value = true
  try {
    await createApproval({
      asset_id: detail.value.id,
      requested_perm: approvalForm.requested_perm,
      duration_days: approvalForm.duration_days,
      reason: approvalForm.reason.trim(),
      applicant_domain_id: detail.value.domain_id
    })
    ElMessage.success('权限申请已提交，请在「审批中心」查看进度')
    approvalModalOpen.value = false
  } catch {
    // Handled by interceptor
  } finally {
    submittingApproval.value = false
  }
}

const downloadingFormat = ref<'zip' | 'md' | null>(null)

async function handleDownloadSkill(format: 'zip' | 'md') {
  if (!detail.value) return
  downloadingFormat.value = format
  const code = detail.value.asset_code || `skill-${detail.value.id}`
  const filename = format === 'zip' ? `${code}.zip` : `${code}-SKILL.md`
  try {
    await downloadSkillPackage(detail.value.id, format, filename)
    ElMessage.success(`已开始下载 ${filename}`)
  } catch {
    // Handled by interceptor
  } finally {
    downloadingFormat.value = null
  }
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制${label}`)
  } catch {
    ElMessage.info(text)
  }
}

function handleCopySourceUrl() {
  const url = detail.value?.skill_config?.source_url
  if (url) copyText(url, ' skills.sh 来源链接')
}

function handleCopyPullCommand() {
  if (!detail.value) return
  const origin = window.location.origin
  const code = detail.value.asset_code || 'skill'
  const cmd = `curl -H "Authorization: Bearer <TOKEN>" "${origin}/api/v1/assets/${detail.value.id}/skill/download?format=zip" -o ${code}.zip`
  copyText(cmd, '平台拉取命令')
}

function handleCopyMcpClientConfig() {
  if (!detail.value || !detail.value.mcp_config) return
  const cfg = detail.value.mcp_config
  const key = detail.value.asset_code || detail.value.name || 'mcp-server'
  let serverEntry: Record<string, unknown>

  if (cfg.transport_type === 'stdio' && cfg.package_command) {
    const parts = cfg.package_command.trim().split(/\s+/)
    serverEntry = {
      command: parts[0],
      args: parts.slice(1)
    }
  } else {
    serverEntry = {
      url: cfg.endpoint_url || `${window.location.origin}/api/v1/invoke/mcp/${key}`,
      transport: cfg.transport_type
    }
    if (cfg.auth_type === 'bearer') {
      serverEntry.headers = { Authorization: 'Bearer <YOUR_TOKEN>' }
    }
  }

  const json = JSON.stringify({ mcpServers: { [key]: serverEntry } }, null, 2)
  copyText(json, ' mcpServers 客户端配置')
}

const downloadingAgentFormat = ref<'md' | 'json' | null>(null)

async function handleDownloadAgent(format: 'md' | 'json') {
  if (!detail.value) return
  downloadingAgentFormat.value = format
  const code = detail.value.asset_code || `agent-${detail.value.id}`
  const filename = format === 'md' ? `${code}.md` : `${code}.json`
  try {
    await downloadAgentConfig(detail.value.id, format, filename)
    ElMessage.success(`已开始导出 ${filename}`)
  } catch {
    // Handled by interceptor
  } finally {
    downloadingAgentFormat.value = null
  }
}

async function handleRunInvoke() {
  if (!detail.value) return
  invokeError.value = ''
  invokeResultText.value = ''

  let parsedBody: Record<string, unknown> = {}
  if (detail.value.asset_type !== 'kb') {
    try {
      parsedBody = JSON.parse(invokePayloadText.value || '{}')
    } catch {
      invokeError.value = '请求参数必须是合法的 JSON 格式'
      return
    }
  } else if (!kbQueryText.value.trim()) {
    invokeError.value = '请输入检索关键词'
    return
  }

  invoking.value = true
  try {
    let res: unknown
    if (detail.value.asset_type === 'mcp') {
      res = await invokeMCP(detail.value.asset_code, parsedBody)
    } else if (detail.value.asset_type === 'skill') {
      res = await invokeSkill(detail.value.asset_code, parsedBody)
    } else if (detail.value.asset_type === 'agent') {
      res = await invokeAgent(detail.value.asset_code, parsedBody)
    } else {
      res = await invokeKBQuery(detail.value.asset_code, {
        query: kbQueryText.value.trim(),
        top_k: kbTopK.value
      })
    }
    invokeResultText.value = JSON.stringify(res, null, 2)
  } catch (err: unknown) {
    invokeError.value = err instanceof Error ? err.message : '调用失败，请确认是否已订阅或通过审批'
  } finally {
    invoking.value = false
  }
}
</script>

<template>
  <el-drawer
    v-model="visible"
    title="AI 资产详情"
    size="660px"
    destroy-on-close
  >
    <div v-if="loading" class="drawer-loading">
      <el-skeleton :rows="8" animated />
    </div>

    <div v-else-if="detail" class="detail-body">
      <div class="detail-header">
        <div class="detail-title-row">
          <el-tag v-if="typeMeta" :type="typeMeta.tagType" effect="light" size="small">
            {{ typeMeta.label }}
          </el-tag>
          <el-tag v-if="statusMeta" :type="statusMeta.tagType" effect="plain" size="small">
            {{ statusMeta.label }}
          </el-tag>
          <span class="version-pill mono-text">{{ detail.current_version || 'v0.1.0' }}</span>
        </div>
        <h3 class="detail-name">{{ detail.name }}</h3>
        <div class="detail-code mono-text">{{ detail.asset_code }}</div>
        <p class="detail-summary">{{ detail.summary || '暂无简介描述' }}</p>

        <div v-if="detail.tags?.length" class="detail-tags">
          <el-tag
            v-for="tag in detail.tags"
            :key="tag"
            size="small"
            type="info"
            effect="plain"
          >
            {{ tag }}
          </el-tag>
        </div>

        <div v-if="showActions && detail.status === 'published'" class="detail-action-bar">
          <el-button
            type="primary"
            size="small"
            :icon="Promotion"
            :loading="subscribing"
            @click="handleSubscribe"
          >
            一键订阅
          </el-button>
          <el-button
            size="small"
            :icon="Key"
            @click="openApprovalModal"
          >
            发起权限申请
          </el-button>
        </div>
      </div>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="配置与元数据" name="overview">
          <div class="tab-section">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="所属域">
                {{ detail.domain_name || `域 #${detail.domain_id}` }}
              </el-descriptions-item>
              <el-descriptions-item label="资产负责人">
                {{ ownerDisplay }}
              </el-descriptions-item>
              <el-descriptions-item label="可见范围">
                {{ getVisibilityLabel(detail.visibility) }}
              </el-descriptions-item>
              <el-descriptions-item label="累计调用次数">
                <span class="tabular-nums">{{ detail.invoke_count ?? 0 }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="创建时间">
                {{ formatDateTime(detail.created_at) }}
              </el-descriptions-item>
              <el-descriptions-item label="更新时间">
                {{ formatDateTime(detail.updated_at) }}
              </el-descriptions-item>
            </el-descriptions>

            <!-- MCP Config -->
            <div v-if="detail.asset_type === 'mcp' && detail.mcp_config" class="config-section">
              <div class="section-header-row">
                <h4 class="section-title" style="margin-bottom: 0">MCP 服务配置</h4>
                <el-button
                  size="small"
                  type="primary"
                  plain
                  :icon="CopyDocument"
                  @click="handleCopyMcpClientConfig"
                >
                  复制 mcpServers 客户端配置
                </el-button>
              </div>
              <el-descriptions :column="1" border size="small">
                <el-descriptions-item label="传输协议">
                  <span class="mono-text">{{ detail.mcp_config.transport_type }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="服务端点 URL">
                  <span class="mono-text break-all">{{ detail.mcp_config.endpoint_url || '-' }}</span>
                </el-descriptions-item>
                <el-descriptions-item v-if="detail.mcp_config.package_command" label="启动命令">
                  <span class="mono-text break-all">{{ detail.mcp_config.package_command }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="鉴权方式">
                  <span class="mono-text">{{ detail.mcp_config.auth_type }}</span>
                </el-descriptions-item>
                <el-descriptions-item v-if="detail.mcp_config.source_url" label="来源链接">
                  <a
                    :href="detail.mcp_config.source_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="source-link mono-text"
                  >
                    {{ detail.mcp_config.source_url }}
                  </a>
                </el-descriptions-item>
                <el-descriptions-item v-if="detail.mcp_config.health_status" label="健康状态">
                  <el-tag
                    :type="detail.mcp_config.health_status === 'healthy' ? 'success' : 'info'"
                    size="small"
                  >
                    {{ detail.mcp_config.health_status }}
                  </el-tag>
                </el-descriptions-item>
              </el-descriptions>
              <div v-if="detail.mcp_config.tools_schema?.length" class="mt-12">
                <div class="code-block-label">已探测 Tools 清单 ({{ detail.mcp_config.tools_schema.length }})</div>
                <pre class="code-block mono-text">{{ JSON.stringify(detail.mcp_config.tools_schema, null, 2) }}</pre>
              </div>
            </div>

            <!-- Skill Config -->
            <div v-else-if="detail.asset_type === 'skill' && detail.skill_config" class="config-section">
              <h4 class="section-title">Skills 技能配置</h4>
              <div class="skill-download-bar">
                <el-button
                  type="primary"
                  size="small"
                  :icon="Download"
                  :loading="downloadingFormat === 'zip'"
                  @click="handleDownloadSkill('zip')"
                >
                  下载技能包 (.zip)
                </el-button>
                <el-button
                  size="small"
                  :icon="Download"
                  :loading="downloadingFormat === 'md'"
                  @click="handleDownloadSkill('md')"
                >
                  下载 SKILL.md
                </el-button>
                <el-button
                  v-if="detail.skill_config.source_url"
                  size="small"
                  :icon="CopyDocument"
                  @click="handleCopySourceUrl"
                >
                  复制 skills.sh 来源
                </el-button>
                <el-button
                  size="small"
                  :icon="CopyDocument"
                  @click="handleCopyPullCommand"
                >
                  复制平台拉取命令
                </el-button>
              </div>
              <el-descriptions :column="2" border size="small" class="mb-12">
                <el-descriptions-item label="技能类型">
                  <span class="mono-text">{{ detail.skill_config.skill_type }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="运行环境">
                  <span class="mono-text">{{ detail.skill_config.runtime_env || '-' }}</span>
                </el-descriptions-item>
                <el-descriptions-item
                  v-if="detail.skill_config.source_url"
                  label="来源链接"
                  :span="2"
                >
                  <a
                    :href="detail.skill_config.source_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="source-link mono-text"
                  >
                    {{ detail.skill_config.source_url }}
                  </a>
                </el-descriptions-item>
              </el-descriptions>
              <div v-if="detail.skill_config.package_files?.length" class="mb-12">
                <div class="code-block-label">技能包文件清单 ({{ detail.skill_config.package_files.length }})</div>
                <div class="package-files-tags">
                  <el-tag
                    v-for="file in detail.skill_config.package_files"
                    :key="file"
                    size="small"
                    type="info"
                    effect="plain"
                    class="mono-text"
                  >
                    {{ file }}
                  </el-tag>
                </div>
              </div>
              <div class="code-block-label">技能定义 / Prompt 内容</div>
              <pre class="code-block mono-text">{{ detail.skill_config.content || '暂无内容' }}</pre>
            </div>

            <!-- Agent Config -->
            <div v-else-if="detail.asset_type === 'agent' && detail.agent_config" class="config-section">
              <div class="section-header-row">
                <h4 class="section-title" style="margin-bottom: 0">Agent 实例配置</h4>
                <div style="display: flex; gap: 8px">
                  <el-button
                    type="primary"
                    size="small"
                    plain
                    :icon="Download"
                    :loading="downloadingAgentFormat === 'md'"
                    @click="handleDownloadAgent('md')"
                  >
                    导出 Agent (.md)
                  </el-button>
                  <el-button
                    size="small"
                    :icon="Download"
                    :loading="downloadingAgentFormat === 'json'"
                    @click="handleDownloadAgent('json')"
                  >
                    导出配置 (.json)
                  </el-button>
                </div>
              </div>
              <el-descriptions :column="2" border size="small" class="mb-12">
                <el-descriptions-item label="模型厂商">
                  {{ detail.agent_config.model_provider || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="模型名称">
                  <span class="mono-text">{{ detail.agent_config.model_name || '-' }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="Temperature">
                  <span class="tabular-nums">{{ detail.agent_config.temperature }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="Max Tokens">
                  <span class="tabular-nums">{{ detail.agent_config.max_tokens }}</span>
                </el-descriptions-item>
                <el-descriptions-item
                  v-if="detail.agent_config.source_url"
                  label="来源链接"
                  :span="2"
                >
                  <a
                    :href="detail.agent_config.source_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="source-link mono-text"
                  >
                    {{ detail.agent_config.source_url }}
                  </a>
                </el-descriptions-item>
              </el-descriptions>
              <div class="code-block-label">System Prompt</div>
              <pre class="code-block mono-text">{{ detail.agent_config.system_prompt || '暂无系统提示词' }}</pre>
            </div>

            <!-- KB Config -->
            <div v-else-if="detail.asset_type === 'kb' && detail.kb_config" class="config-section">
              <div class="section-header-row">
                <h4 class="section-title" style="margin-bottom: 0">私有知识库配置与文件</h4>
                <el-button
                  type="primary"
                  size="small"
                  plain
                  :icon="Download"
                  @click="handleDownloadKbAll"
                >
                  下载整个知识库
                </el-button>
              </div>
              <el-descriptions :column="2" border size="small" class="mb-12">
                <el-descriptions-item label="切片大小 (Chunk Size)">
                  <span class="tabular-nums">{{ detail.kb_config.chunk_size }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="切片重叠 (Overlap)">
                  <span class="tabular-nums">{{ detail.kb_config.chunk_overlap }}</span>
                </el-descriptions-item>
                <el-descriptions-item v-if="detail.kb_config.doc_count !== undefined" label="文档数量">
                  <span class="tabular-nums">{{ detail.kb_config.doc_count }}</span>
                </el-descriptions-item>
                <el-descriptions-item v-if="detail.kb_config.total_chunks !== undefined" label="切片总数">
                  <span class="tabular-nums">{{ detail.kb_config.total_chunks }}</span>
                </el-descriptions-item>
              </el-descriptions>

              <div class="code-block-label">目录与文件浏览（点击文件可在线预览或下载）</div>
              <div v-loading="loadingKbTree" class="kb-tree-box">
                <el-empty
                  v-if="kbRootEmpty"
                  description="该知识库暂无文件"
                  :image-size="48"
                />
                <el-tree
                  v-show="!kbRootEmpty"
                  :key="kbTreeKey"
                  lazy
                  :load="loadKbTreeNode"
                  :props="kbTreeProps"
                  node-key="key"
                  :default-expand-all="false"
                  :expand-on-click-node="false"
                  @node-click="handleKbNodeClick"
                >
                  <template #default="{ data }">
                    <div class="kb-tree-node">
                      <span class="kb-node-left">
                        <el-icon :size="15" :color="data.isFolder ? '#D97706' : isImageFile(data.label) ? '#059669' : '#4F46E5'">
                          <Folder v-if="data.isFolder" />
                          <Picture v-else-if="isImageFile(data.label)" />
                          <Document v-else />
                        </el-icon>
                        <span class="kb-node-name">{{ data.label }}</span>
                        <span v-if="!data.isFolder" class="kb-node-size tabular-nums">
                          {{ formatFileSize(data.fileSize) }}
                        </span>
                      </span>
                      <el-button
                        v-if="data.isFolder"
                        link
                        type="primary"
                        size="small"
                        @click.stop="handleDownloadKbFolder(data)"
                      >
                        下载目录
                      </el-button>
                      <el-button
                        v-else-if="data.doc"
                        link
                        type="primary"
                        size="small"
                        @click.stop="handleDownloadKbFile(data.doc.id, data.doc.file_name)"
                      >
                        下载
                      </el-button>
                    </div>
                  </template>
                </el-tree>
              </div>

              <div v-if="kbPreviewLoading" class="mt-12">
                <el-skeleton :rows="4" animated />
              </div>
              <div v-else-if="kbImageDoc" class="kb-inline-preview mt-12">
                <div class="kb-preview-head">
                  <span class="mono-text kb-preview-name">
                    {{ kbImageDoc.file_name }} ({{ formatFileSize(kbImageDoc.file_size) }})
                  </span>
                  <div class="kb-preview-actions">
                    <div v-if="sameFolderImages.length > 0" class="kb-img-nav">
                      <el-button
                        size="small"
                        :icon="ArrowLeft"
                        :disabled="currentKbImageIndex <= 0"
                        @click="selectKbImageAtIndex(currentKbImageIndex - 1)"
                      >
                        上一张
                      </el-button>
                      <span class="kb-img-counter tabular-nums">
                        {{ currentKbImageIndex + 1 }} / {{ sameFolderImages.length }}
                      </span>
                      <el-button
                        size="small"
                        :disabled="currentKbImageIndex >= sameFolderImages.length - 1"
                        @click="selectKbImageAtIndex(currentKbImageIndex + 1)"
                      >
                        下一张
                        <el-icon class="el-icon--right"><ArrowRight /></el-icon>
                      </el-button>
                    </div>
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :icon="Download"
                      @click="handleDownloadKbFile(kbImageDoc.id, kbImageDoc.file_name)"
                    >
                      下载图片
                    </el-button>
                  </div>
                </div>
                <div v-loading="loadingKbImageBlob" class="kb-image-wrap">
                  <el-image
                    v-if="kbImagePreviewUrl"
                    :src="kbImagePreviewUrl"
                    :preview-src-list="folderImageUrls"
                    :initial-index="currentKbImageIndex"
                    fit="contain"
                    style="max-height: 300px; width: 100%"
                    @switch="handleKbLightboxSwitch"
                  />
                </div>
              </div>
              <div v-else-if="kbBinaryDoc" class="kb-inline-preview mt-12">
                <div class="kb-binary-card">
                  <div>
                    <div class="mono-text kb-preview-name">{{ kbBinaryDoc.file_name }}</div>
                    <div class="kb-binary-meta">
                      <el-tag size="small" type="info" effect="plain" class="mono-text">
                        {{ kbBinaryDoc.file_ext || 'binary' }}
                      </el-tag>
                      <span class="tabular-nums">{{ formatFileSize(kbBinaryDoc.file_size) }}</span>
                    </div>
                  </div>
                  <el-button
                    type="primary"
                    size="small"
                    :icon="Download"
                    @click="handleDownloadKbFile(kbBinaryDoc.id, kbBinaryDoc.file_name)"
                  >
                    下载文件
                  </el-button>
                </div>
              </div>
              <div v-else-if="kbPreviewDoc" class="kb-inline-preview mt-12">
                <div class="kb-preview-head">
                  <span class="mono-text kb-preview-name">{{ kbPreviewDoc.file_name }}</span>
                  <div class="kb-preview-actions">
                    <el-radio-group v-model="kbPreviewMode" size="small">
                      <el-radio-button v-if="isMarkdownFile(kbPreviewDoc.file_name)" value="rendered">
                        渲染预览
                      </el-radio-button>
                      <el-radio-button value="raw">原文</el-radio-button>
                    </el-radio-group>
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :icon="Download"
                      @click="handleDownloadKbFile(kbPreviewDoc.id, kbPreviewDoc.file_name)"
                    >
                      下载
                    </el-button>
                  </div>
                </div>
                <div
                  v-if="kbPreviewMode === 'rendered' && isMarkdownFile(kbPreviewDoc.file_name)"
                  class="markdown-body kb-preview-body"
                  v-html="kbRenderedHtml"
                />
                <pre v-else class="code-block mono-text">{{ kbPreviewDoc.content }}</pre>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="在线调试" name="invoke">
          <div class="invoke-panel">
            <div v-if="detail.asset_type === 'kb'" class="kb-invoke-inputs">
              <el-input
                v-model="kbQueryText"
                placeholder="输入检索问题或关键词..."
                clearable
              />
              <el-input-number v-model="kbTopK" :min="1" :max="20" style="width: 120px" />
            </div>
            <div v-else>
              <div class="code-block-label">请求载荷 (JSON)</div>
              <el-input
                v-model="invokePayloadText"
                type="textarea"
                :rows="6"
                class="mono-text"
              />
            </div>

            <div class="invoke-actions">
              <el-button
                type="primary"
                :icon="VideoPlay"
                :loading="invoking"
                @click="handleRunInvoke"
              >
                发起调用测试
              </el-button>
            </div>

            <el-alert
              v-if="invokeError"
              :title="invokeError"
              type="error"
              show-icon
              :closable="false"
            />

            <div v-if="invokeResultText">
              <div class="code-block-label">响应结果</div>
              <pre class="code-block mono-text">{{ invokeResultText }}</pre>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <el-empty v-else description="未能加载资产详情" />

    <!-- Request Approval Modal -->
    <el-dialog v-model="approvalModalOpen" title="发起跨团队/受控权限申请" width="460px" append-to-body>
      <el-form
        ref="approvalFormRef"
        :model="approvalForm"
        :rules="approvalRules"
        label-position="top"
      >
        <el-form-item label="申请权限级别" prop="requested_perm">
          <el-select v-model="approvalForm.requested_perm" style="width: 100%">
            <el-option label="调用权限 (invoke)" value="invoke" />
            <el-option label="协作编辑 (edit)" value="edit" />
          </el-select>
        </el-form-item>
        <el-form-item label="授权有效期 (天)" prop="duration_days">
          <el-input-number v-model="approvalForm.duration_days" :min="1" :max="365" style="width: 100%" />
        </el-form-item>
        <el-form-item label="申请理由与使用场景" prop="reason">
          <el-input
            v-model="approvalForm.reason"
            type="textarea"
            :rows="3"
            placeholder="请说明调用该资产的业务场景与预期用途..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="approvalModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="submittingApproval" @click="handleSubmitApproval">
          提交申请
        </el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<style scoped>
.drawer-loading {
  padding: 12px 0;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.detail-header {
  padding-bottom: 14px;
  border-bottom: 1px solid var(--ascp-border);
}

.detail-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.version-pill {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 999px;
  background-color: #f1f5f9;
  color: #475569;
}

.detail-name {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.detail-code {
  margin-top: 4px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.detail-summary {
  margin: 10px 0 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--ascp-text-secondary);
}

.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.detail-action-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
}

.tab-section {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.config-section {
  display: flex;
  flex-direction: column;
}

.section-title {
  margin: 0 0 12px;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.mb-12 {
  margin-bottom: 12px;
}

.mt-12 {
  margin-top: 12px;
}

.code-block-label {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--ascp-text-secondary);
  margin-bottom: 6px;
}

.code-block {
  margin: 0;
  padding: 12px 14px;
  background-color: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 280px;
  overflow-y: auto;
}

.break-all {
  word-break: break-all;
}

.invoke-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.kb-invoke-inputs {
  display: flex;
  gap: 10px;
}

.invoke-actions {
  display: flex;
  justify-content: flex-end;
}

.skill-download-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.source-link {
  color: var(--ascp-primary);
  text-decoration: none;
  word-break: break-all;
}

.source-link:hover {
  text-decoration: underline;
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 12px;
}

.package-files-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.kb-tree-box {
  border: 1px solid var(--ascp-border);
  border-radius: 8px;
  padding: 8px;
  background: #f8fafc;
  max-height: 240px;
  overflow-y: auto;
}

.kb-tree-node {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-right: 6px;
  min-width: 0;
}

.kb-node-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.kb-node-name {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-node-size {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
}

.kb-inline-preview {
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  overflow: hidden;
  background: #ffffff;
}

.kb-preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 14px;
  background: #f8fafc;
  border-bottom: 1px solid var(--ascp-border);
  flex-wrap: wrap;
}

.kb-preview-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.kb-preview-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.kb-img-nav {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.kb-img-counter {
  font-size: 12px;
  color: var(--ascp-text-secondary);
  padding: 0 2px;
}

.kb-preview-body {
  padding: 14px 16px;
  max-height: 320px;
  overflow-y: auto;
}

.kb-image-wrap {
  padding: 14px;
  display: flex;
  justify-content: center;
  background: #f8fafc;
}

.kb-binary-card {
  padding: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.kb-binary-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}
</style>
