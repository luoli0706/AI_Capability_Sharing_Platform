import { ref } from 'vue'
import { defineStore } from 'pinia'

export interface KbClipboardItem {
  kbId: number
  docId: number
  fileName: string
  isFolder: boolean
  refDomainId?: number
  domainName?: string
}

const KB_CLIPBOARD_KEY = 'ascp_kb_clipboard'

function readClipboardFromSession(): KbClipboardItem | null {
  try {
    const raw = sessionStorage.getItem(KB_CLIPBOARD_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<KbClipboardItem> | null
    if (
      !parsed ||
      typeof parsed.kbId !== 'number' ||
      typeof parsed.docId !== 'number' ||
      typeof parsed.fileName !== 'string' ||
      typeof parsed.isFolder !== 'boolean'
    ) {
      return null
    }
    return {
      kbId: parsed.kbId,
      docId: parsed.docId,
      fileName: parsed.fileName,
      isFolder: parsed.isFolder,
      refDomainId: typeof parsed.refDomainId === 'number' ? parsed.refDomainId : undefined,
      domainName: typeof parsed.domainName === 'string' ? parsed.domainName : undefined
    }
  } catch {
    return null
  }
}

export const useKbClipboardStore = defineStore('kbClipboard', () => {
  const item = ref<KbClipboardItem | null>(readClipboardFromSession())

  function setItem(next: KbClipboardItem) {
    item.value = { ...next }
    try {
      sessionStorage.setItem(KB_CLIPBOARD_KEY, JSON.stringify(item.value))
    } catch {
      // Ignore sessionStorage quota errors
    }
  }

  function clear() {
    item.value = null
    try {
      sessionStorage.removeItem(KB_CLIPBOARD_KEY)
    } catch {
      // Ignore sessionStorage errors
    }
  }

  return {
    item,
    setItem,
    clear
  }
})
