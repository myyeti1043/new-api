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

/**
 * Audit log detail stored in the Other field (JSON format)
 */
export interface AuditLogDetail {
  direction: 'input' | 'output'
  sensitive_words: string[]
  action: 'blocked' | 'replaced' | 'logged'
  content_preview: string
  rule_level: 'block' | 'warn' | 'log'
  category: string
}

/**
 * Audit log item from the API
 */
export interface AuditLog {
  id: number
  user_id: number
  created_at: number
  type: number
  content: string
  username: string
  token_name: string
  model_name: string
  group: string
  request_id: string
  other: string // JSON string of AuditLogDetail
}

/**
 * Parsed audit log with detail
 */
export interface ParsedAuditLog extends AuditLog {
  detail?: AuditLogDetail
}

/**
 * Query parameters for audit log API
 */
export interface GetAuditLogsParams {
  p?: number
  page_size?: number
  start_timestamp?: number
  end_timestamp?: number
  username?: string
  direction?: string
  level?: string
  group?: string
}

/**
 * API response for audit log queries
 */
export interface GetAuditLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: AuditLog[]
    total: number
    page: number
    page_size: number
  }
}
