import { useEffect } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  SettingsForm,
  SettingsSwitchItem,
  SettingsSwitchContent,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type PIITypeConfig = {
  enabled: boolean
  action: string
}

type PIICOnfigData = {
  enabled: boolean
  input_action: string
  output_action: string
  audit_log_enabled: boolean
  types: Record<string, PIITypeConfig>
}

const piiConfigSchema = z.object({
  enabled: z.boolean(),
  input_action: z.string(),
  output_action: z.string(),
  audit_log_enabled: z.boolean(),
})

type PIIConfigFormValues = z.infer<typeof piiConfigSchema>

type PIIConfigSectionProps = {
  defaultValues: PIICOnfigData
}

const PII_TYPES = [
  { key: 'phone', label: 'Phone Number', description: 'Chinese mobile numbers (11 digits)' },
  { key: 'idcard', label: 'ID Card', description: '18-digit Chinese national ID' },
  { key: 'bankcard', label: 'Bank Card', description: '16-19 digit bank card numbers' },
  { key: 'email', label: 'Email', description: 'Email addresses' },
  { key: 'ipv4', label: 'IPv4', description: 'IPv4 addresses' },
]

export function PIIConfigSection({ defaultValues }: PIIConfigSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<PIIConfigFormValues>({
    resolver: zodResolver(piiConfigSchema),
    defaultValues: {
      enabled: defaultValues.enabled,
      input_action: defaultValues.input_action,
      output_action: defaultValues.output_action,
      audit_log_enabled: defaultValues.audit_log_enabled,
    },
  })

  useEffect(() => {
    form.reset({
      enabled: defaultValues.enabled,
      input_action: defaultValues.input_action,
      output_action: defaultValues.output_action,
      audit_log_enabled: defaultValues.audit_log_enabled,
    })
  }, [defaultValues, form])

  const onSubmit = async (values: PIIConfigFormValues) => {
    // Merge form values with types config
    const config: PIICOnfigData = {
      ...values,
      types: defaultValues.types || {},
    }
    await updateOption.mutateAsync({
      key: 'PIIConfig',
      value: JSON.stringify(config),
    })
  }

  const handleTypeToggle = async (typeKey: string, enabled: boolean) => {
    const newTypes = { ...(defaultValues.types || {}) }
    if (!newTypes[typeKey]) {
      newTypes[typeKey] = { enabled: false, action: 'mask' }
    }
    newTypes[typeKey].enabled = enabled
    const config: PIICOnfigData = {
      enabled: form.getValues('enabled'),
      input_action: form.getValues('input_action'),
      output_action: form.getValues('output_action'),
      audit_log_enabled: form.getValues('audit_log_enabled'),
      types: newTypes,
    }
    await updateOption.mutateAsync({
      key: 'PIIConfig',
      value: JSON.stringify(config),
    })
  }

  const handleTypeActionChange = async (typeKey: string, action: string) => {
    const newTypes = { ...(defaultValues.types || {}) }
    if (!newTypes[typeKey]) {
      newTypes[typeKey] = { enabled: false, action: 'mask' }
    }
    newTypes[typeKey].action = action
    const config: PIICOnfigData = {
      enabled: form.getValues('enabled'),
      input_action: form.getValues('input_action'),
      output_action: form.getValues('output_action'),
      audit_log_enabled: form.getValues('audit_log_enabled'),
      types: newTypes,
    }
    await updateOption.mutateAsync({
      key: 'PIIConfig',
      value: JSON.stringify(config),
    })
  }

  return (
    <SettingsSection title={t('PII Detection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save PII config'
          />
          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable PII detection')}</FormLabel>
                    <FormDescription>
                      {t('Scan requests and responses for personally identifiable information.')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='input_action'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Input default action')}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='mask'>{t('Mask')} - {t('Replace PII with ***')}</SelectItem>
                      <SelectItem value='block'>{t('Block')} - {t('Reject request')}</SelectItem>
                      <SelectItem value='log'>{t('Log')} - {t('Record only')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Default action for PII found in user prompts')}
                  </FormDescription>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='output_action'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Output default action')}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='mask'>{t('Mask')} - {t('Replace PII with ***')}</SelectItem>
                      <SelectItem value='block'>{t('Block')} - {t('Block response')}</SelectItem>
                      <SelectItem value='log'>{t('Log')} - {t('Record only')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Default action for PII found in model responses')}
                  </FormDescription>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='audit_log_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Audit logging')}</FormLabel>
                    <FormDescription>
                      {t('Record PII detections in audit logs')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </div>

          <div className='mt-6 space-y-4'>
            <h4 className='text-sm font-medium'>{t('PII Types')}</h4>
            {PII_TYPES.map((piiType) => {
              const typeConfig = defaultValues.types?.[piiType.key]
              return (
                <div
                  key={piiType.key}
                  className='flex items-center justify-between rounded-lg border p-3'
                >
                  <div className='space-y-0.5'>
                    <FormLabel>{t(piiType.label)}</FormLabel>
                    <FormDescription className='text-xs'>
                      {t(piiType.description)}
                    </FormDescription>
                  </div>
                  <div className='flex items-center gap-3'>
                    <Select
                      value={typeConfig?.action || 'mask'}
                      onValueChange={(v) =>
                        v && handleTypeActionChange(piiType.key, v)
                      }
                    >
                      <SelectTrigger className='w-[100px]'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='mask'>{t('Mask')}</SelectItem>
                        <SelectItem value='block'>{t('Block')}</SelectItem>
                        <SelectItem value='log'>{t('Log')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <Switch
                      checked={typeConfig?.enabled ?? false}
                      onCheckedChange={(checked) =>
                        handleTypeToggle(piiType.key, checked)
                      }
                    />
                  </div>
                </div>
              )
            })}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
