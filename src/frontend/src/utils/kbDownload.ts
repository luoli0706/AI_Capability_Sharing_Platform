import { ElMessage, ElMessageBox } from 'element-plus'
import { downloadKbFiles, getKbDownloadPreview, type KbDownloadParams } from '@/api/kb'
import { formatFileSize } from '@/utils/format'
import type { KbDocument } from '@/types/api'

export const MAX_KB_FILE_SIZE = 10 * 1024 * 1024

export function isImageFile(fileName: string): boolean {
  return /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i.test(fileName)
}

export function isMarkdownFile(fileName: string): boolean {
  return /\.(md|markdown)$/i.test(fileName)
}

export function isTxtFile(fileName: string): boolean {
  return /\.txt$/i.test(fileName)
}

export function isTextFile(fileName: string): boolean {
  return (
    isMarkdownFile(fileName) ||
    /\.(txt|json|ya?ml|csv|xml|html?|css|js|ts|go|py|sql|sh|log|ini|conf|bili|douyin|youtube|niconico|vimeo|qq|youku|video)$/i.test(
      fileName
    )
  )
}

export type ExternalVideoPlatform =
  | 'bilibili'
  | 'youtube'
  | 'niconico'
  | 'vimeo'
  | 'douyin'
  | 'tencent'
  | 'youku'

export interface ExternalVideoSource {
  platform: ExternalVideoPlatform
  platformLabel: string
  videoId: string
  suggestedFileName: string
  embedUrl: string
  watchUrl: string
  requiresMagic: boolean
}

export function extractBilibiliBvid(fileName: string): string | null {
  const trimmed = fileName.trim()
  if (!trimmed) return null
  const lastDot = trimmed.lastIndexOf('.')
  const baseName = lastDot > 0 ? trimmed.slice(0, lastDot) : trimmed
  const match = baseName.match(/^(BV[0-9A-Za-z]{10})$/)
  return match ? match[1] : null
}

function getBaseName(raw: string): string {
  const trimmed = raw.trim()
  const lastDot = trimmed.lastIndexOf('.')
  return lastDot > 0 ? trimmed.slice(0, lastDot) : trimmed
}

function getExtension(raw: string): string {
  const trimmed = raw.trim()
  const lastDot = trimmed.lastIndexOf('.')
  return lastDot > 0 ? trimmed.slice(lastDot).toLowerCase() : ''
}

export function extractResolvableShortVideoUrl(input: string): string | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  const match = trimmed.match(/https?:\/\/(?:v\.douyin\.com|b23\.tv)\/[A-Za-z0-9_-]+\/?/i)
  return match ? match[0] : null
}

const EXTERNAL_VIDEO_EXTENSIONS = new Set([
  '.bili',
  '.douyin',
  '.youtube',
  '.niconico',
  '.vimeo',
  '.qq',
  '.youku',
  '.video'
])

function makeBilibiliSource(bvid: string): ExternalVideoSource {
  return {
    platform: 'bilibili',
    platformLabel: '哔哩哔哩',
    videoId: bvid,
    suggestedFileName: `${bvid}.bili`,
    embedUrl: `https://player.bilibili.com/player.html?bvid=${encodeURIComponent(bvid)}&page=1&as_wide=1&high_quality=1&danmaku=0`,
    watchUrl: `https://www.bilibili.com/video/${encodeURIComponent(bvid)}`,
    requiresMagic: false
  }
}

function makeYoutubeSource(id: string): ExternalVideoSource {
  return {
    platform: 'youtube',
    platformLabel: 'YouTube',
    videoId: id,
    suggestedFileName: `${id}.youtube`,
    embedUrl: `https://www.youtube.com/embed/${encodeURIComponent(id)}`,
    watchUrl: `https://www.youtube.com/watch?v=${encodeURIComponent(id)}`,
    requiresMagic: true
  }
}

function makeNiconicoSource(rawId: string): ExternalVideoSource {
  const id = rawId.toLowerCase()
  return {
    platform: 'niconico',
    platformLabel: 'Niconico',
    videoId: id,
    suggestedFileName: `${id}.niconico`,
    embedUrl: `https://embed.nicovideo.jp/watch/${encodeURIComponent(id)}`,
    watchUrl: `https://www.nicovideo.jp/watch/${encodeURIComponent(id)}`,
    requiresMagic: true
  }
}

function makeVimeoSource(id: string): ExternalVideoSource {
  return {
    platform: 'vimeo',
    platformLabel: 'Vimeo',
    videoId: id,
    suggestedFileName: `${id}.vimeo`,
    embedUrl: `https://player.vimeo.com/video/${encodeURIComponent(id)}`,
    watchUrl: `https://vimeo.com/${encodeURIComponent(id)}`,
    requiresMagic: true
  }
}

function makeDouyinSource(douyinId: string): ExternalVideoSource {
  return {
    platform: 'douyin',
    platformLabel: '抖音',
    videoId: douyinId,
    suggestedFileName: `${douyinId}.douyin`,
    embedUrl: `https://open.douyin.com/player/video?vid=${encodeURIComponent(douyinId)}&autoplay=0`,
    watchUrl: `https://www.douyin.com/video/${encodeURIComponent(douyinId)}`,
    requiresMagic: false
  }
}

function makeTencentSource(id: string): ExternalVideoSource {
  return {
    platform: 'tencent',
    platformLabel: '腾讯视频',
    videoId: id,
    suggestedFileName: `${id}.qq`,
    embedUrl: `https://v.qq.com/txp/iframe/player.html?vid=${encodeURIComponent(id)}`,
    watchUrl: `https://v.qq.com/x/page/${encodeURIComponent(id)}.html`,
    requiresMagic: false
  }
}

function makeYoukuSource(id: string): ExternalVideoSource {
  return {
    platform: 'youku',
    platformLabel: '优酷',
    videoId: id,
    suggestedFileName: `${id}.youku`,
    embedUrl: `https://player.youku.com/embed/${encodeURIComponent(id)}`,
    watchUrl: `https://v.youku.com/v_show/id_${encodeURIComponent(id)}.html`,
    requiresMagic: false
  }
}

function parseSingleExternalVideoText(raw: string, fileNameMode: boolean): ExternalVideoSource | null {
  const trimmed = raw.trim()
  if (!trimmed) return null
  const baseName = getBaseName(trimmed)
  const ext = getExtension(trimmed)

  if (fileNameMode) {
    if (!EXTERNAL_VIDEO_EXTENSIONS.has(ext)) {
      return null
    }

    if (ext === '.bili' || ext === '.video') {
      const m = baseName.match(/^(BV[0-9A-Za-z]{10})$/)
      if (m) return makeBilibiliSource(m[1])
    }

    if (ext === '.youtube' || ext === '.video') {
      const m =
        baseName.match(/^(?:yt|youtube)-([A-Za-z0-9_-]{11})$/i) ||
        (ext === '.youtube' ? baseName.match(/^([A-Za-z0-9_-]{11})$/) : null)
      if (m) return makeYoutubeSource(m[1])
    }

    if (ext === '.niconico' || ext === '.video') {
      const m = baseName.match(/^((?:sm|nm|so)\d+)$/i)
      if (m) return makeNiconicoSource(m[1])
    }

    if (ext === '.vimeo' || ext === '.video') {
      const m =
        baseName.match(/^vimeo-(\d+)$/i) ||
        (ext === '.vimeo' ? baseName.match(/^(\d+)$/) : null)
      if (m) return makeVimeoSource(m[1])
    }

    if (ext === '.douyin' || ext === '.video') {
      const m =
        baseName.match(/^douyin-(\d+)$/i) ||
        (ext === '.douyin' ? baseName.match(/^(\d+)$/) : null)
      if (m) return makeDouyinSource(m[1])
    }

    if (ext === '.qq' || ext === '.video') {
      const m =
        baseName.match(/^qq-([A-Za-z0-9]+)$/i) ||
        (ext === '.qq' ? baseName.match(/^([A-Za-z0-9]+)$/) : null)
      if (m) return makeTencentSource(m[1])
    }

    if (ext === '.youku' || ext === '.video') {
      const m =
        baseName.match(/^youku-([A-Za-z0-9=_-]+)$/i) ||
        (ext === '.youku' ? baseName.match(/^([A-Za-z0-9=_-]+)$/) : null)
      if (m) return makeYoukuSource(m[1])
    }

    return null
  }

  // 文本输入模式：先尝试所有平台的完整 URL 正则
  const biliUrlMatch = trimmed.match(/bilibili\.com\/video\/(BV[0-9A-Za-z]{10})/i)
  if (biliUrlMatch) return makeBilibiliSource(biliUrlMatch[1])

  const ytUrlMatch = trimmed.match(
    /(?:youtube\.com\/(?:watch\?(?:[^#\s]*&)?v=|shorts\/|embed\/|live\/)|youtu\.be\/)([A-Za-z0-9_-]{11})/i
  )
  if (ytUrlMatch) return makeYoutubeSource(ytUrlMatch[1])

  const nicoUrlMatch = trimmed.match(/nicovideo\.jp\/watch\/((?:sm|nm|so)\d+)/i)
  if (nicoUrlMatch) return makeNiconicoSource(nicoUrlMatch[1])

  const vimeoUrlMatch = trimmed.match(/(?:player\.)?vimeo\.com\/(?:video\/)?(\d+)/i)
  if (vimeoUrlMatch) return makeVimeoSource(vimeoUrlMatch[1])

  const douyinUrlMatch = trimmed.match(
    /(?:douyin\.com\/video\/(\d+)|iesdouyin\.com\/share\/video\/(\d+)|douyin\.com\/[^\s]*[?&]modal_id=(\d+))/i
  )
  if (douyinUrlMatch) {
    const douyinId = douyinUrlMatch[1] || douyinUrlMatch[2] || douyinUrlMatch[3]
    if (douyinId) return makeDouyinSource(douyinId)
  }

  const tencentUrlMatch = trimmed.match(
    /v\.qq\.com\/x\/(?:page|cover\/[^/]+)\/([A-Za-z0-9]+)\.html/i
  )
  if (tencentUrlMatch) return makeTencentSource(tencentUrlMatch[1])

  const youkuUrlMatch = trimmed.match(/v\.youku\.com\/v_show\/id_([A-Za-z0-9=_-]+)/i)
  if (youkuUrlMatch) return makeYoukuSource(youkuUrlMatch[1])

  // 兜底：纯 ID 或平台前缀 ID 匹配（避免 \bBV...\b 误伤其他平台 URL）
  const biliIdMatch = baseName.match(/^(BV[0-9A-Za-z]{10})$/) || trimmed.match(/\b(BV[0-9A-Za-z]{10})\b/)
  if (biliIdMatch) return makeBilibiliSource(biliIdMatch[1])

  const ytIdMatch = baseName.match(/^(?:yt|youtube)-([A-Za-z0-9_-]{11})$/i)
  if (ytIdMatch) return makeYoutubeSource(ytIdMatch[1])

  const nicoIdMatch = baseName.match(/^((?:sm|nm|so)\d+)$/i)
  if (nicoIdMatch) return makeNiconicoSource(nicoIdMatch[1])

  const vimeoIdMatch = baseName.match(/^vimeo-(\d+)$/i)
  if (vimeoIdMatch) return makeVimeoSource(vimeoIdMatch[1])

  const douyinIdMatch = baseName.match(/^douyin-(\d+)$/i)
  if (douyinIdMatch) return makeDouyinSource(douyinIdMatch[1])

  const tencentIdMatch = baseName.match(/^qq-([A-Za-z0-9]+)$/i)
  if (tencentIdMatch) return makeTencentSource(tencentIdMatch[1])

  const youkuIdMatch = baseName.match(/^youku-([A-Za-z0-9=_-]+)$/i)
  if (youkuIdMatch) return makeYoukuSource(youkuIdMatch[1])

  return null
}

export function parseExternalVideoSource(
  input: string,
  fileName?: string
): ExternalVideoSource | null {
  if (input && input.trim()) {
    const fromInput = parseSingleExternalVideoText(input, false)
    if (fromInput) return fromInput
  }
  if (fileName && fileName.trim()) {
    return parseSingleExternalVideoText(fileName, true)
  }
  return null
}

export function isExternalVideoFile(fileName: string): boolean {
  const trimmed = fileName.trim()
  if (!trimmed) return false
  if (/\.(bili|video|douyin|youtube|niconico|vimeo|qq|youku)$/i.test(trimmed)) return true
  return parseExternalVideoSource('', trimmed) !== null
}

const SCOPE_LABELS: Record<KbDownloadParams['scope'], string> = {
  file: '单个文件',
  folder: '文件夹（递归包含子目录与文件）',
  all: '整个知识库'
}

export async function confirmAndDownloadKb(
  kbId: number,
  params: KbDownloadParams,
  fallbackFilename: string,
  refDomainId?: number
): Promise<void> {
  const preview = await getKbDownloadPreview(kbId, params, refDomainId)
  if (!preview || preview.file_count === 0) {
    ElMessage.warning('所选范围内暂无可下载的文件')
    return
  }

  const scopeLabel = SCOPE_LABELS[preview.scope || params.scope] || params.scope
  const sizeText = formatFileSize(preview.total_bytes)

  try {
    await ElMessageBox.confirm(
      `下载范围：${scopeLabel}\n文件数量：共 ${preview.file_count} 个文件\n预定下载体积：${sizeText}`,
      '确认下载知识库文件',
      {
        confirmButtonText: '确认下载',
        cancelButtonText: '取消',
        type: 'info'
      }
    )
  } catch {
    return
  }

  const finalName =
    params.scope === 'file'
      ? fallbackFilename
      : fallbackFilename.endsWith('.zip')
        ? fallbackFilename
        : `${fallbackFilename}.zip`

  await downloadKbFiles(kbId, params, finalName, refDomainId)
  ElMessage.success('已开始下载')
}

export interface KbTreeNode {
  key: string
  label: string
  isFolder: boolean
  isLeaf?: boolean
  folderPath: string
  fileSize?: number
  doc?: KbDocument
  children: KbTreeNode[]
  isLoadMoreSentinel?: boolean
  sentinelFolderPath?: string
  sentinelNextOffset?: number
  sentinelRemaining?: number
  sentinelLoading?: boolean
}

export function createLoadMoreSentinelNode(
  folderPath: string,
  nextOffset: number,
  remaining: number
): KbTreeNode {
  const clean = folderPath.replace(/^\/+|\/+$/g, '')
  const remainingLabel = remaining > 0 ? `（还有 ${remaining} 项）` : ''
  return {
    key: `sentinel:${clean || '__root__'}:${nextOffset}`,
    label: `加载更多${remainingLabel}`,
    isFolder: false,
    isLeaf: true,
    folderPath: clean,
    children: [],
    isLoadMoreSentinel: true,
    sentinelFolderPath: clean,
    sentinelNextOffset: nextOffset,
    sentinelRemaining: remaining,
    sentinelLoading: false
  }
}

export function buildPaginatedKbTreeNodes(
  docs: KbDocument[],
  parentFolderPath: string,
  pagination?: { hasMore?: boolean; nextOffset?: number; total?: number }
): KbTreeNode[] {
  const nodes = buildImmediateKbTreeNodes(docs, parentFolderPath)
  if (pagination?.hasMore) {
    const nextOffset = pagination.nextOffset ?? docs.length
    const total = pagination.total ?? nextOffset
    const remaining = Math.max(0, total - docs.length)
    nodes.push(createLoadMoreSentinelNode(parentFolderPath, nextOffset, remaining))
  }
  return nodes
}

export function extractImmediateFiles(docs: KbDocument[], folderPath: string): KbDocument[] {
  const cleanParent = folderPath.replace(/^\/+|\/+$/g, '')
  return docs
    .filter(
      (d) => !d.is_folder && (d.folder_path || '').replace(/^\/+|\/+$/g, '') === cleanParent
    )
    .sort((a, b) => a.file_name.localeCompare(b.file_name))
}

export function buildImmediateKbTreeNodes(
  docs: KbDocument[],
  parentFolderPath: string
): KbTreeNode[] {
  const cleanParent = parentFolderPath.replace(/^\/+|\/+$/g, '')
  const prefix = cleanParent ? `${cleanParent}/` : ''
  const folderMap = new Map<string, KbTreeNode>()
  const fileNodes: KbTreeNode[] = []

  const ensureImmediateFolder = (immediatePath: string, label: string): KbTreeNode => {
    const existing = folderMap.get(immediatePath)
    if (existing) return existing
    const node: KbTreeNode = {
      key: `folder:${immediatePath}`,
      label,
      isFolder: true,
      isLeaf: false,
      folderPath: immediatePath,
      children: []
    }
    folderMap.set(immediatePath, node)
    return node
  }

  for (const doc of docs) {
    if (!doc.is_folder) continue
    const clean = (doc.folder_path || doc.file_name || '').replace(/^\/+|\/+$/g, '')
    if (!clean) continue
    if (cleanParent) {
      if (!clean.startsWith(prefix)) continue
      const rest = clean.slice(prefix.length)
      if (!rest) continue
      const seg = rest.split('/')[0]
      const immediatePath = `${cleanParent}/${seg}`
      const node = ensureImmediateFolder(immediatePath, seg)
      if (clean === immediatePath) {
        node.doc = doc
      }
    } else {
      const seg = clean.split('/')[0]
      const node = ensureImmediateFolder(seg, seg)
      if (clean === seg) {
        node.doc = doc
      }
    }
  }

  for (const doc of docs) {
    if (doc.is_folder) continue
    const docFolder = (doc.folder_path || '').replace(/^\/+|\/+$/g, '')
    if (docFolder === cleanParent) {
      fileNodes.push({
        key: `file:${doc.id}`,
        label: doc.file_name,
        isFolder: false,
        isLeaf: true,
        folderPath: docFolder,
        fileSize: doc.file_size,
        doc,
        children: []
      })
    } else if (!cleanParent && docFolder) {
      const seg = docFolder.split('/')[0]
      ensureImmediateFolder(seg, seg)
    } else if (cleanParent && docFolder.startsWith(prefix)) {
      const rest = docFolder.slice(prefix.length)
      if (rest) {
        const seg = rest.split('/')[0]
        ensureImmediateFolder(`${cleanParent}/${seg}`, seg)
      }
    }
  }

  const result = [...folderMap.values(), ...fileNodes]
  result.sort((a, b) => {
    if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1
    return a.label.localeCompare(b.label)
  })
  return result
}

export function buildKbTree(docs: KbDocument[]): KbTreeNode[] {
  const rootNodes: KbTreeNode[] = []
  const folderMap = new Map<string, KbTreeNode>()

  const ensureFolder = (rawPath: string): KbTreeNode => {
    const cleanPath = rawPath.replace(/^\/+|\/+$/g, '')
    if (folderMap.has(cleanPath)) {
      return folderMap.get(cleanPath)!
    }
    const parts = cleanPath.split('/').filter(Boolean)
    const label = parts[parts.length - 1] || cleanPath
    const node: KbTreeNode = {
      key: `folder:${cleanPath}`,
      label,
      isFolder: true,
      folderPath: cleanPath,
      children: []
    }
    folderMap.set(cleanPath, node)

    if (parts.length > 1) {
      const parentPath = parts.slice(0, -1).join('/')
      const parent = ensureFolder(parentPath)
      parent.children.push(node)
    } else {
      rootNodes.push(node)
    }
    return node
  }

  // First pass: explicit folder records
  for (const doc of docs) {
    if (doc.is_folder) {
      const clean = (doc.folder_path || doc.file_name || '').replace(/^\/+|\/+$/g, '')
      if (clean) {
        const folderNode = ensureFolder(clean)
        folderNode.doc = doc
      }
    }
  }

  // Second pass: files
  for (const doc of docs) {
    if (doc.is_folder) continue
    const folderClean = (doc.folder_path || '').replace(/^\/+|\/+$/g, '')
    const fileNode: KbTreeNode = {
      key: `file:${doc.id}`,
      label: doc.file_name,
      isFolder: false,
      folderPath: folderClean,
      fileSize: doc.file_size,
      doc,
      children: []
    }
    if (folderClean) {
      const parent = ensureFolder(folderClean)
      parent.children.push(fileNode)
    } else {
      rootNodes.push(fileNode)
    }
  }

  const sortNodes = (nodes: KbTreeNode[]) => {
    nodes.sort((a, b) => {
      if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1
      return a.label.localeCompare(b.label)
    })
    for (const n of nodes) {
      if (n.children.length > 0) sortNodes(n.children)
    }
  }

  sortNodes(rootNodes)
  return rootNodes
}
