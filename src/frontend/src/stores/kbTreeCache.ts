import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getDocuments } from '@/api/kb'
import type { KbDocument, KbDocumentListResponse, KbUserSummary } from '@/types/api'

export const KB_TREE_PAGE_SIZE = 50
export const KB_TREE_CACHE_TTL_MS = 60_000

export interface KbFolderCacheEntry {
  items: KbDocument[]
  users: Record<number, KbUserSummary>
  total: number
  hasMore: boolean
  nextOffset: number
  fetchedAt: number
}

export interface KbExtGroupCacheEntry {
  items: KbDocument[]
  users: Record<number, KbUserSummary>
  fetchedAt: number
}

export function normalizeKbFolderPath(folderPath?: string): string {
  return (folderPath || '').replace(/^\/+|\/+$/g, '')
}

function normalizeDocResponse(raw: KbDocumentListResponse | KbDocument[] | null | undefined): {
  list: KbDocument[]
  users: Record<number, KbUserSummary>
  total: number
  hasMore: boolean
  nextOffset: number
} {
  if (!raw) {
    return { list: [], users: {}, total: 0, hasMore: false, nextOffset: 0 }
  }
  if (Array.isArray(raw)) {
    return {
      list: raw,
      users: {},
      total: raw.length,
      hasMore: false,
      nextOffset: raw.length
    }
  }
  const list = raw.list ?? raw.items ?? []
  const users = raw.users ?? {}
  const total = typeof raw.total === 'number' ? raw.total : list.length
  const hasMore = Boolean(raw.has_more)
  const nextOffset = typeof raw.next_offset === 'number' ? raw.next_offset : list.length
  return { list, users, total, hasMore, nextOffset }
}

function mergeDocLists(existing: KbDocument[], incoming: KbDocument[]): KbDocument[] {
  if (existing.length === 0) return incoming
  const seen = new Set(existing.map((d) => d.id))
  const merged = [...existing]
  for (const doc of incoming) {
    if (!seen.has(doc.id)) {
      seen.add(doc.id)
      merged.push(doc)
    }
  }
  return merged
}

export const useKbTreeCacheStore = defineStore('kbTreeCache', () => {
  const folderMap = ref<Record<string, KbFolderCacheEntry>>({})
  const extGroupMap = ref<Record<string, KbExtGroupCacheEntry>>({})
  const usersByKb = ref<Record<number, Record<number, KbUserSummary>>>({})

  function makeFolderKey(kbId: number, folderPath: string, refDomainId?: number): string {
    return `${kbId}:${refDomainId || 0}:${normalizeKbFolderPath(folderPath)}`
  }

  function makeExtGroupKey(
    kbId: number,
    folderPath: string,
    extGroup: 'image' | 'txt',
    refDomainId?: number
  ): string {
    return `${kbId}:${refDomainId || 0}:${normalizeKbFolderPath(folderPath)}:${extGroup}`
  }

  function isFresh(fetchedAt: number): boolean {
    return Date.now() - fetchedAt < KB_TREE_CACHE_TTL_MS
  }

  function mergeUsers(kbId: number, users?: Record<number, KbUserSummary>) {
    if (!users || Object.keys(users).length === 0) return
    const prev = usersByKb.value[kbId] ?? {}
    usersByKb.value = {
      ...usersByKb.value,
      [kbId]: { ...prev, ...users }
    }
  }

  function resolveUser(kbId: number, userId?: number): KbUserSummary | undefined {
    if (!kbId || !userId) return undefined
    return usersByKb.value[kbId]?.[userId]
  }

  function getFolderEntry(
    kbId: number,
    folderPath: string,
    refDomainId?: number
  ): KbFolderCacheEntry | null {
    const key = makeFolderKey(kbId, folderPath, refDomainId)
    const entry = folderMap.value[key]
    if (!entry || !isFresh(entry.fetchedAt)) return null
    return entry
  }

  async function fetchFolderPage(
    kbId: number,
    folderPath: string,
    options?: {
      offset?: number
      limit?: number
      refDomainId?: number
      force?: boolean
    }
  ): Promise<KbFolderCacheEntry> {
    const cleanPath = normalizeKbFolderPath(folderPath)
    const offset = options?.offset ?? 0
    const limit = options?.limit ?? KB_TREE_PAGE_SIZE
    const refDomainId = options?.refDomainId
    const force = options?.force ?? false
    const key = makeFolderKey(kbId, cleanPath, refDomainId)

    if (offset === 0 && !force) {
      const cached = getFolderEntry(kbId, cleanPath, refDomainId)
      if (cached) return cached
    }

    const res = await getDocuments(
      kbId,
      {
        lazy: true,
        folder_path: cleanPath,
        offset,
        limit
      },
      refDomainId
    )
    const normalized = normalizeDocResponse(res)
    mergeUsers(kbId, normalized.users)

    const prev = offset > 0 ? folderMap.value[key] : undefined
    const items = prev ? mergeDocLists(prev.items, normalized.list) : normalized.list
    const users = prev ? { ...prev.users, ...normalized.users } : normalized.users
    const entry: KbFolderCacheEntry = {
      items,
      users,
      total: Math.max(normalized.total, items.length),
      hasMore: normalized.hasMore,
      nextOffset: normalized.nextOffset,
      fetchedAt: Date.now()
    }

    folderMap.value = {
      ...folderMap.value,
      [key]: entry
    }
    return entry
  }

  async function fetchExtGroupIndex(
    kbId: number,
    folderPath: string,
    extGroup: 'image' | 'txt',
    options?: { refDomainId?: number; force?: boolean }
  ): Promise<KbDocument[]> {
    const cleanPath = normalizeKbFolderPath(folderPath)
    const refDomainId = options?.refDomainId
    const force = options?.force ?? false
    const key = makeExtGroupKey(kbId, cleanPath, extGroup, refDomainId)

    const cached = extGroupMap.value[key]
    if (!force && cached && isFresh(cached.fetchedAt)) {
      return cached.items
    }

    const res = await getDocuments(
      kbId,
      {
        lazy: true,
        folder_path: cleanPath,
        fields: 'minimal',
        ext_group: extGroup
      },
      refDomainId
    )
    const normalized = normalizeDocResponse(res)
    mergeUsers(kbId, normalized.users)

    extGroupMap.value = {
      ...extGroupMap.value,
      [key]: {
        items: normalized.list,
        users: normalized.users,
        fetchedAt: Date.now()
      }
    }
    return normalized.list
  }

  function invalidateFolder(kbId: number, folderPath: string, refDomainId?: number) {
    const cleanPath = normalizeKbFolderPath(folderPath)
    const nextFolders = { ...folderMap.value }
    const nextExt = { ...extGroupMap.value }

    if (refDomainId !== undefined) {
      delete nextFolders[makeFolderKey(kbId, cleanPath, refDomainId)]
      delete nextExt[makeExtGroupKey(kbId, cleanPath, 'image', refDomainId)]
      delete nextExt[makeExtGroupKey(kbId, cleanPath, 'txt', refDomainId)]
    } else {
      const prefix = `${kbId}:`
      const suffix = `:${cleanPath}`
      for (const k of Object.keys(nextFolders)) {
        if (k.startsWith(prefix) && k.endsWith(suffix)) {
          delete nextFolders[k]
        }
      }
      for (const k of Object.keys(nextExt)) {
        if (
          k.startsWith(prefix) &&
          (k.endsWith(`${suffix}:image`) || k.endsWith(`${suffix}:txt`))
        ) {
          delete nextExt[k]
        }
      }
    }

    folderMap.value = nextFolders
    extGroupMap.value = nextExt
  }

  function invalidateFolderPrefix(kbId: number, folderPath: string, refDomainId?: number) {
    const cleanPath = normalizeKbFolderPath(folderPath)
    if (!cleanPath) {
      invalidateKb(kbId)
      return
    }
    const nextFolders: Record<string, KbFolderCacheEntry> = {}
    const nextExt: Record<string, KbExtGroupCacheEntry> = {}
    const kbPrefix = `${kbId}:`
    const refPrefix = refDomainId !== undefined ? `${kbId}:${refDomainId}:` : null

    const matchesFolder = (folderPart: string) =>
      folderPart === cleanPath || folderPart.startsWith(`${cleanPath}/`)

    for (const [k, v] of Object.entries(folderMap.value)) {
      if (refPrefix ? k.startsWith(refPrefix) : k.startsWith(kbPrefix)) {
        const parts = k.split(':')
        const folderPart = parts.slice(2).join(':')
        if (matchesFolder(folderPart)) continue
      }
      nextFolders[k] = v
    }

    for (const [k, v] of Object.entries(extGroupMap.value)) {
      if (refPrefix ? k.startsWith(refPrefix) : k.startsWith(kbPrefix)) {
        const withoutExt = k.replace(/:(image|txt)$/, '')
        const parts = withoutExt.split(':')
        const folderPart = parts.slice(2).join(':')
        if (matchesFolder(folderPart)) continue
      }
      nextExt[k] = v
    }

    folderMap.value = nextFolders
    extGroupMap.value = nextExt
  }

  function invalidateKb(kbId: number) {
    const prefix = `${kbId}:`
    const nextFolders: Record<string, KbFolderCacheEntry> = {}
    for (const [k, v] of Object.entries(folderMap.value)) {
      if (!k.startsWith(prefix)) nextFolders[k] = v
    }
    const nextExt: Record<string, KbExtGroupCacheEntry> = {}
    for (const [k, v] of Object.entries(extGroupMap.value)) {
      if (!k.startsWith(prefix)) nextExt[k] = v
    }
    folderMap.value = nextFolders
    extGroupMap.value = nextExt
  }

  return {
    folderMap,
    extGroupMap,
    usersByKb,
    getFolderEntry,
    fetchFolderPage,
    fetchExtGroupIndex,
    invalidateFolder,
    invalidateFolderPrefix,
    invalidateKb,
    resolveUser
  }
})
