<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  ArrowLeft,
  CopyDocument,
  Download,
  FolderOpened,
  Key,
  Promotion,
  Share,
  VideoPlay
} from '@element-plus/icons-vue'
import {
  downloadAgentConfig,
  downloadSkillPackage,
  getAssetDetail,
  subscribeAsset
} from '@/api/asset'
import { createApproval } from '@/api/approval'
import { invokeAgent, invokeKBQuery, invokeMCP, invokeSkill } from '@/api/invoke'
import ShareAssetDialog from '@/components/ShareAssetDialog.vue'
import type { AssetItem } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getVisibilityLabel
} from '@/utils/format'

const route = useRoute()
const router = useRouter()

const assetId = computed(() => Number(route.params.id))
const loading = ref(false)
const detail = ref<AssetItem | null>(null)
const activeTab = ref('overview')

const subscribing = ref(false)
const shareDialogOpen = ref(false)
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

const invoking = ref(false)
const invokePayloadText = ref('{\n  "input": "hello"\n}')
const kbQueryText = ref('')
const kbTopK = ref(5)
const invokeResultText = ref('')
const invokeError = ref('')
const downloadingAgentFormat = ref<'md' | 'json' | null>(null)

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

async function fetchDetail() {
  if (!Number.isFinite(assetId.value) || assetId.value <= 0) {
    detail.value = null
    return
  }
  loading.value = true
  try {
    const res = await getAssetDetail(assetId.value)
    detail.value = res
    initDefaultPayload(res)
  } catch {
    detail.value = null
  } finally {
    loading.value = false
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else if (detail.value?.domain_id) {
    router.push(`/domains/${detail.value.domain_id}/marketplace`)
  } else {
    router.push('/domains')
  }
}

function goToKnowledgeBase() {
  if (!detail.value) return
  router.push({
    path: `/domains/${detail.value.domain_id}/knowledge-bases`,
    query: { kbId: String(detail.value.id) }
  })
}

async function handleSubscribe() {
  if (!detail.value) return
  subscribing.value = true
  try {
    await subscribeAsset(detail.value.id)
    ElMessage.success('已成功订阅该能力')
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
      reason: approvalForm.reason,
      applicant_domain_id: detail.value.domain_id
    })
    ElMessage.success('权限申请已提交，请等待审批')
    approvalModalOpen.value = false
  } catch {
    // Handled by interceptor
  } finally {
    submittingApproval.value = false
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

function handleCopyMcpClientConfig() {
  if (!detail.value || !detail.value.mcp_config) return
  const cfg = detail.value.mcp_config
  const key = detail.value.asset_code || detail.value.name || 'mcp-server'
  let serverEntry: Record<string, unknown>

  if (cfg.transport_type === 'stdio' && cfg.package_command) {
    const parts = cfg.package_command.trim().split(/\s+/)
    serverEntry = { command: parts[0], args: parts.slice(1) }
  } else {
    serverEntry = {
      url: cfg.endpoint_url || `${window.location.origin}/api/v1/invoke/mcp/${key}`,
      transport: cfg.transport_type
    }
    if (cfg.auth_type === 'bearer') {
      serverEntry.headers = { Authorization: 'Bearer <YOUR_TOKEN>' }
    }
  }
  copyText(JSON.stringify({ mcpServers: { [key]: serverEntry } }, null, 2), ' mcpServers 客户端配置')
}

async function handleDownloadSkill(format: 'zip' | 'md') {
  if (!detail.value) return
  const code = detail.value.asset_code || `skill-${detail.value.id}`
  const filename = format === 'zip' ? `${code}.zip` : `${code}.md`
  try {
    await downloadSkillPackage(detail.value.id, format, filename)
    ElMessage.success(`已开始下载 ${filename}`)
  } catch {
    // Handled by interceptor
  }
}

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

watch(assetId, () => {
  void fetchDetail()
})

onMounted(() => {
  void fetchDetail()
})
</script>

<template>
  <div class="page-container">
    <div class="top-nav-row">
      <el-button :icon="ArrowLeft" @click="goBack">返回</el-button>
      <div v-if="detail" class="top-actions">
        <el-button
          v-if="detail.asset_type === 'kb'"
          type="primary"
          :icon="FolderOpened"
          @click="goToKnowledgeBase"
        >
          进入知识库
        </el-button>
        <template v-if="detail.status === 'published'">
          <el-button
            type="primary"
            plain
            :icon="Promotion"
            :loading="subscribing"
            @click="handleSubscribe"
          >
            一键订阅
          </el-button>
          <el-button :icon="Key" @click="openApprovalModal">
            发起权限申请
          </el-button>
        </template>
        <el-button :icon="Share" @click="shareDialogOpen = true">分享</el-button>
      </div>
    </div>

    <div v-if="loading" class="surface-card detail-card">
      <el-skeleton :rows="10" animated />
    </div>

    <div v-else-if="detail" class="surface-card detail-card">
      <div class="detail-hero">
        <div class="badge-row">
          <el-tag v-if="typeMeta" :type="typeMeta.tagType" effect="light">
            {{ typeMeta.label }}
          </el-tag>
          <el-tag v-if="statusMeta" :type="statusMeta.tagType" effect="plain">
            {{ statusMeta.label }}
          </el-tag>
          <span class="version-pill mono-text">{{ detail.current_version || 'v0.1.0' }}</span>
        </div>

        <h1 class="asset-title">{{ detail.name }}</h1>
        <div class="asset-code mono-text">{{ detail.asset_code }}</div>
        <p class="asset-summary">{{ detail.summary || '暂无能力描述' }}</p>

        <div v-if="detail.tags?.length" class="tag-row">
          <el-tag v-for="tag in detail.tags" :key="tag" size="small" type="info" effect="plain">
            {{ tag }}
          </el-tag>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="detail-tabs">
        <el-tab-pane label="概览与配置元数据" name="overview">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="所属域">
              {{ detail.domain_name || `域 #${detail.domain_id}` }}
            </el-descriptions-item>
            <el-descriptions-item label="资产负责人">
              <router-link :to="`/users/${detail.owner?.id || detail.owner_id}`" class="owner-link">
                {{ ownerDisplay }}
              </router-link>
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
          <div v-if="detail.asset_type === 'mcp' && detail.mcp_config" class="config-block">
            <div class="config-header">
              <h3>MCP 服务配置</h3>
              <el-button size="small" type="primary" plain :icon="CopyDocument" @click="handleCopyMcpClientConfig">
                复制 mcpServers 配置
              </el-button>
            </div>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="传输协议">
                <span class="mono-text">{{ detail.mcp_config.transport_type }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="鉴权方式">
                <span class="mono-text">{{ detail.mcp_config.auth_type }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="Endpoint / Command" :span="2">
                <span class="mono-text">
                  {{ detail.mcp_config.endpoint_url || detail.mcp_config.package_command || '-' }}
                </span>
              </el-descriptions-item>
            </el-descriptions>

            <div v-if="detail.mcp_config.tools_schema?.length" class="schema-block">
              <div class="schema-label">Tools Schema ({{ detail.mcp_config.tools_schema.length }})</div>
              <pre class="code-block mono-text">{{ JSON.stringify(detail.mcp_config.tools_schema, null, 2) }}</pre>
            </div>
          </div>

          <!-- Skill Config -->
          <div v-if="detail.asset_type === 'skill' && detail.skill_config" class="config-block">
            <div class="config-header">
              <h3>Skill 技能配置与清单</h3>
              <div class="btn-group">
                <el-button size="small" plain :icon="Download" @click="handleDownloadSkill('md')">
                  下载 SKILL.md
                </el-button>
                <el-button size="small" type="primary" plain :icon="Download" @click="handleDownloadSkill('zip')">
                  下载完整包 (.zip)
                </el-button>
              </div>
            </div>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="技能类型">{{ detail.skill_config.skill_type }}</el-descriptions-item>
              <el-descriptions-item label="运行环境">{{ detail.skill_config.runtime_env || '-' }}</el-descriptions-item>
            </el-descriptions>
            <div v-if="detail.skill_config.content" class="schema-block">
              <div class="schema-label">Skill Prompt / 脚本内容</div>
              <pre class="code-block mono-text">{{ detail.skill_config.content }}</pre>
            </div>
            <div v-if="detail.skill_config.input_schema" class="schema-block">
              <div class="schema-label">Input Schema</div>
              <pre class="code-block mono-text">{{ JSON.stringify(detail.skill_config.input_schema, null, 2) }}</pre>
            </div>
            <div v-if="detail.skill_config.output_schema" class="schema-block">
              <div class="schema-label">Output Schema</div>
              <pre class="code-block mono-text">{{ JSON.stringify(detail.skill_config.output_schema, null, 2) }}</pre>
            </div>
          </div>

          <!-- Agent Config -->
          <div v-if="detail.asset_type === 'agent' && detail.agent_config" class="config-block">
            <div class="config-header">
              <h3>Agent 实例配置</h3>
              <div class="btn-group">
                <el-button
                  size="small"
                  plain
                  :icon="Download"
                  :loading="downloadingAgentFormat === 'md'"
                  @click="handleDownloadAgent('md')"
                >
                  导出 Markdown
                </el-button>
                <el-button
                  size="small"
                  type="primary"
                  plain
                  :icon="Download"
                  :loading="downloadingAgentFormat === 'json'"
                  @click="handleDownloadAgent('json')"
                >
                  导出 JSON
                </el-button>
              </div>
            </div>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="模型提供商">{{ detail.agent_config.model_provider }}</el-descriptions-item>
              <el-descriptions-item label="模型名称">{{ detail.agent_config.model_name }}</el-descriptions-item>
              <el-descriptions-item label="Temperature">{{ detail.agent_config.temperature }}</el-descriptions-item>
              <el-descriptions-item label="Max Tokens">{{ detail.agent_config.max_tokens }}</el-descriptions-item>
            </el-descriptions>
            <div v-if="detail.agent_config.system_prompt" class="schema-block">
              <div class="schema-label">System Prompt</div>
              <pre class="code-block mono-text">{{ detail.agent_config.system_prompt }}</pre>
            </div>
          </div>

          <!-- KB Config -->
          <div v-if="detail.asset_type === 'kb' && detail.kb_config" class="config-block">
            <div class="config-header">
              <h3>知识库配置</h3>
              <el-button size="small" type="primary" :icon="FolderOpened" @click="goToKnowledgeBase">
                进入知识库浏览文档
              </el-button>
            </div>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="切片大小 (chunk_size)">{{ detail.kb_config.chunk_size }}</el-descriptions-item>
              <el-descriptions-item label="重叠大小 (chunk_overlap)">{{ detail.kb_config.chunk_overlap }}</el-descriptions-item>
              <el-descriptions-item label="向量模型">{{ detail.kb_config.embedding_model || '-' }}</el-descriptions-item>
              <el-descriptions-item label="文档 / 切片数量">
                {{ detail.kb_config.doc_count ?? 0 }} / {{ detail.kb_config.total_chunks ?? 0 }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </el-tab-pane>

        <el-tab-pane label="在线调用测试" name="invoke">
          <div class="invoke-panel">
            <template v-if="detail.asset_type === 'kb'">
              <div class="kb-invoke-row">
                <el-input v-model="kbQueryText" placeholder="输入检索关键词..." clearable />
                <el-input-number v-model="kbTopK" :min="1" :max="20" />
              </div>
            </template>
            <template v-else>
              <el-input
                v-model="invokePayloadText"
                type="textarea"
                :rows="6"
                class="mono-text"
                placeholder="输入 JSON 格式调用参数..."
              />
            </template>

            <div>
              <el-button type="primary" :icon="VideoPlay" :loading="invoking" @click="handleRunInvoke">
                执行测试调用
              </el-button>
            </div>

            <el-alert v-if="invokeError" :title="invokeError" type="error" show-icon :closable="false" />

            <div v-if="invokeResultText">
              <div class="schema-label">响应结果</div>
              <pre class="code-block mono-text">{{ invokeResultText }}</pre>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <div v-else class="surface-card detail-card">
      <el-empty description="未找到该资产或暂无访问权限" />
    </div>

    <el-dialog v-model="approvalModalOpen" title="发起权限申请" width="460px" append-to-body>
      <el-form ref="approvalFormRef" :model="approvalForm" :rules="approvalRules" label-position="top">
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

    <ShareAssetDialog
      v-if="detail"
      v-model="shareDialogOpen"
      :asset-id="detail.id"
      :asset-name="detail.name"
      :asset-type="detail.asset_type"
      :domain-id="detail.domain_id"
      :domain-name="detail.domain_name"
    />
  </div>
</template>

<style scoped>
.top-nav-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.top-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-card {
  padding: 24px;
}

.detail-hero {
  margin-bottom: 20px;
}

.badge-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.version-pill {
  font-size: 12px;
  color: var(--ascp-text-sub, #475569);
  background: #f1f5f9;
  padding: 2px 8px;
  border-radius: 6px;
}

.asset-title {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 700;
  color: var(--ascp-text-main, #0f172a);
}

.asset-code {
  font-size: 13px;
  color: var(--ascp-text-muted, #64748b);
  margin-bottom: 10px;
}

.asset-summary {
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--ascp-text-sub, #475569);
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.owner-link {
  color: var(--ascp-primary, #4f46e5);
  text-decoration: none;
  font-weight: 500;
}

.config-block {
  margin-top: 22px;
}

.config-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.config-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ascp-text-main, #0f172a);
}

.btn-group {
  display: flex;
  gap: 8px;
}

.schema-block {
  margin-top: 14px;
}

.schema-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--ascp-text-sub, #475569);
  margin-bottom: 6px;
}

.code-block {
  margin: 0;
  padding: 12px 14px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.55;
  overflow-x: auto;
  max-height: 360px;
}

.invoke-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 8px;
}

.kb-invoke-row {
  display: flex;
  gap: 12px;
  align-items: center;
}
</style>
