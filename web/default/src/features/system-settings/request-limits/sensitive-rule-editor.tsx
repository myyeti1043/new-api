import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsSection } from '../components/settings-section'

type SensitiveRule = {
  word: string
  level: string
  category: string
  group: string
}

type SensitiveRuleFormData = {
  word: string
  level: string
  category: string
  group: string
}

const defaultFormData: SensitiveRuleFormData = {
  word: '',
  level: 'block',
  category: '',
  group: '',
}

export function SensitiveRuleEditor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<SensitiveRule | null>(null)
  const [formData, setFormData] =
    useState<SensitiveRuleFormData>(defaultFormData)

  // Fetch rules
  const { data: rules, isLoading } = useQuery<SensitiveRule[]>({
    queryKey: ['sensitive-rules'],
    queryFn: async () => {
      const res = await fetch('/api/sensitive/rules')
      const json = await res.json()
      if (!json.success) throw new Error(json.message)
      return json.data || []
    },
  })

  // Create mutation
  const createMutation = useMutation({
    mutationFn: async (rule: SensitiveRuleFormData) => {
      const res = await fetch('/api/sensitive/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(rule),
      })
      const json = await res.json()
      if (!json.success) throw new Error(json.message)
      return json
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sensitive-rules'] })
      toast.success(t('Rule created'))
      setIsDialogOpen(false)
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: async ({
      oldRule,
      data,
    }: {
      oldRule: SensitiveRule
      data: SensitiveRuleFormData
    }) => {
      const params = new URLSearchParams({
        word: oldRule.word,
        group: oldRule.group || '',
      })
      const res = await fetch(`/api/sensitive/rules?${params}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      const json = await res.json()
      if (!json.success) throw new Error(json.message)
      return json
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sensitive-rules'] })
      toast.success(t('Rule updated'))
      setIsDialogOpen(false)
      setEditingRule(null)
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: async (rule: SensitiveRule) => {
      const params = new URLSearchParams({
        word: rule.word,
        group: rule.group || '',
      })
      const res = await fetch(`/api/sensitive/rules?${params}`, {
        method: 'DELETE',
      })
      const json = await res.json()
      if (!json.success) throw new Error(json.message)
      return json
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sensitive-rules'] })
      toast.success(t('Rule deleted'))
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  // Import mutation
  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/sensitive/rules/import', {
        method: 'POST',
        body: formData,
      })
      const json = await res.json()
      if (!json.success) throw new Error(json.message)
      return json
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['sensitive-rules'] })
      toast.success(t(`Imported {{count}} rules`, { count: data.data.imported }))
    },
    onError: (err: Error) => {
      toast.error(err.message)
    },
  })

  const handleCreate = () => {
    setEditingRule(null)
    setFormData(defaultFormData)
    setIsDialogOpen(true)
  }

  const handleEdit = (rule: SensitiveRule) => {
    setEditingRule(rule)
    setFormData({
      word: rule.word,
      level: rule.level,
      category: rule.category,
      group: rule.group,
    })
    setIsDialogOpen(true)
  }

  const handleDelete = (rule: SensitiveRule) => {
    if (window.confirm(t('Delete this rule?'))) {
      deleteMutation.mutate(rule)
    }
  }

  const handleSubmit = () => {
    if (!formData.word.trim()) {
      toast.error(t('Word is required'))
      return
    }
    if (editingRule) {
      updateMutation.mutate({ oldRule: editingRule, data: formData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const handleImport = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.csv,.txt'
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (file) {
        importMutation.mutate(file)
      }
    }
    input.click()
  }

  const handleExport = () => {
    window.open('/api/sensitive/rules/export', '_blank')
  }

  const levelLabel = (level: string) => {
    switch (level) {
      case 'block':
        return t('Block')
      case 'warn':
        return t('Warn')
      case 'log':
        return t('Log')
      default:
        return level
    }
  }

  const levelColor = (level: string) => {
    switch (level) {
      case 'block':
        return 'text-red-600'
      case 'warn':
        return 'text-yellow-600'
      case 'log':
        return 'text-blue-600'
      default:
        return ''
    }
  }

  return (
    <SettingsSection title={t('Sensitive Rules')}>
      <div className='space-y-4'>
        <div className='flex items-center gap-2'>
          <Button onClick={handleCreate} size='sm'>
            {t('Add Rule')}
          </Button>
          <Button onClick={handleImport} variant='outline' size='sm'>
            {t('Import')}
          </Button>
          <Button onClick={handleExport} variant='outline' size='sm'>
            {t('Export CSV')}
          </Button>
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Word')}</TableHead>
                <TableHead>{t('Level')}</TableHead>
                <TableHead>{t('Category')}</TableHead>
                <TableHead>{t('Group')}</TableHead>
                <TableHead className='w-[100px]'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5} className='text-center'>
                    {t('Loading...')}
                  </TableCell>
                </TableRow>
              ) : !rules || rules.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className='text-center'>
                    {t('No rules defined')}
                  </TableCell>
                </TableRow>
              ) : (
                rules.map((rule, idx) => (
                  <TableRow key={`${rule.word}-${rule.group}-${idx}`}>
                    <TableCell className='font-mono'>{rule.word}</TableCell>
                    <TableCell>
                      <span className={levelColor(rule.level)}>
                        {levelLabel(rule.level)}
                      </span>
                    </TableCell>
                    <TableCell>{rule.category || '-'}</TableCell>
                    <TableCell>{rule.group || '-'}</TableCell>
                    <TableCell>
                      <div className='flex gap-1'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => handleEdit(rule)}
                        >
                          {t('Edit')}
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          className='text-red-600'
                          onClick={() => handleDelete(rule)}
                        >
                          {t('Delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {editingRule ? t('Edit Rule') : t('Add Rule')}
              </DialogTitle>
              <DialogDescription>
                {t('Define a sensitive word rule with level and category')}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4 py-4'>
              <div className='space-y-2'>
                <label className='text-sm font-medium'>{t('Word')}</label>
                <Input
                  value={formData.word}
                  onChange={(e) =>
                    setFormData({ ...formData, word: e.target.value })
                  }
                  placeholder={t('Enter sensitive word')}
                  disabled={!!editingRule}
                />
              </div>
              <div className='space-y-2'>
                <label className='text-sm font-medium'>{t('Level')}</label>
                <Select
                  value={formData.level}
                  onValueChange={(v) =>
                    setFormData({ ...formData, level: v })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='block'>
                      {t('Block')} - {t('Reject request')}
                    </SelectItem>
                    <SelectItem value='warn'>
                      {t('Warn')} - {t('Log and allow')}
                    </SelectItem>
                    <SelectItem value='log'>
                      {t('Log')} - {t('Silent log only')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <label className='text-sm font-medium'>
                  {t('Category')} ({t('optional')})
                </label>
                <Input
                  value={formData.category}
                  onChange={(e) =>
                    setFormData({ ...formData, category: e.target.value })
                  }
                  placeholder={t('e.g., profanity, violence')}
                />
              </div>
              <div className='space-y-2'>
                <label className='text-sm font-medium'>
                  {t('Group')} ({t('optional')})
                </label>
                <Input
                  value={formData.group}
                  onChange={(e) =>
                    setFormData({ ...formData, group: e.target.value })
                  }
                  placeholder={t('Leave empty for all groups')}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant='outline' onClick={() => setIsDialogOpen(false)}>
                {t('Cancel')}
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={createMutation.isPending || updateMutation.isPending}
              >
                {editingRule ? t('Update') : t('Create')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </SettingsSection>
  )
}
