/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createSectionRegistry } from '@/features/system-settings/utils/section-registry'
import { AuditLogsTable } from './components/audit-logs-table'

const AUDIT_LOGS_SECTIONS = [
  {
    id: 'security',
    titleKey: 'Security Audit',
    build: () => <AuditLogsTable />,
  },
] as const

export type AuditLogsSectionId =
  (typeof AUDIT_LOGS_SECTIONS)[number]['id']

const auditLogsRegistry = createSectionRegistry<
  AuditLogsSectionId,
  Record<string, never>,
  []
>({
  sections: AUDIT_LOGS_SECTIONS,
  defaultSection: 'security',
  basePath: '/audit-logs',
  urlStyle: 'path',
})

export const AUDIT_LOGS_SECTION_IDS = auditLogsRegistry.sectionIds
export const AUDIT_LOGS_DEFAULT_SECTION = auditLogsRegistry.defaultSection

export function isAuditLogsSectionId(s: string): s is AuditLogsSectionId {
  return (AUDIT_LOGS_SECTION_IDS as readonly string[]).includes(s)
}
export const getAuditLogsSectionNavItems =
  auditLogsRegistry.getSectionNavItems
