<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Connection, Cpu, Document, Histogram, MagicStick, Search } from '@element-plus/icons-vue'
import { getAssets } from '@/api/asset'
import AssetDetailDrawer from '@/components/AssetDetailDrawer.vue'
import { useDomainStore } from '@/stores/domain'
import type { AssetItem, AssetType } from '@/types/api'
import { getAssetStatusMeta, getAssetTypeMeta, normalizeList } from '@/utils/format'

const route = useRoute()
const domainStore = useDomainStore()

const domainId = computed(() => Number(route.params.domainId))
const currentDomain = computed(() => domainStore.getDomainById(domainId.value))

const loading = ref(false)
const hasLoadedOnce = ref(false)
const isRefreshing = ref(false)
const assets = ref<AssetItem[]>([])
const total = ref(0)

let latestRequestId = 0
let refreshTimer: ReturnType<typeof setTimeout> | null = null

function clearRefreshTimer() {
  if (refreshTimer) {
    clearTimeout(refreshTimer)
    refreshTimer = null
  }
}

const filters = reactive<{
  asset_type: AssetType | ''
  keyword: string
  page: number
  page_size: number
}>({
  asset_type: '',
  keyword: '',
  page: 1,
  page_size: 12
})

const detailDrawerOpen = ref(false)
const selectedAssetId = ref<number | null>(null)

const typeTabs: Array<{ label: string; value: AssetType | '' }> = [
  { label: '全部能力', value: '' },
  { label: 'MCP 服务', value: 'mcp' },
  { label: 'Skills 技能', value: 'skill' },
  { label: 'Agent 实例', value: 'agent' },
  { label: '私有知识库', value: 'kb' }
]

function getTypeIcon(type: AssetType) {
  switch (type) {
    case 'mcp':
      return Connection
    case 'skill':
      return MagicStick
    case 'agent':
      return Cpu
    case 'kb':
      return Document
  }
}

function getOwnerLabel(asset: AssetItem): string {
  return asset.owner?.nickname || asset.owner?.username || asset.owner_name || `用户 #${asset.owner_id}`
}

async function fetchAssets() {
  if (!domainId.value) return
  const requestId = ++latestRequestId
  clearRefreshTimer()

  if (!hasLoadedOnce.value) {
    loading.value = true
  } else {
    refreshTimer = setTimeout(() => {
      if (requestId === latestRequestId) {
        isRefreshing.value = true
      }
    }, 150)
  }

  try {
    const res = await getAssets({
      scope: 'marketplace',
      domain_id: domainId.value,
      asset_type: filters.asset_type || undefined,
      keyword: filters.keyword.trim() || undefined,
      page: filters.page,
      page_size: filters.page_size
    })
    if (requestId !== latestRequestId) return
    const normalized = normalizeList(res)
    assets.value = normalized.items
    total.value = normalized.total
  } catch {
    if (requestId !== latestRequestId) return
    assets.value = []
    total.value = 0
  } finally {
    if (requestId === latestRequestId) {
      clearRefreshTimer()
      loading.value = false
      isRefreshing.value = false
      hasLoadedOnce.value = true
    }
  }
}

function handleTabChange(val: AssetType | '') {
  filters.asset_type = val
  filters.page = 1
  fetchAssets()
}

function handleFilterChange() {
  filters.page = 1
  fetchAssets()
}

function handlePageChange(page: number) {
  filters.page = page
  fetchAssets()
}

function openDetail(asset: AssetItem) {
  selectedAssetId.value = asset.id
  detailDrawerOpen.value = true
}

watch(domainId, () => {
  filters.page = 1
  fetchAssets()
})

onMounted(() => {
  domainStore.fetchDomains()
  fetchAssets()
})

onBeforeUnmount(() => {
  clearRefreshTimer()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          域内能力广场
          <span v-if="currentDomain" class="domain-tag">{{ currentDomain.domain_name }}</span>
        </h1>
        <p class="page-subtitle">探索、订阅并在线调试当前业务域内已发布的 MCP 服务、Skills 技能、Agent 与知识库</p>
      </div>
      <div class="marketplace-count tabular-nums">
        共 <strong>{{ total }}</strong> 项可用能力
      </div>
    </div>

    <!-- Filter Bar -->
    <div class="surface-card filter-bar">
      <div class="type-tabs" role="tablist">
        <button
          v-for="tab in typeTabs"
          :key="tab.value || 'all'"
          type="button"
          class="type-tab"
          :class="{ 'is-active': filters.asset_type === tab.value }"
          @click="handleTabChange(tab.value)"
        >
          {{ tab.label }}
        </button>
      </div>

      <div class="filter-controls">
        <el-input
          v-model="filters.keyword"
          placeholder="搜索资产名称、编码或标签..."
          clearable
          style="width: 260px"
          :prefix-icon="Search"
          @keyup.enter="handleFilterChange"
          @clear="handleFilterChange"
        />

        <el-button type="primary" plain @click="handleFilterChange">搜索</el-button>
      </div>
    </div>

    <!-- Loading Skeleton (initial load only) -->
    <div v-if="loading && !hasLoadedOnce" class="asset-grid">
      <div v-for="i in 6" :key="i" class="surface-card asset-card skeleton-card">
        <el-skeleton :rows="4" animated />
      </div>
    </div>

    <!-- Empty State -->
    <div v-else-if="assets.length === 0" class="surface-card empty-box" :class="{ 'is-refreshing': isRefreshing }">
      <el-empty description="当前业务域暂无符合条件的已发布 AI 能力" />
    </div>

    <!-- Asset Card Grid -->
    <div v-else class="asset-grid" :class="{ 'is-refreshing': isRefreshing }">
      <article
        v-for="asset in assets"
        :key="asset.id"
        class="surface-card asset-card"
        tabindex="0"
        @click="openDetail(asset)"
        @keyup.enter="openDetail(asset)"
      >
        <div class="card-top">
          <div class="type-badge-wrap">
            <span
              class="type-icon-box"
              :style="{
                color: getAssetTypeMeta(asset.asset_type).color,
                backgroundColor: getAssetTypeMeta(asset.asset_type).bgColor
              }"
            >
              <el-icon :size="16">
                <component :is="getTypeIcon(asset.asset_type)" />
              </el-icon>
            </span>
            <el-tag :type="getAssetTypeMeta(asset.asset_type).tagType" size="small" effect="light">
              {{ getAssetTypeMeta(asset.asset_type).shortLabel }}
            </el-tag>
          </div>
          <div class="card-badges">
            <span class="version-chip mono-text">{{ asset.current_version || 'v0.1.0' }}</span>
            <el-tag :type="getAssetStatusMeta(asset.status).tagType" size="small" effect="plain">
              {{ getAssetStatusMeta(asset.status).label }}
            </el-tag>
          </div>
        </div>

        <h3 class="card-title">{{ asset.name }}</h3>
        <div class="card-code mono-text">{{ asset.asset_code }}</div>
        <p class="card-summary">{{ asset.summary || '暂无能力描述' }}</p>

        <div v-if="asset.tags?.length" class="card-tags">
          <el-tag
            v-for="tag in asset.tags.slice(0, 3)"
            :key="tag"
            size="small"
            type="info"
            effect="plain"
          >
            {{ tag }}
          </el-tag>
          <span v-if="asset.tags.length > 3" class="more-tags">+{{ asset.tags.length - 3 }}</span>
        </div>

        <div class="card-footer">
          <div class="footer-meta">
            <span class="meta-domain">{{ asset.domain_name || currentDomain?.domain_name || `域 #${asset.domain_id}` }}</span>
            <span class="meta-dot">·</span>
            <span class="meta-owner">{{ getOwnerLabel(asset) }}</span>
          </div>
          <div class="invoke-stat tabular-nums" title="累计调用次数">
            <el-icon :size="14"><Histogram /></el-icon>
            <span>{{ asset.invoke_count ?? 0 }}</span>
          </div>
        </div>
      </article>
    </div>

    <div v-if="total > filters.page_size" class="pagination-wrap">
      <el-pagination
        v-model:current-page="filters.page"
        :page-size="filters.page_size"
        :total="total"
        layout="total, prev, pager, next"
        background
        @current-change="handlePageChange"
      />
    </div>

    <AssetDetailDrawer
      v-model="detailDrawerOpen"
      :asset-id="selectedAssetId"
      :show-actions="true"
    />
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

.marketplace-count {
  font-size: 13.5px;
  color: var(--ascp-text-muted);
}

.marketplace-count strong {
  color: var(--ascp-primary);
  font-weight: 600;
}

.filter-bar {
  padding: 14px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.type-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.type-tab {
  border: 1px solid transparent;
  background: transparent;
  padding: 7px 14px;
  border-radius: 8px;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--ascp-text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.type-tab:hover {
  background-color: #f1f5f9;
  color: var(--ascp-text-primary);
}

.type-tab.is-active {
  background-color: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  border-color: #c7d2fe;
  font-weight: 600;
}

.filter-controls {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.asset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.asset-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.asset-card:hover {
  transform: translateY(-2px);
  border-color: #a5b4fc;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.07);
}

.skeleton-card {
  cursor: default;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 14px;
}

.type-badge-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.type-icon-box {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.version-chip {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
  background: #f1f5f9;
  padding: 2px 7px;
  border-radius: 999px;
}

.card-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  line-height: 1.35;
}

.card-code {
  margin-top: 3px;
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.card-summary {
  margin: 10px 0 14px;
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--ascp-text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 42px;
}

.card-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-bottom: 14px;
}

.more-tags {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.card-footer {
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.footer-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-domain {
  color: var(--ascp-text-secondary);
  font-weight: 500;
}

.invoke-stat {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--ascp-text-secondary);
  font-weight: 500;
}

.empty-box {
  padding: 48px 16px;
}

.is-refreshing {
  opacity: 0.7;
  transition: opacity 0.15s ease;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  padding-top: 4px;
}
</style>
