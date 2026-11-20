<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getAssetVersions, rollbackAsset } from '@/api/asset'
import type { AssetVersion } from '@/types/api'
import { formatDateTime, normalizeList } from '@/utils/format'

const props = defineProps<{
  modelValue: boolean
  assetId: number | null
  assetName?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  rolledBack: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
})

const loading = ref(false)
const rollingBack = ref<string | null>(null)
const versions = ref<AssetVersion[]>([])

async function fetchVersions() {
  if (!props.assetId) return
  loading.value = true
  try {
    const res = await getAssetVersions(props.assetId)
    versions.value = normalizeList(res).items
  } catch {
    versions.value = []
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.assetId] as const,
  ([isOpen, id]) => {
    if (isOpen && id) {
      fetchVersions()
    }
  }
)

async function handleRollback(ver: AssetVersion) {
  if (!props.assetId) return
  try {
    await ElMessageBox.confirm(
      `确认将资产回滚到版本「${ver.version}」吗？回滚后将恢复该版本的配置快照。`,
      '版本回滚确认',
      {
        confirmButtonText: '确认回滚',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    rollingBack.value = ver.version
    await rollbackAsset(props.assetId, ver.id)
    ElMessage.success(`已回滚至版本 ${ver.version}`)
    await fetchVersions()
    emit('rolledBack')
  } catch {
    // Cancelled or failed
  } finally {
    rollingBack.value = null
  }
}
</script>

<template>
  <el-drawer
    v-model="visible"
    :title="`版本历史与回滚${assetName ? ` - ${assetName}` : ''}`"
    size="560px"
    destroy-on-close
  >
    <div v-if="loading" class="loading-wrap">
      <el-skeleton :rows="6" animated />
    </div>

    <el-empty v-else-if="versions.length === 0" description="暂无历史发布版本" />

    <el-timeline v-else class="version-timeline">
      <el-timeline-item
        v-for="ver in versions"
        :key="ver.id"
        :type="ver.is_active ? 'primary' : 'info'"
        :timestamp="formatDateTime(ver.created_at)"
        placement="top"
      >
        <div class="version-card">
          <div class="version-head">
            <div class="version-left">
              <span class="version-num mono-text">{{ ver.version }}</span>
              <el-tag v-if="ver.is_active" type="success" size="small" effect="light">
                当前生效
              </el-tag>
            </div>
            <el-button
              v-if="!ver.is_active"
              type="primary"
              link
              size="small"
              :loading="rollingBack === ver.version"
              @click="handleRollback(ver)"
            >
              回滚到此版本
            </el-button>
          </div>
          <p class="version-changelog">{{ ver.changelog || '无变更说明' }}</p>
          <div v-if="ver.publisher_name" class="version-publisher">
            发布人：{{ ver.publisher_name }}
          </div>
        </div>
      </el-timeline-item>
    </el-timeline>
  </el-drawer>
</template>

<style scoped>
.loading-wrap {
  padding: 12px 0;
}

.version-timeline {
  padding-left: 4px;
}

.version-card {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 12px 14px;
}

.version-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.version-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.version-num {
  font-weight: 600;
  font-size: 14px;
  color: var(--ascp-text-primary);
}

.version-changelog {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--ascp-text-secondary);
  line-height: 1.5;
}

.version-publisher {
  margin-top: 6px;
  font-size: 12px;
  color: var(--ascp-text-muted);
}
</style>
