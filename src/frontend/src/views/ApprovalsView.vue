<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getHandledApprovals,
  getMyRequests,
  getPendingApprovals,
  handleApproval
} from '@/api/approval'
import type { ApprovalRequest } from '@/types/api'
import { formatDateTime, getApprovalStatusMeta, normalizeList } from '@/utils/format'

const activeTab = ref<'pending' | 'my' | 'handled'>('pending')
const loading = ref(false)
const list = ref<ApprovalRequest[]>([])

const detailDrawerOpen = ref(false)
const activeRequest = ref<ApprovalRequest | null>(null)

const handleModalOpen = ref(false)
const submitting = ref(false)
const handleTarget = ref<ApprovalRequest | null>(null)
const handleForm = reactive<{
  action: 'approve' | 'reject'
  comment: string
}>({
  action: 'approve',
  comment: ''
})

async function fetchList() {
  loading.value = true
  try {
    let res
    if (activeTab.value === 'pending') {
      res = await getPendingApprovals({ page: 1, page_size: 50 })
    } else if (activeTab.value === 'my') {
      res = await getMyRequests({ page: 1, page_size: 50 })
    } else {
      res = await getHandledApprovals({ page: 1, page_size: 50 })
    }
    list.value = normalizeList(res).items
  } catch {
    list.value = []
  } finally {
    loading.value = false
  }
}

function handleTabChange() {
  fetchList()
}

function openDetail(row: ApprovalRequest) {
  activeRequest.value = row
  detailDrawerOpen.value = true
}

function openHandleModal(row: ApprovalRequest, action: 'approve' | 'reject') {
  handleTarget.value = row
  handleForm.action = action
  handleForm.comment = ''
  handleModalOpen.value = true
}

async function submitHandle() {
  if (!handleTarget.value) return
  if (handleForm.action === 'reject' && !handleForm.comment.trim()) {
    ElMessage.warning('驳回申请时请填写审批意见')
    return
  }

  submitting.value = true
  try {
    await handleApproval(handleTarget.value.id, {
      action: handleForm.action,
      comment: handleForm.comment.trim() || undefined
    })
    ElMessage.success(handleForm.action === 'approve' ? '已审批通过' : '已驳回申请')
    handleModalOpen.value = false
    fetchList()
  } catch {
    // Handled by interceptor
  } finally {
    submitting.value = false
  }
}

function getRecordTimelineType(action: string) {
  if (action === 'approve') return 'success'
  if (action === 'reject') return 'danger'
  return 'warning'
}

function getRecordActionLabel(action: string) {
  if (action === 'approve') return '已通过'
  if (action === 'reject') return '已驳回'
  return '待审批'
}

onMounted(() => {
  fetchList()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">审批中心</h1>
        <p class="page-subtitle">处理受控 AI 资产的跨团队权限申请，追踪多级审批进度与历史记录</p>
      </div>
    </div>

    <div class="surface-card approvals-card">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="待我审批" name="pending" />
        <el-tab-pane label="我发起的申请" name="my" />
        <el-tab-pane label="已处理记录" name="handled" />
      </el-tabs>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column label="申请单号" width="175">
          <template #default="{ row }">
            <span class="mono-text req-no">{{ row.request_no }}</span>
          </template>
        </el-table-column>

        <el-table-column label="目标资产" min-width="180">
          <template #default="{ row }">
            <div class="asset-col">
              <span class="asset-name">{{ row.asset_name || `资产 #${row.asset_id}` }}</span>
              <span v-if="row.asset_code" class="asset-code mono-text">{{ row.asset_code }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="申请人" width="120">
          <template #default="{ row }">
            {{ row.applicant_name || `用户 #${row.applicant_id}` }}
          </template>
        </el-table-column>

        <el-table-column label="申请权限 / 时长" width="140">
          <template #default="{ row }">
            <span class="mono-text">{{ row.requested_perm }}</span>
            <span class="duration-text"> ({{ row.duration_days }}天)</span>
          </template>
        </el-table-column>

        <el-table-column label="审批进度" width="110">
          <template #default="{ row }">
            <span class="tabular-nums">第 {{ row.current_step }}/{{ row.total_steps }} 级</span>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getApprovalStatusMeta(row.status).tagType" size="small" effect="light">
              {{ getApprovalStatusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="申请时间" width="155">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>

        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openDetail(row)">
              进度详情
            </el-button>
            <template v-if="activeTab === 'pending' && row.status === 'pending'">
              <el-button link type="success" size="small" @click="openHandleModal(row, 'approve')">
                通过
              </el-button>
              <el-button link type="danger" size="small" @click="openHandleModal(row, 'reject')">
                驳回
              </el-button>
            </template>
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="暂无审批记录" />
        </template>
      </el-table>
    </div>

    <!-- Approval Timeline Drawer -->
    <el-drawer
      v-model="detailDrawerOpen"
      title="审批流进度详情"
      size="520px"
      destroy-on-close
    >
      <div v-if="activeRequest" class="drawer-content">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="申请单号">
            <span class="mono-text">{{ activeRequest.request_no }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="目标资产">
            {{ activeRequest.asset_name || `资产 #${activeRequest.asset_id}` }}
          </el-descriptions-item>
          <el-descriptions-item label="申请权限">
            <span class="mono-text">{{ activeRequest.requested_perm }}</span>（有效期 {{ activeRequest.duration_days }} 天）
          </el-descriptions-item>
          <el-descriptions-item label="申请理由">
            {{ activeRequest.reason || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="getApprovalStatusMeta(activeRequest.status).tagType" size="small">
              {{ getApprovalStatusMeta(activeRequest.status).label }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <div class="timeline-section">
          <h4 class="timeline-title">多级审批时间轴</h4>
          <el-empty
            v-if="!activeRequest.records?.length"
            description="暂无审批节点记录"
            :image-size="56"
          />
          <el-timeline v-else>
            <el-timeline-item
              v-for="rec in activeRequest.records"
              :key="rec.id"
              :type="getRecordTimelineType(rec.action)"
              :timestamp="formatDateTime(rec.handled_at || rec.created_at)"
              placement="top"
            >
              <div class="record-card">
                <div class="record-head">
                  <span class="record-step">第 {{ rec.step_no }} 级：{{ rec.step_name }}</span>
                  <el-tag :type="getRecordTimelineType(rec.action)" size="small" effect="plain">
                    {{ getRecordActionLabel(rec.action) }}
                  </el-tag>
                </div>
                <div class="record-approver">
                  审批人：{{ rec.approver_name || `用户 #${rec.approver_id}` }}
                </div>
                <p v-if="rec.comment" class="record-comment">意见：{{ rec.comment }}</p>
              </div>
            </el-timeline-item>
          </el-timeline>
        </div>
      </div>
    </el-drawer>

    <!-- Handle Approval Modal -->
    <el-dialog
      v-model="handleModalOpen"
      :title="handleForm.action === 'approve' ? '审批通过确认' : '驳回申请确认'"
      width="440px"
      destroy-on-close
    >
      <el-form label-position="top">
        <el-form-item :label="handleForm.action === 'approve' ? '审批备注（选填）' : '驳回原因（必填）'">
          <el-input
            v-model="handleForm.comment"
            type="textarea"
            :rows="3"
            :placeholder="handleForm.action === 'approve' ? '可填写审批备注...' : '请说明驳回理由...'"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="handleModalOpen = false">取消</el-button>
        <el-button
          :type="handleForm.action === 'approve' ? 'success' : 'danger'"
          :loading="submitting"
          @click="submitHandle"
        >
          {{ handleForm.action === 'approve' ? '确认通过' : '确认驳回' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.approvals-card {
  padding: 16px 24px 24px;
}

.req-no {
  font-size: 12.5px;
  color: var(--ascp-text-secondary);
}

.asset-col {
  display: flex;
  flex-direction: column;
}

.asset-name {
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.asset-code {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.duration-text {
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.timeline-title {
  margin: 0 0 14px;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.record-card {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 10px 14px;
}

.record-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.record-step {
  font-weight: 600;
  font-size: 13.5px;
  color: var(--ascp-text-primary);
}

.record-approver {
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.record-comment {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--ascp-text-secondary);
}
</style>
