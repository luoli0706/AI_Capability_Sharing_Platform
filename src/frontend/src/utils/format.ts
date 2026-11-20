import dayjs from 'dayjs'
import type { AssetStatus, AssetType, AssetVisibility, PaginatedData } from '@/types/api'

export interface AssetTypeMeta {
  label: string
  shortLabel: string
  tagType: 'primary' | 'success' | 'warning' | 'info' | 'danger'
  color: string
  bgColor: string
}

const ASSET_TYPE_MAP: Record<AssetType, AssetTypeMeta> = {
  mcp: {
    label: 'MCP 服务',
    shortLabel: 'MCP',
    tagType: 'primary',
    color: '#4F46E5',
    bgColor: '#EEF2FF'
  },
  skill: {
    label: 'Skills 技能',
    shortLabel: 'Skill',
    tagType: 'success',
    color: '#059669',
    bgColor: '#ECFDF5'
  },
  agent: {
    label: 'Agent 实例',
    shortLabel: 'Agent',
    tagType: 'warning',
    color: '#D97706',
    bgColor: '#FFFBEB'
  },
  kb: {
    label: '私有知识库',
    shortLabel: '知识库',
    tagType: 'info',
    color: '#0284C7',
    bgColor: '#F0F9FF'
  }
}

export function getAssetTypeMeta(type: string): AssetTypeMeta {
  return (
    ASSET_TYPE_MAP[type as AssetType] ?? {
      label: type.toUpperCase(),
      shortLabel: type,
      tagType: 'info',
      color: '#475569',
      bgColor: '#F1F5F9'
    }
  )
}

export interface StatusMeta {
  label: string
  tagType: 'primary' | 'success' | 'warning' | 'info' | 'danger'
}

const ASSET_STATUS_MAP: Record<string, StatusMeta> = {
  published: { label: '已发布', tagType: 'success' },
  draft: { label: '草稿', tagType: 'info' },
  pending: { label: '审批中', tagType: 'warning' },
  deprecated: { label: '已下线', tagType: 'danger' },
  offline: { label: '已下线', tagType: 'danger' }
}

export function getAssetStatusMeta(status: AssetStatus | string): StatusMeta {
  return ASSET_STATUS_MAP[status] ?? { label: status, tagType: 'info' }
}

const VISIBILITY_MAP: Record<AssetVisibility, string> = {
  public: '全平台公开',
  domain: '业务域内可见',
  private: '仅自己可见'
}

export function getVisibilityLabel(visibility: string): string {
  return VISIBILITY_MAP[visibility as AssetVisibility] ?? visibility
}

const ROLE_LABEL_MAP: Record<string, string> = {
  super_admin: '平台超管',
  domain_admin: '域管理员',
  developer: '研发人员',
  owner: '域负责人',
  admin: '管理员',
  member: '成员'
}

export function getRoleLabel(role: string): string {
  return ROLE_LABEL_MAP[role] ?? role
}

export function formatDateTime(val?: string | null): string {
  if (!val) return '-'
  const d = dayjs(val)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : val
}

export function formatFileSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const idx = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, idx)).toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
}

export function getApprovalStatusMeta(status: string): StatusMeta {
  const map: Record<string, StatusMeta> = {
    pending: { label: '审批中', tagType: 'warning' },
    approved: { label: '已通过', tagType: 'success' },
    rejected: { label: '已驳回', tagType: 'danger' },
    cancelled: { label: '已取消', tagType: 'info' }
  }
  return map[status] ?? { label: status, tagType: 'info' }
}

export function getKbParseStatusMeta(status: string): StatusMeta {
  const map: Record<string, StatusMeta> = {
    pending: { label: '排队中', tagType: 'info' },
    processing: { label: '切片中', tagType: 'warning' },
    completed: { label: '已完成', tagType: 'success' },
    failed: { label: '解析失败', tagType: 'danger' }
  }
  return map[status] ?? { label: status, tagType: 'info' }
}

export function normalizeList<T>(raw: PaginatedData<T> | T[] | null | undefined): {
  items: T[]
  total: number
} {
  if (!raw) return { items: [], total: 0 }
  if (Array.isArray(raw)) {
    return { items: raw, total: raw.length }
  }
  const items = raw.items ?? raw.list ?? []
  return {
    items,
    total: typeof raw.total === 'number' ? raw.total : items.length
  }
}
