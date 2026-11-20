<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getAssetDetail } from '@/api/asset'
import type { AssetItem } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getVisibilityLabel
} from '@/utils/format'

const props = defineProps<{
  modelValue: boolean
  assetId: number | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const loading = ref(false)
const detail = ref<AssetItem | null>(null)

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

watch(
  () => [props.modelValue, props.assetId] as const,
  async ([isOpen, id]) => {
    if (!isOpen || !id) {
      return
    }
    loading.value = true
    detail.value = null
    try {
      detail.value = await getAssetDetail(id)
    } catch {
      detail.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)
</script>

<template>
  <el-drawer
    v-model="visible"
    title="AI 资产详情"
    size="620px"
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
      </div>

      <el-descriptions :column="2" border size="small" class="meta-descriptions">
        <el-descriptions-item label="所属业务域">
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
        <h4 class="section-title">MCP 服务配置</h4>
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="传输协议">
            <span class="mono-text">{{ detail.mcp_config.transport_type }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="服务端点 URL">
            <span class="mono-text break-all">{{ detail.mcp_config.endpoint_url || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="鉴权方式">
            <span class="mono-text">{{ detail.mcp_config.auth_type }}</span>
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
      </div>

      <!-- Skill Config -->
      <div v-else-if="detail.asset_type === 'skill' && detail.skill_config" class="config-section">
        <h4 class="section-title">Skills 技能配置</h4>
        <el-descriptions :column="2" border size="small" class="mb-12">
          <el-descriptions-item label="技能类型">
            <span class="mono-text">{{ detail.skill_config.skill_type }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="运行环境">
            <span class="mono-text">{{ detail.skill_config.runtime_env || '-' }}</span>
          </el-descriptions-item>
        </el-descriptions>
        <div class="code-block-label">技能定义 / Prompt 内容</div>
        <pre class="code-block mono-text">{{ detail.skill_config.content || '暂无内容' }}</pre>
      </div>

      <!-- Agent Config -->
      <div v-else-if="detail.asset_type === 'agent' && detail.agent_config" class="config-section">
        <h4 class="section-title">Agent 实例配置</h4>
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
        </el-descriptions>
        <div class="code-block-label">System Prompt</div>
        <pre class="code-block mono-text">{{ detail.agent_config.system_prompt || '暂无系统提示词' }}</pre>
      </div>

      <!-- KB Config -->
      <div v-else-if="detail.asset_type === 'kb' && detail.kb_config" class="config-section">
        <h4 class="section-title">私有知识库配置</h4>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="切片大小 (Chunk Size)">
            <span class="tabular-nums">{{ detail.kb_config.chunk_size }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="切片重叠 (Overlap)">
            <span class="tabular-nums">{{ detail.kb_config.chunk_overlap }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="Embedding 模型" :span="2">
            <span class="mono-text">{{ detail.kb_config.embedding_model || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item v-if="detail.kb_config.doc_count !== undefined" label="文档数量">
            <span class="tabular-nums">{{ detail.kb_config.doc_count }}</span>
          </el-descriptions-item>
          <el-descriptions-item v-if="detail.kb_config.total_chunks !== undefined" label="切片总数">
            <span class="tabular-nums">{{ detail.kb_config.total_chunks }}</span>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </div>

    <el-empty v-else description="未能加载资产详情" />
  </el-drawer>
</template>

<style scoped>
.drawer-loading {
  padding: 12px 0;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-header {
  padding-bottom: 16px;
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
  margin: 12px 0 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--ascp-text-secondary);
}

.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
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
</style>
