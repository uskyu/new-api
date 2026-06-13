import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  ArrowUpRight,
  CheckCircle2,
  ClipboardCheck,
  RefreshCw,
  Sparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Progress } from '@/components/ui/progress'
import { SectionPageLayout } from '@/components/layout'
import {
  formatNumber,
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import {
  checkSelfService,
  getSelfServiceInfo,
  upgradeSelfServiceGroup,
} from './api'
import type {
  SelfServiceCheckResult,
  SelfServiceIssueRecord,
  SelfServiceSelfInfo,
  SelfServiceUpgradeRule,
} from './types'

function StatBlock(props: { label: string; value: string; hint?: string }) {
  return (
    <div className='rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 text-lg font-medium'>{props.value}</div>
      {props.hint && (
        <div className='text-muted-foreground mt-1 text-xs'>{props.hint}</div>
      )}
    </div>
  )
}

function SelfServiceSummary(props: { data?: SelfServiceSelfInfo }) {
  const { t } = useTranslation()
  const data = props.data

  return (
    <div className='grid gap-3 md:grid-cols-3'>
      <StatBlock
        label={t('Current Group')}
        value={data?.current_group || '-'}
      />
      <StatBlock
        label={t('Total Balance')}
        value={formatQuota(data?.total_quota ?? 0)}
        hint={t('Remaining balance plus consumed quota')}
      />
      <StatBlock
        label={t('Available Upgrade')}
        value={data?.upgrade_offer?.target_group || t('None')}
        hint={
          data?.upgrade_offer
            ? `${t('Requires')} ${formatQuota(data.upgrade_offer.threshold_quota)}`
            : t('No matching upgrade rule')
        }
      />
    </div>
  )
}

function RecordsTable(props: { records: SelfServiceIssueRecord[] }) {
  const { t } = useTranslation()
  if (props.records.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('No records to display')}
      </div>
    )
  }

  return (
    <>
      <div className='md:hidden'>
        <div className='flex flex-col gap-3'>
          {props.records.map((record) => (
            <RecordCard key={record.log_id} record={record} />
          ))}
        </div>
      </div>
      <div className='hidden overflow-x-auto md:block'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Time')}</TableHead>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Charged')}</TableHead>
              <TableHead>{t('Refunded')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.records.map((record) => (
              <TableRow key={record.log_id}>
                <TableCell>{formatTimestampToDate(record.created_at)}</TableCell>
                <TableCell className='max-w-56 truncate'>
                  {record.model_name || '-'}
                </TableCell>
                <TableCell>{formatQuota(record.quota)}</TableCell>
                <TableCell>{formatQuota(record.refunded_quota)}</TableCell>
                <TableCell>
                  <Badge
                    variant={record.status === 'fixed' ? 'default' : 'outline'}
                  >
                    {record.status === 'fixed' ? t('Processed') : t('Checked')}
                  </Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  )
}

function RecordCard(props: { record: SelfServiceIssueRecord }) {
  const { t } = useTranslation()
  const record = props.record

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='text-muted-foreground text-xs'>
          {formatTimestampToDate(record.created_at)}
        </div>
        <Badge variant={record.status === 'fixed' ? 'default' : 'outline'}>
          {record.status === 'fixed' ? t('Processed') : t('Checked')}
        </Badge>
      </div>
      <div className='mt-3'>
        <div className='text-muted-foreground text-xs'>{t('Model')}</div>
        <div className='mt-1 break-words text-sm font-medium'>
          {record.model_name || '-'}
        </div>
      </div>
      <div className='mt-3 grid grid-cols-3 gap-2'>
        <StatBlock label={t('Log ID')} value={formatNumber(record.log_id)} />
        <StatBlock label={t('Charged')} value={formatQuota(record.quota)} />
        <StatBlock
          label={t('Refunded')}
          value={formatQuota(record.refunded_quota)}
        />
      </div>
    </div>
  )
}

function UpgradeRulesTable(props: {
  rules: SelfServiceUpgradeRule[]
  totalQuota: number
  currentGroup?: string
  offerTarget?: string
}) {
  const { t } = useTranslation()

  if (props.rules.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('No upgrade rules configured')}
      </div>
    )
  }

  return (
    <div className='overflow-x-auto'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Target Group')}</TableHead>
            <TableHead>{t('Threshold')}</TableHead>
            <TableHead>{t('Remaining')}</TableHead>
            <TableHead>{t('Description')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.rules.map((rule) => (
            <TableRow key={rule.id || rule.target_group}>
              <TableCell>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge
                    variant={
                      rule.target_group === props.currentGroup
                        ? 'default'
                        : 'secondary'
                    }
                  >
                    {rule.target_group}
                  </Badge>
                  {rule.target_group === props.offerTarget && (
                    <Badge variant='outline'>{t('Available now')}</Badge>
                  )}
                </div>
              </TableCell>
              <TableCell>{formatQuota(rule.threshold_quota)}</TableCell>
              <TableCell>
                {formatQuota(
                  Math.max(0, rule.threshold_quota - props.totalQuota)
                )}
              </TableCell>
              <TableCell className='max-w-80 break-words'>
                {rule.description || '-'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function SelfService() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [checkResult, setCheckResult] = useState<SelfServiceCheckResult | null>(
    null
  )

  const infoQuery = useQuery({
    queryKey: ['self-service-info'],
    queryFn: async () => {
      const res = await getSelfServiceInfo()
      return res.data
    },
  })

  const currentInfo = checkResult || infoQuery.data
  const records = checkResult?.records || []
  const upgradeRules = (currentInfo?.upgrade_rules || []).filter(
    (rule) => rule.enabled
  )
  const nextRule = upgradeRules
    .filter((rule) => rule.target_group !== currentInfo?.current_group)
    .sort((a, b) => a.threshold_quota - b.threshold_quota)[0]
  const nextTarget = currentInfo?.upgrade_offer || nextRule
  const quotaGap = nextTarget
    ? Math.max(0, nextTarget.threshold_quota - (currentInfo?.total_quota || 0))
    : 0
  const progressValue = nextTarget
    ? Math.min(
        100,
        Math.round(
          ((currentInfo?.total_quota || 0) / nextTarget.threshold_quota) * 100
        )
      )
    : currentInfo?.upgrade_offer
      ? 100
      : 0

  const checkMutation = useMutation({
    mutationFn: checkSelfService,
    onSuccess: (res) => {
      setCheckResult(res.data)
      toast.success(t(res.data.message))
      queryClient.invalidateQueries({ queryKey: ['self-service-info'] })
    },
  })

  const upgradeMutation = useMutation({
    mutationFn: upgradeSelfServiceGroup,
    onSuccess: () => {
      toast.success(t('Group upgraded successfully'))
      setCheckResult(null)
      queryClient.invalidateQueries({ queryKey: ['self-service-info'] })
    },
  })

  const checkStats = useMemo(() => {
    if (!checkResult) return []
    return [
      [t('Scanned Records'), formatNumber(checkResult.scanned_count)],
      [t('Empty Output Records'), formatNumber(checkResult.candidate_count)],
      [t('New Records'), formatNumber(checkResult.new_candidate_count)],
      [t('Current Refund Quota'), formatQuota(checkResult.refunded_quota)],
      [t('Total Refunded Quota'), formatQuota(checkResult.total_refunded_quota ?? 0)],
    ]
  }, [checkResult, t])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Self-Service Platform')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Check empty output charges and upgrade your group when eligible')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          <Card>
            <CardHeader>
              <CardTitle>
                {!currentInfo?.enabled
                  ? t('Self-service is disabled')
                  : currentInfo?.upgrade_offer
                    ? t('You are eligible for self-service upgrade')
                    : t('Self-service upgrade progress')}
              </CardTitle>
              <CardDescription>
                {!currentInfo?.enabled
                  ? t('Please contact an administrator if you need assistance.')
                  : currentInfo?.upgrade_offer
                    ? `${t('Your consumed total has reached')} ${formatQuota(
                        currentInfo.total_quota
                      )}, ${t('you can upgrade to')} ${
                        currentInfo.upgrade_offer.target_group
                      }.`
                    : nextTarget
                      ? `${t('Your consumed total has reached')} ${formatQuota(
                          currentInfo?.total_quota || 0
                        )}, ${t('remaining to upgrade')} ${formatQuota(
                          quotaGap
                        )}.`
                      : t('No upgrade rules configured')}
              </CardDescription>
            </CardHeader>
            <CardContent className='flex flex-col gap-4'>
              <SelfServiceSummary data={currentInfo} />
              {nextTarget && (
                <Progress value={progressValue}>
                  <div className='flex w-full items-center justify-between gap-3 text-sm'>
                    <span className='font-medium'>
                      {currentInfo?.current_group || '-'} {' -> '}
                      {nextTarget.target_group}
                    </span>
                    <span className='text-muted-foreground'>
                      {progressValue}%
                    </span>
                  </div>
                </Progress>
              )}
            </CardContent>
          </Card>

          {!currentInfo?.enabled && (
            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2'>
                  <AlertCircle className='size-4' />
                  {t('Self-service is disabled')}
                </CardTitle>
                <CardDescription>
                  {t('Please contact an administrator if you need assistance.')}
                </CardDescription>
              </CardHeader>
            </Card>
          )}

          <div className='grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]'>
            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2'>
                  <ClipboardCheck className='size-4' />
                  {t('Empty Output Check')}
                </CardTitle>
                <CardDescription>
                  {t('Find charged calls where completion tokens are zero.')}
                </CardDescription>
                <CardAction>
                  <Button
                    onClick={() => checkMutation.mutate()}
                    disabled={!currentInfo?.enabled || checkMutation.isPending}
                  >
                    <RefreshCw
                      className={
                        checkMutation.isPending
                          ? 'size-4 animate-spin'
                          : 'size-4'
                      }
                    />
                    {checkMutation.isPending ? t('Checking...') : t('Check Now')}
                  </Button>
                </CardAction>
              </CardHeader>
              <CardContent className='space-y-4'>
                {checkStats.length > 0 && (
                  <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
                    {checkStats.map(([label, value]) => (
                      <StatBlock key={label} label={label} value={value} />
                    ))}
                  </div>
                )}
                <RecordsTable records={records} />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2'>
                  <Sparkles className='size-4' />
                  {t('Self-Service Upgrade')}
                </CardTitle>
                <CardDescription>
                  {t('Upgrade based on your historical total balance.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                {currentInfo?.upgrade_offer ? (
                  <div className='rounded-lg border p-4'>
                    <div className='flex items-center justify-between gap-3'>
                      <div>
                        <div className='text-sm font-medium'>
                          {currentInfo.current_group} {' -> '}
                          {currentInfo.upgrade_offer.target_group}
                        </div>
                        <div className='text-muted-foreground mt-1 text-sm'>
                          {currentInfo.upgrade_offer.description ||
                            t('You meet this upgrade threshold.')}
                        </div>
                      </div>
                      <CheckCircle2 className='text-primary size-5' />
                    </div>
                    <Button
                      className='mt-4 w-full'
                      onClick={() => upgradeMutation.mutate()}
                      disabled={upgradeMutation.isPending}
                    >
                      <ArrowUpRight className='size-4' />
                      {upgradeMutation.isPending
                        ? t('Upgrading...')
                        : t('Upgrade Now')}
                    </Button>
                  </div>
                ) : (
                  <div className='text-muted-foreground rounded-lg border p-4 text-sm'>
                    {t('No available upgrade yet.')}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>{t('Upgrade Rules')}</CardTitle>
              <CardDescription>
                {t('Rules configured by administrators for self-service upgrades.')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <UpgradeRulesTable
                rules={upgradeRules}
                totalQuota={currentInfo?.total_quota || 0}
                currentGroup={currentInfo?.current_group}
                offerTarget={currentInfo?.upgrade_offer?.target_group}
              />
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
