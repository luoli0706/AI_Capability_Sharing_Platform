<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Check,
  Delete,
  Download,
  FolderOpened,
  Key,
  Lock,
  Refresh,
  Unlock,
  UploadFilled
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import {
  decryptSevenO,
  downloadSevenORelease,
  encryptSevenO,
  getSevenOReleases,
  inspectSevenO,
  parseBlobErrorMessage,
  triggerBlobDownload
} from '@/api/tools'
import type { SevenOInspectResult, SevenOReleaseItem } from '@/types/api'
import { extractFilesFromDrop, getFileRelativePath, isFileDragEvent } from '@/utils/dropFiles'
import { formatDateTime, formatFileSize } from '@/utils/format'

const releasesLoading = ref(false)
const releases = ref<SevenOReleaseItem[]>([])
const downloadingFile = ref<string>('')

// Encrypt state
const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)
const selectedFiles = ref<File[]>([])
const encryptKey = ref('7thcv')
const encryptLevel = ref<1 | 3 | 9>(3)
const encrypting = ref(false)
const encryptError = ref('')
const encryptDragDepth = ref(0)

// Decrypt state
const decryptInputRef = ref<HTMLInputElement | null>(null)
const decryptFile = ref<File | null>(null)
const decryptKey = ref('7thcv')
const inspecting = ref(false)
const inspectResult = ref<SevenOInspectResult | null>(null)
const inspectError = ref('')
const decrypting = ref(false)
const decryptError = ref('')
const decryptDragDepth = ref(0)

const LEVEL_OPTIONS: Array<{ value: 1 | 3 | 9; label: string; desc: string }> = [
  { value: 1, label: '极速 (Level 1)', desc: '压缩速度最快，适合大体积临时归档' },
  { value: 3, label: '均衡 (Level 3, 默认)', desc: '速度与压缩率兼顾，日常推荐' },
  { value: 9, label: '极限 (Level 9)', desc: '更高压缩率，适合离线分发存档' }
]

const totalSelectedSize = computed(() =>
  selectedFiles.value.reduce((acc, f) => acc + (f.size || 0), 0)
)

const previewFileNames = computed(() =>
  selectedFiles.value.slice(0, 5).map((f) => getFileRelativePath(f))
)

const isWindowsExe = (item: SevenOReleaseItem): boolean =>
  item.file_name.toLowerCase().endsWith('.exe') ||
  item.platform.toLowerCase().includes('windows')

const getPlatformLabel = (item: SevenOReleaseItem): string => {
  if (isWindowsExe(item)) return 'Windows GUI 客户端'
  if (item.platform.toLowerCase().includes('linux') || item.kind.toLowerCase().includes('server')) {
    return 'Linux 服务端引擎'
  }
  return item.platform || item.kind || '客户端发行版'
}

const isSevenOPackage = (kind: string): boolean => {
  const lower = (kind || '').toLowerCase()
  return (
    lower.includes('7thcv') ||
    lower.includes('7o') ||
    lower.includes('obf') ||
    lower === 'encrypted'
  )
}

async function fetchReleases() {
  releasesLoading.value = true
  try {
    const data = await getSevenOReleases()
    releases.value = Array.isArray(data) ? data : []
  } catch {
    releases.value = []
  } finally {
    releasesLoading.value = false
  }
}

async function handleDownloadRelease(item: SevenOReleaseItem) {
  downloadingFile.value = item.file_name
  try {
    const blob = await downloadSevenORelease(item.file_name)
    triggerBlobDownload(blob, item.file_name)
    ElMessage.success(`已开始下载 ${item.file_name}`)
  } catch (err) {
    const msg = await parseBlobErrorMessage(err, `下载 ${item.file_name} 失败`)
    ElMessage.error(msg)
  } finally {
    downloadingFile.value = ''
  }
}

function triggerFilePicker() {
  fileInputRef.value?.click()
}

function triggerFolderPicker() {
  folderInputRef.value?.click()
}

function handleEncryptFilesChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  if (files.length > 0) {
    selectedFiles.value = files
    encryptError.value = ''
  }
  input.value = ''
}

function clearEncryptFiles() {
  selectedFiles.value = []
  encryptError.value = ''
}

function handleEncryptDragEnter(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  encryptDragDepth.value++
}

function handleEncryptDragOver(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

function handleEncryptDragLeave(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  encryptDragDepth.value = Math.max(0, encryptDragDepth.value - 1)
}

async function handleEncryptDrop(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  encryptDragDepth.value = 0
  const files = await extractFilesFromDrop(e)
  if (files.length > 0) {
    selectedFiles.value = files
    encryptError.value = ''
  }
}

async function handleEncryptSubmit() {
  encryptError.value = ''
  if (selectedFiles.value.length === 0) {
    ElMessage.warning('请先选择需要加密压缩的文件或文件夹')
    return
  }
  const key = encryptKey.value.trim()
  if (!key) {
    ElMessage.warning('请输入混淆密钥')
    return
  }

  encrypting.value = true
  try {
    const formData = new FormData()
    for (const file of selectedFiles.value) {
      const rel = getFileRelativePath(file)
      formData.append('files', file, rel)
    }
    formData.append('key', key)
    formData.append('level', String(encryptLevel.value))

    const blob = await encryptSevenO(formData)
    triggerBlobDownload(blob, 'ascp_7o_package.zip')
    ElMessage.success('加密压缩完成，已触发下载 ascp_7o_package.zip')
  } catch (err) {
    const msg = await parseBlobErrorMessage(err, '加密压缩失败，请稍后重试')
    encryptError.value = msg
  } finally {
    encrypting.value = false
  }
}

function triggerDecryptPicker() {
  decryptInputRef.value?.click()
}

async function processDecryptFile(file: File) {
  decryptFile.value = file
  inspectResult.value = null
  inspectError.value = ''
  decryptError.value = ''

  inspecting.value = true
  try {
    inspectResult.value = await inspectSevenO(file)
  } catch (err) {
    const msg = err instanceof Error ? err.message : '无法识别压缩包格式'
    inspectError.value = msg
  } finally {
    inspecting.value = false
  }
}

async function handleDecryptFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  input.value = ''
  if (!file) return
  await processDecryptFile(file)
}

function handleDecryptDragEnter(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  decryptDragDepth.value++
}

function handleDecryptDragOver(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

function handleDecryptDragLeave(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  decryptDragDepth.value = Math.max(0, decryptDragDepth.value - 1)
}

async function handleDecryptDrop(e: DragEvent) {
  if (!isFileDragEvent(e)) return
  e.preventDefault()
  decryptDragDepth.value = 0
  const files = await extractFilesFromDrop(e)
  const file = files[0]
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    ElMessage.warning('请拖入 .zip 格式的压缩包文件')
    return
  }
  await processDecryptFile(file)
}

function clearDecryptFile() {
  decryptFile.value = null
  inspectResult.value = null
  inspectError.value = ''
  decryptError.value = ''
}

async function handleDecryptSubmit() {
  decryptError.value = ''
  if (!decryptFile.value) {
    ElMessage.warning('请先选择需要解密解压的 .zip 文件')
    return
  }
  const key = decryptKey.value.trim()
  if (!key) {
    ElMessage.warning('请输入解密密钥')
    return
  }

  decrypting.value = true
  try {
    const formData = new FormData()
    formData.append('file', decryptFile.value)
    formData.append('key', key)

    const blob = await decryptSevenO(formData)
    const baseName = decryptFile.value.name.replace(/\.zip$/i, '') || 'ascp_7o'
    const downloadName = `${baseName}_extracted.zip`
    triggerBlobDownload(blob, downloadName)
    ElMessage.success(`解密解压完成，已触发下载 ${downloadName}`)
  } catch (err) {
    const msg = await parseBlobErrorMessage(err, '解密失败：请检查密钥是否正确或压缩包是否损坏')
    decryptError.value = msg
  } finally {
    decrypting.value = false
  }
}

onMounted(() => {
  fetchReleases()
})
</script>

<template>
  <div class="page-container seveno-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">7o 加密工具</h1>
        <p class="page-subtitle">
          7thCV Obfuscation (7o) 离线桌面分发与在线加解密工作台 · 基于
          <span class="mono-text">tar + zstd + 64-bit 密钥流混淆</span>
        </p>
      </div>
      <el-button :icon="Refresh" :loading="releasesLoading" @click="fetchReleases">
        刷新发行版
      </el-button>
    </div>

    <!-- Top Section: 二进制客户端分发与工具发布 -->
    <section class="surface-card release-section" aria-label="二进制客户端分发与工具发布">
      <div class="section-head">
        <div class="section-title-wrap">
          <div class="section-icon">
            <el-icon :size="18"><Download /></el-icon>
          </div>
          <div>
            <h2 class="section-title">二进制客户端分发与工具发布</h2>
            <p class="section-desc">
              支持大文件与完整目录本地高速处理：采用 <span class="mono-text">tar</span> 归档、<span
                class="mono-text"
                >zstd</span
              >
              高压缩比编码与 <span class="mono-text">64-bit</span> 密钥流混淆，适合内网安全传输与离线归档。
            </p>
          </div>
        </div>
      </div>

      <div v-loading="releasesLoading" class="release-list">
        <el-empty
          v-if="!releasesLoading && releases.length === 0"
          description="暂无可用的客户端发行版"
          :image-size="68"
        />

        <div
          v-for="item in releases"
          :key="item.file_name"
          class="release-item"
          :class="{ 'is-featured': isWindowsExe(item) }"
        >
          <div class="release-main">
            <div class="release-name-row">
              <span class="release-filename mono-text">{{ item.file_name }}</span>
              <el-tag
                size="small"
                :type="isWindowsExe(item) ? 'primary' : 'info'"
                effect="light"
              >
                {{ getPlatformLabel(item) }}
              </el-tag>
              <el-tag v-if="isWindowsExe(item)" size="small" type="success" effect="plain">
                桌面端推荐
              </el-tag>
            </div>

            <div class="release-meta tabular-nums">
              <span>体积：{{ formatFileSize(item.size) }}</span>
              <span class="meta-dot">·</span>
              <span class="mono-text" :title="item.sha256">
                SHA-256：{{ item.sha256 ? `${item.sha256.slice(0, 12)}...` : '-' }}
              </span>
              <template v-if="item.updated_at">
                <span class="meta-dot">·</span>
                <span>更新：{{ formatDateTime(item.updated_at) }}</span>
              </template>
            </div>
          </div>

          <div class="release-actions">
            <el-button
              :type="isWindowsExe(item) ? 'primary' : 'default'"
              :plain="!isWindowsExe(item)"
              :icon="Download"
              :loading="downloadingFile === item.file_name"
              @click="handleDownloadRelease(item)"
            >
              下载客户端
            </el-button>
          </div>
        </div>
      </div>
    </section>

    <!-- Main Section: 在线压缩加密 / 在线解压解密 -->
    <section class="tools-grid" aria-label="在线加密压缩与解密解压">
      <!-- Left Card: 在线加密压缩 -->
      <div class="surface-card tool-card">
        <div class="tool-card-header">
          <div class="tool-badge is-encrypt">
            <el-icon :size="18"><Lock /></el-icon>
          </div>
          <div>
            <h2 class="tool-title">在线加密压缩</h2>
            <p class="tool-subtitle">选择本地文件或目录，打包为 7o 混淆加密 ZIP 包</p>
          </div>
        </div>

        <div class="tool-form">
          <div
            class="form-block drop-zone"
            :class="{ 'is-dragover': encryptDragDepth > 0 }"
            @dragenter="handleEncryptDragEnter"
            @dragover="handleEncryptDragOver"
            @dragleave="handleEncryptDragLeave"
            @drop="handleEncryptDrop"
          >
            <label class="block-label">1. 选择文件或文件夹</label>
            <input
              ref="fileInputRef"
              type="file"
              multiple
              class="hidden-input"
              @change="handleEncryptFilesChange"
            />
            <input
              ref="folderInputRef"
              type="file"
              webkitdirectory
              directory
              multiple
              class="hidden-input"
              @change="handleEncryptFilesChange"
            />

            <div class="picker-actions">
              <el-button :icon="UploadFilled" @click="triggerFilePicker">
                选择文件（支持多选）
              </el-button>
              <el-button :icon="FolderOpened" @click="triggerFolderPicker">
                选择文件夹
              </el-button>
              <el-button
                v-if="selectedFiles.length > 0"
                link
                type="danger"
                :icon="Delete"
                @click="clearEncryptFiles"
              >
                清空
              </el-button>
            </div>

            <div v-if="selectedFiles.length > 0" class="selected-summary">
              <div class="summary-stats tabular-nums">
                <span>
                  已选择 <strong>{{ selectedFiles.length }}</strong> 个文件
                </span>
                <span>总体积 {{ formatFileSize(totalSelectedSize) }}</span>
              </div>
              <ul class="summary-list">
                <li v-for="(name, idx) in previewFileNames" :key="idx" class="mono-text">
                  {{ name }}
                </li>
                <li v-if="selectedFiles.length > 5" class="more-hint">
                  … 其余 {{ selectedFiles.length - 5 }} 个文件
                </li>
              </ul>
            </div>
            <div v-else class="empty-picker-hint">
              支持点击选择或直接从桌面拖入单个/多个文件或完整文件夹
            </div>
          </div>

          <div class="form-block">
            <label class="block-label">2. 混淆密钥</label>
            <el-input
              v-model="encryptKey"
              type="password"
              show-password
              clearable
              placeholder="请输入混淆密钥"
              :prefix-icon="Key"
            />
            <p class="field-hint">默认密钥为 7thcv，可自定义修改</p>
          </div>

          <div class="form-block">
            <label class="block-label">3. 压缩等级</label>
            <el-select v-model="encryptLevel" class="full-width">
              <el-option
                v-for="opt in LEVEL_OPTIONS"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              >
                <div class="level-option">
                  <span>{{ opt.label }}</span>
                  <span class="level-desc">{{ opt.desc }}</span>
                </div>
              </el-option>
            </el-select>
          </div>

          <el-alert
            v-if="encryptError"
            :title="encryptError"
            type="error"
            show-icon
            :closable="false"
          />

          <div class="submit-row">
            <el-button
              type="primary"
              size="large"
              class="full-width"
              :icon="Lock"
              :loading="encrypting"
              :disabled="selectedFiles.length === 0"
              @click="handleEncryptSubmit"
            >
              开始加密压缩
            </el-button>
          </div>
        </div>
      </div>

      <!-- Right Card: 在线解密解压 -->
      <div class="surface-card tool-card">
        <div class="tool-card-header">
          <div class="tool-badge is-decrypt">
            <el-icon :size="18"><Unlock /></el-icon>
          </div>
          <div>
            <h2 class="tool-title">在线解密解压</h2>
            <p class="tool-subtitle">自动检测压缩包格式并还原为标准 ZIP 下载</p>
          </div>
        </div>

        <div class="tool-form">
          <div
            class="form-block drop-zone"
            :class="{ 'is-dragover': decryptDragDepth > 0 }"
            @dragenter="handleDecryptDragEnter"
            @dragover="handleDecryptDragOver"
            @dragleave="handleDecryptDragLeave"
            @drop="handleDecryptDrop"
          >
            <label class="block-label">1. 选择 .zip 压缩包</label>
            <input
              ref="decryptInputRef"
              type="file"
              accept=".zip"
              class="hidden-input"
              @change="handleDecryptFileChange"
            />

            <div class="picker-actions">
              <el-button :icon="UploadFilled" @click="triggerDecryptPicker">
                选择 .zip 文件
              </el-button>
              <el-button
                v-if="decryptFile"
                link
                type="danger"
                :icon="Delete"
                @click="clearDecryptFile"
              >
                移除
              </el-button>
            </div>

            <div v-if="decryptFile" class="selected-summary">
              <div class="summary-stats tabular-nums">
                <span class="mono-text file-name-ellipsis" :title="decryptFile.name">
                  {{ decryptFile.name }}
                </span>
                <span>{{ formatFileSize(decryptFile.size) }}</span>
              </div>

              <div v-if="inspecting" class="inspect-box is-loading">
                正在检测压缩包格式...
              </div>
              <div v-else-if="inspectResult" class="inspect-box">
                <el-tag
                  size="small"
                  :type="isSevenOPackage(inspectResult.kind) ? 'success' : 'info'"
                  effect="dark"
                >
                  <el-icon v-if="isSevenOPackage(inspectResult.kind)" style="margin-right: 4px">
                    <Check />
                  </el-icon>
                  {{ isSevenOPackage(inspectResult.kind) ? '7thCV 加密包' : '普通标准 ZIP' }}
                </el-tag>
                <span class="inspect-detail">{{ inspectResult.detail }}</span>
              </div>
              <div v-else-if="inspectError" class="inspect-box is-error">
                格式检测失败：{{ inspectError }}
              </div>
            </div>
            <div v-else class="empty-picker-hint">
              支持点击选择或直接从桌面拖入 .zip 压缩包（自动识别是否为 7thCV 加密包）
            </div>
          </div>

          <div class="form-block">
            <label class="block-label">2. 解密密钥</label>
            <el-input
              v-model="decryptKey"
              type="password"
              show-password
              clearable
              placeholder="请输入解密密钥"
              :prefix-icon="Key"
            />
            <p class="field-hint">默认密钥为 7thcv；若压缩时使用了自定义密钥，请保持一致</p>
          </div>

          <el-alert
            v-if="decryptError"
            :title="decryptError"
            type="error"
            show-icon
            :closable="false"
          />

          <div class="submit-row submit-row-spacer">
            <el-button
              type="primary"
              size="large"
              class="full-width"
              :icon="Unlock"
              :loading="decrypting"
              :disabled="!decryptFile"
              @click="handleDecryptSubmit"
            >
              开始解密解压
            </el-button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.seveno-page {
  padding-bottom: 24px;
}

.release-section {
  padding: 20px 22px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-title-wrap {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.section-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.section-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.section-desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--ascp-text-secondary);
}

.release-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 72px;
}

.release-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  background: #f8fafc;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.release-item.is-featured {
  background: #f5f7ff;
  border-color: #c7d2fe;
}

.release-item:hover {
  border-color: var(--ascp-border-hover);
}

.release-main {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.release-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.release-filename {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  word-break: break-all;
}

.release-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.meta-dot {
  color: var(--ascp-border-hover);
}

.release-actions {
  flex-shrink: 0;
}

.tools-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  align-items: stretch;
}

.tool-card {
  padding: 20px 22px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.tool-card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--ascp-border);
}

.tool-badge {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.tool-badge.is-encrypt {
  background: #eef2ff;
  color: #4f46e5;
}

.tool-badge.is-decrypt {
  background: #ecfdf5;
  color: #059669;
}

.tool-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.tool-subtitle {
  margin: 2px 0 0;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.tool-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.form-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.block-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.hidden-input {
  display: none;
}

.picker-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.selected-summary {
  padding: 10px 12px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-stats {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: var(--ascp-text-primary);
}

.file-name-ellipsis {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.summary-list {
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
  color: var(--ascp-text-secondary);
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 110px;
  overflow-y: auto;
}

.summary-list li {
  word-break: break-all;
}

.more-hint {
  list-style: none;
  margin-left: -16px;
  color: var(--ascp-text-muted);
}

.empty-picker-hint {
  padding: 12px;
  border-radius: 8px;
  border: 1px dashed var(--ascp-border-hover);
  background: #f8fafc;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
  text-align: center;
}

.drop-zone {
  padding: 10px;
  margin: -10px;
  border-radius: 10px;
  border: 1.5px dashed transparent;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.drop-zone.is-dragover {
  border-color: var(--ascp-primary);
  background: rgba(79, 70, 229, 0.05);
}

.drop-zone.is-dragover .empty-picker-hint,
.drop-zone.is-dragover .selected-summary {
  border-color: var(--ascp-primary);
  background: rgba(79, 70, 229, 0.04);
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.inspect-box {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12.5px;
}

.inspect-box.is-loading {
  color: var(--ascp-text-muted);
}

.inspect-box.is-error {
  color: #dc2626;
}

.inspect-detail {
  color: var(--ascp-text-secondary);
  word-break: break-word;
}

.level-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.level-desc {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.full-width {
  width: 100%;
}

.submit-row {
  margin-top: auto;
  padding-top: 4px;
}

@media (max-width: 992px) {
  .tools-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .release-section,
  .tool-card {
    padding: 16px;
  }

  .release-item {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .release-actions {
    width: 100%;
  }

  .release-actions .el-button {
    width: 100%;
  }

  .picker-actions .el-button {
    flex: 1;
    min-width: 130px;
    margin-left: 0;
  }
}
</style>
