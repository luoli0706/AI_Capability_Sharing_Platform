<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import { getAuditLogs } from '@/api/audit'
import type { AssetType, AuditLogItem } from '@/types/api'
import { formatDateTime, getAssetTypeMeta, normalizeList } from '@/utils/format'

const loading = ref(false)
const logs = ref<AuditLogItem[]>([])
const total = ref(0)

const filters = reactive<{
  asset_type: AssetType | 'domain' | 'org' | ''
  action: string
  keyword: string
  page: number
  page_size: number
}>({
  asset_type: '',
  action: '',
  keyword: '',
  page: 1,
  page_size: 20
})

const actionGroups = [
  {
    label: '资产生命周期',
    options: [
      { label: '创建资产', value: 'asset_create' },
      { label: '更新资产', value: 'asset_update' },
      { label: '提交发布', value: 'asset_publish_submit' },
      { label: '审批通过发布', value: 'asset_publish_approve' },
      { label: '审批驳回', value: 'asset_publish_reject' },
      { label: '下线资产', value: 'asset_offline' },
      { label: '删除资产', value: 'asset_delete' },
      { label: '版本回滚', value: 'asset_rollback' }
    ]
  },
  {
    label: '知识库文件操作',
    options: [
      { label: '新建文件夹', value: 'kb_folder_create' },
      { label: '上传文件', value: 'kb_file_upload' },
      { label: '移动文件', value: 'kb_file_move' },
      { label: '复制文件', value: 'kb_file_copy' },
      { label: '删除文件/文件夹', value: 'kb_file_delete' }
    ]
  },
  {
    label: '域管理操作',
    options: [
      { label: '创建域', value: 'domain.create' },
      { label: '更新域', value: 'domain.update' },
      { label: '删除域', value: 'domain.delete' },
      { label: '添加域成员', value: 'domain.member_add' },
      { label: '移除域成员', value: 'domain.member_remove' }
    ]
  },
  {
    label: '组织管理操作',
    options: [
      { label: '创建组织', value: 'org.create' },
      { label: '更新组织', value: 'org.update' },
      { label: '删除组织', value: 'org.delete' },
      { label: '添加组织成员', value: 'org.member_add' },
      { label: '移除组织成员', value: 'org.member_remove' }
    ]
  }
]

const ACTION_META_MAP: Record<
  string,
  { label: string; tagType: 'primary' | 'success' | 'warning' | 'info' | 'danger' }
> = {
  asset_create: { label: '创建资产', tagType: 'primary' },
  asset_update: { label: '更新资产', tagType: 'info' },
  asset_publish_submit: { label: '提交发布', tagType: 'warning' },
  asset_publish_approve: { label: '发布通过', tagType: 'success' },
  asset_publish_reject: { label: '发布驳回', tagType: 'danger' },
  asset_offline: { label: '下线资产', tagType: 'warning' },
  asset_delete: { label: '删除资产', tagType: 'danger' },
  asset_rollback: { label: '版本回滚', tagType: 'warning' },
  kb_folder_create: { label: '新建文件夹', tagType: 'primary' },
  kb_file_upload: { label: '上传文件', tagType: 'success' },
  kb_file_move: { label: '移动文件', tagType: 'warning' },
  kb_file_copy: { label: '复制文件', tagType: 'info' },
  kb_file_delete: { label: '删除文件', tagType: 'danger' },
  'domain.create': { label: '创建域', tagType: 'primary' },
  'domain.update': { label: '更新域', tagType: 'info' },
  'domain.delete': { label: '删除域', tagType: 'danger' },
  'domain.member_add': { label: '添加域成员', tagType: 'success' },
  'domain.member_remove': { label: '移除域成员', tagType: 'warning' },
  'org.create': { label: '创建组织', tagType: 'primary' },
  'org.update': { label: '更新组织', tagType: 'info' },
  'org.delete': { label: '删除组织', tagType: 'danger' },
  'org.member_add': { label: '添加组织成员', tagType: 'success' },
  'org.member_remove': { label: '移除组织成员', tagType: 'warning' }
}

function getActionMeta(action: string) {
  return ACTION_META_MAP[action] ?? { label: action, tagType: 'info' as const }
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await getAuditLogs({
      asset_type: filters.asset_type || undefined,
      action: filters.action || undefined,
      keyword: filters.keyword.trim() || undefined,
      page: filters.page,
      page_size: filters.page_size
    })
    const normalized = normalizeList(res)
    logs.value = normalized.items
    total.value = normalized.total
  } catch {
    logs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  filters.page = 1
  fetchLogs()
}

function handleSizeChange(size: number) {
  filters.page_size = size
  filters.page = 1
  fetchLogs()
}

function handlePageChange(newPage: number) {
  filters.page = newPage
  fetchLogs()
}

onMounted(() => {
  fetchLogs()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">日志中心</h1>
        <p class="page-subtitle">按页懒加载追踪团队内 MCP、Skills、Agent、知识库、域与组织的操作审计日志</p>
      </div>
      <el-button :icon="Refresh" @click="fetchLogs">刷新</el-button>
    </div>

    <div class="surface-card toolbar-card">
      <div class="toolbar-filters">
        <el-select
          v-model="filters.asset_type"
          placeholder="全部对象类型"
          clearable
          style="width: 150px"
          @change="handleSearch"
        >
          <el-option label="MCP 服务" value="mcp" />
          <el-option label="Skills 技能" value="skill" />
          <el-option label="Agent 实例" value="agent" />
          <el-option label="私有知识库" value="kb" />
          <el-option label="域" value="domain" />
          <el-option label="组织" value="org" />
        </el-select>

        <el-select
          v-model="filters.action"
          placeholder="全部操作动作"
          clearable
          style="width: 190px"
          @change="handleSearch"
        >
          <el-option-group
            v-for="grp in actionGroups"
            :key="grp.label"
            :label="grp.label"
          >
            <el-option
              v-for="opt in grp.options"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-option-group>
        </el-select>

        <el-input
          v-model="filters.keyword"
          placeholder="搜索操作人、资产名称、变动对象或详情..."
          clearable
          style="width: 300px"
          :prefix-icon="Search"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />

        <el-button type="primary" plain @click="handleSearch">查询</el-button>
      </div>
    </div>

    <div class="surface-card table-card">
      <el-table v-loading="loading" :data="logs" stripe style="width: 100%">
        <el-table-column label="操作时间" width="160">
          <template #default="{ row }">
            <span class="tabular-nums">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作人" width="120">
          <template #default="{ row }">
            {{ row.operator_name || `用户 #${row.operator_id}` }}
          </template>
        </el-table-column>

        <el-table-column label="所属域" min-width="130">
          <template #default="{ row }">
            {{ row.domain_name || (row.domain_id ? `域 #${row.domain_id}` : '-') }}
          </template>
        </el-table-column>

        <el-table-column label="对象名称 / 编码" min-width="200">
          <template #default="{ row }">
            <div class="asset-col">
              <div class="asset-title-row">
                <el-tag
                  v-if="row.asset_type"
                  :type="getAssetTypeMeta(row.asset_type).tagType"
                  size="small"
                  effect="light"
                >
                  {{ getAssetTypeMeta(row.asset_type).shortLabel }}
                </el-tag>
                <span class="asset-name">{{ row.asset_name || (row.asset_id ? `#${row.asset_id}` : '-') }}</span>
              </div>
              <span v-if="row.asset_code" class="asset-code mono-text">{{ row.asset_code }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作动作" width="125">
          <template #default="{ row }">
            <el-tag :type="getActionMeta(row.action).tagType" size="small" effect="plain">
              {{ getActionMeta(row.action).label }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="变动对象" min-width="160">
          <template #default="{ row }">
            <span class="mono-text target-name">{{ row.target_name || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="变动详情" min-width="220">
          <template #default="{ row }">
            <span class="detail-text">{{ row.detail || '-' }}</span>
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="暂无操作审计日志" />
        </template>
      </el-table>

      <div class="table-pagination">
        <el-pagination
          v-model:current-page="filters.page"
          v-model:page-size="filters.page_size"
          :page-sizes="[10, 20, 50]"
          :total="total"
          layout="total, sizes, prev, pager, next"
          background
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </div>
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

.asset-col {
  display: flex;
  flex-direction: column;
}

.asset-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.asset-name {
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.asset-code {
  font-size: 12px;
  color: var(--ascp-text-muted);
  margin-top: 2px;
}

.target-name {
  font-size: 12.5px;
  color: var(--ascp-text-secondary);
}

.detail-text {
  font-size: 13px;
  color: var(--ascp-text-secondary);
}

.table-pagination {
  padding: 14px 18px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--ascp-border);
}
</style>
