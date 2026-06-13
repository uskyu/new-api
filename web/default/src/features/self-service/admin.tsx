import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  History,
  ListChecks,
  Plus,
  Save,
  Settings2,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import {
  formatNumber,
  formatQuota,
  formatTimestampToDate,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import {
  getSelfServiceAdminConfig,
  getSelfServiceClaimAttempts,
  getSelfServiceRefundHistories,
  getSelfServiceUpgradeHistories,
  updateSelfServiceConfig,
  updateSelfServiceRules,
} from './api'
import type {
  SelfServiceClaimAttempt,
  SelfServiceConfig,
  SelfServiceHistory,
  SelfServiceUpgradeHistory,
  SelfServiceUpgradeRule,
} from './types'

type EditableRule = Partial<SelfServiceUpgradeRule> & {
  threshold_amount?: number
}

const emptyConfig: SelfServiceConfig = {
  enabled: true,
  lookback_hours: 24,
  max_refunds_per_claim: 10,
  daily_refund_limit: 10,
  display_limit: 10,
  refund_percent: 100,
  exclude_models: 'gpt-image,dall-e',
  scan_limit: 200,
}

function NumberField(props: {
  label: string
  value: number
  min?: number
  max?: number
  onChange: (value: number) => void
}) {
  return (
    <div className='space-y-2'>
      <Label>{props.label}</Label>
      <Input
        type='number'
        min={props.min ?? 0}
        max={props.max}
        value={props.value}
        onChange={(event) => props.onChange(Number(event.target.value))}
      />
    </div>
  )
}

function AdminStats(props: { stats: Record<string, number> }) {
  const { t } = useTranslation()
  const values = [
    [t('Today Refunds'), formatNumber(props.stats.today_refunds || 0)],
    [t('Today Refunded Quota'), formatQuota(props.stats.today_refund_quota || 0)],
    [t('Total Refunds'), formatNumber(props.stats.total_refunds || 0)],
    [t('Total Upgrades'), formatNumber(props.stats.total_upgrades || 0)],
  ]
  return (
    <div className='grid gap-3 md:grid-cols-4'>
      {values.map(([label, value]) => (
        <div key={label} className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>{label}</div>
          <div className='mt-1 text-lg font-medium'>{value}</div>
        </div>
      ))}
    </div>
  )
}

function ConfigPanel(props: {
  config: SelfServiceConfig
  onSave: (config: SelfServiceConfig) => void
  saving: boolean
}) {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SelfServiceConfig>(props.config)

  useEffect(() => {
    setConfig(props.config)
  }, [props.config])

  const patch = (next: Partial<SelfServiceConfig>) =>
    setConfig((current) => ({ ...current, ...next }))

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Settings2 className='size-4' />
          {t('Self-Service Settings')}
        </CardTitle>
        <CardDescription>
          {t('Control empty output refunds and user-facing limits.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex items-center justify-between rounded-lg border p-3'>
          <div>
            <div className='font-medium'>{t('Enable Self-Service')}</div>
            <div className='text-muted-foreground text-sm'>
              {t('Allow users to check records and upgrade themselves.')}
            </div>
          </div>
          <Switch
            checked={config.enabled}
            onCheckedChange={(enabled) => patch({ enabled })}
          />
        </div>

        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          <NumberField
            label={t('Lookback Hours')}
            value={config.lookback_hours}
            min={1}
            onChange={(lookback_hours) => patch({ lookback_hours })}
          />
          <NumberField
            label={t('Daily Refund Limit')}
            value={config.daily_refund_limit}
            min={1}
            onChange={(daily_refund_limit) => patch({ daily_refund_limit })}
          />
          <NumberField
            label={t('Max Refunds Per Check')}
            value={config.max_refunds_per_claim}
            min={1}
            onChange={(max_refunds_per_claim) =>
              patch({ max_refunds_per_claim })
            }
          />
          <NumberField
            label={t('Display Limit')}
            value={config.display_limit}
            min={1}
            onChange={(display_limit) => patch({ display_limit })}
          />
          <NumberField
            label={t('Refund Percent')}
            value={config.refund_percent}
            min={1}
            max={100}
            onChange={(refund_percent) => patch({ refund_percent })}
          />
          <NumberField
            label={t('Scan Limit')}
            value={config.scan_limit}
            min={20}
            onChange={(scan_limit) => patch({ scan_limit })}
          />
        </div>

        <div className='space-y-2'>
          <Label>{t('Excluded Models')}</Label>
          <Input
            value={config.exclude_models}
            onChange={(event) => patch({ exclude_models: event.target.value })}
            placeholder='gpt-image,dall-e'
          />
        </div>

        <Button onClick={() => props.onSave(config)} disabled={props.saving}>
          <Save className='size-4' />
          {props.saving ? t('Saving...') : t('Save Settings')}
        </Button>
      </CardContent>
    </Card>
  )
}

function RulesPanel(props: {
  rules: SelfServiceUpgradeRule[]
  onSave: (rules: Partial<SelfServiceUpgradeRule>[]) => void
  saving: boolean
}) {
  const { t } = useTranslation()
  const [rules, setRules] = useState<EditableRule[]>([])

  useEffect(() => {
    setRules(
      props.rules
        .filter((rule) => rule.enabled)
        .map((rule) => ({
          ...rule,
          threshold_amount: quotaUnitsToDollars(rule.threshold_quota),
        }))
    )
  }, [props.rules])

  const updateRule = (index: number, next: Partial<EditableRule>) => {
    setRules((current) =>
      current.map((rule, ruleIndex) =>
        ruleIndex === index ? { ...rule, ...next } : rule
      )
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <ListChecks className='size-4' />
          {t('Upgrade Rules')}
        </CardTitle>
        <CardDescription>
          {t('Set total balance thresholds and target groups.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='space-y-3'>
          {rules.map((rule, index) => (
            <div
              key={`${rule.id || 'new'}-${index}`}
              className='grid gap-3 rounded-lg border p-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.2fr)_auto]'
            >
              <Input
                type='number'
                min={1}
                value={rule.threshold_amount ?? 0}
                onChange={(event) =>
                  updateRule(index, {
                    threshold_amount: Number(event.target.value),
                  })
                }
                placeholder={t('Threshold Amount')}
              />
              <Input
                value={rule.target_group ?? ''}
                onChange={(event) =>
                  updateRule(index, { target_group: event.target.value })
                }
                placeholder={t('Target Group')}
              />
              <Input
                value={rule.description ?? ''}
                onChange={(event) =>
                  updateRule(index, { description: event.target.value })
                }
                placeholder={t('Description')}
              />
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={() =>
                  setRules((current) =>
                    current.filter((_, ruleIndex) => ruleIndex !== index)
                  )
                }
              >
                <Trash2 className='size-4' />
              </Button>
            </div>
          ))}
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              setRules((current) => [
                ...current,
                {
                  threshold_amount: 1,
                  target_group: '',
                  description: '',
                },
              ])
            }
          >
            <Plus className='size-4' />
            {t('Add Rule')}
          </Button>
          <Button
            onClick={() =>
              props.onSave(
                rules.map((rule) => ({
                  id: rule.id,
                  threshold_quota: parseQuotaFromDollars(
                    Number(rule.threshold_amount || 0)
                  ),
                  target_group: rule.target_group,
                  description: rule.description,
                  enabled: rule.enabled ?? true,
                }))
              )
            }
            disabled={props.saving}
          >
            <Save className='size-4' />
            {props.saving ? t('Saving...') : t('Save Rules')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function ClaimAttemptTable(props: { items: SelfServiceClaimAttempt[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('User')}</TableHead>
          <TableHead>{t('Scanned Records')}</TableHead>
          <TableHead>{t('Empty Output Records')}</TableHead>
          <TableHead>{t('New Records')}</TableHead>
          <TableHead>{t('Refunded Quota')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{formatTimestampToDate(item.created_at)}</TableCell>
            <TableCell>{item.username || item.user_id}</TableCell>
            <TableCell>{formatNumber(item.scanned_count)}</TableCell>
            <TableCell>{formatNumber(item.candidate_count)}</TableCell>
            <TableCell>{formatNumber(item.new_candidate_count)}</TableCell>
            <TableCell>{formatQuota(item.refunded_quota)}</TableCell>
            <TableCell>
              <Badge variant={item.status === 'success' ? 'default' : 'outline'}>
                {item.status}
              </Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function RefundHistoryTable(props: { items: SelfServiceHistory[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('User')}</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead>{t('Original Quota')}</TableHead>
          <TableHead>{t('Refunded Quota')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{formatTimestampToDate(item.created_at)}</TableCell>
            <TableCell>{item.username || item.user_id}</TableCell>
            <TableCell className='max-w-56 truncate'>{item.model_name}</TableCell>
            <TableCell>{formatQuota(item.original_quota)}</TableCell>
            <TableCell>{formatQuota(item.refunded_quota)}</TableCell>
            <TableCell>
              <Badge variant={item.status === 'success' ? 'default' : 'outline'}>
                {item.status}
              </Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function UpgradeHistoryTable(props: { items: SelfServiceUpgradeHistory[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('User')}</TableHead>
          <TableHead>{t('From Group')}</TableHead>
          <TableHead>{t('To Group')}</TableHead>
          <TableHead>{t('Total Balance')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{formatTimestampToDate(item.created_at)}</TableCell>
            <TableCell>{item.username || item.user_id}</TableCell>
            <TableCell>{item.from_group || '-'}</TableCell>
            <TableCell>{item.to_group || '-'}</TableCell>
            <TableCell>{formatQuota(item.total_quota)}</TableCell>
            <TableCell>
              <Badge variant={item.status === 'success' ? 'default' : 'outline'}>
                {item.status}
              </Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function SelfServiceAdmin() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const adminQuery = useQuery({
    queryKey: ['self-service-admin-config'],
    queryFn: async () => {
      const res = await getSelfServiceAdminConfig()
      return res.data
    },
  })

  const refundQuery = useQuery({
    queryKey: ['self-service-refund-histories'],
    queryFn: async () => {
      const res = await getSelfServiceRefundHistories({ p: 1, page_size: 20 })
      return res.data.items || []
    },
  })

  const attemptQuery = useQuery({
    queryKey: ['self-service-claim-attempts'],
    queryFn: async () => {
      const res = await getSelfServiceClaimAttempts({ p: 1, page_size: 20 })
      return res.data.items || []
    },
  })

  const upgradeQuery = useQuery({
    queryKey: ['self-service-upgrade-histories'],
    queryFn: async () => {
      const res = await getSelfServiceUpgradeHistories({ p: 1, page_size: 20 })
      return res.data.items || []
    },
  })

  const config = adminQuery.data?.config || emptyConfig
  const rules = adminQuery.data?.rules || []
  const stats = useMemo(() => adminQuery.data?.stats || {}, [adminQuery.data])

  const saveConfig = useMutation({
    mutationFn: updateSelfServiceConfig,
    onSuccess: () => {
      toast.success(t('Settings saved'))
      queryClient.invalidateQueries({ queryKey: ['self-service-admin-config'] })
    },
  })

  const saveRules = useMutation({
    mutationFn: updateSelfServiceRules,
    onSuccess: () => {
      toast.success(t('Rules saved'))
      queryClient.invalidateQueries({ queryKey: ['self-service-admin-config'] })
    },
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Self-Service Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Manage empty output refunds and self-service upgrades')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          <AdminStats stats={stats} />
          <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)]'>
            <ConfigPanel
              config={config}
              onSave={(next) => saveConfig.mutate(next)}
              saving={saveConfig.isPending}
            />
            <RulesPanel
              rules={rules}
              onSave={(next) => saveRules.mutate(next)}
              saving={saveRules.isPending}
            />
          </div>

          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <History className='size-4' />
                {t('Self-Service Records')}
              </CardTitle>
              <CardDescription>
                {t('Recent refund and upgrade activity.')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue='refunds'>
                <TabsList>
                  <TabsTrigger value='checks'>{t('Check Records')}</TabsTrigger>
                  <TabsTrigger value='refunds'>{t('Refund Records')}</TabsTrigger>
                  <TabsTrigger value='upgrades'>{t('Upgrade Records')}</TabsTrigger>
                </TabsList>
                <TabsContent value='checks' className='mt-4'>
                  <ClaimAttemptTable items={attemptQuery.data || []} />
                </TabsContent>
                <TabsContent value='refunds' className='mt-4'>
                  <RefundHistoryTable items={refundQuery.data || []} />
                </TabsContent>
                <TabsContent value='upgrades' className='mt-4'>
                  <UpgradeHistoryTable items={upgradeQuery.data || []} />
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
