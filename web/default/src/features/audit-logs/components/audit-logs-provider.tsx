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
import { createContext, useContext, useState, type ReactNode } from 'react'

interface AuditLogsContextValue {
  sensitiveVisible: boolean
  setSensitiveVisible: (v: boolean) => void
}

const AuditLogsContext = createContext<AuditLogsContextValue | null>(null)

export function AuditLogsProvider({ children }: { children: ReactNode }) {
  const [sensitiveVisible, setSensitiveVisible] = useState(false)

  return (
    <AuditLogsContext.Provider
      value={{ sensitiveVisible, setSensitiveVisible }}
    >
      {children}
    </AuditLogsContext.Provider>
  )
}

export function useAuditLogsContext() {
  const ctx = useContext(AuditLogsContext)
  if (!ctx) {
    throw new Error(
      'useAuditLogsContext must be used within AuditLogsProvider'
    )
  }
  return ctx
}
