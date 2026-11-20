import { apiGet } from './request'
import type { AuditLogItem, AuditLogQuery, PaginatedData } from '@/types/api'

export function getAuditLogs(
  params?: AuditLogQuery
): Promise<PaginatedData<AuditLogItem> | AuditLogItem[]> {
  return apiGet<PaginatedData<AuditLogItem> | AuditLogItem[]>('/audit-logs', { params })
}
