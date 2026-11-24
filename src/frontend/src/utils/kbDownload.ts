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
    /\.(txt|json|ya?ml|csv|xml|html?|css|js|ts|go|py|sql|sh|log|ini|conf)$/i.test(fileName)
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

function parseSingleExternalVideoText(raw: string, fileNameMode: boolean): ExternalVideoSource | null {
  const trimmed = raw.trim()
  if (!trimmed) return null
  const baseName = getBaseName(trimmed)
  const ext = getExtension(trimmed)

  // 1. Bilibili
  const biliUrlMatch = !fileNameMode
    ? trimmed.match(/bilibili\.com\/video\/(BV[0-9A-Za-z]{10})/i)
    : null
  const biliIdMatch = biliUrlMatch || baseName.match(/^(BV[0-9A-Za-z]{10})$/) || (!fileNameMode ? trimmed.match(/\b(BV[0-9A-Za-z]{10})\b/) : null)
  if (biliIdMatch) {
    const bvid = biliIdMatch[1]
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

  // 2. YouTube
  const ytUrlMatch = !fileNameMode
    ? trimmed.match(/(?:youtube\.com\/(?:watch\?(?:[^#\s]*&)?v=|shorts\/|embed\/)|youtu\.be\/)([A-Za-z0-9_-]{11})/i)
    : null
  const ytFileMatch =
    baseName.match(/^(?:yt|youtube)-([A-Za-z0-9_-]{11})$/i) ||
    (ext === '.youtube' ? baseName.match(/^([A-Za-z0-9_-]{11})$/) : null)
  const ytMatch = ytUrlMatch || ytFileMatch
  if (ytMatch) {
    const id = ytMatch[1]
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

  // 3. Niconico
  const nicoUrlMatch = !fileNameMode
    ? trimmed.match(/nicovideo\.jp\/watch\/((?:sm|nm|so)\d+)/i)
    : null
  const nicoFileMatch = baseName.match(/^((?:sm|nm|so)\d+)$/i)
  const nicoMatch = nicoUrlMatch || nicoFileMatch
  if (nicoMatch) {
    const id = nicoMatch[1].toLowerCase()
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

  // 4. Vimeo
  const vimeoUrlMatch = !fileNameMode
    ? trimmed.match(/(?:player\.)?vimeo\.com\/(?:video\/)?(\d+)/i)
    : null
  const vimeoFileMatch =
    baseName.match(/^vimeo-(\d+)$/i) || (ext === '.vimeo' ? baseName.match(/^(\d+)$/) : null)
  const vimeoMatch = vimeoUrlMatch || vimeoFileMatch
  if (vimeoMatch) {
    const id = vimeoMatch[1]
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

  // 5. Douyin
  const douyinUrlMatch = !fileNameMode
    ? trimmed.match(
        /(?:douyin\.com\/video\/(\d+)|iesdouyin\.com\/share\/video\/(\d+)|douyin\.com\/[^\s]*[?&]modal_id=(\d+))/i
      )
    : null
  const douyinFileMatch =
    baseName.match(/^douyin-(\d+)$/i) || (ext === '.douyin' ? baseName.match(/^(\d+)$/) : null)
  const douyinId = douyinUrlMatch
    ? douyinUrlMatch[1] || douyinUrlMatch[2] || douyinUrlMatch[3]
    : douyinFileMatch?.[1]
  if (douyinId) {
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

  // 6. Tencent Video
  const tencentUrlMatch = !fileNameMode
    ? trimmed.match(/v\.qq\.com\/x\/(?:page|cover\/[^/]+)\/([A-Za-z0-9]+)\.html/i)
    : null
  const tencentFileMatch =
    baseName.match(/^qq-([A-Za-z0-9]+)$/i) ||
    (ext === '.qq' ? baseName.match(/^([A-Za-z0-9]+)$/) : null)
  const tencentMatch = tencentUrlMatch || tencentFileMatch
  if (tencentMatch) {
    const id = tencentMatch[1]
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

  // 7. Youku
  const youkuUrlMatch = !fileNameMode
    ? trimmed.match(/v\.youku\.com\/v_show\/id_([A-Za-z0-9=]+)/i)
    : null
  const youkuFileMatch =
    baseName.match(/^youku-([A-Za-z0-9=]+)$/i) ||
    (ext === '.youku' ? baseName.match(/^([A-Za-z0-9=]+)$/) : null)
  const youkuMatch = youkuUrlMatch || youkuFileMatch
  if (youkuMatch) {
    const id = youkuMatch[1]
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
