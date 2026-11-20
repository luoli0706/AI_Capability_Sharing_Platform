<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Collection,
  Delete,
  Document,
  Download,
  Folder,
  FolderAdd,
  FolderOpened,
  Refresh,
  Search,
  Upload
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getAssets } from '@/api/asset'
import {
  createKbFolder,
  deleteDocument,
  getDocumentContent,
  getDocuments,
  searchKB,
  uploadDocument
} from '@/api/kb'
import { useDomainStore } from '@/stores/domain'
import type { AssetItem, KbChunk, KbContentResult, KbDocument } from '@/types/api'
import { formatFileSize, getKbParseStatusMeta, normalizeList } from '@/utils/format'
import { buildKbTree, confirmAndDownloadKb, type KbTreeNode } from '@/utils/kbDownload'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const domainStore = useDomainStore()

const domainId = computed(() => Number(route.params.domainId))
const currentDomain = computed(() => domainStore.getDomainById(domainId.value))

const loadingKbs = ref(false)
const kbList = ref<AssetItem[]>([])
const selectedKbId = ref<number | null>(null)
const selectedKb = computed(
  () => kbList.value.find((k) => k.id === selectedKbId.value) ?? null
)

const loadingDocs = ref(false)
const documents = ref<KbDocument[]>([])
const treeNodes = computed(() => buildKbTree(documents.value))

const selectedFolderPath = ref('')
const uploading = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)

// Create Folder Modal
const createFolderOpen = ref(false)
const newFolderName = ref('')
const creatingFolder = ref(false)

// Right Pane Tabs & Preview
const rightTab = ref<'preview' | 'search'>('preview')
const loadingPreview = ref(false)
const previewDoc = ref<KbContentResult | null>(null)
const previewMode = ref<'rendered' | 'raw'>('rendered')
const renderedHtml = computed(() =>
  previewDoc.value ? renderMarkdown(previewDoc.value.content) : ''
)

// Keyword Search
const searchQuery = ref('')
const searchTopK = ref(5)
const searching = ref(false)
const searchResults = ref<KbChunk[]>([])
const hasSearched = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPollingIfNeeded() {
  stopPolling()
  const hasPending = documents.value.some(
    (d) => !d.is_folder && (d.parse_status === 'pending' || d.parse_status === 'processing')
  )
  if (hasPending && selectedKbId.value) {
    pollTimer = setInterval(() => {
      fetchDocuments(true)
    }, 3000)
  }
}

async function fetchKbs() {
  if (!domainId.value) return
  loadingKbs.value = true
  try {
    const res = await getAssets({
      scope: 'workspace',
      domain_id: domainId.value,
      asset_type: 'kb',
      page: 1,
      page_size: 100
    })
    kbList.value = normalizeList(res).items
    if (kbList.value.length > 0) {
      if (!kbList.value.some((k) => k.id === selectedKbId.value)) {
        selectedKbId.value = kbList.value[0].id
      }
      await fetchDocuments()
    } else {
      selectedKbId.value = null
      documents.value = []
    }
  } catch {
    kbList.value = []
    selectedKbId.value = null
    documents.value = []
  } finally {
    loadingKbs.value = false
  }
}

async function fetchDocuments(silent = false) {
  if (!selectedKbId.value) return
  if (!silent) loadingDocs.value = true
  try {
    const res = await getDocuments(selectedKbId.value)
    documents.value = normalizeList(res).items
    startPollingIfNeeded()
  } catch {
    if (!silent) documents.value = []
  } finally {
    if (!silent) loadingDocs.value = false
  }
}

function handleSelectKb(id: number) {
  selectedKbId.value = id
  selectedFolderPath.value = ''
  previewDoc.value = null
  searchResults.value = []
  hasSearched.value = false
  fetchDocuments()
}

function openCreateFolderModal() {
  newFolderName.value = selectedFolderPath.value ? `${selectedFolderPath.value}/` : ''
  createFolderOpen.value = true
}

async function handleCreateFolder() {
  if (!selectedKbId.value) return
  const clean = newFolderName.value.trim().replace(/^\/+|\/+$/g, '')
  if (!clean) {
    ElMessage.warning('请输入文件夹路径，例如 docs/api')
    return
  }
  creatingFolder.value = true
  try {
    await createKbFolder(selectedKbId.value, clean)
    ElMessage.success('文件夹已创建')
    createFolderOpen.value = false
    selectedFolderPath.value = clean
    await fetchDocuments()
  } catch {
    // Handled by interceptor
  } finally {
    creatingFolder.value = false
  }
}

function triggerFileUpload(targetFolder?: string) {
  if (typeof targetFolder === 'string') {
    selectedFolderPath.value = targetFolder
  }
  fileInputRef.value?.click()
}

function triggerFolderUpload() {
  folderInputRef.value?.click()
}

async function handleFilesSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0 || !selectedKbId.value) return

  uploading.value = true
  let uploadedCount = 0
  try {
    for (const file of Array.from(files)) {
      await uploadDocument(
        selectedKbId.value,
        file,
        selectedFolderPath.value || undefined
      )
      uploadedCount++
    }
    ElMessage.success(`成功上传 ${uploadedCount} 个 Markdown 文件`)
    await fetchDocuments()
  } catch {
    // Handled by interceptor
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function handleFolderSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0 || !selectedKbId.value) return

  const mdFiles = Array.from(files).filter((f) => /\.(md|markdown)$/i.test(f.name))
  if (mdFiles.length === 0) {
    ElMessage.warning('所选文件夹中未包含 .md Markdown 文件')
    input.value = ''
    return
  }

  uploading.value = true
  let uploadedCount = 0
  try {
    for (const file of mdFiles) {
      const relPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
      await uploadDocument(
        selectedKbId.value,
        file,
        selectedFolderPath.value || undefined,
        relPath
      )
      uploadedCount++
    }
    ElMessage.success(`成功上传文件夹内 ${uploadedCount} 个 Markdown 文件`)
    await fetchDocuments()
  } catch {
    // Handled by interceptor
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function handleNodeClick(node: KbTreeNode) {
  if (node.isFolder) {
    selectedFolderPath.value = node.folderPath
    return
  }
  if (!node.doc || !selectedKbId.value) return
  rightTab.value = 'preview'
  loadingPreview.value = true
  try {
    previewDoc.value = await getDocumentContent(selectedKbId.value, node.doc.id)
  } catch {
    previewDoc.value = null
  } finally {
    loadingPreview.value = false
  }
}

async function handleDeleteNode(node: KbTreeNode) {
  if (!selectedKbId.value || !node.doc) return
  const targetLabel = node.isFolder ? `文件夹「${node.label}」` : `文件「${node.label}」`
  try {
    await ElMessageBox.confirm(`确认删除${targetLabel}吗？`, '删除确认', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await deleteDocument(selectedKbId.value, node.doc.id)
    ElMessage.success('已删除')
    if (!node.isFolder && previewDoc.value?.id === node.doc.id) {
      previewDoc.value = null
    }
    if (node.isFolder && selectedFolderPath.value === node.folderPath) {
      selectedFolderPath.value = ''
    }
    await fetchDocuments()
  } catch {
    // Cancelled or failed
  }
}

async function handleDownloadAll() {
  if (!selectedKbId.value || !selectedKb.value) return
  await confirmAndDownloadKb(
    selectedKbId.value,
    { scope: 'all' },
    `${selectedKb.value.asset_code || 'knowledge-base'}.zip`
  )
}

async function handleDownloadFolder(folderPath: string, folderLabel?: string) {
  if (!selectedKbId.value) return
  const name = folderLabel || folderPath.split('/').pop() || 'folder'
  await confirmAndDownloadKb(
    selectedKbId.value,
    { scope: 'folder', folder_path: folderPath },
    `${name}.zip`
  )
}

async function handleDownloadFile(docId: number, fileName: string) {
  if (!selectedKbId.value) return
  await confirmAndDownloadKb(selectedKbId.value, { scope: 'file', doc_id: docId }, fileName)
}

async function handleSearchTest() {
  if (!selectedKbId.value) return
  if (!searchQuery.value.trim()) {
    ElMessage.warning('请输入检索关键词')
    return
  }
  searching.value = true
  hasSearched.value = true
  try {
    const res = await searchKB(selectedKbId.value, {
      query: searchQuery.value.trim(),
      top_k: searchTopK.value
    })
    searchResults.value = normalizeList(res).items
  } catch {
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

watch(domainId, () => {
  stopPolling()
  fetchKbs()
})

onMounted(() => {
  domainStore.fetchDomains()
  fetchKbs()
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          域内知识库
          <span v-if="currentDomain" class="domain-tag">{{ currentDomain.domain_name }}</span>
        </h1>
        <p class="page-subtitle">以文件夹与 Markdown 文档形式组织团队知识资产，支持在线渲染预览、目录递归下载与关键词检索</p>
      </div>
      <el-button @click="router.push(`/domains/${domainId}/assets`)">
        前往资产工作台管理知识库
      </el-button>
    </div>

    <div v-if="loadingKbs" class="surface-card p-24">
      <el-skeleton :rows="4" animated />
    </div>

    <div v-else-if="kbList.length === 0" class="surface-card empty-box">
      <el-empty description="当前业务域尚未创建私有知识库资产">
        <el-button type="primary" @click="router.push(`/domains/${domainId}/assets`)">
          去创建知识库资产
        </el-button>
      </el-empty>
    </div>

    <div v-else class="kb-workspace">
      <!-- KB Selector & Toolbar -->
      <div class="surface-card kb-toolbar-card">
        <div class="toolbar-top">
          <div class="selector-left">
            <el-icon :size="18" color="#4F46E5"><Collection /></el-icon>
            <span class="selector-label">当前知识库：</span>
            <el-select
              :model-value="selectedKbId"
              style="width: 280px"
              @change="handleSelectKb"
            >
              <el-option
                v-for="kb in kbList"
                :key="kb.id"
                :label="`${kb.name} (${kb.asset_code})`"
                :value="kb.id"
              />
            </el-select>
          </div>

          <div class="toolbar-actions">
            <el-button :icon="FolderAdd" @click="openCreateFolderModal">新建文件夹</el-button>
            <el-button
              type="primary"
              plain
              :icon="Upload"
              :loading="uploading"
              @click="triggerFileUpload()"
            >
              上传 Markdown 文件
            </el-button>
            <el-button
              :icon="FolderOpened"
              :loading="uploading"
              @click="triggerFolderUpload"
            >
              上传文件夹
            </el-button>
            <el-button
              type="primary"
              :icon="Download"
              @click="handleDownloadAll"
            >
              下载整个知识库
            </el-button>
            <el-button :icon="Refresh" @click="fetchDocuments()">刷新</el-button>
          </div>
        </div>

        <div v-if="selectedFolderPath" class="active-folder-bar">
          <span>
            当前选中目录：<strong class="mono-text">{{ selectedFolderPath }}</strong>
          </span>
          <div class="folder-bar-btns">
            <el-button link type="primary" size="small" @click="triggerFileUpload(selectedFolderPath)">
              在此上传
            </el-button>
            <el-button
              link
              type="primary"
              size="small"
              @click="handleDownloadFolder(selectedFolderPath)"
            >
              下载该文件夹(递归)
            </el-button>
            <el-button link type="info" size="small" @click="selectedFolderPath = ''">
              回到根目录
            </el-button>
          </div>
        </div>

        <!-- Hidden file & directory inputs -->
        <input
          ref="fileInputRef"
          type="file"
          accept=".md,.markdown"
          multiple
          style="display: none"
          @change="handleFilesSelected"
        />
        <input
          ref="folderInputRef"
          type="file"
          webkitdirectory
          directory
          multiple
          style="display: none"
          @change="handleFolderSelected"
        />
      </div>

      <!-- Split Tree + Preview/Search Workspace -->
      <div class="kb-columns">
        <!-- Left: Folder & File Tree -->
        <div class="surface-card kb-panel tree-panel">
          <div class="panel-header">
            <h3 class="panel-title">目录与 Markdown 文件</h3>
            <span class="doc-count tabular-nums">{{ documents.length }} 项</span>
          </div>

          <div v-loading="loadingDocs" class="tree-scroll">
            <el-empty
              v-if="treeNodes.length === 0"
              description="暂无文件或文件夹，请点击上方按钮上传 Markdown"
              :image-size="64"
            />
            <el-tree
              v-else
              :data="treeNodes"
              node-key="key"
              default-expand-all
              highlight-current
              :expand-on-click-node="false"
              @node-click="handleNodeClick"
            >
              <template #default="{ data }">
                <div class="tree-node-row">
                  <div class="tree-node-left">
                    <el-icon :size="15" :color="data.isFolder ? '#D97706' : '#4F46E5'">
                      <Folder v-if="data.isFolder" />
                      <Document v-else />
                    </el-icon>
                    <span class="tree-node-label">{{ data.label }}</span>
                    <span v-if="!data.isFolder" class="tree-node-size tabular-nums">
                      {{ formatFileSize(data.fileSize) }}
                    </span>
                    <el-tag
                      v-if="!data.isFolder && data.doc && data.doc.parse_status !== 'completed'"
                      :type="getKbParseStatusMeta(data.doc.parse_status).tagType"
                      size="small"
                      effect="plain"
                    >
                      {{ getKbParseStatusMeta(data.doc.parse_status).label }}
                    </el-tag>
                  </div>

                  <div class="tree-node-actions">
                    <el-button
                      v-if="data.isFolder"
                      link
                      type="primary"
                      size="small"
                      @click.stop="handleDownloadFolder(data.folderPath, data.label)"
                    >
                      下载
                    </el-button>
                    <el-button
                      v-else-if="data.doc"
                      link
                      type="primary"
                      size="small"
                      @click.stop="handleDownloadFile(data.doc.id, data.doc.file_name)"
                    >
                      下载
                    </el-button>
                    <el-button
                      v-if="data.doc"
                      link
                      type="danger"
                      size="small"
                      :icon="Delete"
                      @click.stop="handleDeleteNode(data)"
                    />
                  </div>
                </div>
              </template>
            </el-tree>
          </div>
        </div>

        <!-- Right: Markdown Preview + Keyword Search -->
        <div class="surface-card kb-panel">
          <el-tabs v-model="rightTab">
            <el-tab-pane label="Markdown 预览" name="preview">
              <div v-if="loadingPreview" class="mt-16">
                <el-skeleton :rows="6" animated />
              </div>

              <div v-else-if="previewDoc" class="preview-container">
                <div class="preview-header">
                  <div class="preview-meta">
                    <span class="preview-filename mono-text">{{ previewDoc.file_name }}</span>
                    <span class="preview-sub mono-text">
                      {{ previewDoc.folder_path || '/' }} · {{ formatFileSize(previewDoc.file_size) }}
                    </span>
                  </div>
                  <div class="preview-controls">
                    <el-radio-group v-model="previewMode" size="small">
                      <el-radio-button value="rendered">渲染预览</el-radio-button>
                      <el-radio-button value="raw">Markdown 原文</el-radio-button>
                    </el-radio-group>
                    <el-button
                      type="primary"
                      size="small"
                      plain
                      :icon="Download"
                      @click="handleDownloadFile(previewDoc.id, previewDoc.file_name)"
                    >
                      下载此文件
                    </el-button>
                  </div>
                </div>

                <div
                  v-if="previewMode === 'rendered'"
                  class="markdown-body preview-content"
                  v-html="renderedHtml"
                />
                <pre v-else class="raw-markdown mono-text">{{ previewDoc.content }}</pre>
              </div>

              <div v-else class="preview-placeholder">
                在左侧目录树点击任意 <code>.md</code> 文件，即可在此处预览渲染效果或查看源码。
              </div>
            </el-tab-pane>

            <el-tab-pane label="关键词检索" name="search">
              <div class="recall-controls">
                <el-input
                  v-model="searchQuery"
                  placeholder="输入检索关键词..."
                  clearable
                  :prefix-icon="Search"
                  @keyup.enter="handleSearchTest"
                />
                <div class="recall-sub-row">
                  <div class="topk-wrap">
                    <span>Top K：</span>
                    <el-input-number v-model="searchTopK" :min="1" :max="20" size="small" />
                  </div>
                  <el-button type="primary" :loading="searching" @click="handleSearchTest">
                    执行检索
                  </el-button>
                </div>
              </div>

              <div v-if="searching" class="mt-16">
                <el-skeleton :rows="4" animated />
              </div>

              <div v-else-if="hasSearched && searchResults.length === 0" class="mt-16">
                <el-empty description="未匹配到相关文档内容" :image-size="64" />
              </div>

              <div v-else-if="searchResults.length > 0" class="recall-results">
                <div v-for="(chunk, idx) in searchResults" :key="chunk.id || idx" class="chunk-card">
                  <div class="chunk-head">
                    <span class="chunk-idx mono-text">#{{ idx + 1 }} (Chunk {{ chunk.chunk_index }})</span>
                    <span v-if="chunk.file_name" class="chunk-file">{{ chunk.file_name }}</span>
                    <el-tag v-if="chunk.score !== undefined" size="small" type="success" effect="plain">
                      匹配度 {{ Number(chunk.score).toFixed(3) }}
                    </el-tag>
                  </div>
                  <p class="chunk-content">{{ chunk.content }}</p>
                </div>
              </div>

              <div v-else class="preview-placeholder">
                输入关键词后点击「执行检索」，测试当前知识库文档的全文检索召回效果。
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>

    <!-- Create Folder Modal -->
    <el-dialog v-model="createFolderOpen" title="新建文件夹" width="420px" destroy-on-close>
      <el-form label-position="top" @submit.prevent="handleCreateFolder">
        <el-form-item label="文件夹路径">
          <el-input
            v-model="newFolderName"
            placeholder="例如：docs/architecture"
            class="mono-text"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createFolderOpen = false">取消</el-button>
        <el-button type="primary" :loading="creatingFolder" @click="handleCreateFolder">
          确认创建
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

.p-24 {
  padding: 24px;
}

.empty-box {
  padding: 48px 16px;
}

.kb-workspace {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.kb-toolbar-card {
  padding: 14px 18px;
}

.toolbar-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}

.selector-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.selector-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.active-folder-bar {
  margin-top: 12px;
  padding: 8px 12px;
  background: var(--ascp-primary-soft);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: var(--ascp-text-secondary);
  flex-wrap: wrap;
}

.folder-bar-btns {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kb-columns {
  display: grid;
  grid-template-columns: 380px 1fr;
  gap: 18px;
  align-items: start;
}

.kb-panel {
  padding: 18px 20px;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.panel-title {
  margin: 0;
  font-size: 15.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.doc-count {
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.tree-scroll {
  max-height: 580px;
  overflow-y: auto;
}

.tree-node-row {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-right: 4px;
  min-width: 0;
}

.tree-node-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
}

.tree-node-label {
  font-size: 13.5px;
  color: var(--ascp-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-node-size {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
  flex-shrink: 0;
}

.tree-node-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.preview-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ascp-border);
  flex-wrap: wrap;
}

.preview-meta {
  display: flex;
  flex-direction: column;
}

.preview-filename {
  font-size: 15px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.preview-sub {
  font-size: 12px;
  color: var(--ascp-text-muted);
  margin-top: 2px;
}

.preview-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.preview-content {
  max-height: 540px;
  overflow-y: auto;
  padding: 4px 2px;
}

.raw-markdown {
  margin: 0;
  padding: 14px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 540px;
  overflow-y: auto;
}

.preview-placeholder {
  margin-top: 16px;
  padding: 48px 20px;
  text-align: center;
  font-size: 13.5px;
  color: var(--ascp-text-muted);
  background: #f8fafc;
  border-radius: 10px;
  border: 1px dashed var(--ascp-border);
}

.recall-controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.recall-sub-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.topk-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--ascp-text-secondary);
}

.mt-16 {
  margin-top: 16px;
}

.recall-results {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 16px;
}

.chunk-card {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 12px 14px;
}

.chunk-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.chunk-idx {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ascp-primary);
}

.chunk-file {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.chunk-content {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--ascp-text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1080px) {
  .kb-columns {
    grid-template-columns: 1fr;
  }
}
</style>
