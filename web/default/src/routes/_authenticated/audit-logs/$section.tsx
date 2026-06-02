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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuditLogs } from '@/features/audit-logs'
import {
  isAuditLogsSectionId,
  AUDIT_LOGS_DEFAULT_SECTION,
} from '@/features/audit-logs/section-registry'

const auditLogsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  direction: z.string().optional().catch(''),
  level: z.string().optional().catch(''),
  username: z.string().optional().catch(''),
  group: z.string().optional().catch(''),
  startTime: z.number().optional(),
  endTime: z.number().optional(),
})

export const Route = createFileRoute('/_authenticated/audit-logs/$section')({
  beforeLoad: ({ params }) => {
    if (!isAuditLogsSectionId(params.section)) {
      throw redirect({
        to: '/audit-logs/$section',
        params: { section: AUDIT_LOGS_DEFAULT_SECTION },
      })
    }
  },
  validateSearch: auditLogsSearchSchema,
  component: AuditLogs,
})
