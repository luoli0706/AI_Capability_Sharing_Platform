<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { storeToRefs } from 'pinia'
import { MoreFilled, Plus, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteAsset,
  getAssets,
  mcpHealthCheck,
  mcpSyncTools,
  offlineAsset,
  publishAsset
} from '@/api/asset'
import AssetDetailDrawer from '@/components/AssetDetailDrawer.vue'
import AssetFormDrawer from '@/components/AssetFormDrawer.vue'
import AssetVersionsDrawer from '@/components/AssetVersionsDrawer.vue'
import { useDomainStore } from '@/stores/domain'
import type { AssetItem, AssetType } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getVisibilityLabel,
  normalizeList
} from '@/utils/format'

const route = useRoute()
const domainStore = useDomainStore()
const { domains } = storeToRefs(domainStore)

const domainId = computed(() => Number(route.params.domainId))
const currentDomain = computed(() => domainStore.getDomainById(domainId.value))

const loading = ref(false)
const assets = ref<AssetItem[]>([])
const total = ref(0)

const filters = reactive<{
  asset_type: AssetType | ''
  status: string
  keyword: string
  page: number
  page_size: number
}>({
  asset_type: '',
  status: '',
  keyword: '',
  page: 1,
  page_size: 10
})

const formDrawerOpen = ref(false)
const editAssetId = ref<number | null>(null)

const detailDrawerOpen = ref(false)
const detailAssetId = ref<number | null>(null)

const versionsDrawerOpen = ref(false)
const versionsAsset = ref<AssetItem | null>(null)

const publishModalOpen = ref(false)
const publishing = ref(false)
const publishTarget = ref<AssetItem | null>(null)
const publishForm = reactive({
  version: '',
  changelog: ''
})

async function fetchWorkspaceAssets() {
  if (!domainId.value) return
  loading.value = true
  try {
    const res = await getAssets({
      scope: 'workspace',
      domain_id: domainId.value,
      asset_type: filters.asset_type || undefined,
      status: filters.status || undefined,
      keyword: filters.keyword.trim() || undefined,
      page: filters.page,
      page_size: filters.page_size
    })
    const normalized = normalizeList(res)
    assets.value = normalized.items
    total.value = normalized.total
  } catch {
    assets.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  filters.page = 1
  fetchWorkspaceAssets()
}

function openCreate() {
  editAssetId.value = null
  formDrawerOpen.value = true
}

function openEdit(asset: AssetItem) {
  editAssetId.value = asset.id
  formDrawerOpen.value = true
}

function openDetail(asset: AssetItem) {
  detailAssetId.value = asset.id
  detailDrawerOpen.value = true
}

function openVersions(asset: AssetItem) {
  versionsAsset.value = asset
  versionsDrawerOpen.value = true
}

function openPublishModal(asset: AssetItem) {
  publishTarget.value = asset
  publishForm.version = asset.current_version || 'v0.1.0'
  publishForm.changelog = ''
  publishModalOpen.value = true
}

async function handleConfirmPublish() {
  if (!publishTarget.value) return
  publishing.value = true
  try {
    await publishAsset(publishTarget.value.id, {
      version: publishForm.version.trim() || undefined,
      changelog: publishForm.changelog.trim() || undefined
    })
    ElMessage.success('已提交发布申请，请等待域所有者或管理员在审批中心审批')
    publishModalOpen.value = false
    fetchWorkspaceAssets()
  } catch {
    // Handled by interceptor
  } finally {
    publishing.value = false
  }
}

async function handleOffline(asset: AssetItem) {
  try {
    await ElMessageBox.confirm(`确认下线资产「${asset.name}」吗？下线后能力广场将不可见。`, '下线确认', {
      confirmButtonText: '确认下线',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await offlineAsset(asset.id)
    ElMessage.success('已下线')
    fetchWorkspaceAssets()
  } catch {
    // Cancelled or failed
  }
}

async function handleDelete(asset: AssetItem) {
  try {
    await ElMessageBox.confirm(`确定删除资产「${asset.name}」吗？该操作不可恢复。`, '删除确认', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'error'
    })
    await deleteAsset(asset.id)
    ElMessage.success('删除成功')
    fetchWorkspaceAssets()
  } catch {
    // Cancelled or failed
  }
}

async function handleMoreCommand(command: string, asset: AssetItem) {
  if (command === 'versions') {
    openVersions(asset)
    return
  }
  if (command === 'health-check') {
    try {
      const res = await mcpHealthCheck(asset.id)
      ElMessage.success(`MCP 健康检查：${res.health_status || 'healthy'}`)
    } catch {
      // Handled by interceptor
    }
    return
  }
  if (command === 'sync-tools') {
    try {
      const res = await mcpSyncTools(asset.id)
      const count = res.count ?? res.tools_schema?.length ?? 0
      ElMessage.success(`已同步 ${count} 个 MCP Tools`)
    } catch {
      // Handled by interceptor
    }
    return
  }
  if (command === 'delete') {
    handleDelete(asset)
  }
}

watch(domainId, () => {
  filters.page = 1
  fetchWorkspaceAssets()
})

onMounted(() => {
  domainStore.fetchDomains()
  fetchWorkspaceAssets()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          域内资产工作台
          <span v-if="currentDomain" class="domain-tag">{{ currentDomain.domain_name }}</span>
        </h1>
        <p class="page-subtitle">在当前域内创建、配置与发布 MCP 服务、Skills 技能、Agent 与私有知识库</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建资产</el-button>
    </div>

    <div class="surface-card toolbar-card">
      <div class="toolbar-filters">
        <el-select
          v-model="filters.asset_type"
          placeholder="全部类型"
          clearable
          class="filter-select-type"
          @change="handleSearch"
        >
          <el-option label="MCP 服务" value="mcp" />
          <el-option label="Skills 技能" value="skill" />
          <el-option label="Agent 实例" value="agent" />
          <el-option label="私有知识库" value="kb" />
        </el-select>

        <el-select
          v-model="filters.status"
          placeholder="全部状态"
          clearable
          class="filter-select-status"
          @change="handleSearch"
        >
          <el-option label="草稿" value="draft" />
          <el-option label="待审批" value="pending" />
          <el-option label="已发布" value="published" />
          <el-option label="已下线" value="deprecated" />
        </el-select>

        <el-input
          v-model="filters.keyword"
          placeholder="搜索资产名称或编码..."
          clearable
          class="filter-search-input"
          :prefix-icon="Search"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />

        <el-button @click="handleSearch">查询</el-button>
      </div>
    </div>

    <div class="surface-card table-card">
      <el-table v-loading="loading" :data="assets" stripe style="width: 100%">
        <el-table-column label="资产名称 / 编码" min-width="220">
          <template #default="{ row }">
            <div class="asset-name-cell">
              <span class="cell-name" @click="openDetail(row)">{{ row.name }}</span>
              <span class="cell-code mono-text">{{ row.asset_code }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag :type="getAssetTypeMeta(row.asset_type).tagType" size="small" effect="light">
              {{ getAssetTypeMeta(row.asset_type).shortLabel }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="可见范围" width="120">
          <template #default="{ row }">
            {{ getVisibilityLabel(row.visibility) }}
          </template>
        </el-table-column>

        <el-table-column label="版本" width="95">
          <template #default="{ row }">
            <span class="mono-text">{{ row.current_version || 'v0.1.0' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getAssetStatusMeta(row.status).tagType" size="small" effect="plain">
              {{ getAssetStatusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="调用量" width="90" align="right">
          <template #default="{ row }">
            <span class="tabular-nums">{{ row.invoke_count ?? 0 }}</span>
          </template>
        </el-table-column>

        <el-table-column label="更新时间" width="155">
          <template #default="{ row }">
            {{ formatDateTime(row.updated_at) }}
          </template>
        </el-table-column>

        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <div class="action-group">
              <el-button link type="primary" size="small" @click="openDetail(row)">详情</el-button>
              <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
              <el-button
                v-if="row.status === 'draft' || row.status === 'deprecated'"
                link
                type="success"
                size="small"
                @click="openPublishModal(row)"
              >
                提交发布
              </el-button>
              <el-button
                v-else-if="row.status === 'pending'"
                link
                type="warning"
                size="small"
                disabled
              >
                待审批
              </el-button>
              <el-button
                v-else-if="row.status === 'published'"
                link
                type="warning"
                size="small"
                @click="handleOffline(row)"
              >
                下线
              </el-button>
              <el-dropdown trigger="click" @command="(cmd: string) => handleMoreCommand(cmd, row)">
                <el-button link type="info" size="small">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="versions">版本历史与回滚</el-dropdown-item>
                    <el-dropdown-item v-if="row.asset_type === 'mcp'" command="health-check">
                      MCP 健康检查
                    </el-dropdown-item>
                    <el-dropdown-item v-if="row.asset_type === 'mcp'" command="sync-tools">
                      同步 MCP Tools
                    </el-dropdown-item>
                    <el-dropdown-item divided command="delete">
                      <span style="color: #ef4444">删除资产</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="当前域暂无 AI 资产，点击右上角「新建资产」开始创建" />
        </template>
      </el-table>

      <div v-if="total > 0" class="table-pagination">
        <el-pagination
          v-model:current-page="filters.page"
          v-model:page-size="filters.page_size"
          :page-sizes="[10, 20, 50]"
          :total="total"
          layout="total, sizes, prev, pager, next"
          background
          @size-change="handleSearch"
          @current-change="fetchWorkspaceAssets"
        />
      </div>
    </div>

    <AssetFormDrawer
      v-model="formDrawerOpen"
      :edit-asset-id="editAssetId"
      :domains="domains"
      :fixed-domain-id="domainId"
      @saved="fetchWorkspaceAssets"
    />

    <AssetDetailDrawer
      v-model="detailDrawerOpen"
      :asset-id="detailAssetId"
      :show-actions="false"
    />

    <AssetVersionsDrawer
      v-model="versionsDrawerOpen"
      :asset-id="versionsAsset?.id ?? null"
      :asset-name="versionsAsset?.name"
      @rolled-back="fetchWorkspaceAssets"
    />

    <!-- Publish Modal -->
    <el-dialog v-model="publishModalOpen" title="提交发布申请" width="460px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="发布版本号">
          <el-input
            v-model="publishForm.version"
            placeholder="例如：v0.1.0 / v1.0.0（留空由系统自动递增）"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="版本变更说明 (Changelog)">
          <el-input
            v-model="publishForm.changelog"
            type="textarea"
            :rows="3"
            placeholder="说明本次发布新增或修复的能力，将提交给域所有者/管理员审批..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="publishModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="handleConfirmPublish">
          确认提交
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.domain-tag {
  font-size: 14px;
  font-weight: 500;
  color: var(--ascp-primary);
  background: var(--ascp-primary-soft);
  padding: 2px 10px;
  border-radius: 999px;
  margin-left: 8px;
  vertical-align: middle;
}

.toolbar-card {
  padding: 14px 18px;
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-select-type {
  width: 150px;
  max-width: 100%;
}

.filter-select-status {
  width: 140px;
  max-width: 100%;
}

.filter-search-input {
  width: 240px;
  max-width: 100%;
}

.table-card {
  overflow-x: auto;
}

.asset-name-cell {
  display: flex;
  flex-direction: column;
}

.cell-name {
  font-weight: 600;
  color: var(--ascp-text-primary);
  cursor: pointer;
}

.cell-name:hover {
  color: var(--ascp-primary);
}

.cell-code {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.action-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.table-pagination {
  padding: 14px 18px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--ascp-border);
}

@media (max-width: 768px) {
  .filter-select-type,
  .filter-select-status,
  .filter-search-input {
    width: 100%;
  }
}
</style>
