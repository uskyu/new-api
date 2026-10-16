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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { IntelligentRoutingPage } from '..'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))

function mockGet(routing: unknown) {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: url === '/api/intelligent-routing/token-access'
      ? { success: true, data: { items: [] } }
      : { success: true, data: routing },
  }) as never)
}
afterEach(cleanup)

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><IntelligentRoutingPage /></QueryClientProvider>)
}

describe('intelligent routing management', () => {
  it('moves a channel and saves the configured global priority', async () => {
    mockGet({ global_channel_order: [11, 12], global_cost_group: 'default', groups: [], channels: [{ id: 11, name: 'Alpha', status: 1, models: ['m-a'], priority: 0, enabled_cost_groups: { default: ['m-a'], economy: ['m-a'] } }, { id: 12, name: 'Beta', status: 1, models: ['m-b'], priority: 1, enabled_cost_groups: { default: ['m-b'], economy: ['m-b'] } }], cost_groups: { default: 1, economy: 0.5 } })
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: 'Move Alpha down' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(api.put).toHaveBeenCalledWith('/api/intelligent-routing', { global_channel_order: [12, 11], global_cost_group: 'default', groups: [] })
  })

  it('saves edited resource pool channels and cost group', async () => {
    mockGet({ global_channel_order: [], global_cost_group: 'default', groups: [{ id: 'pool', name: 'Pool', cost_group: 'default', channel_order: [11] }], channels: [{ id: 11, name: 'Alpha', status: 1, models: ['m-a'], priority: 0, enabled_cost_groups: { default: ['m-a'], economy: ['m-a'] } }, { id: 12, name: 'Beta', status: 1, models: ['m-b'], priority: 1, enabled_cost_groups: { default: ['m-b'], economy: ['m-b'] } }], cost_groups: { default: 1, economy: 0.5 } })
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: 'Edit' }))
    await user.selectOptions(screen.getByRole('combobox', { name: 'Cost group' }), 'economy')
    await user.click(screen.getByRole('checkbox', { name: /Beta/ }))
    await user.click(screen.getByRole('button', { name: 'Add or update group' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(api.put).toHaveBeenCalledWith('/api/intelligent-routing', {
      global_channel_order: [],
      global_cost_group: 'default',
      groups: [{ id: 'pool', name: 'Pool', cost_group: 'economy', channel_order: [11, 12] }],
    })
  })

  it('confirms deletion before removing a resource pool group', async () => {
    mockGet({ global_channel_order: [], global_cost_group: 'default', groups: [{ id: 'pool', name: 'Pool', cost_group: 'default', channel_order: [] }], channels: [], cost_groups: { default: 1 } })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /^Delete$/ }))
    expect(await screen.findByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /^Delete$/ }))
    expect(screen.queryByRole('heading', { name: /Pool/ })).not.toBeInTheDocument()
  })
})
