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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

 type Channel = { id: number; name: string; status: number; models: string[]; priority: number; enabled_cost_groups: Record<string, string[]> }
type RoutingGroup = { id: string; name: string; cost_group: string; channel_order: number[] }
type RoutingConfig = { global_channel_order: number[]; global_cost_group: string; groups: RoutingGroup[]; channels: Channel[]; cost_groups: Record<string, number> }
type TokenAccess = { token_id: number; token_name: string; groups: string[] }
type ApiResponse<T> = { success: boolean; message?: string; data: T }

async function fetchRouting() {
  const response = await api.get<ApiResponse<RoutingConfig>>('/api/intelligent-routing')
  return requireServerSuccess(response.data)
}

async function saveRouting(config: Pick<RoutingConfig, 'global_channel_order' | 'global_cost_group' | 'groups'>) {
  const response = await api.put<ApiResponse<RoutingConfig>>('/api/intelligent-routing', config)
  return requireServerSuccess(response.data)
}

async function fetchTokenAccess() {
  const response = await api.get<ApiResponse<{ items: TokenAccess[] }>>('/api/intelligent-routing/token-access')
  return requireServerSuccess(response.data).data.items
}

async function saveTokenAccess(tokenID: number, groups: string[]) {
  const response = await api.put<ApiResponse<TokenAccess>>(`/api/intelligent-routing/token-access/${tokenID}`, { groups })
  return requireServerSuccess(response.data)
}

function moveItem<T>(items: T[], index: number, direction: 'up' | 'down') {
  const destination = direction === 'up' ? index - 1 : index + 1
  if (destination < 0 || destination >= items.length) return items
  const result = [...items]
  ;[result[index], result[destination]] = [result[destination], result[index]]
  return result
}

export function IntelligentRoutingPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['intelligent-routing'], queryFn: fetchRouting })
  const tokenAccessQuery = useQuery({ queryKey: ['intelligent-routing-token-access'], queryFn: fetchTokenAccess })
  const [config, setConfig] = useState<RoutingConfig | null>(null)
  const [tokenIDInput, setTokenIDInput] = useState('')
  const [selectedTokenID, setSelectedTokenID] = useState<number | null>(null)
  const [selectedTokenGroups, setSelectedTokenGroups] = useState<string[]>([])
  const [groupId, setGroupId] = useState('')
  const [groupName, setGroupName] = useState('')
  const [costGroup, setCostGroup] = useState('')
  const [selectedGlobalCostGroup, setSelectedGlobalCostGroup] = useState('')
  const [selectedChannels, setSelectedChannels] = useState<number[]>([])
  const [channelSearch, setChannelSearch] = useState('')
  const [deleting, setDeleting] = useState<RoutingGroup | null>(null)
  const mutation = useMutation({
    mutationFn: saveRouting,
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['intelligent-routing'] }); setConfig(null) },
    onError: (error) => handleServerError(error, t('Failed to save intelligent routing')),
  })
  const tokenAccessMutation = useMutation({
    mutationFn: ({ tokenID, groups }: { tokenID: number; groups: string[] }) => saveTokenAccess(tokenID, groups),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['intelligent-routing-token-access'] }); setSelectedTokenID(null); setSelectedTokenGroups([]) },
    onError: (error) => handleServerError(error, t('Failed to save token resource pool access')),
  })
  const data = config ?? query.data?.data
  const availableChannels = useMemo(() => {
    const queryText = channelSearch.trim().toLowerCase()
    return (data?.channels ?? []).filter((channel) => {
      if (!queryText) return true
      return channel.name.toLowerCase().includes(queryText) || channel.models.some((model) => model.toLowerCase().includes(queryText))
    })
  }, [channelSearch, data?.channels])
  const compatibleChannels = useMemo(() => availableChannels.filter((channel) => (channel.enabled_cost_groups?.[costGroup] ?? []).length > 0), [availableChannels, costGroup])
  const globalCostGroup = selectedGlobalCostGroup || data?.global_cost_group || Object.keys(data?.cost_groups ?? {})[0] || ''
  const globalCompatibleChannels = useMemo(() => availableChannels.filter((channel) => (channel.enabled_cost_groups?.[globalCostGroup] ?? []).length > 0), [availableChannels, globalCostGroup])

  if (query.isLoading) return <LoadingState message={t('Loading intelligent routing...')} />
  if (query.isError || !data) return <ErrorState title={t('Failed to load intelligent routing')} onRetry={() => void query.refetch()} />

  const update = (next: RoutingConfig) => setConfig(next)
  const save = () => mutation.mutate({ global_channel_order: data.global_channel_order, global_cost_group: selectedGlobalCostGroup || data.global_cost_group || Object.keys(data.cost_groups)[0] || '', groups: data.groups })
  const persistGroup = () => {
    if (!groupId.trim() || !groupName.trim() || !costGroup || selectedChannels.length === 0) return
    const existing = data.groups.find((group) => group.id === groupId.trim())
    const group: RoutingGroup = { id: groupId.trim(), name: groupName.trim(), cost_group: costGroup, channel_order: selectedChannels }
    update({ ...data, groups: existing ? data.groups.map((item) => item.id === group.id ? group : item) : [...data.groups, group] })
    setGroupId(''); setGroupName(''); setCostGroup(''); setSelectedChannels([])
  }
  const removeGroup = () => {
    if (!deleting) return
    update({ ...data, groups: data.groups.filter((group) => group.id !== deleting.id) })
    setDeleting(null)
  }
  const chooseToken = (tokenID: number) => {
    const access = tokenAccessQuery.data?.find((item) => item.token_id === tokenID)
    setSelectedTokenID(tokenID)
    setTokenIDInput(String(tokenID))
    setSelectedTokenGroups(access?.groups ?? [])
  }
  const saveSelectedTokenAccess = () => {
    if (selectedTokenID === null) return
    tokenAccessMutation.mutate({ tokenID: selectedTokenID, groups: selectedTokenGroups })
  }

  return (
    <main className='flex flex-col gap-6 p-6'>
      <header><h1 className='text-2xl font-semibold'>{t('Intelligent routing')}</h1><p className='text-muted-foreground mt-1 text-sm'>{t('Groups are resource pools. Harness sends the group ID in X-Route-Group; channel IDs are not exposed to Harness.')}</p></header>
      <section className='rounded-lg border p-4'>
        <h2 className='font-semibold'>{t('Global channel priority')}</h2>
        <p className='text-muted-foreground mb-3 text-sm'>{t('Channels earlier in the list have higher priority.')}</p>
        <label className='mb-3 block text-sm'>{t('Global cost group')}<select aria-label={t('Global cost group')} className='mt-1 h-9 w-full rounded-md border bg-background px-3 text-sm' value={selectedGlobalCostGroup || data.global_cost_group || Object.keys(data.cost_groups)[0] || ''} onChange={(event) => setSelectedGlobalCostGroup(event.target.value)}><option value=''>{t('Select cost group')}</option>{Object.entries(data.cost_groups).map(([name, ratio]) => <option key={name} value={name}>{name} — {t('Multiplier')}: {ratio}</option>)}</select></label>
        {data.global_channel_order.length ? <ol className='flex flex-col gap-2'>{data.global_channel_order.map((id, index) => { const channel = data.channels.find((item) => item.id === id); const label = channel?.name ?? String(id); return <li key={id} className='flex items-center gap-2 rounded-md border p-2'><span className='min-w-0 flex-1'>{label}</span><Button size='icon-sm' variant='ghost' aria-label={t('Move {{group}} up', { group: label })} disabled={index === 0} onClick={() => update({ ...data, global_channel_order: moveItem(data.global_channel_order, index, 'up') })}><ArrowUp aria-hidden='true' /></Button><Button size='icon-sm' variant='ghost' aria-label={t('Move {{group}} down', { group: label })} disabled={index === data.global_channel_order.length - 1} onClick={() => update({ ...data, global_channel_order: moveItem(data.global_channel_order, index, 'down') })}><ArrowDown aria-hidden='true' /></Button><Button size='icon-sm' variant='ghost' aria-label={t('Remove {{group}}', { group: label })} onClick={() => update({ ...data, global_channel_order: data.global_channel_order.filter((item) => item !== id) })}><Trash2 aria-hidden='true' /></Button></li> })}</ol> : <EmptyState title={t('No channels available')} />}
        <div className='mt-3 flex gap-2'><select aria-label={t('Add channel to global order')} className='h-9 rounded-md border bg-background px-3 text-sm' value='' onChange={(event) => { const id = Number(event.target.value); if (id && !data.global_channel_order.includes(id)) update({ ...data, global_channel_order: [...data.global_channel_order, id] }) }}><option value=''>{t('Add channel')}</option>{globalCompatibleChannels.filter((channel) => !data.global_channel_order.includes(channel.id)).map((channel) => <option key={channel.id} value={channel.id}>{channel.name}</option>)}</select></div>
      </section>
      <section className='rounded-lg border p-4'>
        <h2 className='mb-3 font-semibold'>{t('Resource pool groups')}</h2>
        <div className='grid gap-2 md:grid-cols-4'><Input aria-label={t('Group ID')} placeholder={t('Group ID')} value={groupId} onChange={(event) => setGroupId(event.target.value)} /><Input aria-label={t('Group name')} placeholder={t('Group name')} value={groupName} onChange={(event) => setGroupName(event.target.value)} /><select aria-label={t('Cost group')} className='h-9 rounded-md border bg-background px-3 text-sm' value={costGroup} onChange={(event) => setCostGroup(event.target.value)}><option value=''>{t('Select cost group')}</option>{Object.entries(data.cost_groups).map(([name, ratio]) => <option key={name} value={name}>{name} — {t('Multiplier')}: {ratio}</option>)}</select><Button onClick={persistGroup}>{t('Add or update group')}</Button></div>
        <div className='mt-3 space-y-2'>
          <label className='block text-sm'>{t('Search channels')}<Input className='mt-1' value={channelSearch} onChange={(event) => setChannelSearch(event.target.value)} /></label>
          <fieldset className='max-h-64 space-y-1 overflow-y-auto rounded-md border p-2'>
            <legend className='px-1 text-sm'>{t('Channels in group')}</legend>
            {compatibleChannels.map((channel) => {
              const checked = selectedChannels.includes(channel.id)
              return <label key={channel.id} className='hover:bg-muted/40 flex cursor-pointer items-start gap-2 rounded px-2 py-1.5 text-sm'>
                <input type='checkbox' checked={checked} onChange={() => setSelectedChannels(checked ? selectedChannels.filter((id) => id !== channel.id) : [...selectedChannels, channel.id])} />
                <span><span className='font-medium'>{channel.name}</span><span className='text-muted-foreground ml-2 text-xs'>{channel.models.join(', ')}</span></span>
              </label>
            })}
            {availableChannels.length === 0 ? <p className='text-muted-foreground p-2 text-sm'>{t('No channels match the search.')}</p> : null}
          </fieldset>
          <p className='text-muted-foreground text-xs'>{t('Selected channels')}: {selectedChannels.length}</p>
        </div>
        <div className='mt-4 space-y-3'>{data.groups.map((group) => <article key={group.id} className='rounded-md border p-3'><div className='flex items-center justify-between gap-3'><div><h3 className='font-medium'>{group.name} <span className='text-muted-foreground'>({group.id})</span></h3><p className='text-muted-foreground text-sm'>{t('Cost group')}: {group.cost_group} · {t('Multiplier')}: {data.cost_groups[group.cost_group] ?? '—'}</p></div><div className='flex gap-2'><Button variant='outline' onClick={() => { setGroupId(group.id); setGroupName(group.name); setCostGroup(group.cost_group); setSelectedChannels(group.channel_order) }}>{t('Edit')}</Button><Button variant='destructive' onClick={() => setDeleting(group)}>{t('Delete')}</Button></div></div><p className='text-muted-foreground mt-2 text-sm'>{t('Models')}: {[...new Set(group.channel_order.flatMap((id) => data.channels.find((channel) => channel.id === id)?.models ?? []))].join(', ') || t('No models listed')}</p><ol className='mt-2 flex flex-col gap-2'>{group.channel_order.map((id, index) => { const channel = data.channels.find((item) => item.id === id); const label = channel?.name ?? String(id); return <li key={id} className='flex items-center gap-2 rounded-md border p-2'><span className='min-w-0 flex-1'>{label}</span><Button size='icon-sm' variant='ghost' aria-label={t('Move {{group}} up', { group: label })} disabled={index === 0} onClick={() => update({ ...data, groups: data.groups.map((item) => item.id === group.id ? { ...item, channel_order: moveItem(item.channel_order, index, 'up') } : item) })}><ArrowUp aria-hidden='true' /></Button><Button size='icon-sm' variant='ghost' aria-label={t('Move {{group}} down', { group: label })} disabled={index === group.channel_order.length - 1} onClick={() => update({ ...data, groups: data.groups.map((item) => item.id === group.id ? { ...item, channel_order: moveItem(item.channel_order, index, 'down') } : item) })}><ArrowDown aria-hidden='true' /></Button><Button size='icon-sm' variant='ghost' aria-label={t('Remove {{group}}', { group: label })} onClick={() => update({ ...data, groups: data.groups.map((item) => item.id === group.id ? { ...item, channel_order: item.channel_order.filter((channelId) => channelId !== id) } : item) })}><Trash2 aria-hidden='true' /></Button></li> })}</ol><p className='text-muted-foreground mt-2 text-xs'>{t('Current cost group multiplier; this is not a per-channel currency price.')}</p></article>)}</div>
      </section>
      <section className='rounded-lg border p-4'>
        <h2 className='font-semibold'>{t('Harness token resource pool access')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>{t('Assign resource pools to a full-model service token. The token key is never displayed here.')}</p>
        {tokenAccessQuery.isError ? <ErrorState title={t('Failed to load token access')} onRetry={() => void tokenAccessQuery.refetch()} /> : null}
        {tokenAccessQuery.isLoading ? <LoadingState message={t('Loading token access...')} /> : null}
        <div className='mt-3 flex flex-wrap gap-2'>
          <Input aria-label={t('Token ID')} placeholder={t('Token ID')} inputMode='numeric' value={tokenIDInput} onChange={(event) => setTokenIDInput(event.target.value)} />
          <Button variant='outline' onClick={() => { const tokenID = Number(tokenIDInput); if (Number.isSafeInteger(tokenID) && tokenID > 0) chooseToken(tokenID) }}>{t('Load token access')}</Button>
          {(tokenAccessQuery.data ?? []).map((access) => <Button key={access.token_id} variant='outline' onClick={() => chooseToken(access.token_id)}>{access.token_name} · {access.token_id}</Button>)}
        </div>
        {selectedTokenID !== null ? <div className='mt-3 space-y-2'>
          <p className='text-sm'>{t('Authorized resource pools for token {{id}}', { id: selectedTokenID })}</p>
          {[...(data.global_channel_order.length || data.global_cost_group ? ['global'] : []), ...data.groups.map((group) => group.id)].map((groupID) => {
            const label = groupID === 'global' ? t('Global priority') : (data.groups.find((group) => group.id === groupID)?.name ?? groupID)
            const checked = selectedTokenGroups.includes(groupID)
            return <label key={groupID} className='flex items-center gap-2 text-sm'><input type='checkbox' checked={checked} onChange={() => setSelectedTokenGroups(checked ? selectedTokenGroups.filter((item) => item !== groupID) : [...selectedTokenGroups, groupID])} />{label} <span className='text-muted-foreground'>({groupID})</span></label>
          })}
          <Button onClick={saveSelectedTokenAccess} disabled={tokenAccessMutation.isPending}>{tokenAccessMutation.isPending ? t('Saving...') : t('Save token access')}</Button>
        </div> : null}
      </section>
      <div className='flex justify-end'><Button onClick={save} disabled={mutation.isPending}>{mutation.isPending ? t('Saving...') : t('Save changes')}</Button></div>
      <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(null)} title={t('Delete resource pool group?')} desc={t('This group will be removed from the pending configuration. Save changes to apply it.')} destructive confirmText={t('Delete')} handleConfirm={removeGroup} />
    </main>
  )
}
