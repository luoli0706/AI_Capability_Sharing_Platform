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

export function isTextFile(fileName: string): boolean {
  return (
    isMarkdownFile(fileName) ||
    /\.(txt|json|ya?ml|csv|xml|html?|css|js|ts|go|py|sql|sh|log|ini|conf)$/i.test(fileName)
  )
}

const SCOPE_LABELS: Record<KbDownloadParams['scope'], string> = {
  file: '单个文件',
  folder: '文件夹（递归包含子目录与文件）',
  all: '整个知识库'
}

export async function confirmAndDownloadKb(
  kbId: number,
  params: KbDownloadParams,
  fallbackFilename: string
): Promise<void> {
  const preview = await getKbDownloadPreview(kbId, params)
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

  await downloadKbFiles(kbId, params, finalName)
  ElMessage.success('已开始下载')
}

export interface KbTreeNode {
  key: string
  label: string
  isFolder: boolean
  folderPath: string
  fileSize?: number
  doc?: KbDocument
  children: KbTreeNode[]
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
