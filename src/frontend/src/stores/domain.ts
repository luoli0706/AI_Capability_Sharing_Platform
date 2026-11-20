import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getDomains } from '@/api/domain'
import type { DomainItem } from '@/types/api'
import { normalizeList } from '@/utils/format'

const LAST_DOMAIN_KEY = 'ascp_last_domain_id'

function readLastDomainId(): number | null {
  const raw = localStorage.getItem(LAST_DOMAIN_KEY)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

export const useDomainStore = defineStore('domain', () => {
  const domains = ref<DomainItem[]>([])
  const loading = ref(false)
  const lastDomainId = ref<number | null>(readLastDomainId())

  const lastDomain = computed(
    () => domains.value.find((d) => d.id === lastDomainId.value) ?? null
  )

  function setLastDomainId(id: number) {
    lastDomainId.value = id
    localStorage.setItem(LAST_DOMAIN_KEY, String(id))
  }

  function getDomainById(id: number): DomainItem | null {
    return domains.value.find((d) => d.id === id) ?? null
  }

  async function fetchDomains(force = false) {
    if (!force && domains.value.length > 0) {
      return domains.value
    }
    loading.value = true
    try {
      const res = await getDomains({ tab: 'mine', page: 1, page_size: 100 })
      domains.value = normalizeList(res).items
      return domains.value
    } catch {
      domains.value = []
      return []
    } finally {
      loading.value = false
    }
  }

  return {
    domains,
    loading,
    lastDomainId,
    lastDomain,
    setLastDomainId,
    getDomainById,
    fetchDomains
  }
})
