<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Connection, Delete, Plus, Refresh, View } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createCrossRef,
  deleteCrossRef,
  getCrossRefSourceDomains,
  getCrossRefSourceItems,
  getDomainCrossRefs
} from '@/api/crossRef'
import type {
  AssetItem,
  CrossRefSourceKbFile,
  CrossRefStatus,
  CrossRefType,
  DomainCrossRefItem,
  DomainItem
} from '@/types/api'
import { formatDateTime, formatFileSize, getAssetTypeMeta, normalizeList } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    domainId: number
    defaultRefType?: CrossRefType
  }>(),
  {
    defaultRefType: 'asset'
  }
)

const router = useRouter()

const loading = ref(false)
const crossRefs = ref<DomainCrossRefItem[]>([])

const createModalOpen = ref(false)
const loadingSourceDomains = ref(false)
const loadingSourceItems = ref(false)
const submitting = ref(false)

const sourceDomains = ref<DomainItem[]>([])
const sourceAssets = ref<AssetItem[]>([])
const sourceKbFiles = ref<CrossRefSourceKbFile[]>([])

const form = reactive<{
  source_domain_id: number | undefined
  ref_type: CrossRefType
  source_asset_id: number | undefined
  source_doc_id: number | undefined
  description: string
}>({
  source_domain_id: undefined,
  ref_type: props.defaultRefType,
  source_asset_id: undefined,
  source_doc_id: undefined,
  description: ''
})

const canSubmit = computed(() => {
  if (!form.source_domain_id) return false
  if (form.ref_type === 'asset') return Boolean(form.source_asset_id)
  return Boolean(form.source_doc_id && form.source_asset_id)
})

function getStatusMeta(status: CrossRefStatus | string): {
  label: string
  tagType: 'success' | 'warning' | 'danger' | 'info'
} {
  if (status === 'active') return { label: '已生效', tagType: 'success' }
  if (status === 'pending') return { label: '待审批', tagType: 'warning' }
  if (status === 'rejected') return { label: '已驳回', tagType: 'danger' }
  return { label: status || '未知', tagType: 'info' }
}

async function fetchCrossRefs() {
  if (!props.domainId) return
  loading.value = true
  try {
    const res = await getDomainCrossRefs(props.domainId, true)
    crossRefs.value = normalizeList(res).items
  } catch {
    crossRefs.value = []
  } finally {
    loading.value = false
  }
}

async function openCreateModal() {
  form.source_domain_id = undefined
  form.ref_type = props.defaultRefType
  form.source_asset_id = undefined
  form.source_doc_id = undefined
  form.description = ''
  sourceAssets.value = []
  sourceKbFiles.value = []
  createModalOpen.value = true

  loadingSourceDomains.value = true
  try {
    const res = await getCrossRefSourceDomains(props.domainId)
    sourceDomains.value = normalizeList(res).items
  } catch {
    sourceDomains.value = []
  } finally {
    loadingSourceDomains.value = false
  }
}

async function handleSourceDomainChange(domainId: number | undefined) {
  form.source_asset_id = undefined
  form.source_doc_id = undefined
  sourceAssets.value = []
  sourceKbFiles.value = []
  if (!domainId) return

  loadingSourceItems.value = true
  try {
    const res = await getCrossRefSourceItems(domainId)
    sourceAssets.value = Array.isArray(res?.assets) ? res.assets : []
    sourceKbFiles.value = Array.isArray(res?.kb_files) ? res.kb_files : []
  } catch {
    sourceAssets.value = []
    sourceKbFiles.value = []
  } finally {
    loadingSourceItems.value = false
  }
}

function handleRefTypeChange() {
  form.source_asset_id = undefined
  form.source_doc_id = undefined
}

function handleSelectKbFile(docId: number | undefined) {
  if (!docId) {
    form.source_asset_id = undefined
    return
  }
  const found = sourceKbFiles.value.find((f) => f.id === docId)
  form.source_asset_id = found?.kb_asset_id
}

async function handleSubmit() {
  if (!props.domainId || !form.source_domain_id) {
    ElMessage.warning('请选择源域')
    return
  }
  if (form.ref_type === 'asset' && !form.source_asset_id) {
    ElMessage.warning('请选择要引用的资产')
    return
  }
  if (form.ref_type === 'kb_file' && (!form.source_doc_id || !form.source_asset_id)) {
    ElMessage.warning('请选择要引用的知识库文件')
    return
  }

  submitting.value = true
  try {
    const created = await createCrossRef(props.domainId, {
      source_domain_id: form.source_domain_id,
      ref_type: form.ref_type,
      source_asset_id: form.source_asset_id || 0,
      source_doc_id: form.ref_type === 'kb_file' ? form.source_doc_id || 0 : 0,
      description: form.description.trim()
    })
    if (created?.status === 'active') {
      ElMessage.success('跨域引用已直接生效')
    } else {
      ElMessage.success('已提交审批，需域管理员或所有者审批通过后生效')
    }
    createModalOpen.value = false
    await fetchCrossRefs()
  } catch {
    // Handled by interceptor
  } finally {
    submitting.value = false
  }
}

function canBrowseContent(row: DomainCrossRefItem): boolean {
  return row.status === 'active' && (row.ref_type === 'kb_file' || row.asset_type === 'kb')
}

function handleBrowseContent(row: DomainCrossRefItem) {
  router.push({
    path: `/domains/${props.domainId}/knowledge-bases`,
    query: { crossRefId: String(row.id) }
  })
}

async function handleDelete(item: DomainCrossRefItem) {
  if (!props.domainId) return
  try {
    await ElMessageBox.confirm(`确认移除跨域引用「${item.title}」吗？`, '移除引用确认', {
      confirmButtonText: '确认移除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await deleteCrossRef(props.domainId, item.id)
    ElMessage.success('已移除跨域引用')
    await fetchCrossRefs()
  } catch {
    // Cancelled or failed
  }
}

watch(
  () => props.domainId,
  () => {
    fetchCrossRefs()
  }
)

onMounted(() => {
  fetchCrossRefs()
})
</script>

<template>
  <section class="surface-card cross-ref-card" aria-label="跨域引用">
    <div class="cross-ref-header">
      <div class="cross-ref-title-wrap">
        <div class="cross-ref-icon">
          <el-icon :size="18"><Connection /></el-icon>
        </div>
        <div>
          <h2 class="cross-ref-title">跨域引用</h2>
          <p class="cross-ref-subtitle">
            将其他可见域的 AI 资产或知识库文件引入当前域使用，支持按权限直接生效或发起审批
          </p>
        </div>
      </div>
      <div class="cross-ref-actions">
        <el-button :icon="Refresh" :loading="loading" @click="fetchCrossRefs">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreateModal">发起跨域引用</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="crossRefs" stripe style="width: 100%">
      <el-table-column label="引用类型" width="130">
        <template #default="{ row }">
          <el-tag
            :type="row.ref_type === 'kb_file' ? 'success' : 'primary'"
            size="small"
            effect="light"
          >
            {{ row.ref_type === 'kb_file' ? '引用知识库文件' : '引用资产' }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="引用对象" min-width="210">
        <template #default="{ row }">
          <div class="ref-target-cell">
            <span class="ref-target-title">{{ row.title }}</span>
            <div class="ref-target-meta">
              <el-tag
                v-if="row.ref_type === 'asset' && row.asset_type"
                :type="getAssetTypeMeta(row.asset_type).tagType"
                size="small"
                effect="plain"
              >
                {{ getAssetTypeMeta(row.asset_type).shortLabel }}
              </el-tag>
              <template v-else-if="row.ref_type === 'kb_file'">
                <el-tag v-if="row.file_ext" size="small" type="info" effect="plain" class="mono-text">
                  {{ row.file_ext }}
                </el-tag>
                <span v-if="row.file_size" class="meta-size tabular-nums">
                  {{ formatFileSize(row.file_size) }}
                </span>
              </template>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="源域" min-width="140">
        <template #default="{ row }">
          <span class="source-domain-pill">{{ row.source_domain_name || `域 #${row.source_domain_id}` }}</span>
        </template>
      </el-table-column>

      <el-table-column label="状态" width="95">
        <template #default="{ row }">
          <el-tag :type="getStatusMeta(row.status).tagType" size="small" effect="plain">
            {{ getStatusMeta(row.status).label }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="创建人" width="115">
        <template #default="{ row }">
          {{ row.creator_name || `用户 #${row.created_by}` }}
        </template>
      </el-table-column>

      <el-table-column label="备注说明" min-width="180">
        <template #default="{ row }">
          <span class="desc-cell">{{ row.description || '-' }}</span>
        </template>
      </el-table-column>

      <el-table-column label="创建时间" width="155">
        <template #default="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </el-table-column>

      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <div class="row-actions">
            <el-button
              v-if="canBrowseContent(row)"
              link
              type="primary"
              size="small"
              :icon="View"
              @click="handleBrowseContent(row)"
            >
              浏览内容
            </el-button>
            <el-button link type="danger" size="small" :icon="Delete" @click="handleDelete(row)">
              删除
            </el-button>
          </div>
        </template>
      </el-table-column>

      <template #empty>
        <el-empty description="当前域暂无跨域引用，点击右上角「发起跨域引用」引入外部资源" :image-size="64" />
      </template>
    </el-table>

    <!-- Create Cross-Ref Modal -->
    <el-dialog v-model="createModalOpen" title="发起跨域引用" width="520px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="选择源域" required>
          <el-select
            v-model="form.source_domain_id"
            placeholder="请选择可见的源域"
            filterable
            :loading="loadingSourceDomains"
            style="width: 100%"
            @change="handleSourceDomainChange"
          >
            <el-option
              v-for="d in sourceDomains"
              :key="d.id"
              :label="`${d.domain_name} (${d.domain_code})`"
              :value="d.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="引用类型" required>
          <el-radio-group v-model="form.ref_type" @change="handleRefTypeChange">
            <el-radio value="asset">引用资产</el-radio>
            <el-radio value="kb_file">引用知识库文件</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="form.ref_type === 'asset'" label="选择资产" required>
          <el-select
            v-model="form.source_asset_id"
            placeholder="请先选择源域，再选择要引用的资产"
            filterable
            :disabled="!form.source_domain_id"
            :loading="loadingSourceItems"
            style="width: 100%"
          >
            <el-option
              v-for="asset in sourceAssets"
              :key="asset.id"
              :label="`${asset.name} (${asset.asset_code}) · ${getAssetTypeMeta(asset.asset_type).shortLabel}`"
              :value="asset.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item v-else label="选择知识库文件" required>
          <el-select
            v-model="form.source_doc_id"
            placeholder="请先选择源域，再选择知识库文件"
            filterable
            :disabled="!form.source_domain_id"
            :loading="loadingSourceItems"
            style="width: 100%"
            @change="handleSelectKbFile"
          >
            <el-option
              v-for="file in sourceKbFiles"
              :key="file.id"
              :label="`${file.kb_name} / ${file.folder_path ? `${file.folder_path}/` : ''}${file.file_name} (${formatFileSize(file.file_size)})`"
              :value="file.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="引用说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="300"
            show-word-limit
            placeholder="请说明引入该资产或知识库文件的用途..."
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="createModalOpen = false">取消</el-button>
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!canSubmit"
          @click="handleSubmit"
        >
          提交引用
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.cross-ref-card {
  margin-top: 18px;
  padding: 20px;
}

.cross-ref-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.cross-ref-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.cross-ref-icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.cross-ref-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.cross-ref-subtitle {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--ascp-text-muted);
}

.cross-ref-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ref-target-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ref-target-title {
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.ref-target-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.meta-size {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.source-domain-pill {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12.5px;
  background: #f1f5f9;
  color: var(--ascp-text-secondary);
}

.desc-cell {
  font-size: 13px;
  color: var(--ascp-text-secondary);
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
