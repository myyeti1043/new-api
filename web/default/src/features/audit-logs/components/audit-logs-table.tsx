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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import {
  useReactTable,
  getCoreRowModel,
  type ColumnDef,
} from '@tanstack/react-table'
import { DataTablePage } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { getAuditLogs } from '../api'
import type { AuditLog, AuditLogDetail, ParsedAuditLog } from '../types'
import { useAuditLogsContext } from './audit-logs-provider'

function parseAuditDetail(log: AuditLog): ParsedAuditLog {
  try {
    const detail = JSON.parse(log.other) as AuditLogDetail
    return { ...log, detail }
  } catch {
    return { ...log }
  }
}

function LevelBadge({ level }: { level?: string }) {
  const variant =
    level === 'block'
      ? 'destructive'
      : level === 'warn'
        ? 'warning'
        : 'secondary'
  return <Badge variant={variant}>{level || 'unknown'}</Badge>
}

function DirectionBadge({ direction }: { direction?: string }) {
  return (
    <Badge variant={direction === 'input' ? 'default' : 'outline'}>
      {direction === 'input' ? '🔽 Input' : '🔼 Output'}
    </Badge>
  )
}

function DetailDialog({
  log,
  open,
  onOpenChange,
}: {
  log: ParsedAuditLog | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const { sensitiveVisible } = useAuditLogsContext()

  if (!log) return null
  const d = log.detail

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Audit Log Detail')}</DialogTitle>
        </DialogHeader>
        <div className='space-y-3 text-sm'>
          <div className='grid grid-cols-2 gap-2'>
            <span className='text-muted-foreground'>ID</span>
            <span>{log.id}</span>
            <span className='text-muted-foreground'>{t('Time')}</span>
            <span>
              {log.created_at
                ? new Date(log.created_at * 1000).toLocaleString()
                : '-'}
            </span>
            <span className='text-muted-foreground'>{t('User')}</span>
            <span>
              {log.username}
              {log.group ? ` (${log.group})` : ''}
            </span>
            <span className='text-muted-foreground'>{t('Model')}</span>
            <span>{log.model_name || '-'}</span>
            <span className='text-muted-foreground'>{t('Request ID')}</span>
            <span className='break-all font-mono text-xs'>
              {log.request_id || '-'}
            </span>
          </div>

          {d && (
            <>
              <div className='grid grid-cols-2 gap-2'>
                <span className='text-muted-foreground'>{t('Direction')}</span>
                <DirectionBadge direction={d.direction} />
                <span className='text-muted-foreground'>{t('Level')}</span>
                <LevelBadge level={d.rule_level} />
                <span className='text-muted-foreground'>{t('Action')}</span>
                <span>{d.action}</span>
                <span className='text-muted-foreground'>{t('Category')}</span>
                <span>{d.category || '-'}</span>
              </div>

              {d.sensitive_words?.length > 0 && (
                <div>
                  <span className='text-muted-foreground'>
                    {t('Matched Words')}
                  </span>
                  <div className='mt-1 flex flex-wrap gap-1'>
                    {d.sensitive_words.map((w) => (
                      <Badge key={w} variant='outline'>
                        {sensitiveVisible ? w : '***'}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}

              {d.content_preview && (
                <div>
                  <span className='text-muted-foreground'>
                    {t('Content Preview')}
                  </span>
                  <pre className='mt-1 max-h-40 overflow-auto rounded bg-muted p-2 text-xs'>
                    {sensitiveVisible ? d.content_preview : '***'}
                  </pre>
                </div>
              )}
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

export function AuditLogsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [selectedLog, setSelectedLog] = useState<ParsedAuditLog | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  // TODO: add filter state and UI
  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['audit-logs', page, pageSize],
    queryFn: () => getAuditLogs({ p: page, page_size: pageSize }),
    refetchInterval: 30000,
  })

  const parsedLogs = useMemo(
    () => (data?.data?.items || []).map(parseAuditDetail),
    [data?.data?.items]
  )

  const columns = useMemo<ColumnDef<ParsedAuditLog>[]>(
    () => [
      {
        accessorKey: 'created_at',
        header: t('Time'),
        cell: ({ row }) => {
          const ts = row.original.created_at
          return ts ? new Date(ts * 1000).toLocaleString() : '-'
        },
      },
      {
        accessorKey: 'username',
        header: t('User'),
      },
      {
        id: 'direction',
        header: t('Direction'),
        cell: ({ row }) => (
          <DirectionBadge direction={row.original.detail?.direction} />
        ),
      },
      {
        id: 'level',
        header: t('Level'),
        cell: ({ row }) => (
          <LevelBadge level={row.original.detail?.rule_level} />
        ),
      },
      {
        id: 'action',
        header: t('Action'),
        cell: ({ row }) => row.original.detail?.action || '-',
      },
      {
        accessorKey: 'model_name',
        header: t('Model'),
      },
      {
        id: 'preview',
        header: t('Content Preview'),
        cell: ({ row }) => {
          const preview = row.original.detail?.content_preview
          if (!preview) return '-'
          return (
            <span className='max-w-[200px] truncate'>{preview}</span>
          )
        },
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <Button
            variant='ghost'
            size='sm'
            onClick={() => {
              setSelectedLog(row.original)
              setDetailOpen(true)
            }}
          >
            {t('Detail')}
          </Button>
        ),
      },
    ],
    [t]
  )

  const table = useReactTable({
    data: parsedLogs,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: data?.data
      ? Math.ceil(data.data.total / pageSize)
      : -1,
    state: {
      pagination: { pageIndex: page - 1, pageSize },
    },
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function'
          ? updater({ pageIndex: page - 1, pageSize })
          : updater
      setPage(next.pageIndex + 1)
    },
  })

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No audit logs')}
      />
      <DetailDialog
        log={selectedLog}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />
    </>
  )
}
