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
import { Drag01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'
import { Reorder, useDragControls } from 'motion/react'
import {
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

type Channel = {
  id: number
  name: string
  status: number
  models: string[]
  priority: number
  enabled_groups?: Array<{ id: string; label: string }>
}
type RoutingGroup = {
  id: string
  name: string
  channel_order: number[]
}
type PricingGroupOption = { id: string; name: string; ratio: number }
type RoutingConfig = {
  global_channel_order: number[]
  pricing_group_order: string[]
  pricing_group_options: PricingGroupOption[]
  groups: RoutingGroup[]
  channels: Channel[]
}
type TokenAccess = { token_id: number; token_name: string; groups: string[] }
type RoutingToken = {
  id: number
  name: string
  status: number
  group?: string | null
  model_limits_enabled?: boolean
  key?: string
}
type ApiResponse<T> = { success: boolean; message?: string; data: T }

async function fetchRouting() {
  const response = await api.get<ApiResponse<RoutingConfig>>(
    '/api/intelligent-routing'
  )
  return requireServerSuccess(response.data)
}

async function saveRouting(
  config: Pick<
    RoutingConfig,
    'global_channel_order' | 'groups' | 'pricing_group_order'
  >
) {
  const response = await api.put<ApiResponse<RoutingConfig>>(
    '/api/intelligent-routing',
    config
  )
  return requireServerSuccess(response.data)
}

async function fetchTokenAccess() {
  const response = await api.get<ApiResponse<{ items: TokenAccess[] }>>(
    '/api/intelligent-routing/token-access'
  )
  return requireServerSuccess(response.data).data.items
}

// The routing page lists only the administrator's own tokens, using the same
// endpoint the console token page uses.
async function fetchRoutingTokens() {
  const response = await api.get<
    ApiResponse<{ items?: RoutingToken[]; page?: number; page_size?: number }>
  >('/api/token/?p=1&size=100')
  const items = requireServerSuccess(response.data).data?.items ?? []
  return items
}

async function fetchRoutingTokenKey(tokenID: number) {
  const response = await api.post<ApiResponse<{ key: string }>>(
    `/api/token/${tokenID}/key`
  )
  return requireServerSuccess(response.data).data.key
}

async function saveTokenAccess(tokenID: number, groups: string[]) {
  const response = await api.put<ApiResponse<TokenAccess>>(
    `/api/intelligent-routing/token-access/${tokenID}`,
    { groups }
  )
  return requireServerSuccess(response.data)
}

function moveItem<T>(items: T[], index: number, direction: 'up' | 'down') {
  const destination = direction === 'up' ? index - 1 : index + 1
  if (destination < 0 || destination >= items.length) return items
  const result = [...items]
  ;[result[index], result[destination]] = [result[destination], result[index]]
  return result
}

type ChannelOrderRowProps = {
  id: number
  channel?: Channel
  label: string
  index: number
  count: number
  onMove: (direction: 'up' | 'down') => void
}

function ChannelOrderRow(props: ChannelOrderRowProps) {
  const { t } = useTranslation()
  const controls = useDragControls()
  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      event.preventDefault()
      props.onMove(event.key === 'ArrowUp' ? 'up' : 'down')
    }
  }
  const handlePointerDown = (event: PointerEvent<HTMLButtonElement>) =>
    controls.start(event)

  return (
    <Reorder.Item
      value={props.id}
      dragListener={false}
      dragControls={controls}
      dragMomentum={false}
      className='bg-background flex items-center gap-2 rounded-md border p-2'
    >
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        aria-label={t('Drag {{channel}} to reorder', { channel: props.label })}
        onPointerDown={handlePointerDown}
        onKeyDown={handleKeyDown}
      >
        <HugeiconsIcon icon={Drag01Icon} strokeWidth={2} aria-hidden='true' />
      </Button>
      <span className='min-w-0 flex-1 font-medium'>{props.label}</span>
      <span className='flex flex-wrap gap-1'>
        {(props.channel?.enabled_groups ?? []).map((group) => (
          <span key={group.id} className='bg-muted rounded px-2 py-0.5 text-xs'>
            {group.label}
          </span>
        ))}
      </span>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        disabled={props.index === 0}
        aria-label={t('Move {{channel}} up', { channel: props.label })}
        onClick={() => props.onMove('up')}
      >
        ↑
      </Button>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        disabled={props.index === props.count - 1}
        aria-label={t('Move {{channel}} down', { channel: props.label })}
        onClick={() => props.onMove('down')}
      >
        ↓
      </Button>
    </Reorder.Item>
  )
}

export function IntelligentRoutingPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { section } = useParams({
    from: '/_authenticated/system-settings/intelligent-routing/$section',
  })
  const query = useQuery({
    queryKey: ['intelligent-routing'],
    queryFn: fetchRouting,
  })
  const tokenAccessQuery = useQuery({
    queryKey: ['intelligent-routing-token-access'],
    queryFn: fetchTokenAccess,
  })
  const tokensQuery = useQuery({
    queryKey: ['intelligent-routing-tokens'],
    queryFn: fetchRoutingTokens,
  })
  const [config, setConfig] = useState<RoutingConfig | null>(null)
  const savingSection = useRef<'global' | 'pools' | 'pricing' | null>(null)
  const [globalDraft, setGlobalDraft] = useState<number[] | null>(null)
  const [poolDrafts, setPoolDrafts] = useState<RoutingGroup[] | null>(null)
  const [pricingDraft, setPricingDraft] = useState<string[] | null>(null)
  const [tokenSearch, setTokenSearch] = useState('')
  const [tokenDraft, setTokenDraft] = useState<number[] | null>(null)
  const [groupId, setGroupId] = useState('')
  const [groupName, setGroupName] = useState('')
  const [selectedChannels, setSelectedChannels] = useState<number[]>([])
  const [channelSearch, setChannelSearch] = useState('')
  const [deleting, setDeleting] = useState<RoutingGroup | null>(null)
  const mutation = useMutation({
    mutationFn: saveRouting,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['intelligent-routing'] })
      setConfig(null)
      if (savingSection.current === 'global') setGlobalDraft(null)
      if (savingSection.current === 'pools') setPoolDrafts(null)
      if (savingSection.current === 'pricing') setPricingDraft(null)
    },
    onError: (error) =>
      handleServerError(error, t('Failed to save intelligent routing')),
  })
  const tokenAccessMutation = useMutation({
    mutationFn: ({ tokenID, groups }: { tokenID: number; groups: string[] }) =>
      saveTokenAccess(tokenID, groups),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['intelligent-routing-token-access'],
      })
    },
    onError: (error) =>
      handleServerError(error, t('Failed to save token resource pool access')),
  })
  const tokenKeyMutation = useMutation({
    mutationFn: (tokenID: number) => fetchRoutingTokenKey(tokenID),
    onError: (error) =>
      handleServerError(error, t('Failed to load the token key')),
  })
  const data = config ?? query.data?.data
  const globalOrder = globalDraft ?? data?.global_channel_order ?? []
  const poolGroups = poolDrafts ?? data?.groups ?? []
  const pricingOrder = pricingDraft ?? data?.pricing_group_order ?? []
  // The global pool only exists once its channel order holds at least one
  // channel; offering it earlier makes the authorization call fail with an
  // unknown-resource-pool error.
  const configuredRoutingGroups = useMemo(() => {
    const poolIDs = (data?.groups ?? []).map((group) => group.id)
    if ((data?.global_channel_order ?? []).length > 0) {
      return ['global', ...poolIDs]
    }
    return poolIDs
  }, [data?.global_channel_order, data?.groups])
  const accessibleTokenIDs = useMemo(() => {
    const ids = new Set(
      (tokenAccessQuery.data ?? []).map((item) => item.token_id)
    )
    return ids
  }, [tokenAccessQuery.data])
  const tokenCandidates = useMemo(() => {
    const text = tokenSearch.trim().toLowerCase()
    return (tokensQuery.data ?? []).filter((token) => {
      if (!text) return true
      return (
        token.name.toLowerCase().includes(text) ||
        String(token.id).includes(text)
      )
    })
  }, [tokensQuery.data, tokenSearch])
  const availableChannels = useMemo(() => {
    const queryText = channelSearch.trim().toLowerCase()
    return (data?.channels ?? []).filter((channel) => {
      if (!queryText) return true
      return (
        channel.name.toLowerCase().includes(queryText) ||
        channel.models.some((model) => model.toLowerCase().includes(queryText))
      )
    })
  }, [channelSearch, data?.channels])
  const compatibleChannels = availableChannels

  if (query.isLoading) {
    return <LoadingState message={t('Loading intelligent routing...')} />
  }
  if (query.isError || !data) {
    return (
      <ErrorState
        title={t('Failed to load intelligent routing')}
        onRetry={() => void query.refetch()}
      />
    )
  }

  const updatePools = (groups: RoutingGroup[]) => setPoolDrafts(groups)
  const saveGlobal = () => {
    savingSection.current = 'global'
    mutation.mutate({
      global_channel_order: globalOrder,
      groups: data.groups,
      pricing_group_order: data.pricing_group_order ?? [],
    })
  }
  const savePools = () => {
    savingSection.current = 'pools'
    mutation.mutate({
      global_channel_order: data.global_channel_order,
      groups: poolGroups,
      pricing_group_order: data.pricing_group_order ?? [],
    })
  }
  const savePricing = () => {
    savingSection.current = 'pricing'
    mutation.mutate({
      global_channel_order: data.global_channel_order,
      groups: data.groups,
      pricing_group_order: pricingOrder,
    })
  }
  const reorderGlobal = (ids: number[]) => setGlobalDraft(ids)
  const reorderPool = (groupID: string, ids: number[]) =>
    updatePools(
      poolGroups.map((group) =>
        group.id === groupID ? { ...group, channel_order: ids } : group
      )
    )
  const persistGroup = () => {
    if (!groupId.trim() || !groupName.trim() || selectedChannels.length === 0) {
      return
    }
    const existing = data.groups.find((group) => group.id === groupId.trim())
    const group: RoutingGroup = {
      id: groupId.trim(),
      name: groupName.trim(),
      channel_order: selectedChannels,
    }
    updatePools(
      existing
        ? poolGroups.map((item) => (item.id === group.id ? group : item))
        : [...poolGroups, group]
    )
    setGroupId('')
    setGroupName('')
    setSelectedChannels([])
  }
  const removeGroup = () => {
    if (!deleting) return
    updatePools(poolGroups.filter((group) => group.id !== deleting.id))
    setDeleting(null)
  }
  // A routing token always receives every routing group: the caller still
  // chooses the group per request through X-Route-Group.
  const revokeToken = (tokenID: number) => {
    tokenAccessMutation.mutate({ tokenID, groups: [] })
  }
  const selectedTokenIDs = tokenDraft ?? []
  const toggleTokenDraft = (tokenID: number) => {
    setTokenDraft(
      selectedTokenIDs.includes(tokenID)
        ? selectedTokenIDs.filter((id) => id !== tokenID)
        : [...selectedTokenIDs, tokenID]
    )
  }
  const grantDraftedTokens = () => {
    if (configuredRoutingGroups.length === 0) {
      toast.error(
        t('Save the global channel priority or a resource pool first')
      )
      return
    }
    for (const tokenID of selectedTokenIDs) {
      tokenAccessMutation.mutate({ tokenID, groups: configuredRoutingGroups })
    }
    setTokenDraft([])
  }
  const copyTokenKey = async (tokenID: number) => {
    const key = await tokenKeyMutation.mutateAsync(tokenID)
    await navigator.clipboard.writeText(`sk-${key}`)
    toast.success(t('Token key copied to clipboard'))
  }

  const showGlobal = section === 'global-priority'
  const showPools = section === 'resource-pools'
  const showPricing = section === 'pricing-groups'
  let sectionTitle = t('Pricing group priority')
  if (showGlobal) {
    sectionTitle = t('全渠道优先级设置')
  } else if (showPools) {
    sectionTitle = t('资源池分组')
  }
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{sectionTitle}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-6'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Groups are resource pools. Harness sends the group ID in X-Route-Group, and the actual channel pricing information is returned in the response headers.'
            )}
          </p>
          {showGlobal ? (
            <section className='rounded-lg border p-4'>
              <h2 className='font-semibold'>{t('Global channel priority')}</h2>
              <p className='text-muted-foreground mb-3 text-sm'>
                {t('Channels earlier in the list have higher priority.')}
              </p>
              <fieldset className='mb-3 space-y-2'>
                <legend className='text-sm font-medium'>
                  {t('Available enabled channels')}
                </legend>
                <div className='max-h-48 space-y-1 overflow-y-auto rounded-md border p-2'>
                  {availableChannels.map((channel) => (
                    <label
                      key={channel.id}
                      className='flex items-center gap-2 text-sm'
                    >
                      <input
                        type='checkbox'
                        checked={globalOrder.includes(channel.id)}
                        onChange={() =>
                          reorderGlobal(
                            globalOrder.includes(channel.id)
                              ? globalOrder.filter((id) => id !== channel.id)
                              : [...globalOrder, channel.id]
                          )
                        }
                      />
                      <span>{channel.name}</span>
                      <span className='text-muted-foreground text-xs'>
                        {(channel.enabled_groups ?? [])
                          .map((group) => group.label)
                          .join(', ')}
                      </span>
                    </label>
                  ))}
                </div>
              </fieldset>
              {globalOrder.length ? (
                <Reorder.Group
                  axis='y'
                  values={globalOrder}
                  onReorder={reorderGlobal}
                  className='flex flex-col gap-2'
                >
                  {globalOrder.map((id, index) => {
                    const channel = data.channels.find((item) => item.id === id)
                    const label = channel?.name ?? String(id)
                    return (
                      <ChannelOrderRow
                        key={id}
                        id={id}
                        channel={channel}
                        label={label}
                        index={index}
                        count={globalOrder.length}
                        onMove={(direction) =>
                          reorderGlobal(moveItem(globalOrder, index, direction))
                        }
                      />
                    )
                  })}
                </Reorder.Group>
              ) : (
                <EmptyState title={t('No channels available')} />
              )}
            </section>
          ) : null}
          {showPools ? (
            <section className='rounded-lg border p-4'>
              <h2 className='mb-3 font-semibold'>
                {t('Resource pool groups')}
              </h2>
              <div className='grid gap-2 md:grid-cols-4'>
                <Input
                  aria-label={t('Group ID')}
                  placeholder={t('Group ID')}
                  value={groupId}
                  onChange={(event) => setGroupId(event.target.value)}
                />
                <Input
                  aria-label={t('Group name')}
                  placeholder={t('Group name')}
                  value={groupName}
                  onChange={(event) => setGroupName(event.target.value)}
                />
                <Button onClick={persistGroup}>
                  {t('Add or update group')}
                </Button>
              </div>
              <div className='mt-3 space-y-2'>
                <label className='block text-sm'>
                  {t('Search channels')}
                  <Input
                    className='mt-1'
                    value={channelSearch}
                    onChange={(event) => setChannelSearch(event.target.value)}
                  />
                </label>
                <fieldset className='max-h-64 space-y-1 overflow-y-auto rounded-md border p-2'>
                  <legend className='px-1 text-sm'>
                    {t('Channels in group')}
                  </legend>
                  {compatibleChannels.map((channel) => {
                    const checked = selectedChannels.includes(channel.id)
                    return (
                      <label
                        key={channel.id}
                        className='hover:bg-muted/40 flex cursor-pointer items-start gap-2 rounded px-2 py-1.5 text-sm'
                      >
                        <input
                          type='checkbox'
                          checked={checked}
                          onChange={() =>
                            setSelectedChannels(
                              checked
                                ? selectedChannels.filter(
                                    (id) => id !== channel.id
                                  )
                                : [...selectedChannels, channel.id]
                            )
                          }
                        />
                        <span>
                          <span className='font-medium'>{channel.name}</span>
                          <span className='text-muted-foreground ml-2 text-xs'>
                            {channel.models.join(', ')}
                          </span>
                        </span>
                      </label>
                    )
                  })}
                  {availableChannels.length === 0 ? (
                    <p className='text-muted-foreground p-2 text-sm'>
                      {t('No channels match the search.')}
                    </p>
                  ) : null}
                </fieldset>
                <p className='text-muted-foreground text-xs'>
                  {t('Selected channels')}: {selectedChannels.length}
                </p>
              </div>
              <div className='mt-4 space-y-3'>
                {poolGroups.map((group) => (
                  <article key={group.id} className='rounded-md border p-3'>
                    <div className='flex items-center justify-between gap-3'>
                      <div>
                        <h3 className='font-medium'>
                          {group.name}{' '}
                          <span className='text-muted-foreground'>
                            ({group.id})
                          </span>
                        </h3>
                      </div>
                      <div className='flex gap-2'>
                        <Button
                          variant='outline'
                          onClick={() => {
                            setGroupId(group.id)
                            setGroupName(group.name)
                            setSelectedChannels(group.channel_order)
                          }}
                        >
                          {t('Edit')}
                        </Button>
                        <Button
                          variant='destructive'
                          onClick={() => setDeleting(group)}
                        >
                          {t('Delete')}
                        </Button>
                      </div>
                    </div>
                    <p className='text-muted-foreground mt-2 text-sm'>
                      {t('Models')}:{' '}
                      {[
                        ...new Set(
                          group.channel_order.flatMap(
                            (id) =>
                              data.channels.find((channel) => channel.id === id)
                                ?.models ?? []
                          )
                        ),
                      ].join(', ') || t('No models listed')}
                    </p>
                    <Reorder.Group
                      axis='y'
                      values={group.channel_order}
                      onReorder={(ids) => reorderPool(group.id, ids)}
                      className='mt-2 flex flex-col gap-2'
                    >
                      {group.channel_order.map((id, index) => {
                        const channel = data.channels.find(
                          (item) => item.id === id
                        )
                        const label = channel?.name ?? String(id)
                        return (
                          <ChannelOrderRow
                            key={id}
                            id={id}
                            channel={channel}
                            label={label}
                            index={index}
                            count={group.channel_order.length}
                            onMove={(direction) =>
                              reorderPool(
                                group.id,
                                moveItem(group.channel_order, index, direction)
                              )
                            }
                          />
                        )
                      })}
                    </Reorder.Group>
                  </article>
                ))}
              </div>
              <div className='mt-4 flex justify-end'>
                <Button onClick={savePools} disabled={mutation.isPending}>
                  {mutation.isPending ? t('Saving...') : t('Save changes')}
                </Button>
              </div>
            </section>
          ) : null}
          {showPricing ? (
            <section className='rounded-lg border p-4'>
              <h2 className='font-semibold'>{t('Pricing group priority')}</h2>
              <p className='text-muted-foreground my-3 text-sm'>
                {t(
                  'Only checked pricing groups participate. Channels without a matching selected group are skipped.'
                )}
              </p>
              <fieldset className='mb-4 space-y-2'>
                <legend className='text-sm font-medium'>
                  {t('Available pricing groups')}
                </legend>
                {(data.pricing_group_options ?? []).map((group) => (
                  <label
                    key={group.id}
                    className='flex items-center gap-2 rounded-md border p-2 text-sm'
                  >
                    <input
                      type='checkbox'
                      checked={pricingOrder.includes(group.id)}
                      onChange={() =>
                        setPricingDraft(
                          pricingOrder.includes(group.id)
                            ? pricingOrder.filter((id) => id !== group.id)
                            : [...pricingOrder, group.id]
                        )
                      }
                    />
                    <span>
                      {group.name} ({group.id})
                    </span>
                    <span className='text-muted-foreground ml-auto text-xs'>
                      {t('Ratio')}: {group.ratio}
                    </span>
                  </label>
                ))}
              </fieldset>
              <h3 className='mb-2 text-sm font-medium'>
                {t('Enabled pricing groups, highest first')}
              </h3>
              <Reorder.Group
                axis='y'
                values={pricingOrder}
                onReorder={setPricingDraft}
                className='flex flex-col gap-2'
              >
                {pricingOrder.map((groupID, index) => {
                  const group = (data.pricing_group_options ?? []).find(
                    (item) => item.id === groupID
                  )
                  return (
                    <Reorder.Item
                      key={groupID}
                      value={groupID}
                      className='flex items-center gap-2 rounded-md border p-2'
                    >
                      <span className='min-w-0 flex-1'>
                        {index + 1}. {group?.name ?? groupID} ({groupID})
                      </span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        disabled={index === 0}
                        aria-label={t('Move pricing group {{group}} up', {
                          group: groupID,
                        })}
                        onClick={() =>
                          setPricingDraft(moveItem(pricingOrder, index, 'up'))
                        }
                      >
                        ↑
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        disabled={index === pricingOrder.length - 1}
                        aria-label={t('Move pricing group {{group}} down', {
                          group: groupID,
                        })}
                        onClick={() =>
                          setPricingDraft(moveItem(pricingOrder, index, 'down'))
                        }
                      >
                        ↓
                      </Button>
                    </Reorder.Item>
                  )
                })}
              </Reorder.Group>
              {pricingOrder.length === 0 ? (
                <p className='text-muted-foreground mt-3 text-sm'>
                  {t(
                    'No pricing groups selected: intelligent routing has no eligible channels.'
                  )}
                </p>
              ) : null}
              <div className='mt-4 flex justify-end'>
                <Button onClick={savePricing} disabled={mutation.isPending}>
                  {mutation.isPending ? t('Saving...') : t('Save changes')}
                </Button>
              </div>
            </section>
          ) : null}
          {showGlobal ? (
            <section className='rounded-lg border p-4'>
              <h2 className='font-semibold'>{t('Harness routing tokens')}</h2>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t(
                  'Create the token in Console → Tokens, then authorize it here. Authorized tokens must send X-Route-Group on every model generation request or receive HTTP 403. Model listing and task-result reads do not require this header. Revoking authorization restores normal token routing.'
                )}
              </p>
              {tokenAccessQuery.isError ? (
                <ErrorState
                  title={t('Failed to load token access')}
                  onRetry={() => void tokenAccessQuery.refetch()}
                />
              ) : null}
              {tokenAccessQuery.isLoading ? (
                <LoadingState message={t('Loading token access...')} />
              ) : null}

              <div className='mt-4 space-y-3'>
                <div className='flex flex-wrap items-end gap-2'>
                  <label className='block flex-1 text-sm'>
                    {t('Search tokens')}
                    <Input
                      className='mt-1'
                      value={tokenSearch}
                      onChange={(event) => setTokenSearch(event.target.value)}
                    />
                  </label>
                  <Button
                    onClick={grantDraftedTokens}
                    disabled={
                      selectedTokenIDs.length === 0 ||
                      tokenAccessMutation.isPending
                    }
                  >
                    {t('Authorize {{count}} token(s)', {
                      count: selectedTokenIDs.length,
                    })}
                  </Button>
                </div>
                {tokensQuery.isError ? (
                  <ErrorState
                    title={t('Failed to load tokens')}
                    onRetry={() => void tokensQuery.refetch()}
                  />
                ) : null}
                {tokensQuery.isLoading ? (
                  <LoadingState message={t('Loading tokens...')} />
                ) : null}
                {!tokensQuery.isLoading && tokenCandidates.length === 0 ? (
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'No tokens found. Create one in Console → Tokens, then reload this page.'
                    )}
                  </p>
                ) : null}
                <fieldset className='max-h-64 space-y-1 overflow-y-auto rounded-md border p-2'>
                  <legend className='px-1 text-sm'>
                    {t('Tokens available for routing')}
                  </legend>
                  {tokenCandidates.map((token) => {
                    const authorized = accessibleTokenIDs.has(token.id)
                    return (
                      <label
                        key={token.id}
                        className='hover:bg-muted/40 flex cursor-pointer items-start gap-2 rounded px-2 py-1.5 text-sm'
                      >
                        <input
                          type='checkbox'
                          checked={selectedTokenIDs.includes(token.id)}
                          onChange={() => toggleTokenDraft(token.id)}
                        />
                        <span className='min-w-0 flex-1'>
                          <span className='font-medium'>{token.name}</span>
                          <span className='text-muted-foreground ml-2 text-xs'>
                            #{token.id}
                          </span>
                          {token.model_limits_enabled ? (
                            <span className='ml-2 text-xs text-amber-600'>
                              {t('Limited model scope can not use routing')}
                            </span>
                          ) : null}
                          {authorized ? (
                            <span className='text-success ml-2 text-xs'>
                              {t('Authorized')}
                            </span>
                          ) : null}
                        </span>
                      </label>
                    )
                  })}
                </fieldset>
              </div>

              <h3 className='mt-4 mb-2 text-sm font-medium'>
                {t('Authorized routing tokens')}
              </h3>
              {(tokenAccessQuery.data ?? []).length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No token is authorized for routing yet.')}
                </p>
              ) : (
                <div className='space-y-2'>
                  {(tokenAccessQuery.data ?? []).map((access) => (
                    <div
                      key={access.token_id}
                      className='flex flex-wrap items-center gap-2 rounded-md border p-2 text-sm'
                    >
                      <span className='font-medium'>{access.token_name}</span>
                      <span className='text-muted-foreground text-xs'>
                        #{access.token_id}
                      </span>
                      <span className='text-muted-foreground text-xs'>
                        {t('Groups')}: {access.groups.join(', ')}
                      </span>
                      <div className='ml-auto flex gap-2'>
                        <Button
                          variant='outline'
                          onClick={() => void copyTokenKey(access.token_id)}
                          disabled={tokenKeyMutation.isPending}
                        >
                          {t('Copy key')}
                        </Button>
                        <Button
                          variant='destructive'
                          onClick={() => revokeToken(access.token_id)}
                          disabled={tokenAccessMutation.isPending}
                        >
                          {t('Revoke')}
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          ) : null}
          {showGlobal ? (
            <div className='flex justify-end'>
              <Button onClick={saveGlobal} disabled={mutation.isPending}>
                {mutation.isPending ? t('Saving...') : t('Save changes')}
              </Button>
            </div>
          ) : null}
          <ConfirmDialog
            open={!!deleting}
            onOpenChange={(open) => !open && setDeleting(null)}
            title={t('Delete resource pool group?')}
            desc={t(
              'This group will be removed from the pending configuration. Save changes to apply it.'
            )}
            destructive
            confirmText={t('Delete')}
            handleConfirm={removeGroup}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
