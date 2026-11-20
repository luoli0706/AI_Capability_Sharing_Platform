<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Plus, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteAsset, getAssets, offlineAsset, publishAsset } from '@/api/asset'
import { getDomains } from '@/api/domain'
import AssetDetailDrawer from '@/components/AssetDetailDrawer.vue'
import AssetFormDrawer from '@/components/AssetFormDrawer.vue'
import type { AssetItem, AssetType, DomainItem } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getVisibilityLabel,
  normalizeList
} from '@/utils/format'

const loading = ref(false)
const assets = ref<AssetItem[]>([])
const total = ref(0)
const domains = ref<DomainItem[]>([])

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

async function fetchDomains() {
  try {
    const res = await getDomains({ page: 1, page_size: 100 })
    domains.value = normalizeList(res).items
  } catch {
    domains.value = []
  }
}

async function fetchWorkspaceAssets() {
  loading.value = true
  try {
    const res = await getAssets({
      scope: 'workspace',
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

async function handlePublish(asset: AssetItem) {
  try {
    await ElMessageBox.confirm(`确认发布资产「${asset.name}」到能力广场吗？`, '发布确认', {
      confirmButtonText: '确认发布',
      cancelButtonText: '取消',
      type: 'info'
    })
    await publishAsset(asset.id)
    ElMessage.success('发布成功')
    fetchWorkspaceAssets()
  } catch {
    // Cancelled or failed
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

onMounted(() => {
  fetchDomains()
  fetchWorkspaceAssets()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">资产工作台</h1>
        <p class="page-subtitle">创建、配置并管理您的 MCP 服务、Skills 技能、Agent 与私有知识库生命周期</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建资产</el-button>
    </div>

    <div class="surface-card toolbar-card">
      <div class="toolbar-filters">
        <el-select
          v-model="filters.asset_type"
          placeholder="全部类型"
          clearable
          style="width: 150px"
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
          style="width: 140px"
          @change="handleSearch"
        >
          <el-option label="草稿" value="draft" />
          <el-option label="已发布" value="published" />
          <el-option label="已下线" value="deprecated" />
        </el-select>

        <el-input
          v-model="filters.keyword"
          placeholder="搜索资产名称或编码..."
          clearable
          style="width: 240px"
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

        <el-table-column label="类型" width="115">
          <template #default="{ row }">
            <el-tag :type="getAssetTypeMeta(row.asset_type).tagType" size="small" effect="light">
              {{ getAssetTypeMeta(row.asset_type).shortLabel }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="所属业务域" min-width="130">
          <template #default="{ row }">
            {{ row.domain_name || `域 #${row.domain_id}` }}
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

        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <div class="action-group">
              <el-button link type="primary" size="small" @click="openDetail(row)">详情</el-button>
              <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
              <el-button
                v-if="row.status !== 'published'"
                link
                type="success"
                size="small"
                @click="handlePublish(row)"
              >
                发布
              </el-button>
              <el-button
                v-else
                link
                type="warning"
                size="small"
                @click="handleOffline(row)"
              >
                下线
              </el-button>
              <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="工作台暂无 AI 资产，点击右上角「新建资产」开始创建" />
        </template>
      </el-table>

      <div v-if="total > filters.page_size" class="table-pagination">
        <el-pagination
          v-model:current-page="filters.page"
          :page-size="filters.page_size"
          :total="total"
          layout="total, prev, pager, next"
          background
          @current-change="fetchWorkspaceAssets"
        />
      </div>
    </div>

    <AssetFormDrawer
      v-model="formDrawerOpen"
      :edit-asset-id="editAssetId"
      :domains="domains"
      @saved="fetchWorkspaceAssets"
    />

    <AssetDetailDrawer
      v-model="detailDrawerOpen"
      :asset-id="detailAssetId"
    />
  </div>
</template>

<style scoped>
.toolbar-card {
  padding: 14px 18px;
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.table-card {
  overflow: hidden;
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
  gap: 4px;
}

.table-pagination {
  padding: 14px 18px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--ascp-border);
}
</style>
