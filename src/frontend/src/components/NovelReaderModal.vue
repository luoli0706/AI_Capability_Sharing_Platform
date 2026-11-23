<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ArrowLeft,
  ArrowRight,
  Close,
  List,
  Notebook
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getDocumentContent, getReadingProgress, saveReadingProgress } from '@/api/kb'
import type { KbContentResult, KbDocument } from '@/types/api'

type ReaderTheme = 'sepia' | 'light' | 'dark'

const props = defineProps<{
  visible: boolean
  kbId: number
  initialDoc: KbContentResult
  chapters: KbDocument[]
  refDomainId?: number
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'chapter-changed', doc: KbDocument): void
}>()

const theme = ref<ReaderTheme>('sepia')
const fontSize = ref(18)
const drawerOpen = ref(false)
const loadingChapter = ref(false)

const currentDocId = ref(props.initialDoc.id)
const currentFileName = ref(props.initialDoc.file_name)
const currentContent = ref(props.initialDoc.content)

const pages = ref<string[]>([''])
const currentPage = ref(1)
const jumpPageInput = ref<number | undefined>(undefined)
const restoredHint = ref('')

const pageViewportRef = ref<HTMLElement | null>(null)
const measureBoxRef = ref<HTMLElement | null>(null)

const totalPages = computed(() => Math.max(1, pages.value.length))
const currentPageText = computed(() => pages.value[currentPage.value - 1] ?? '')

const currentChapterIndex = computed(() =>
  props.chapters.findIndex((c) => c.id === currentDocId.value)
)
const hasPrevChapter = computed(() => currentChapterIndex.value > 0)
const hasNextChapter = computed(
  () => currentChapterIndex.value >= 0 && currentChapterIndex.value < props.chapters.length - 1
)

let sessionSeq = 0
let saveTimer: ReturnType<typeof setTimeout> | null = null
let resizeTimer: ReturnType<typeof setTimeout> | null = null
let pendingSavePayload: {
  kbId: number
  docId: number
  current_page: number
  total_pages: number
  refDomainId?: number
} | null = null

const THEME_LABELS: Array<{ value: ReaderTheme; label: string }> = [
  { value: 'sepia', label: '护眼米黄' },
  { value: 'light', label: '纸白' },
  { value: 'dark', label: '夜间深色' }
]

function clearSaveTimer() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
}

async function flushPendingSave() {
  clearSaveTimer()
  if (!pendingSavePayload) return
  const payload = pendingSavePayload
  pendingSavePayload = null
  try {
    await saveReadingProgress(
      payload.kbId,
      payload.docId,
      { current_page: payload.current_page, total_pages: payload.total_pages },
      payload.refDomainId
    )
  } catch {
    // Ignore background save error
  }
}

function scheduleSaveProgress() {
  if (!props.kbId || !currentDocId.value) return
  pendingSavePayload = {
    kbId: props.kbId,
    docId: currentDocId.value,
    current_page: currentPage.value,
    total_pages: totalPages.value,
    refDomainId: props.refDomainId
  }
  clearSaveTimer()
  saveTimer = setTimeout(() => {
    void flushPendingSave()
  }, 500)
}

function paginateText(): string[] {
  const text = currentContent.value || ''
  if (!text) return ['（空文档）']

  const viewport = pageViewportRef.value
  const measureEl = measureBoxRef.value
  if (!viewport || !measureEl) {
    return [text]
  }

  const style = window.getComputedStyle(viewport)
  const padTop = parseFloat(style.paddingTop) || 0
  const padBottom = parseFloat(style.paddingBottom) || 0
  const padLeft = parseFloat(style.paddingLeft) || 0
  const padRight = parseFloat(style.paddingRight) || 0
  const maxHeight = Math.max(160, viewport.clientHeight - padTop - padBottom)
  const contentWidth = Math.max(240, viewport.clientWidth - padLeft - padRight)

  measureEl.style.width = `${contentWidth}px`
  measureEl.style.fontSize = `${fontSize.value}px`

  const fits = (candidate: string): boolean => {
    measureEl.textContent = candidate
    return measureEl.scrollHeight <= maxHeight + 1
  }

  const lines = text.split(/\r?\n/)
  const result: string[] = []
  let lineIdx = 0

  while (lineIdx < lines.length) {
    // Binary search largest ending line index [lineIdx, lines.length] that fits
    let low = lineIdx + 1
    let high = lines.length
    let best = lineIdx

    while (low <= high) {
      const mid = (low + high) >> 1
      const candidate = lines.slice(lineIdx, mid).join('\n')
      if (fits(candidate)) {
        best = mid
        low = mid + 1
      } else {
        high = mid - 1
      }
    }

    if (best > lineIdx) {
      result.push(lines.slice(lineIdx, best).join('\n'))
      lineIdx = best
    } else {
      // Single line is taller than maxHeight; split by character binary search
      let longLine = lines[lineIdx]
      while (longLine.length > 0) {
        let cLow = 1
        let cHigh = longLine.length
        let cBest = 1
        while (cLow <= cHigh) {
          const cMid = (cLow + cHigh) >> 1
          if (fits(longLine.slice(0, cMid))) {
            cBest = cMid
            cLow = cMid + 1
          } else {
            cHigh = cMid - 1
          }
        }
        result.push(longLine.slice(0, cBest))
        longLine = longLine.slice(cBest)
      }
      lineIdx++
    }
  }

  return result.length > 0 ? result : ['']
}

async function recomputePagination(preserveRatio = false) {
  const oldTotal = totalPages.value
  const oldPage = currentPage.value
  await nextTick()
  pages.value = paginateText()
  if (preserveRatio && oldTotal > 1) {
    const ratio = (oldPage - 1) / Math.max(1, oldTotal - 1)
    const mapped = Math.round(ratio * Math.max(0, totalPages.value - 1)) + 1
    currentPage.value = Math.min(Math.max(1, mapped), totalPages.value)
  } else {
    currentPage.value = Math.min(Math.max(1, currentPage.value), totalPages.value)
  }
}

async function restoreProgressForDoc(docId: number, seq: number) {
  restoredHint.value = ''
  try {
    const progress = await getReadingProgress(props.kbId, docId, props.refDomainId)
    if (seq !== sessionSeq || !props.visible || currentDocId.value !== docId) return
    if (progress && progress.total_pages > 0 && progress.current_page > 1) {
      const targetPage = Math.min(Math.max(1, progress.current_page), totalPages.value)
      currentPage.value = targetPage
      const hint = `已自动跳转至上次阅读进度：第 ${targetPage}/${totalPages.value} 页`
      restoredHint.value = hint
      ElMessage.info(hint)
    }
  } catch {
    // Ignore if no progress yet or API unavailable
  }
}

async function openChapter(doc: KbDocument) {
  if (doc.id === currentDocId.value && currentContent.value) {
    drawerOpen.value = false
    return
  }
  await flushPendingSave()
  const seq = ++sessionSeq
  loadingChapter.value = true
  restoredHint.value = ''
  try {
    const contentRes = await getDocumentContent(props.kbId, doc.id, props.refDomainId)
    if (seq !== sessionSeq || !props.visible) return
    currentDocId.value = doc.id
    currentFileName.value = contentRes.file_name
    currentContent.value = contentRes.content
    currentPage.value = 1
    drawerOpen.value = false
    emit('chapter-changed', doc)
    await recomputePagination(false)
    if (seq !== sessionSeq || !props.visible) return
    await restoreProgressForDoc(doc.id, seq)
  } catch {
    // Handled by interceptor
  } finally {
    if (seq === sessionSeq) {
      loadingChapter.value = false
    }
  }
}

function goToPage(page: number) {
  const clamped = Math.min(Math.max(1, Math.floor(page)), totalPages.value)
  if (clamped === currentPage.value) return
  currentPage.value = clamped
  if (pageViewportRef.value) {
    pageViewportRef.value.scrollTop = 0
  }
  scheduleSaveProgress()
}

function prevPage() {
  if (currentPage.value > 1) {
    goToPage(currentPage.value - 1)
  } else if (hasPrevChapter.value) {
    void openChapter(props.chapters[currentChapterIndex.value - 1])
  }
}

function nextPage() {
  if (currentPage.value < totalPages.value) {
    goToPage(currentPage.value + 1)
  } else if (hasNextChapter.value) {
    void openChapter(props.chapters[currentChapterIndex.value + 1])
  }
}

function handleJumpPage() {
  if (!jumpPageInput.value || !Number.isFinite(jumpPageInput.value)) return
  goToPage(jumpPageInput.value)
  jumpPageInput.value = undefined
}

function adjustFontSize(delta: number) {
  const next = Math.min(26, Math.max(14, fontSize.value + delta))
  if (next === fontSize.value) return
  fontSize.value = next
  void recomputePagination(true).then(() => {
    scheduleSaveProgress()
  })
}

async function handleClose() {
  await flushPendingSave()
  emit('update:visible', false)
}

function handleReaderKeydown(e: KeyboardEvent) {
  if (!props.visible) return
  const target = e.target as HTMLElement | null
  if (
    target &&
    (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)
  ) {
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    void handleClose()
  } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
    e.preventDefault()
    prevPage()
  } else if (e.key === 'ArrowRight' || e.key === 'PageDown' || e.key === ' ') {
    e.preventDefault()
    nextPage()
  }
}

function handleResize() {
  if (!props.visible) return
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    void recomputePagination(true)
  }, 150)
}

watch(
  () => props.visible,
  async (val) => {
    if (val) {
      const seq = ++sessionSeq
      currentDocId.value = props.initialDoc.id
      currentFileName.value = props.initialDoc.file_name
      currentContent.value = props.initialDoc.content
      currentPage.value = 1
      drawerOpen.value = false
      await recomputePagination(false)
      if (seq !== sessionSeq || !props.visible) return
      await restoreProgressForDoc(props.initialDoc.id, seq)
    } else {
      await flushPendingSave()
    }
  },
  { immediate: true }
)

onMounted(() => {
  window.addEventListener('keydown', handleReaderKeydown)
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleReaderKeydown)
  window.removeEventListener('resize', handleResize)
  if (resizeTimer) clearTimeout(resizeTimer)
  void flushPendingSave()
})
</script>

<template>
  <teleport to="body">
    <div v-if="visible" class="novel-reader-overlay" :class="`theme-${theme}`">
      <!-- Top Toolbar -->
      <header class="novel-header">
        <div class="novel-header-left">
          <el-icon :size="18"><Notebook /></el-icon>
          <span class="novel-title" :title="currentFileName">{{ currentFileName }}</span>
          <el-tag v-if="chapters.length > 1 && currentChapterIndex >= 0" size="small" effect="plain">
            第 {{ currentChapterIndex + 1 }} / {{ chapters.length }} 章
          </el-tag>
          <span v-if="restoredHint" class="novel-restored-badge">{{ restoredHint }}</span>
        </div>

        <div class="novel-header-right">
          <el-button size="small" :icon="List" @click="drawerOpen = !drawerOpen">
            章节目录 ({{ chapters.length }})
          </el-button>

          <div class="font-controls">
            <el-button size="small" :disabled="fontSize <= 14" @click="adjustFontSize(-2)">
              A-
            </el-button>
            <span class="font-size-label tabular-nums">{{ fontSize }}px</span>
            <el-button size="small" :disabled="fontSize >= 26" @click="adjustFontSize(2)">
              A+
            </el-button>
          </div>

          <el-radio-group v-model="theme" size="small">
            <el-radio-button v-for="t in THEME_LABELS" :key="t.value" :value="t.value">
              {{ t.label }}
            </el-radio-button>
          </el-radio-group>

          <el-button size="small" type="info" plain :icon="Close" @click="handleClose">
            退出阅读
          </el-button>
        </div>
      </header>

      <!-- Main Reading Area -->
      <div class="novel-body">
        <!-- Chapter Drawer -->
        <aside v-if="drawerOpen" class="novel-chapter-drawer">
          <div class="drawer-header">
            <span>章节目录 ({{ chapters.length }})</span>
            <el-button link :icon="Close" @click="drawerOpen = false" />
          </div>
          <div class="drawer-list">
            <button
              v-for="(chap, idx) in chapters"
              :key="chap.id"
              type="button"
              class="chapter-item"
              :class="{ 'is-active': chap.id === currentDocId }"
              @click="openChapter(chap)"
            >
              <span class="chapter-idx tabular-nums">{{ idx + 1 }}.</span>
              <span class="chapter-name">{{ chap.file_name }}</span>
            </button>
          </div>
        </aside>

        <!-- Paginated Page Card -->
        <main v-loading="loadingChapter" class="novel-page-stage">
          <div ref="pageViewportRef" class="novel-page-card">
            <div
              class="novel-page-text"
              :style="{ fontSize: `${fontSize}px` }"
            >{{ currentPageText }}</div>
          </div>

          <!-- Hidden measurement box for exact line-safe pagination -->
          <div ref="measureBoxRef" class="novel-measure-box" aria-hidden="true" />
        </main>
      </div>

      <!-- Footer Navigation -->
      <footer class="novel-footer">
        <div class="footer-group">
          <el-button
            size="small"
            :disabled="!hasPrevChapter"
            :icon="ArrowLeft"
            @click="openChapter(chapters[currentChapterIndex - 1])"
          >
            上一章
          </el-button>
          <el-button
            size="small"
            :disabled="!hasNextChapter"
            @click="openChapter(chapters[currentChapterIndex + 1])"
          >
            下一章
            <el-icon class="el-icon--right"><ArrowRight /></el-icon>
          </el-button>
        </div>

        <div class="footer-group footer-page-nav">
          <el-button
            size="small"
            :disabled="currentPage <= 1 && !hasPrevChapter"
            @click="prevPage"
          >
            上一页
          </el-button>
          <span class="page-indicator tabular-nums">
            当前第 {{ currentPage }} 页 / 共 {{ totalPages }} 页
          </span>
          <el-button
            size="small"
            :disabled="currentPage >= totalPages && !hasNextChapter"
            @click="nextPage"
          >
            下一页
          </el-button>
        </div>

        <div class="footer-group">
          <span class="jump-label">跳至</span>
          <el-input-number
            v-model="jumpPageInput"
            size="small"
            :min="1"
            :max="totalPages"
            :controls="false"
            placeholder="页码"
            class="jump-input"
            @keyup.enter="handleJumpPage"
          />
          <el-button size="small" @click="handleJumpPage">跳转</el-button>
        </div>
      </footer>
    </div>
  </teleport>
</template>

<style scoped>
.novel-reader-overlay {
  position: fixed;
  inset: 0;
  z-index: 2100;
  display: flex;
  flex-direction: column;
  --nr-bg: #f4ecd8;
  --nr-surface: #fbf6ea;
  --nr-text: #3b3228;
  --nr-muted: #786b59;
  --nr-border: #e5d8bc;
  --nr-accent: #4f46e5;
  background: var(--nr-bg);
  color: var(--nr-text);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.novel-reader-overlay.theme-light {
  --nr-bg: #f1f5f9;
  --nr-surface: #ffffff;
  --nr-text: #1e293b;
  --nr-muted: #64748b;
  --nr-border: #e2e8f0;
}

.novel-reader-overlay.theme-dark {
  --nr-bg: #09090b;
  --nr-surface: #18181b;
  --nr-text: #e4e4e7;
  --nr-muted: #a1a1aa;
  --nr-border: #27272a;
}

.novel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 20px;
  background: var(--nr-surface);
  border-bottom: 1px solid var(--nr-border);
  flex-wrap: wrap;
}

.novel-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.novel-title {
  font-size: 15px;
  font-weight: 600;
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-restored-badge {
  font-size: 12px;
  color: var(--nr-accent);
  background: rgba(79, 70, 229, 0.1);
  padding: 2px 8px;
  border-radius: 999px;
}

.novel-header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.font-controls {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.font-size-label {
  font-size: 12.5px;
  color: var(--nr-muted);
  min-width: 34px;
  text-align: center;
}

.novel-body {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  justify-content: center;
  padding: 18px 20px;
}

.novel-chapter-drawer {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  width: 300px;
  z-index: 20;
  background: var(--nr-surface);
  border-right: 1px solid var(--nr-border);
  box-shadow: 4px 0 18px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-size: 14px;
  font-weight: 600;
  border-bottom: 1px solid var(--nr-border);
}

.drawer-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chapter-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  padding: 8px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--nr-text);
  font-size: 13px;
  cursor: pointer;
}

.chapter-item:hover {
  background: rgba(79, 70, 229, 0.08);
}

.chapter-item.is-active {
  background: rgba(79, 70, 229, 0.14);
  color: var(--nr-accent);
  font-weight: 600;
}

.chapter-idx {
  color: var(--nr-muted);
  flex-shrink: 0;
}

.chapter-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-page-stage {
  position: relative;
  width: 100%;
  max-width: 820px;
  height: 100%;
  min-height: 0;
}

.novel-page-card {
  width: 100%;
  height: 100%;
  padding: 32px 42px;
  background: var(--nr-surface);
  border: 1px solid var(--nr-border);
  border-radius: 12px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.06);
  overflow: hidden;
  box-sizing: border-box;
}

.novel-page-text,
.novel-measure-box {
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', 'Noto Serif SC', sans-serif;
  line-height: 1.85;
  white-space: pre-wrap;
  word-break: break-word;
  letter-spacing: 0.02em;
}

.novel-measure-box {
  position: fixed;
  top: -99999px;
  left: -99999px;
  visibility: hidden;
  pointer-events: none;
  height: auto;
  overflow: visible;
}

.novel-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 24px;
  background: var(--nr-surface);
  border-top: 1px solid var(--nr-border);
  flex-wrap: wrap;
}

.footer-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-indicator {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--nr-text);
  padding: 0 8px;
}

.jump-label {
  font-size: 12.5px;
  color: var(--nr-muted);
}

.jump-input {
  width: 72px;
}

@media (max-width: 768px) {
  .novel-page-card {
    padding: 20px 18px;
  }
}
</style>
