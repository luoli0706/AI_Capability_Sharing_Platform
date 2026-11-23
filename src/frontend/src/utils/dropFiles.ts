export interface FileWithRelativePath extends File {
  _relativePath?: string
  webkitRelativePath: string
}

interface DropEntry {
  isFile: boolean
  isDirectory: boolean
  name: string
  file?: (
    successCallback: (file: File) => void,
    errorCallback?: (error: DOMException) => void
  ) => void
  createReader?: () => {
    readEntries: (
      successCallback: (entries: DropEntry[]) => void,
      errorCallback?: (error: DOMException) => void
    ) => void
  }
}

export function getFileRelativePath(file: File): string {
  const f = file as Partial<FileWithRelativePath>
  return f._relativePath || f.webkitRelativePath || file.name
}

export function isFileDragEvent(e: DragEvent): boolean {
  const types = e.dataTransfer?.types
  if (!types) return false
  return Array.from(types).includes('Files')
}

function attachRelativePath(file: File, relPath: string): File {
  ;(file as FileWithRelativePath)._relativePath = relPath
  return file
}

function readEntryAsFile(entry: DropEntry, parentPath: string): Promise<File | null> {
  return new Promise((resolve) => {
    if (!entry.file) {
      resolve(null)
      return
    }
    entry.file(
      (file) => {
        const relPath = parentPath ? `${parentPath}/${file.name}` : file.name
        resolve(attachRelativePath(file, relPath))
      },
      () => resolve(null)
    )
  })
}

function readAllDirectoryEntries(entry: DropEntry): Promise<DropEntry[]> {
  return new Promise((resolve) => {
    if (!entry.createReader) {
      resolve([])
      return
    }
    const reader = entry.createReader()
    const allEntries: DropEntry[] = []

    const readBatch = () => {
      reader.readEntries(
        (batch) => {
          if (!batch || batch.length === 0) {
            resolve(allEntries)
            return
          }
          allEntries.push(...batch)
          readBatch()
        },
        () => resolve(allEntries)
      )
    }

    readBatch()
  })
}

async function traverseEntry(entry: DropEntry, parentPath = ''): Promise<File[]> {
  if (entry.isFile) {
    const file = await readEntryAsFile(entry, parentPath)
    return file ? [file] : []
  }

  if (entry.isDirectory) {
    const nextParent = parentPath ? `${parentPath}/${entry.name}` : entry.name
    const children = await readAllDirectoryEntries(entry)
    const nested = await Promise.all(children.map((child) => traverseEntry(child, nextParent)))
    return nested.flat()
  }

  return []
}

/**
 * 从 DragEvent 中递归解析拖入的文件与文件夹，保留相对路径到 (file as any)._relativePath
 */
export async function extractFilesFromDrop(e: DragEvent): Promise<File[]> {
  const dt = e.dataTransfer
  if (!dt) return []

  const items = dt.items ? Array.from(dt.items) : []
  // 必须在任何 await 之前同步收集 webkitGetAsEntry、dt.files 与 fallback File
  const dtFilesSnapshot = dt.files ? Array.from(dt.files) : []
  const entries: DropEntry[] = []
  const fallbackFiles: File[] = []

  for (const item of items) {
    if (item.kind !== 'file') continue
    const getter = (item as DataTransferItem & { webkitGetAsEntry?: () => DropEntry | null })
      .webkitGetAsEntry
    const entry = typeof getter === 'function' ? getter.call(item) : null
    if (entry) {
      entries.push(entry)
    } else {
      const f = item.getAsFile()
      if (f) fallbackFiles.push(attachRelativePath(f, f.name))
    }
  }

  if (entries.length > 0) {
    const traversed = await Promise.all(entries.map((entry) => traverseEntry(entry, '')))
    const entryFiles = [...traversed.flat(), ...fallbackFiles]
    if (entryFiles.length > 0) {
      return entryFiles
    }
  }

  if (dtFilesSnapshot.length > 0) {
    return dtFilesSnapshot.map((f) =>
      attachRelativePath(f, (f as Partial<FileWithRelativePath>).webkitRelativePath || f.name)
    )
  }

  return fallbackFiles
}
