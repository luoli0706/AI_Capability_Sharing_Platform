<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  ArrowRight,
  ChatLineSquare,
  Check,
  Close,
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
  Pointer,
  Refresh,
  RefreshRight,
  Search,
  Star,
  StarFilled,
  Upload,
  ZoomIn,
  ZoomOut
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getAssets } from '@/api/asset'
import { getDomainCrossRefs } from '@/api/crossRef'
import CrossRefSection from '@/components/CrossRefSection.vue'
import { extractFilesFromDrop, getFileRelativePath, isFileDragEvent } from '@/utils/dropFiles'
import {
  copyDocument,
  createDocumentComment,
  createKbFile,
  createKbFolder,
  deleteDocument,
  deleteDocumentComment,
  getDocumentContent,
  getDocumentInteractions,
  getDocumentRawBlob,
  getDocuments,
  listDocumentComments,
  moveDocument,
  searchKB,
  toggleDocumentFavorite,
  toggleDocumentLike,
  updateKbFileContent,
  uploadDocument
} from '@/api/kb'
import { useDomainStore } from '@/stores/domain'
import type {
  AssetItem,
  DomainCrossRefItem,
  KbChunk,
  KbCommentItem,
  KbContentResult,
  KbDocument,
  KbDocumentInteractions,
  UserBrief
} from '@/types/api'
import {
  formatDateTime,
  formatFileSize,
  getKbParseStatusMeta,
  normalizeList
} from '@/utils/format'
import {
  buildImmediateKbTreeNodes,
  confirmAndDownloadKb,
  extractImmediateFiles,
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

type KbEntryKind = 'own' | 'cross_kb' | 'cross_files'

interface KbSelectorEntry {
  key: string
  kind: KbEntryKind
  id: number
  name: string
  assetCode?: string
  sourceDomainName?: string
  crossRefId?: number
  asset?: AssetItem
  crossRef?: DomainCrossRefItem
}

const loadingKbs = ref(false)
const kbList = ref<AssetItem[]>([])
const activeCrossRefs = ref<DomainCrossRefItem[]>([])
const kbTotal = ref(0)
const kbPage = ref(1)
const kbPageSize = ref(20)
const selectedEntryKey = ref<string>('')
const selectedCrossFileRef = ref<DomainCrossRefItem | null>(null)

const crossKbRefs = computed(() =>
  activeCrossRefs.value.filter((r) => r.ref_type === 'asset' && r.asset_type === 'kb')
)
const crossFileRefs = computed(() =>
  activeCrossRefs.value.filter((r) => r.ref_type === 'kb_file')
)

const kbSelectorEntries = computed<KbSelectorEntry[]>(() => {
  const entries: KbSelectorEntry[] = kbList.value.map((kb) => ({
    key: `own:${kb.id}`,
    kind: 'own',
    id: kb.id,
    name: kb.name,
    assetCode: kb.asset_code,
    asset: kb
  }))
  for (const refItem of crossKbRefs.value) {
    entries.push({
      key: `cross-kb:${refItem.id}`,
      kind: 'cross_kb',
      id: refItem.source_asset_id,
      name: refItem.title,
      sourceDomainName: refItem.source_domain_name || `域 #${refItem.source_domain_id}`,
      crossRefId: refItem.id,
      crossRef: refItem
    })
  }
  if (crossFileRefs.value.length > 0) {
    entries.push({
      key: 'cross-files',
      kind: 'cross_files',
      id: -1,
      name: '跨域引用文件'
    })
  }
  return entries
})

const selectedEntry = computed(
  () => kbSelectorEntries.value.find((e) => e.key === selectedEntryKey.value) ?? null
)
const isCrossKbMode = computed(() => selectedEntry.value?.kind === 'cross_kb')
const isCrossFilesMode = computed(() => selectedEntry.value?.kind === 'cross_files')
const isReadOnlyKb = computed(() => isCrossKbMode.value || isCrossFilesMode.value)
const activeRefDomainId = computed(() => (isReadOnlyKb.value ? domainId.value : undefined))
const selectedKbId = computed<number | null>(() => {
  if (!selectedEntry.value) return null
  if (selectedEntry.value.kind === 'cross_files') {
    return selectedCrossFileRef.value?.source_asset_id ?? null
  }
  return selectedEntry.value.id
})
const selectedKb = computed(() => selectedEntry.value?.asset ?? null)

function getEntryLabel(entry: KbSelectorEntry): string {
  if (entry.kind === 'own') {
    return entry.assetCode ? `${entry.name} (${entry.assetCode})` : entry.name
  }
  if (entry.kind === 'cross_kb') {
    return `${entry.name} [跨引用 · 只读]`
  }
  return `${entry.name} [跨引用 · 只读]`
}

const loadingDocs = ref(false)
const treeRefreshSeed = ref(0)
const treeKey = computed(() => `${selectedEntryKey.value || 'none'}-${treeRefreshSeed.value}`)
const treeProps = {
  label: 'label',
  children: 'children',
  isLeaf: 'isLeaf'
}
const folderFilesCache = ref<Record<string, KbDocument[]>>({})
const rootEmpty = ref(false)
const loadedDocCount = computed(() =>
  Object.values(folderFilesCache.value).reduce((sum, list) => sum + list.length, 0)
)

const selectedFolderPath = ref('')
const selectedFileNode = ref<KbTreeNode | null>(null)
const clipboardFile = ref<{ kbId: number; docId: number; fileName: string } | null>(null)
const pasting = ref(false)
const uploading = ref(false)
const kbDragDepth = ref(0)
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
const imageUrlMap = ref<Record<number, string>>({})
const sameFolderImages = ref<KbDocument[]>([])
const loadingImageBlob = ref(false)
const imageDoc = ref<KbDocument | null>(null)
const binaryDoc = ref<KbDocument | null>(null)
const previewMode = ref<'rendered' | 'edit' | 'split' | 'raw'>('rendered')
const editFileName = ref('')
const editContent = ref('')
const savingEdit = ref(false)

const imagePreviewUrl = computed(() =>
  imageDoc.value ? imageUrlMap.value[imageDoc.value.id] || '' : ''
)
const currentImageIndex = computed(() => {
  if (!imageDoc.value) return 0
  const idx = sameFolderImages.value.findIndex((d) => d.id === imageDoc.value?.id)
  return idx >= 0 ? idx : 0
})

// Background image preloading & controlled lightbox state
const imageLoadPromises = new Map<number, Promise<string | null>>()
const decodedImageElements = new Map<number, HTMLImageElement>()
let previewRequestSeq = 0
let previewSessionSeq = 0
const lightboxOpen = ref(false)
const lightboxScale = ref(1)
const lightboxTranslateX = ref(0)
const lightboxTranslateY = ref(0)
const lightboxRotate = ref(0)
const lightboxDragging = ref(false)
let dragStartX = 0
let dragStartY = 0
let dragOriginX = 0
let dragOriginY = 0

function decodeImageIntoMemory(docId: number, url: string) {
  if (!url || decodedImageElements.has(docId)) return
  const img = new Image()
  img.src = url
  if (typeof img.decode === 'function') {
    img.decode().catch(() => {})
  }
  decodedImageElements.set(docId, img)
}

function setImageUrl(docId: number, url: string) {
  const previousUrl = imageUrlMap.value[docId]
  if (previousUrl && previousUrl !== url) {
    URL.revokeObjectURL(previousUrl)
  }
  imageUrlMap.value = { ...imageUrlMap.value, [docId]: url }
}

function revokeAllImageUrls() {
  for (const url of Object.values(imageUrlMap.value)) {
    if (url) URL.revokeObjectURL(url)
  }
  imageUrlMap.value = {}
  imageLoadPromises.clear()
  decodedImageElements.clear()
}

function resetPreviewState() {
  previewSessionSeq += 1
  previewRequestSeq += 1
  lightboxOpen.value = false
  loadingPreview.value = false
  loadingImageBlob.value = false
  revokeAllImageUrls()
  imageLoadPromises.clear()
  sameFolderImages.value = []
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
  if (isCrossFilesMode.value) return
  const hasPending = Object.values(folderFilesCache.value)
    .flat()
    .some((d) => !d.is_folder && (d.parse_status === 'pending' || d.parse_status === 'processing'))
  if (hasPending && selectedKbId.value) {
    pollTimer = setInterval(() => {
      pollPendingFolders()
    }, 3000)
  }
}

async function pollPendingFolders() {
  if (!selectedKbId.value || isCrossFilesMode.value) return
  const kbId = selectedKbId.value
  const refDomId = activeRefDomainId.value
  const pendingEntries = Object.entries(folderFilesCache.value).filter(([, list]) =>
    list.some((d) => !d.is_folder && (d.parse_status === 'pending' || d.parse_status === 'processing'))
  )
  if (pendingEntries.length === 0) {
    stopPolling()
    return
  }
  let anyChanged = false
  for (const [folderPath, prevList] of pendingEntries) {
    try {
      const res = await getDocuments(kbId, { lazy: true, folder_path: folderPath }, refDomId)
      const items = normalizeList(res).items
      const nextFiles = extractImmediateFiles(items, folderPath)
      const prevStatusMap = new Map(prevList.map((d) => [d.id, d.parse_status]))
      if (nextFiles.some((d) => prevStatusMap.get(d.id) !== d.parse_status)) {
        anyChanged = true
      }
      folderFilesCache.value = { ...folderFilesCache.value, [folderPath]: nextFiles }
    } catch {
      // Ignore polling error
    }
  }
  if (anyChanged) {
    treeRefreshSeed.value++
  }
  startPollingIfNeeded()
}

async function fetchKbs() {
  if (!domainId.value) return
  loadingKbs.value = true
  try {
    const [assetsRes, crossRefsRes] = await Promise.all([
      getAssets({
        scope: 'workspace',
        domain_id: domainId.value,
        asset_type: 'kb',
        page: kbPage.value,
        page_size: kbPageSize.value
      }),
      getDomainCrossRefs(domainId.value, false).catch(() => [])
    ])
    const normalized = normalizeList(assetsRes)
    kbList.value = normalized.items
    kbTotal.value = normalized.total
    activeCrossRefs.value = normalizeList(crossRefsRes).items

    if (kbSelectorEntries.value.length > 0) {
      await applyRouteOrDefaultSelection()
    } else {
      selectedEntryKey.value = ''
      selectedCrossFileRef.value = null
      folderFilesCache.value = {}
      rootEmpty.value = true
    }
  } catch {
    kbList.value = []
    activeCrossRefs.value = []
    kbTotal.value = 0
    selectedEntryKey.value = ''
    selectedCrossFileRef.value = null
    folderFilesCache.value = {}
  } finally {
    loadingKbs.value = false
  }
}

async function applyRouteOrDefaultSelection() {
  const queryCrossRefId = Number(route.query.crossRefId)
  if (Number.isFinite(queryCrossRefId) && queryCrossRefId > 0) {
    const targetRef = activeCrossRefs.value.find((r) => r.id === queryCrossRefId)
    if (targetRef) {
      if (targetRef.ref_type === 'asset' && targetRef.asset_type === 'kb') {
        const entryKey = `cross-kb:${targetRef.id}`
        if (kbSelectorEntries.value.some((e) => e.key === entryKey)) {
          handleSelectEntry(entryKey)
          return
        }
      } else if (targetRef.ref_type === 'kb_file') {
        if (kbSelectorEntries.value.some((e) => e.key === 'cross-files')) {
          handleSelectEntry('cross-files')
          await openCrossFileRef(targetRef)
          return
        }
      }
    }
  }

  const queryKbId = Number(route.query.kbId)
  if (Number.isFinite(queryKbId) && queryKbId > 0) {
    const ownKey = `own:${queryKbId}`
    if (kbSelectorEntries.value.some((e) => e.key === ownKey)) {
      selectedEntryKey.value = ownKey
      selectedCrossFileRef.value = null
      refreshTree()
      await tryOpenQueryDocument()
      return
    }
  }

  if (!kbSelectorEntries.value.some((e) => e.key === selectedEntryKey.value)) {
    selectedEntryKey.value = kbSelectorEntries.value[0].key
    selectedCrossFileRef.value = null
  }
  if (!isCrossFilesMode.value) {
    refreshTree()
    await tryOpenQueryDocument()
  }
}

function refreshTree() {
  stopPolling()
  folderFilesCache.value = {}
  rootEmpty.value = false
  treeRefreshSeed.value++
}

async function loadKbTreeNode(
  node: { level: number; data?: KbTreeNode },
  resolve: (data: KbTreeNode[]) => void
) {
  if (isCrossFilesMode.value || !selectedKbId.value) {
    resolve([])
    return
  }
  const kbId = selectedKbId.value
  const refDomId = activeRefDomainId.value
  const folderPath = node.level === 0 ? '' : (node.data?.folderPath || '')
  if (node.level === 0) {
    loadingDocs.value = true
    rootEmpty.value = false
  }
  try {
    const res = await getDocuments(kbId, { lazy: true, folder_path: folderPath }, refDomId)
    const items = normalizeList(res).items
    const files = extractImmediateFiles(items, folderPath)
    folderFilesCache.value = { ...folderFilesCache.value, [folderPath]: files }
    const nodes = buildImmediateKbTreeNodes(items, folderPath)
    if (node.level === 0) {
      rootEmpty.value = nodes.length === 0
    }
    resolve(nodes)
    startPollingIfNeeded()
  } catch {
    if (node.level === 0) {
      rootEmpty.value = true
    }
    resolve([])
  } finally {
    if (node.level === 0) {
      loadingDocs.value = false
    }
  }
}

function handleSelectEntry(key: string) {
  selectedEntryKey.value = key
  selectedCrossFileRef.value = null
  selectedFolderPath.value = ''
  selectedFileNode.value = null
  resetPreviewState()
  resetSocialState()
  searchResults.value = []
  hasSearched.value = false
  if (isCrossFilesMode.value) {
    rightTab.value = 'preview'
    stopPolling()
    folderFilesCache.value = {}
    rootEmpty.value = crossFileRefs.value.length === 0
  } else {
    refreshTree()
  }
}

async function openCrossFileRef(refItem: DomainCrossRefItem) {
  selectedCrossFileRef.value = refItem
  const syntheticDoc: KbDocument = {
    id: refItem.source_doc_id,
    kb_asset_id: refItem.source_asset_id,
    file_name: refItem.title,
    file_ext: refItem.file_ext || '',
    file_size: refItem.file_size || 0,
    folder_path: refItem.folder_path || '',
    is_folder: false,
    parse_status: 'completed',
    chunk_count: 0,
    uploader_id: refItem.created_by,
    created_at: refItem.created_at,
    updated_at: refItem.created_at
  }
  await openFileDoc(syntheticDoc, refItem.source_asset_id)
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
    refreshTree()
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
    refreshTree()
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

async function uploadFileList(files: File[], preserveRelativePath = false) {
  if (!selectedKbId.value || files.length === 0) return

  uploading.value = true
  let uploadedCount = 0
  try {
    for (const file of files) {
      if (file.size > MAX_KB_FILE_SIZE) {
        ElMessage.warning(`文件「${file.name}」超过 10MB 限制，已跳过`)
        continue
      }
      const relPath = getFileRelativePath(file)
      const useRelPath = preserveRelativePath || relPath.includes('/')
      await uploadDocument(
        selectedKbId.value,
        file,
        selectedFolderPath.value || undefined,
        useRelPath ? relPath : undefined
      )
      uploadedCount++
    }
    if (uploadedCount > 0) {
      ElMessage.success(`成功上传 ${uploadedCount} 个文件`)
      refreshTree()
    }
  } catch {
    // Handled by interceptor
  } finally {
    uploading.value = false
  }
}

async function handleFilesSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  input.value = ''
  await uploadFileList(files, false)
}

async function handleFolderSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  input.value = ''
  await uploadFileList(files, true)
}

function handleKbDragEnter(e: DragEvent) {
  if (isReadOnlyKb.value || !isFileDragEvent(e) || !selectedKbId.value) return
  e.preventDefault()
  kbDragDepth.value++
}

function handleKbDragOver(e: DragEvent) {
  if (isReadOnlyKb.value || !isFileDragEvent(e) || !selectedKbId.value) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

function handleKbDragLeave(e: DragEvent) {
  if (isReadOnlyKb.value || !isFileDragEvent(e)) return
  kbDragDepth.value = Math.max(0, kbDragDepth.value - 1)
}

async function handleKbDrop(e: DragEvent) {
  if (isReadOnlyKb.value || !isFileDragEvent(e)) return
  e.preventDefault()
  kbDragDepth.value = 0
  if (!selectedKbId.value) return
  const files = await extractFilesFromDrop(e)
  if (files.length === 0) return
  await uploadFileList(files, false)
}

async function loadPreviewDoc(
  docId: number,
  targetMode?: 'rendered' | 'edit' | 'split' | 'raw'
) {
  const kbId = selectedKbId.value
  if (!kbId) return
  const requestSeq = ++previewRequestSeq
  rightTab.value = 'preview'
  if (targetMode) {
    previewMode.value =
      isReadOnlyKb.value && (targetMode === 'edit' || targetMode === 'split')
        ? 'rendered'
        : targetMode
  }
  loadingPreview.value = true
  try {
    const doc = await getDocumentContent(kbId, docId, activeRefDomainId.value)
    if (requestSeq !== previewRequestSeq || selectedKbId.value !== kbId) return
    previewDoc.value = doc
    editFileName.value = doc.file_name
    editContent.value = doc.content
  } catch {
    if (requestSeq !== previewRequestSeq || selectedKbId.value !== kbId) return
    previewDoc.value = null
  } finally {
    if (requestSeq === previewRequestSeq) {
      loadingPreview.value = false
    }
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
    refreshTree()
  } catch {
    // Handled by interceptor
  } finally {
    savingEdit.value = false
  }
}

async function fetchAndCacheImageUrl(doc: KbDocument): Promise<string | null> {
  const requestKbId = selectedKbId.value
  if (!requestKbId) return null

  const existing = imageUrlMap.value[doc.id]
  if (existing) {
    decodeImageIntoMemory(doc.id, existing)
    return existing
  }

  const inFlight = imageLoadPromises.get(doc.id)
  if (inFlight) {
    return inFlight
  }

  const requestSessionSeq = previewSessionSeq
  const refDomId = activeRefDomainId.value
  const promise = (async (): Promise<string | null> => {
    const blob = await getDocumentRawBlob(requestKbId, doc.id, doc.file_name, refDomId)
    const url = URL.createObjectURL(blob)
    if (selectedKbId.value !== requestKbId || previewSessionSeq !== requestSessionSeq) {
      URL.revokeObjectURL(url)
      return null
    }
    setImageUrl(doc.id, url)
    decodeImageIntoMemory(doc.id, url)
    return url
  })()

  imageLoadPromises.set(doc.id, promise)
  try {
    return await promise
  } finally {
    if (imageLoadPromises.get(doc.id) === promise) {
      imageLoadPromises.delete(doc.id)
    }
  }
}

async function ensureImageUrl(doc: KbDocument): Promise<string | null> {
  if (!selectedKbId.value) return null
  const existing = imageUrlMap.value[doc.id]
  if (existing) {
    decodeImageIntoMemory(doc.id, existing)
    return existing
  }
  loadingImageBlob.value = true
  try {
    return await fetchAndCacheImageUrl(doc)
  } finally {
    if (!imageDoc.value || imageDoc.value.id === doc.id) {
      loadingImageBlob.value = false
    }
  }
}

async function preloadImageDoc(doc: KbDocument | undefined) {
  if (!doc || !selectedKbId.value) return
  try {
    await fetchAndCacheImageUrl(doc)
  } catch {
    // 后台预加载静默失败，切换时会按需重试
  }
}

function preloadAdjacentImages(centerIndex: number) {
  const nextDoc = sameFolderImages.value[centerIndex + 1]
  const prevDoc = sameFolderImages.value[centerIndex - 1]
  if (nextDoc) void preloadImageDoc(nextDoc)
  if (prevDoc) void preloadImageDoc(prevDoc)
}

async function selectImageAtIndex(idx: number) {
  const target = sameFolderImages.value[idx]
  const kbId = selectedKbId.value
  if (!target || !kbId) return
  const sessionSeq = previewSessionSeq
  imageDoc.value = target
  const url = await ensureImageUrl(target)
  if (
    !url ||
    sessionSeq !== previewSessionSeq ||
    selectedKbId.value !== kbId ||
    imageDoc.value?.id !== target.id
  ) {
    return
  }
  preloadAdjacentImages(idx)
}

function openLightbox() {
  if (!imagePreviewUrl.value) return
  lightboxScale.value = 1
  lightboxTranslateX.value = 0
  lightboxTranslateY.value = 0
  lightboxRotate.value = 0
  lightboxOpen.value = true
  preloadAdjacentImages(currentImageIndex.value)
}

function closeLightbox() {
  lightboxOpen.value = false
  lightboxDragging.value = false
}

function stepLightbox(delta: number) {
  const nextIdx = currentImageIndex.value + delta
  if (nextIdx < 0 || nextIdx >= sameFolderImages.value.length) return
  // 保持当前缩放、平移与旋转状态，不关闭重建灯箱
  void selectImageAtIndex(nextIdx)
}

function zoomLightbox(delta: number) {
  const next = Math.min(5, Math.max(0.25, Number((lightboxScale.value + delta).toFixed(2))))
  lightboxScale.value = next
}

function resetLightboxTransform() {
  lightboxScale.value = 1
  lightboxTranslateX.value = 0
  lightboxTranslateY.value = 0
  lightboxRotate.value = 0
}

function rotateLightbox() {
  lightboxRotate.value = (lightboxRotate.value + 90) % 360
}

function handleLightboxWheel(e: WheelEvent) {
  const step = e.deltaY < 0 ? 0.2 : -0.2
  zoomLightbox(step)
}

function handleLightboxMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.preventDefault()
  lightboxDragging.value = true
  dragStartX = e.clientX
  dragStartY = e.clientY
  dragOriginX = lightboxTranslateX.value
  dragOriginY = lightboxTranslateY.value
  window.addEventListener('mousemove', handleLightboxMouseMove)
  window.addEventListener('mouseup', handleLightboxMouseUp)
}

function handleLightboxMouseMove(e: MouseEvent) {
  if (!lightboxDragging.value) return
  lightboxTranslateX.value = dragOriginX + (e.clientX - dragStartX)
  lightboxTranslateY.value = dragOriginY + (e.clientY - dragStartY)
}

function handleLightboxMouseUp() {
  lightboxDragging.value = false
  window.removeEventListener('mousemove', handleLightboxMouseMove)
  window.removeEventListener('mouseup', handleLightboxMouseUp)
}

// Document Interactions (Likes, Own Favorite, Annotations/Comments)
const currentSelectedDoc = computed(() => selectedFileNode.value?.doc ?? null)
const interactions = ref<KbDocumentInteractions>({
  like_count: 0,
  liked_by_me: false,
  like_users: [],
  favorited_by_me: false,
  comment_count: 0
})
const loadingInteractions = ref(false)
const togglingLike = ref(false)
const togglingFavorite = ref(false)
const comments = ref<KbCommentItem[]>([])
const loadingComments = ref(false)
const commentQuote = ref('')
const commentContent = ref('')
const submittingComment = ref(false)

function getUserDisplay(u?: UserBrief | null, fallbackId?: number): string {
  if (u) return u.nickname || u.username || `用户 #${u.id}`
  return fallbackId ? `用户 #${fallbackId}` : '未知用户'
}

function getUserInitial(u?: UserBrief | null, fallbackId?: number): string {
  return getUserDisplay(u, fallbackId).slice(0, 1).toUpperCase()
}

function goToUserProfile(targetUserId?: number | null) {
  if (!targetUserId) return
  router.push(`/users/${targetUserId}`)
}

function resetSocialState() {
  loadingInteractions.value = false
  loadingComments.value = false
  interactions.value = {
    like_count: 0,
    liked_by_me: false,
    like_users: [],
    favorited_by_me: false,
    comment_count: 0
  }
  comments.value = []
  commentQuote.value = ''
  commentContent.value = ''
}

async function loadDocSocialData(kbId: number, docId: number, requestSeq = previewRequestSeq) {
  const refDomId = activeRefDomainId.value
  const isStale = () =>
    requestSeq !== previewRequestSeq ||
    selectedKbId.value !== kbId ||
    currentSelectedDoc.value?.id !== docId

  loadingInteractions.value = true
  loadingComments.value = true
  try {
    const res = await getDocumentInteractions(kbId, docId, refDomId)
    if (isStale()) return
    interactions.value = res
  } catch {
    // Keep defaults
  } finally {
    if (!isStale()) {
      loadingInteractions.value = false
    }
  }
  if (isStale()) return
  try {
    const res = await listDocumentComments(kbId, docId, refDomId)
    if (isStale()) return
    comments.value = Array.isArray(res) ? res : (res.list ?? [])
  } catch {
    if (isStale()) return
    comments.value = []
  } finally {
    if (!isStale()) {
      loadingComments.value = false
    }
  }
}

async function handleToggleLike() {
  if (!selectedKbId.value || !currentSelectedDoc.value || togglingLike.value) return
  const refDomId = activeRefDomainId.value
  togglingLike.value = true
  try {
    const res = await toggleDocumentLike(selectedKbId.value, currentSelectedDoc.value.id, refDomId)
    interactions.value.like_count = res.like_count
    interactions.value.liked_by_me = Boolean(res.liked_by_me ?? res.liked)
    if (res.like_users) {
      interactions.value.like_users = res.like_users
    } else {
      const fresh = await getDocumentInteractions(selectedKbId.value, currentSelectedDoc.value.id, refDomId)
      interactions.value = fresh
    }
  } catch {
    // Handled by interceptor
  } finally {
    togglingLike.value = false
  }
}

async function handleToggleFavorite() {
  if (!selectedKbId.value || !currentSelectedDoc.value || togglingFavorite.value) return
  togglingFavorite.value = true
  try {
    const res = await toggleDocumentFavorite(
      selectedKbId.value,
      currentSelectedDoc.value.id,
      activeRefDomainId.value
    )
    const nextFav = Boolean(res.favorited_by_me ?? res.favorited)
    interactions.value.favorited_by_me = nextFav
    ElMessage.success(nextFav ? '已添加到我的收藏' : '已取消收藏')
  } catch {
    // Handled by interceptor
  } finally {
    togglingFavorite.value = false
  }
}

async function handleSubmitComment() {
  if (!selectedKbId.value || !currentSelectedDoc.value || submittingComment.value) return
  const content = commentContent.value.trim()
  if (!content) {
    ElMessage.warning('请输入评论内容')
    return
  }
  submittingComment.value = true
  try {
    const created = await createDocumentComment(
      selectedKbId.value,
      currentSelectedDoc.value.id,
      {
        content,
        quote_text: commentQuote.value.trim() || undefined
      },
      activeRefDomainId.value
    )
    comments.value = [created, ...comments.value]
    interactions.value.comment_count = comments.value.length
    commentQuote.value = ''
    commentContent.value = ''
    ElMessage.success('批注与评论已发布')
  } catch {
    // Handled by interceptor
  } finally {
    submittingComment.value = false
  }
}

async function handleDeleteComment(comment: KbCommentItem) {
  if (!selectedKbId.value || !currentSelectedDoc.value) return
  try {
    await ElMessageBox.confirm('确认删除这条批注/评论吗？', '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await deleteDocumentComment(
      selectedKbId.value,
      currentSelectedDoc.value.id,
      comment.id,
      activeRefDomainId.value
    )
    comments.value = comments.value.filter((c) => c.id !== comment.id)
    interactions.value.comment_count = comments.value.length
    ElMessage.success('已删除')
  } catch {
    // Cancelled or failed
  }
}

async function openFileDoc(doc: KbDocument, overrideKbId?: number) {
  const kbId = overrideKbId ?? selectedKbId.value
  if (!kbId) return
  const refDomId = activeRefDomainId.value
  selectedFileNode.value = {
    key: `file-${doc.id}`,
    label: doc.file_name,
    isFolder: false,
    isLeaf: true,
    folderPath: doc.folder_path || '',
    fileSize: doc.file_size,
    doc,
    children: []
  }
  rightTab.value = 'preview'
  resetPreviewState()
  resetSocialState()
  const requestSeq = ++previewRequestSeq
  const isStale = () =>
    requestSeq !== previewRequestSeq ||
    selectedKbId.value !== kbId ||
    currentSelectedDoc.value?.id !== doc.id

  void loadDocSocialData(kbId, doc.id, requestSeq)

  if (isImageFile(doc.file_name)) {
    if (isCrossFilesMode.value) {
      sameFolderImages.value = [doc]
    } else {
      const cleanFolder = (doc.folder_path || '').replace(/^\/+|\/+$/g, '')
      let folderDocs = folderFilesCache.value[cleanFolder]
      if (!folderDocs) {
        try {
          const res = await getDocuments(kbId, { lazy: true, folder_path: cleanFolder }, refDomId)
          if (isStale()) return
          folderDocs = extractImmediateFiles(normalizeList(res).items, cleanFolder)
          folderFilesCache.value = { ...folderFilesCache.value, [cleanFolder]: folderDocs }
        } catch {
          if (isStale()) return
          folderDocs = [doc]
        }
      }
      if (isStale()) return
      const imgs = folderDocs
        .filter((d) => !d.is_folder && isImageFile(d.file_name))
        .sort((a, b) => a.file_name.localeCompare(b.file_name))
      sameFolderImages.value = imgs.length > 0 ? imgs : [doc]
    }
    imageDoc.value = doc
    try {
      const url = await ensureImageUrl(doc)
      if (!url || isStale()) return
      preloadAdjacentImages(currentImageIndex.value)
    } catch {
      if (!isStale()) {
        resetPreviewState()
      }
    }
    return
  }

  loadingPreview.value = true
  try {
    if (isTextFile(doc.file_name)) {
      const contentRes = await getDocumentContent(kbId, doc.id, refDomId)
      if (isStale()) return
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
    if (!isStale()) {
      resetPreviewState()
    }
  } finally {
    if (requestSeq === previewRequestSeq) {
      loadingPreview.value = false
    }
  }
}

async function tryOpenQueryDocument() {
  const queryDocId = Number(route.query.docId)
  if (!Number.isFinite(queryDocId) || queryDocId <= 0 || !selectedKbId.value) return
  try {
    const res = await getDocuments(selectedKbId.value, undefined, activeRefDomainId.value)
    const docs = normalizeList(res).items
    const match = docs.find((d) => !d.is_folder && d.id === queryDocId)
    if (match) {
      await openFileDoc(match)
    }
  } catch {
    // Ignore
  }
}

async function handleNodeClick(node: KbTreeNode) {
  if (node.isFolder) {
    selectedFolderPath.value = node.folderPath
    selectedFileNode.value = null
    return
  }
  if (!node.doc || !selectedKbId.value) return
  await openFileDoc(node.doc)
}

function allowDrag(draggingNode: { data: KbTreeNode }): boolean {
  if (isReadOnlyKb.value) return false
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
    refreshTree()
  } catch {
    refreshTree()
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
    refreshTree()
  } catch {
    // Handled by interceptor
  } finally {
    pasting.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (lightboxOpen.value) {
    if (e.key === 'Escape') {
      e.preventDefault()
      closeLightbox()
      return
    }
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      stepLightbox(-1)
      return
    }
    if (e.key === 'ArrowRight') {
      e.preventDefault()
      stepLightbox(1)
      return
    }
    if (e.key === '+' || e.key === '=') {
      e.preventDefault()
      zoomLightbox(0.25)
      return
    }
    if (e.key === '-') {
      e.preventDefault()
      zoomLightbox(-0.25)
      return
    }
  }

  if (!(e.ctrlKey || e.metaKey)) return
  const key = e.key.toLowerCase()
  if (
    !isReadOnlyKb.value &&
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
  if (isReadOnlyKb.value) return
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
    refreshTree()
  } catch {
    // Cancelled or failed
  }
}

async function handleDownloadAll() {
  if (!selectedKbId.value || isCrossFilesMode.value) return
  const fallbackName = selectedKb.value?.asset_code || selectedEntry.value?.name || 'knowledge-base'
  await confirmAndDownloadKb(
    selectedKbId.value,
    { scope: 'all' },
    `${fallbackName}.zip`,
    activeRefDomainId.value
  )
}

async function handleDownloadFolder(folderPath: string, folderLabel?: string) {
  if (!selectedKbId.value || isCrossFilesMode.value) return
  const name = folderLabel || folderPath.split('/').pop() || 'folder'
  await confirmAndDownloadKb(
    selectedKbId.value,
    { scope: 'folder', folder_path: folderPath },
    `${name}.zip`,
    activeRefDomainId.value
  )
}

async function handleDownloadFile(docId: number, fileName: string, overrideKbId?: number) {
  const kbId = overrideKbId ?? selectedKbId.value
  if (!kbId) return
  await confirmAndDownloadKb(
    kbId,
    { scope: 'file', doc_id: docId },
    fileName,
    activeRefDomainId.value
  )
}

async function handleSearchTest() {
  if (!selectedKbId.value || isCrossFilesMode.value) return
  if (!searchQuery.value.trim()) {
    ElMessage.warning('请输入检索关键词')
    return
  }
  searching.value = true
  hasSearched.value = true
  searchPage.value = 1
  try {
    const res = await searchKB(
      selectedKbId.value,
      {
        query: searchQuery.value.trim(),
        top_k: searchTopK.value
      },
      activeRefDomainId.value
    )
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

watch(
  () => route.query.crossRefId,
  async (val) => {
    if (!val || loadingKbs.value) return
    await applyRouteOrDefaultSelection()
  }
)

onMounted(() => {
  domainStore.fetchDomains()
  fetchKbs()
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  stopPolling()
  revokeAllImageUrls()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('mousemove', handleLightboxMouseMove)
  window.removeEventListener('mouseup', handleLightboxMouseUp)
})

onUnmounted(() => {
  imageLoadPromises.clear()
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

    <div v-else-if="kbSelectorEntries.length === 0" class="surface-card empty-box">
      <el-empty description="当前域尚未创建私有知识库资产，也暂无生效的跨域知识库引用">
        <el-button type="primary" @click="router.push(`/domains/${domainId}/assets`)">
          去创建知识库资产
        </el-button>
      </el-empty>
    </div>

    <div v-else class="kb-workspace">
      <el-alert
        v-if="isReadOnlyKb"
        type="info"
        :closable="false"
        show-icon
        class="readonly-banner"
      >
        <template #title>
          <span>
            只读模式：当前正在浏览跨域引用的知识库内容（不可新增、编辑、移动或删除原域资源）
            <template v-if="isCrossKbMode && selectedEntry?.sourceDomainName">
              · 来源域：<strong>{{ selectedEntry.sourceDomainName }}</strong>
            </template>
          </span>
        </template>
      </el-alert>

      <!-- KB Selector & Toolbar -->
      <div class="surface-card kb-toolbar-card">
        <div class="toolbar-top">
          <div class="selector-left">
            <el-icon :size="18" color="#4F46E5"><Collection /></el-icon>
            <span class="selector-label">当前知识库：</span>
            <el-select
              :model-value="selectedEntryKey"
              class="kb-select"
              @change="handleSelectEntry"
            >
              <el-option
                v-for="entry in kbSelectorEntries"
                :key="entry.key"
                :label="getEntryLabel(entry)"
                :value="entry.key"
              >
                <div class="kb-option-row">
                  <span class="kb-option-name">
                    {{ entry.kind === 'own' && entry.assetCode ? `${entry.name} (${entry.assetCode})` : entry.name }}
                  </span>
                  <div v-if="entry.kind === 'cross_kb'" class="kb-option-badges">
                    <el-tag size="small" type="warning" effect="light">跨引用</el-tag>
                    <el-tag size="small" type="info" effect="plain">只读</el-tag>
                    <span class="kb-option-source">来自: {{ entry.sourceDomainName }}</span>
                  </div>
                  <div v-else-if="entry.kind === 'cross_files'" class="kb-option-badges">
                    <el-tag size="small" type="success" effect="light">跨引用 · 只读</el-tag>
                    <span class="kb-option-source">{{ crossFileRefs.length }} 个文件</span>
                  </div>
                </div>
              </el-option>
            </el-select>
            <template v-if="isCrossKbMode && selectedEntry">
              <el-tag size="small" type="warning" effect="light">跨引用</el-tag>
              <el-tag size="small" type="info" effect="plain">只读</el-tag>
              <span class="selector-source-hint">来自: {{ selectedEntry.sourceDomainName }}</span>
            </template>
            <template v-else-if="isCrossFilesMode">
              <el-tag size="small" type="success" effect="light">跨引用 · 只读</el-tag>
            </template>
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
            <template v-if="!isReadOnlyKb">
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
            </template>
            <el-button
              v-if="!isCrossFilesMode"
              type="primary"
              :icon="Download"
              @click="handleDownloadAll"
            >
              下载整个知识库
            </el-button>
            <el-button :icon="Refresh" @click="isCrossFilesMode ? fetchKbs() : refreshTree()">
              刷新
            </el-button>
          </div>
        </div>

        <div v-if="selectedFolderPath && !isCrossFilesMode" class="active-folder-bar">
          <span>
            当前选中目录：<strong class="mono-text">{{ selectedFolderPath }}</strong>
          </span>
          <div class="folder-bar-btns">
            <el-button
              v-if="!isReadOnlyKb"
              link
              type="primary"
              size="small"
              @click="triggerFileUpload(selectedFolderPath)"
            >
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
        <!-- Left: Folder & File Tree (supports desktop file/folder drag & drop) -->
        <div
          class="surface-card kb-panel tree-panel"
          :class="{ 'is-file-dragover': kbDragDepth > 0 }"
          @dragenter="handleKbDragEnter"
          @dragover="handleKbDragOver"
          @dragleave="handleKbDragLeave"
          @drop="handleKbDrop"
        >
          <div v-if="kbDragDepth > 0" class="kb-drop-overlay">
            <el-icon :size="32"><Upload /></el-icon>
            <div class="kb-drop-title">释放鼠标即可上传文件或文件夹</div>
            <div class="kb-drop-sub mono-text">
              目标目录：{{ selectedFolderPath ? `/${selectedFolderPath}` : '/ (根目录)' }}
            </div>
          </div>

          <div class="panel-header">
            <h3 class="panel-title">{{ isCrossFilesMode ? '跨域引用文件' : '目录与文件' }}</h3>
            <span class="doc-count tabular-nums">
              <template v-if="isCrossFilesMode">{{ crossFileRefs.length }} 个文件 · 只读浏览</template>
              <template v-else>{{ loadedDocCount }} 项 · {{ isReadOnlyKb ? '只读浏览' : '支持桌面拖拽上传' }}</template>
            </span>
          </div>

          <div v-if="isCrossFilesMode" class="tree-scroll cross-files-list">
            <el-empty
              v-if="crossFileRefs.length === 0"
              description="暂无生效的跨域引用知识库文件"
              :image-size="64"
            />
            <div
              v-for="refItem in crossFileRefs"
              :key="refItem.id"
              class="cross-file-item"
              :class="{ 'is-active': selectedCrossFileRef?.id === refItem.id }"
              @click="openCrossFileRef(refItem)"
            >
              <div class="cross-file-main">
                <el-icon :size="16" :color="isImageFile(refItem.title) ? '#059669' : '#4F46E5'">
                  <Picture v-if="isImageFile(refItem.title)" />
                  <Document v-else />
                </el-icon>
                <div class="cross-file-info">
                  <div class="cross-file-title-row">
                    <span class="cross-file-name">{{ refItem.title }}</span>
                    <span v-if="refItem.file_size" class="tree-node-size tabular-nums">
                      {{ formatFileSize(refItem.file_size) }}
                    </span>
                  </div>
                  <div class="cross-file-meta">
                    <span v-if="refItem.kb_name" class="cross-file-kb">{{ refItem.kb_name }}</span>
                    <span v-if="refItem.folder_path" class="cross-file-path mono-text">/{{ refItem.folder_path }}</span>
                    <span class="cross-file-domain">来自: {{ refItem.source_domain_name || `域 #${refItem.source_domain_id}` }}</span>
                  </div>
                </div>
              </div>
              <el-button
                link
                type="primary"
                size="small"
                @click.stop="handleDownloadFile(refItem.source_doc_id, refItem.title, refItem.source_asset_id)"
              >
                下载
              </el-button>
            </div>
          </div>

          <div v-else v-loading="loadingDocs" class="tree-scroll">
            <el-empty
              v-if="rootEmpty"
              :description="isReadOnlyKb ? '该知识库暂无文件或文件夹' : '暂无文件或文件夹，请点击上方按钮上传文件或新建 Markdown'"
              :image-size="64"
            />
            <el-tree
              v-show="!rootEmpty"
              :key="treeKey"
              lazy
              :load="loadKbTreeNode"
              :props="treeProps"
              node-key="key"
              :default-expand-all="false"
              highlight-current
              :draggable="!isReadOnlyKb"
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
                      v-if="!isReadOnlyKb && data.isFolder && clipboardFile"
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
                        v-if="!isReadOnlyKb"
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
                      v-if="!isReadOnlyKb && data.doc"
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
            <el-tab-pane :label="isReadOnlyKb ? '文件预览' : '文件预览与编辑'" name="preview">
              <div v-if="loadingPreview" class="mt-16">
                <el-skeleton :rows="6" animated />
              </div>

              <template v-else-if="currentSelectedDoc && (imageDoc || binaryDoc || previewDoc)">
                <!-- Uploader & Interactions Bar -->
                <div class="doc-social-bar">
                  <div class="doc-uploader-info">
                    <span class="uploader-label">上传者</span>
                    <button
                      type="button"
                      class="user-chip-btn"
                      @click="goToUserProfile(currentSelectedDoc.uploader?.id || currentSelectedDoc.uploader_id)"
                    >
                      <el-avatar :size="22" :src="currentSelectedDoc.uploader?.avatar || undefined">
                        {{ getUserInitial(currentSelectedDoc.uploader, currentSelectedDoc.uploader_id) }}
                      </el-avatar>
                      <span class="user-chip-name">
                        {{ getUserDisplay(currentSelectedDoc.uploader, currentSelectedDoc.uploader_id) }}
                      </span>
                    </button>
                    <span class="doc-time-sep">·</span>
                    <span class="doc-time-text tabular-nums">
                      更新于 {{ formatDateTime(currentSelectedDoc.updated_at || currentSelectedDoc.created_at) }}
                    </span>
                  </div>

                  <div class="doc-action-btns">
                    <el-button
                      size="small"
                      :type="interactions.liked_by_me ? 'primary' : 'default'"
                      :plain="!interactions.liked_by_me"
                      :loading="togglingLike"
                      :icon="Pointer"
                      @click="handleToggleLike"
                    >
                      {{ interactions.liked_by_me ? '已赞' : '点赞' }}
                      <span class="tabular-nums" style="margin-left: 4px">{{ interactions.like_count }}</span>
                    </el-button>

                    <el-popover
                      v-if="interactions.like_count > 0 && interactions.like_users.length > 0"
                      placement="bottom"
                      :width="240"
                      trigger="click"
                    >
                      <template #reference>
                        <el-button size="small" link type="info">
                          谁赞过 ({{ interactions.like_users.length }})
                        </el-button>
                      </template>
                      <div class="like-users-popover">
                        <div class="popover-title">点赞用户</div>
                        <div class="like-users-list">
                          <button
                            v-for="u in interactions.like_users"
                            :key="u.id"
                            type="button"
                            class="like-user-item"
                            @click="goToUserProfile(u.id)"
                          >
                            <el-avatar :size="24" :src="u.avatar || undefined">
                              {{ getUserInitial(u) }}
                            </el-avatar>
                            <span>{{ getUserDisplay(u) }}</span>
                          </button>
                        </div>
                      </div>
                    </el-popover>

                    <el-button
                      size="small"
                      :type="interactions.favorited_by_me ? 'warning' : 'default'"
                      :plain="!interactions.favorited_by_me"
                      :loading="togglingFavorite"
                      :icon="interactions.favorited_by_me ? StarFilled : Star"
                      @click="handleToggleFavorite"
                    >
                      {{ interactions.favorited_by_me ? '已收藏' : '收藏' }}
                    </el-button>
                  </div>
                </div>

                <div v-if="imageDoc" class="preview-container">
                <div class="preview-header">
                  <div class="preview-meta">
                    <span class="preview-filename mono-text">{{ imageDoc.file_name }}</span>
                    <span class="preview-sub mono-text">
                      {{ imageDoc.folder_path || '/' }} · {{ formatFileSize(imageDoc.file_size) }}
                    </span>
                  </div>
                  <div class="preview-controls">
                    <div v-if="sameFolderImages.length > 0" class="image-nav-controls">
                      <el-button
                        size="small"
                        :icon="ArrowLeft"
                        :disabled="currentImageIndex <= 0"
                        @click="selectImageAtIndex(currentImageIndex - 1)"
                      >
                        上一张
                      </el-button>
                      <span class="image-nav-counter tabular-nums">
                        {{ currentImageIndex + 1 }} / {{ sameFolderImages.length }}
                      </span>
                      <el-button
                        size="small"
                        :disabled="currentImageIndex >= sameFolderImages.length - 1"
                        @click="selectImageAtIndex(currentImageIndex + 1)"
                      >
                        下一张
                        <el-icon class="el-icon--right"><ArrowRight /></el-icon>
                      </el-button>
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
                </div>
                <div v-loading="loadingImageBlob" class="image-preview-box">
                  <div
                    v-if="imagePreviewUrl"
                    class="image-preview-trigger"
                    title="点击放大并支持缩放状态下连续浏览"
                    @click="openLightbox"
                  >
                    <img :src="imagePreviewUrl" :alt="imageDoc.file_name" class="inline-preview-img" />
                    <span class="image-zoom-hint">点击放大连续浏览</span>
                  </div>
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
                      <el-radio-button v-if="!isReadOnlyKb" value="edit">在线编辑</el-radio-button>
                      <el-radio-button v-if="!isReadOnlyKb" value="split">分屏编辑</el-radio-button>
                      <el-radio-button value="raw">原文</el-radio-button>
                    </el-radio-group>
                    <el-button
                      v-if="!isReadOnlyKb && (previewMode === 'edit' || previewMode === 'split')"
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

              <!-- Annotations & Comments Section -->
              <div class="doc-comments-section">
                <div class="comments-header">
                  <div class="comments-title-wrap">
                    <el-icon :size="16" color="#4F46E5"><ChatLineSquare /></el-icon>
                    <h4 class="comments-title">批注与评论</h4>
                    <el-tag size="small" type="info" effect="plain" class="tabular-nums">
                      {{ comments.length }}
                    </el-tag>
                  </div>
                </div>

                <div class="comment-composer">
                  <el-input
                    v-model="commentQuote"
                    size="small"
                    placeholder="批注摘录 / 定位（选填，如：第3段或图片区域说明）"
                    clearable
                  />
                  <el-input
                    v-model="commentContent"
                    type="textarea"
                    :rows="2"
                    placeholder="写下你的批注或评论..."
                  />
                  <div class="comment-composer-actions">
                    <el-button
                      type="primary"
                      size="small"
                      :loading="submittingComment"
                      :disabled="!commentContent.trim()"
                      @click="handleSubmitComment"
                    >
                      发布评论
                    </el-button>
                  </div>
                </div>

                <div v-loading="loadingComments" class="comments-list">
                  <div
                    v-for="c in comments"
                    :key="c.id"
                    class="comment-item"
                  >
                    <div class="comment-item-header">
                      <button
                        type="button"
                        class="user-chip-btn"
                        @click="goToUserProfile(c.author?.id || c.user_id)"
                      >
                        <el-avatar :size="24" :src="c.author?.avatar || undefined">
                          {{ getUserInitial(c.author, c.user_id) }}
                        </el-avatar>
                        <span class="user-chip-name">{{ getUserDisplay(c.author, c.user_id) }}</span>
                      </button>
                      <div class="comment-item-right">
                        <span class="comment-time tabular-nums">{{ formatDateTime(c.created_at) }}</span>
                        <el-button
                          v-if="c.can_delete"
                          link
                          type="danger"
                          size="small"
                          :icon="Delete"
                          @click="handleDeleteComment(c)"
                        >
                          删除
                        </el-button>
                      </div>
                    </div>
                    <blockquote v-if="c.quote_text" class="comment-quote">
                      {{ c.quote_text }}
                    </blockquote>
                    <div class="comment-content">{{ c.content }}</div>
                  </div>

                  <el-empty
                    v-if="!loadingComments && comments.length === 0"
                    description="暂无批注或评论，欢迎发表第一条看法"
                    :image-size="48"
                  />
                </div>
              </div>
              </template>

              <div v-else class="preview-placeholder">
                {{
                  isReadOnlyKb
                    ? '在左侧点击任意文件（支持 Markdown、文本、代码、图片及二进制文件），即可在此处预览或下载。'
                    : '在左侧目录树点击任意文件（支持 Markdown、文本、代码、图片及二进制文件），即可在此处预览、编辑或下载。'
                }}
              </div>
            </el-tab-pane>

            <el-tab-pane v-if="!isCrossFilesMode" label="关键词检索" name="search">
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

    <!-- Cross-Domain References Section -->
    <CrossRefSection :domain-id="domainId" default-ref-type="kb_file" />

    <!-- Controlled Persistent-Zoom Image Lightbox -->
    <Teleport to="body">
      <div
        v-if="lightboxOpen && imageDoc"
        class="kb-lightbox-backdrop"
        @click.self="closeLightbox"
        @wheel.prevent="handleLightboxWheel"
      >
        <div class="kb-lightbox-topbar">
          <div class="kb-lightbox-title">
            <span class="mono-text">{{ imageDoc.file_name }}</span>
            <span class="kb-lightbox-meta tabular-nums">
              {{ currentImageIndex + 1 }} / {{ sameFolderImages.length }} ·
              {{ formatFileSize(imageDoc.file_size) }} ·
              缩放 {{ Math.round(lightboxScale * 100) }}%
            </span>
          </div>
          <button type="button" class="kb-lightbox-close" title="关闭 (Esc)" @click="closeLightbox">
            <el-icon :size="20"><Close /></el-icon>
          </button>
        </div>

        <button
          v-if="sameFolderImages.length > 1"
          type="button"
          class="kb-lightbox-nav is-prev"
          :disabled="currentImageIndex <= 0"
          title="上一张 (←)"
          @click="stepLightbox(-1)"
        >
          <el-icon :size="24"><ArrowLeft /></el-icon>
        </button>

        <div class="kb-lightbox-stage" @click.self="closeLightbox">
          <img
            v-if="imagePreviewUrl"
            :src="imagePreviewUrl"
            :alt="imageDoc.file_name"
            class="kb-lightbox-img"
            :class="{ 'is-dragging': lightboxDragging }"
            :style="{
              transform: `translate3d(${lightboxTranslateX}px, ${lightboxTranslateY}px, 0) scale(${lightboxScale}) rotate(${lightboxRotate}deg)`
            }"
            draggable="false"
            @mousedown="handleLightboxMouseDown"
          />
          <div v-else class="kb-lightbox-loading">正在加载图片...</div>
        </div>

        <button
          v-if="sameFolderImages.length > 1"
          type="button"
          class="kb-lightbox-nav is-next"
          :disabled="currentImageIndex >= sameFolderImages.length - 1"
          title="下一张 (→)"
          @click="stepLightbox(1)"
        >
          <el-icon :size="24"><ArrowRight /></el-icon>
        </button>

        <div class="kb-lightbox-toolbar">
          <button
            type="button"
            class="kb-tb-btn"
            :disabled="currentImageIndex <= 0"
            title="上一张 (←)"
            @click="stepLightbox(-1)"
          >
            <el-icon :size="16"><ArrowLeft /></el-icon>
          </button>
          <button type="button" class="kb-tb-btn" title="缩小 (-)" @click="zoomLightbox(-0.25)">
            <el-icon :size="16"><ZoomOut /></el-icon>
          </button>
          <span class="kb-tb-scale tabular-nums">{{ Math.round(lightboxScale * 100) }}%</span>
          <button type="button" class="kb-tb-btn" title="放大 (+)" @click="zoomLightbox(0.25)">
            <el-icon :size="16"><ZoomIn /></el-icon>
          </button>
          <button type="button" class="kb-tb-btn kb-tb-text" title="重置缩放与位置" @click="resetLightboxTransform">
            1:1
          </button>
          <button type="button" class="kb-tb-btn" title="旋转 90°" @click="rotateLightbox">
            <el-icon :size="16"><RefreshRight /></el-icon>
          </button>
          <button
            type="button"
            class="kb-tb-btn"
            :disabled="currentImageIndex >= sameFolderImages.length - 1"
            title="下一张 (→)"
            @click="stepLightbox(1)"
          >
            <el-icon :size="16"><ArrowRight /></el-icon>
          </button>
        </div>
      </div>
    </Teleport>
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
  flex-wrap: wrap;
}

.kb-select {
  width: 320px;
  max-width: 100%;
}

.readonly-banner {
  border-radius: 10px;
}

.kb-option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.kb-option-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-option-badges {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.kb-option-source,
.selector-source-hint {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.cross-files-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.cross-file-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.cross-file-item:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
}

.cross-file-item.is-active {
  background: var(--ascp-primary-soft);
  border-color: var(--ascp-primary);
}

.cross-file-main {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.cross-file-info {
  min-width: 0;
  flex: 1;
}

.cross-file-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cross-file-name {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--ascp-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cross-file-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 3px;
  font-size: 11.5px;
  color: var(--ascp-text-muted);
}

.cross-file-kb {
  color: var(--ascp-text-secondary);
  font-weight: 500;
}

.cross-file-path {
  color: var(--ascp-text-muted);
}

.cross-file-domain {
  color: var(--ascp-primary);
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

.tree-panel {
  position: relative;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.tree-panel.is-file-dragover {
  border-color: var(--ascp-primary);
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.14);
}

.kb-drop-overlay {
  position: absolute;
  inset: 8px;
  z-index: 20;
  border-radius: 10px;
  border: 2px dashed var(--ascp-primary);
  background: rgba(238, 242, 255, 0.92);
  color: var(--ascp-primary);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  padding: 16px;
  pointer-events: none;
}

.kb-drop-title {
  font-size: 14px;
  font-weight: 600;
}

.kb-drop-sub {
  font-size: 12px;
  color: var(--ascp-text-secondary);
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
  flex-wrap: wrap;
}

.image-nav-controls {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.image-nav-counter {
  font-size: 12.5px;
  color: var(--ascp-text-secondary);
  padding: 0 4px;
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

.image-preview-trigger {
  position: relative;
  display: inline-flex;
  justify-content: center;
  align-items: center;
  cursor: zoom-in;
  max-width: 100%;
}

.inline-preview-img {
  max-height: 480px;
  max-width: 100%;
  object-fit: contain;
  border-radius: 6px;
}

.image-zoom-hint {
  position: absolute;
  bottom: 10px;
  right: 10px;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.72);
  color: #f8fafc;
  font-size: 12px;
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.image-preview-trigger:hover .image-zoom-hint {
  opacity: 1;
}

.kb-lightbox-backdrop {
  position: fixed;
  inset: 0;
  z-index: 3000;
  background: rgba(15, 23, 42, 0.9);
  display: flex;
  align-items: center;
  justify-content: center;
  user-select: none;
}

.kb-lightbox-topbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 3002;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  background: linear-gradient(to bottom, rgba(15, 23, 42, 0.75), transparent);
  color: #f8fafc;
}

.kb-lightbox-title {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.kb-lightbox-meta {
  font-size: 12.5px;
  color: #cbd5e1;
}

.kb-lightbox-close {
  width: 36px;
  height: 36px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(30, 41, 59, 0.75);
  color: #f8fafc;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.kb-lightbox-close:hover {
  background: rgba(51, 65, 85, 0.9);
}

.kb-lightbox-stage {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.kb-lightbox-img {
  max-width: 88vw;
  max-height: 82vh;
  object-fit: contain;
  cursor: grab;
  transition: transform 0.08s ease-out;
}

.kb-lightbox-img.is-dragging {
  cursor: grabbing;
  transition: none;
}

.kb-lightbox-loading {
  color: #e2e8f0;
  font-size: 14px;
}

.kb-lightbox-nav {
  position: fixed;
  top: 50%;
  transform: translateY(-50%);
  z-index: 3002;
  width: 44px;
  height: 44px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(30, 41, 59, 0.75);
  color: #f8fafc;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.kb-lightbox-nav.is-prev {
  left: 20px;
}

.kb-lightbox-nav.is-next {
  right: 20px;
}

.kb-lightbox-nav:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.kb-lightbox-nav:not(:disabled):hover {
  background: rgba(79, 70, 229, 0.85);
}

.kb-lightbox-toolbar {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 3002;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(30, 41, 59, 0.88);
  border: 1px solid rgba(255, 255, 255, 0.16);
  color: #f8fafc;
}

.kb-tb-btn {
  width: 32px;
  height: 32px;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: #f8fafc;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.kb-tb-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.kb-tb-btn:not(:disabled):hover {
  background: rgba(255, 255, 255, 0.14);
}

.kb-tb-text {
  width: auto;
  padding: 0 8px;
  font-size: 12.5px;
  font-weight: 600;
}

.kb-tb-scale {
  min-width: 46px;
  text-align: center;
  font-size: 12.5px;
  color: #e2e8f0;
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

@media (max-width: 992px) {
  .kb-columns {
    grid-template-columns: 1fr;
  }

  .split-editor-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .selector-left,
  .kb-select {
    width: 100%;
  }

  .kb-panel {
    padding: 14px 12px;
  }
}

.doc-social-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 14px;
  margin-bottom: 14px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
}

.doc-uploader-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 13px;
  color: #64748b;
}

.uploader-label {
  color: #94a3b8;
  font-size: 12px;
}

.user-chip-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  padding: 2px 6px 2px 2px;
  border-radius: 999px;
  cursor: pointer;
  color: #0f172a;
  font-size: 13px;
  font-weight: 500;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.user-chip-btn:hover {
  background: #e0e7ff;
  color: #4f46e5;
}

.doc-time-text {
  font-size: 12px;
  color: #94a3b8;
}

.doc-action-btns {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.like-users-popover .popover-title {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 8px;
}

.like-users-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 220px;
  overflow-y: auto;
}

.like-user-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #0f172a;
  text-align: left;
}

.like-user-item:hover {
  background: #f1f5f9;
  color: #4f46e5;
}

.doc-comments-section {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid #e2e8f0;
}

.comments-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.comments-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.comments-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #0f172a;
}

.comment-composer {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  margin-bottom: 16px;
}

.comment-composer-actions {
  display: flex;
  justify-content: flex-end;
}

.comments-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.comment-item {
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
}

.comment-item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.comment-item-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.comment-time {
  font-size: 12px;
  color: #94a3b8;
}

.comment-quote {
  margin: 0 0 8px;
  padding: 6px 10px;
  border-left: 3px solid #6366f1;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  border-radius: 0 6px 6px 0;
}

.comment-content {
  font-size: 13px;
  line-height: 1.6;
  color: #1e293b;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
