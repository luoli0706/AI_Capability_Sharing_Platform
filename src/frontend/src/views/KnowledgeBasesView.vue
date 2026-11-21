<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Check,
  Collection,
  CopyDocument,
  Delete,
  Document,
  DocumentAdd,
  DocumentCopy,
  Download,
  Folder,
  FolderAdd,
  FolderOpened,
  Picture,
  Refresh,
  Search,
  Upload
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getAssets } from '@/api/asset'
import {
  copyDocument,
  createKbFile,
  createKbFolder,
  deleteDocument,
  getDocumentContent,
  getDocumentRawBlob,
  getDocuments,
  moveDocument,
  searchKB,
  updateKbFileContent,
  uploadDocument
} from '@/api/kb'
import { useDomainStore } from '@/stores/domain'
import type { AssetItem, KbChunk, KbContentResult, KbDocument } from '@/types/api'
import { formatFileSize, getKbParseStatusMeta, normalizeList } from '@/utils/format'
import {
  buildKbTree,
  confirmAndDownloadKb,
  isImageFile,
  isMarkdownFile,
  isTextFile,
  MAX_KB_FILE_SIZE,
  type KbTreeNode
} from '@/utils/kbDownload'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const domainStore = useDomainStore()

const domainId = computed(() => Number(route.params.domainId))
const currentDomain = computed(() => domainStore.getDomainById(domainId.value))

const loadingKbs = ref(false)
const kbList = ref<AssetItem[]>([])
const kbTotal = ref(0)
const kbPage = ref(1)
const kbPageSize = ref(20)
const selectedKbId = ref<number | null>(null)
const selectedKb = computed(
  () => kbList.value.find((k) => k.id === selectedKbId.value) ?? null
)

const loadingDocs = ref(false)
const documents = ref<KbDocument[]>([])
const treeNodes = computed(() => buildKbTree(documents.value))

const selectedFolderPath = ref('')
const selectedFileNode = ref<KbTreeNode | null>(null)
const clipboardFile = ref<{ kbId: number; docId: number; fileName: string } | null>(null)
const pasting = ref(false)
const uploading = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)

// Create Folder Modal
const createFolderOpen = ref(false)
const newFolderName = ref('')
const creatingFolder = ref(false)

// Create Markdown File Modal
const createFileOpen = ref(false)
const creatingFile = ref(false)
const createFileForm = reactive({
  file_name: '',
  content: ''
})

// Right Pane Tabs & Preview / Editor
const rightTab = ref<'preview' | 'search'>('preview')
const loadingPreview = ref(false)
const previewDoc = ref<KbContentResult | null>(null)
const imagePreviewUrl = ref('')
const imageDoc = ref<KbDocument | null>(null)
const binaryDoc = ref<KbDocument | null>(null)
const previewMode = ref<'rendered' | 'edit' | 'split' | 'raw'>('rendered')
const editFileName = ref('')
const editContent = ref('')
const savingEdit = ref(false)

function revokeImageUrl() {
  if (imagePreviewUrl.value) {
    URL.revokeObjectURL(imagePreviewUrl.value)
    imagePreviewUrl.value = ''
  }
}

function resetPreviewState() {
  revokeImageUrl()
  previewDoc.value = null
  imageDoc.value = null
  binaryDoc.value = null
}

const renderedHtml = computed(() =>
  previewDoc.value ? renderMarkdown(previewDoc.value.content) : ''
)
const liveRenderedHtml = computed(() => renderMarkdown(editContent.value))
const hasUnsavedChanges = computed(() => {
  if (!previewDoc.value) return false
  return (
    editFileName.value !== previewDoc.value.file_name ||
    editContent.value !== previewDoc.value.content
  )
})

// Keyword Search
const searchQuery = ref('')
const searchTopK = ref(5)
const searching = ref(false)
const searchResults = ref<KbChunk[]>([])
const hasSearched = ref(false)
const searchPage = ref(1)
const searchPageSize = ref(5)
const pagedSearchResults = computed(() => {
  const start = (searchPage.value - 1) * searchPageSize.value
  return searchResults.value.slice(start, start + searchPageSize.value)
})

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
      page: kbPage.value,
      page_size: kbPageSize.value
    })
    const normalized = normalizeList(res)
    kbList.value = normalized.items
    kbTotal.value = normalized.total
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
    kbTotal.value = 0
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
  resetPreviewState()
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

function openCreateFileModal() {
  createFileForm.file_name = ''
  createFileForm.content = ''
  createFileOpen.value = true
}

async function handleCreateFile() {
  if (!selectedKbId.value) return
  let fileName = createFileForm.file_name.trim()
  if (!fileName) {
    ElMessage.warning('请输入 Markdown 文件名，例如 guide.md')
    return
  }
  if (!/\.(md|markdown)$/i.test(fileName)) {
    fileName = `${fileName}.md`
  }

  creatingFile.value = true
  try {
    const created = await createKbFile(selectedKbId.value, {
      folder_path: selectedFolderPath.value || undefined,
      file_name: fileName,
      content: createFileForm.content
    })
    ElMessage.success(`已新建文档「${created.file_name}」`)
    createFileOpen.value = false
    await fetchDocuments()
    await loadPreviewDoc(created.id, 'split')
  } catch {
    // Handled by interceptor
  } finally {
    creatingFile.value = false
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
      if (file.size > MAX_KB_FILE_SIZE) {
        ElMessage.warning(`文件「${file.name}」超过 10MB 限制，已跳过`)
        continue
      }
      await uploadDocument(
        selectedKbId.value,
        file,
        selectedFolderPath.value || undefined
      )
      uploadedCount++
    }
    if (uploadedCount > 0) {
      ElMessage.success(`成功上传 ${uploadedCount} 个文件`)
      await fetchDocuments()
    }
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

  const allFiles = Array.from(files)
  uploading.value = true
  let uploadedCount = 0
  try {
    for (const file of allFiles) {
      if (file.size > MAX_KB_FILE_SIZE) {
        ElMessage.warning(`文件「${file.name}」超过 10MB 限制，已跳过`)
        continue
      }
      const relPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
      await uploadDocument(
        selectedKbId.value,
        file,
        selectedFolderPath.value || undefined,
        relPath
      )
      uploadedCount++
    }
    if (uploadedCount > 0) {
      ElMessage.success(`成功上传文件夹内 ${uploadedCount} 个文件`)
      await fetchDocuments()
    }
  } catch {
    // Handled by interceptor
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function loadPreviewDoc(
  docId: number,
  targetMode?: 'rendered' | 'edit' | 'split' | 'raw'
) {
  if (!selectedKbId.value) return
  rightTab.value = 'preview'
  if (targetMode) previewMode.value = targetMode
  loadingPreview.value = true
  try {
    const doc = await getDocumentContent(selectedKbId.value, docId)
    previewDoc.value = doc
    editFileName.value = doc.file_name
    editContent.value = doc.content
  } catch {
    previewDoc.value = null
  } finally {
    loadingPreview.value = false
  }
}

async function handleSaveEdit() {
  if (!selectedKbId.value || !previewDoc.value) return
  const trimmedName = editFileName.value.trim()
  if (!trimmedName) {
    ElMessage.warning('文件名不能为空')
    return
  }
  savingEdit.value = true
  try {
    const updated = await updateKbFileContent(selectedKbId.value, previewDoc.value.id, {
      file_name: trimmedName,
      content: editContent.value
    })
    previewDoc.value = updated
    editFileName.value = updated.file_name
    editContent.value = updated.content
    ElMessage.success('文档已保存')
    await fetchDocuments(true)
  } catch {
    // Handled by interceptor
  } finally {
    savingEdit.value = false
  }
}

async function handleNodeClick(node: KbTreeNode) {
  if (node.isFolder) {
    selectedFolderPath.value = node.folderPath
    selectedFileNode.value = null
    return
  }
  if (!node.doc || !selectedKbId.value) return
  const doc = node.doc
  selectedFileNode.value = node
  rightTab.value = 'preview'
  resetPreviewState()
  loadingPreview.value = true
  try {
    if (isImageFile(doc.file_name)) {
      const blob = await getDocumentRawBlob(selectedKbId.value, doc.id)
      imagePreviewUrl.value = URL.createObjectURL(blob)
      imageDoc.value = doc
    } else if (isTextFile(doc.file_name)) {
      const contentRes = await getDocumentContent(selectedKbId.value, doc.id)
      if (contentRes.is_binary) {
        binaryDoc.value = doc
      } else {
        previewDoc.value = contentRes
        editFileName.value = contentRes.file_name
        editContent.value = contentRes.content
        previewMode.value = isMarkdownFile(doc.file_name) ? 'rendered' : 'raw'
      }
    } else {
      binaryDoc.value = doc
    }
  } catch {
    resetPreviewState()
  } finally {
    loadingPreview.value = false
  }
}

function allowDrag(draggingNode: { data: KbTreeNode }): boolean {
  return !draggingNode.data.isFolder && Boolean(draggingNode.data.doc)
}

function allowDrop(
  _draggingNode: { data: KbTreeNode },
  dropNode: { data: KbTreeNode; level: number },
  type: 'prev' | 'inner' | 'next'
): boolean {
  if (type === 'inner') {
    return dropNode.data.isFolder
  }
  return dropNode.level === 1
}

async function handleNodeDrop(
  draggingNode: { data: KbTreeNode },
  dropNode: { data: KbTreeNode; level: number },
  dropType: string
) {
  const doc = draggingNode.data.doc
  if (!doc || !selectedKbId.value) return
  const targetFolderPath = dropType === 'inner' ? dropNode.data.folderPath : ''
  try {
    await moveDocument(selectedKbId.value, doc.id, targetFolderPath)
    ElMessage.success(
      targetFolderPath ? `已移动「${doc.file_name}」至 ${targetFolderPath}` : `已移动「${doc.file_name}」至根目录`
    )
    await fetchDocuments()
  } catch {
    await fetchDocuments()
  }
}

function setClipboard(node: KbTreeNode) {
  if (node.isFolder || !node.doc || !selectedKbId.value) return
  clipboardFile.value = {
    kbId: selectedKbId.value,
    docId: node.doc.id,
    fileName: node.doc.file_name
  }
  ElMessage.success(`已复制「${node.doc.file_name}」，可选择目录后点击粘贴或按 Ctrl+V`)
}

async function handlePaste(targetFolder?: string) {
  if (!clipboardFile.value || !selectedKbId.value) return
  const targetPath = typeof targetFolder === 'string' ? targetFolder : selectedFolderPath.value
  pasting.value = true
  try {
    await copyDocument(
      clipboardFile.value.kbId,
      clipboardFile.value.docId,
      targetPath || ''
    )
    ElMessage.success(
      targetPath ? `已粘贴至目录 ${targetPath}` : '已粘贴至根目录'
    )
    await fetchDocuments()
  } catch {
    // Handled by interceptor
  } finally {
    pasting.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (!(e.ctrlKey || e.metaKey)) return
  const key = e.key.toLowerCase()
  if (
    key === 's' &&
    previewDoc.value &&
    rightTab.value === 'preview' &&
    (previewMode.value === 'edit' || previewMode.value === 'split')
  ) {
    e.preventDefault()
    handleSaveEdit()
    return
  }

  const target = e.target as HTMLElement | null
  if (
    target &&
    (target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.isContentEditable)
  ) {
    return
  }
  if (key === 'c' && selectedFileNode.value && !selectedFileNode.value.isFolder) {
    e.preventDefault()
    setClipboard(selectedFileNode.value)
  } else if (key === 'v' && clipboardFile.value) {
    e.preventDefault()
    handlePaste()
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
  searchPage.value = 1
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
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  stopPolling()
  revokeImageUrl()
  window.removeEventListener('keydown', handleKeydown)
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
      <el-empty description="当前域尚未创建私有知识库资产">
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
              style="width: 260px"
              @change="handleSelectKb"
            >
              <el-option
                v-for="kb in kbList"
                :key="kb.id"
                :label="`${kb.name} (${kb.asset_code})`"
                :value="kb.id"
              />
            </el-select>
            <el-pagination
              v-if="kbTotal > kbPageSize"
              v-model:current-page="kbPage"
              :page-size="kbPageSize"
              :total="kbTotal"
              size="small"
              layout="prev, pager, next"
              @current-change="fetchKbs"
            />
          </div>

          <div class="toolbar-actions">
            <el-button type="primary" :icon="DocumentAdd" @click="openCreateFileModal">
              新建 Markdown
            </el-button>
            <el-button :icon="FolderAdd" @click="openCreateFolderModal">新建文件夹</el-button>
            <el-button
              type="primary"
              plain
              :icon="Upload"
              :loading="uploading"
              @click="triggerFileUpload()"
            >
              上传文件
            </el-button>
            <el-button
              :icon="FolderOpened"
              :loading="uploading"
              @click="triggerFolderUpload"
            >
              上传文件夹
            </el-button>
            <el-button
              :icon="DocumentCopy"
              :disabled="!clipboardFile"
              :loading="pasting"
              @click="handlePaste()"
            >
              {{ clipboardFile ? `粘贴 (${clipboardFile.fileName})` : '粘贴' }}
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
            <h3 class="panel-title">目录与文件</h3>
            <span class="doc-count tabular-nums">{{ documents.length }} 项</span>
          </div>

          <div v-loading="loadingDocs" class="tree-scroll">
            <el-empty
              v-if="treeNodes.length === 0"
              description="暂无文件或文件夹，请点击上方按钮上传文件或新建 Markdown"
              :image-size="64"
            />
            <el-tree
              v-else
              :data="treeNodes"
              node-key="key"
              default-expand-all
              highlight-current
              draggable
              :allow-drag="allowDrag"
              :allow-drop="allowDrop"
              :expand-on-click-node="false"
              @node-click="handleNodeClick"
              @node-drop="handleNodeDrop"
            >
              <template #default="{ data }">
                <div class="tree-node-row">
                  <div class="tree-node-left">
                    <el-icon :size="15" :color="data.isFolder ? '#D97706' : isImageFile(data.label) ? '#059669' : '#4F46E5'">
                      <Folder v-if="data.isFolder" />
                      <Picture v-else-if="isImageFile(data.label)" />
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
                      v-if="data.isFolder && clipboardFile"
                      link
                      type="success"
                      size="small"
                      @click.stop="handlePaste(data.folderPath)"
                    >
                      粘贴
                    </el-button>
                    <el-button
                      v-if="data.isFolder"
                      link
                      type="primary"
                      size="small"
                      @click.stop="handleDownloadFolder(data.folderPath, data.label)"
                    >
                      下载
                    </el-button>
                    <template v-else-if="data.doc">
                      <el-button
                        link
                        type="info"
                        size="small"
                        :icon="CopyDocument"
                        title="复制文件 (Ctrl+C)"
                        @click.stop="setClipboard(data)"
                      />
                      <el-button
                        link
                        type="primary"
                        size="small"
                        @click.stop="handleDownloadFile(data.doc.id, data.doc.file_name)"
                      >
                        下载
                      </el-button>
                    </template>
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
            <el-tab-pane label="文件预览与编辑" name="preview">
              <div v-if="loadingPreview" class="mt-16">
                <el-skeleton :rows="6" animated />
              </div>

              <div v-else-if="imageDoc && imagePreviewUrl" class="preview-container">
                <div class="preview-header">
                  <div class="preview-meta">
                    <span class="preview-filename mono-text">{{ imageDoc.file_name }}</span>
                    <span class="preview-sub mono-text">
                      {{ imageDoc.folder_path || '/' }} · {{ formatFileSize(imageDoc.file_size) }}
                    </span>
                  </div>
                  <el-button
                    type="primary"
                    size="small"
                    plain
                    :icon="Download"
                    @click="handleDownloadFile(imageDoc.id, imageDoc.file_name)"
                  >
                    下载图片
                  </el-button>
                </div>
                <div class="image-preview-box">
                  <el-image
                    :src="imagePreviewUrl"
                    :preview-src-list="[imagePreviewUrl]"
                    fit="contain"
                    style="max-height: 480px; width: 100%"
                  />
                </div>
              </div>

              <div v-else-if="binaryDoc" class="binary-file-card">
                <div class="binary-file-info">
                  <el-icon :size="36" color="#64748B"><Document /></el-icon>
                  <div>
                    <div class="preview-filename mono-text">{{ binaryDoc.file_name }}</div>
                    <div class="binary-file-meta">
                      <el-tag size="small" type="info" effect="plain" class="mono-text">
                        {{ binaryDoc.file_ext || 'binary' }}
                      </el-tag>
                      <span class="tabular-nums">{{ formatFileSize(binaryDoc.file_size) }}</span>
                      <span>暂不支持在线文本预览，请下载后查看</span>
                    </div>
                  </div>
                </div>
                <el-button
                  type="primary"
                  :icon="Download"
                  @click="handleDownloadFile(binaryDoc.id, binaryDoc.file_name)"
                >
                  下载文件
                </el-button>
              </div>

              <div v-else-if="previewDoc" class="preview-container">
                <div class="preview-header">
                  <div class="preview-meta">
                    <span class="preview-filename mono-text">
                      {{ previewDoc.file_name }}
                      <span v-if="hasUnsavedChanges" class="unsaved-dot" title="有未保存修改">●</span>
                    </span>
                    <span class="preview-sub mono-text">
                      {{ previewDoc.folder_path || '/' }} · {{ formatFileSize(previewDoc.file_size) }}
                    </span>
                  </div>
                  <div class="preview-controls">
                    <el-radio-group v-model="previewMode" size="small">
                      <el-radio-button value="rendered">渲染预览</el-radio-button>
                      <el-radio-button value="edit">在线编辑</el-radio-button>
                      <el-radio-button value="split">分屏编辑</el-radio-button>
                      <el-radio-button value="raw">原文</el-radio-button>
                    </el-radio-group>
                    <el-button
                      v-if="previewMode === 'edit' || previewMode === 'split'"
                      type="primary"
                      size="small"
                      :icon="Check"
                      :loading="savingEdit"
                      @click="handleSaveEdit"
                    >
                      保存修改
                    </el-button>
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
                <div v-else-if="previewMode === 'edit'" class="editor-pane">
                  <el-input
                    v-model="editFileName"
                    size="small"
                    placeholder="文件名（如 guide.md）"
                    class="mono-text"
                  >
                    <template #prepend>文件名</template>
                  </el-input>
                  <el-input
                    v-model="editContent"
                    type="textarea"
                    :rows="18"
                    placeholder="在此编写 Markdown 内容（支持 Ctrl+S 保存）..."
                    class="mono-text"
                  />
                </div>
                <div v-else-if="previewMode === 'split'" class="split-editor-grid">
                  <div class="editor-pane">
                    <el-input
                      v-model="editFileName"
                      size="small"
                      placeholder="文件名（如 guide.md）"
                      class="mono-text"
                    >
                      <template #prepend>文件名</template>
                    </el-input>
                    <el-input
                      v-model="editContent"
                      type="textarea"
                      :rows="18"
                      placeholder="在此编写 Markdown 内容（支持 Ctrl+S 保存）..."
                      class="mono-text"
                    />
                  </div>
                  <div class="markdown-body editor-split-preview" v-html="liveRenderedHtml" />
                </div>
                <pre v-else class="raw-markdown mono-text">{{ previewDoc.content }}</pre>
              </div>

              <div v-else class="preview-placeholder">
                在左侧目录树点击任意文件（支持 Markdown、文本、代码、图片及二进制文件），即可在此处预览、编辑或下载。
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
                <div v-for="(chunk, idx) in pagedSearchResults" :key="chunk.id || idx" class="chunk-card">
                  <div class="chunk-head">
                    <span class="chunk-idx mono-text">
                      #{{ (searchPage - 1) * searchPageSize + idx + 1 }} (Chunk {{ chunk.chunk_index }})
                    </span>
                    <span v-if="chunk.file_name" class="chunk-file">{{ chunk.file_name }}</span>
                    <el-tag v-if="chunk.score !== undefined" size="small" type="success" effect="plain">
                      匹配度 {{ Number(chunk.score).toFixed(3) }}
                    </el-tag>
                  </div>
                  <p class="chunk-content">{{ chunk.content }}</p>
                </div>

                <div class="search-pagination">
                  <el-pagination
                    v-model:current-page="searchPage"
                    :page-size="searchPageSize"
                    :total="searchResults.length"
                    layout="total, prev, pager, next"
                    size="small"
                    background
                  />
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

    <!-- Create Markdown File Modal -->
    <el-dialog v-model="createFileOpen" title="新建 Markdown 文档" width="540px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="目标目录">
          <el-input
            :model-value="selectedFolderPath ? `/${selectedFolderPath}` : '/ (根目录)'"
            disabled
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="文件名">
          <el-input
            v-model="createFileForm.file_name"
            placeholder="例如：guide.md（未带后缀将自动补充 .md）"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="初始内容（选填）">
          <el-input
            v-model="createFileForm.content"
            type="textarea"
            :rows="6"
            placeholder="# 文档标题&#10;&#10;在此输入 Markdown 内容..."
            class="mono-text"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createFileOpen = false">取消</el-button>
        <el-button type="primary" :loading="creatingFile" @click="handleCreateFile">
          创建并编辑
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

.search-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 6px;
}

.unsaved-dot {
  color: #f59e0b;
  margin-left: 4px;
  font-size: 12px;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.split-editor-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  align-items: stretch;
}

.editor-split-preview {
  border: 1px solid var(--ascp-border);
  border-radius: 8px;
  padding: 12px 14px;
  background: #ffffff;
  max-height: 440px;
  overflow-y: auto;
}

.image-preview-box {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  justify-content: center;
}

.binary-file-card {
  margin-top: 16px;
  padding: 24px;
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.binary-file-info {
  display: flex;
  align-items: center;
  gap: 14px;
}

.binary-file-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 13px;
  color: var(--ascp-text-muted);
}

@media (max-width: 1080px) {
  .kb-columns {
    grid-template-columns: 1fr;
  }

  .split-editor-grid {
    grid-template-columns: 1fr;
  }
}
</style>
