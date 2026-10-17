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
import { useParams } from '@tanstack/react-router'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { IntelligentRoutingPage } from '..'

vi.mock('@tanstack/react-router', () => ({ useParams: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))

function mockGet(routing: unknown) {
  vi.mocked(api.get).mockImplementation(
    async (url) =>
      ({
        data:
          url === '/api/intelligent-routing/token-access'
            ? { success: true, data: { items: [] } }
            : { success: true, data: routing },
      }) as never
  )
}
afterEach(cleanup)

function renderPage(section = 'global-priority') {
  vi.mocked(useParams).mockReturnValue({ section } as never)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <IntelligentRoutingPage />
    </QueryClientProvider>
  )
}

describe('intelligent routing management', () => {
  it('moves a channel and saves the configured global priority', async () => {
    mockGet({
      global_channel_order: [11, 12],
      groups: [],
      channels: [
        {
          id: 11,
          name: 'Alpha',
          status: 1,
          models: ['m-a'],
          priority: 0,
          enabled_cost_groups: { default: ['m-a'], economy: ['m-a'] },
          enabled_groups: [{ id: 'vip', label: 'VIP group' }],
        },
        {
          id: 12,
          name: 'Beta',
          status: 1,
          models: ['m-b'],
          priority: 1,
          enabled_cost_groups: { default: ['m-b'], economy: ['m-b'] },
          enabled_groups: [],
        },
      ],
    })
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage()
    expect(
      await screen.findByRole('heading', { name: 'Global channel priority' })
    ).toBeVisible()
    expect(screen.getAllByText('VIP group')).toHaveLength(2)
    await user.click(screen.getByRole('checkbox', { name: /Beta/ }))
    expect(screen.getByRole('checkbox', { name: /Beta/ })).not.toBeChecked()
    await user.click(screen.getByRole('checkbox', { name: /Beta/ }))
    expect(screen.getByRole('checkbox', { name: /Beta/ })).toBeChecked()
    await user.click(
      await screen.findByRole('button', { name: 'Move Alpha down' })
    )
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(api.put).toHaveBeenCalledWith('/api/intelligent-routing', {
      global_channel_order: [12, 11],
      groups: [],
    })
  })

  it('saves resource pool channels, discount, and tax rate', async () => {
    mockGet({
      global_channel_order: [],
      groups: [
        {
          id: 'pool',
          name: 'Pool',
          discount_percent: 0,
          tax_rate_percent: 0,
          channel_order: [11],
        },
      ],
      channels: [
        {
          id: 11,
          name: 'Alpha',
          status: 1,
          models: ['m-a'],
          priority: 0,
          enabled_cost_groups: { default: ['m-a'], economy: ['m-a'] },
          enabled_groups: [{ id: 'vip', label: 'VIP group' }],
        },
        {
          id: 12,
          name: 'Beta',
          status: 1,
          models: ['m-b'],
          priority: 1,
          enabled_cost_groups: { default: ['m-b'], economy: ['m-b'] },
          enabled_groups: [],
        },
      ],
    })
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage('resource-pools')
    expect(
      await screen.findByRole('heading', { name: 'Resource pool groups' })
    ).toBeVisible()
    await user.click(await screen.findByRole('button', { name: 'Edit' }))
    await user.clear(
      screen.getByRole('spinbutton', { name: 'Discount percentage' })
    )
    await user.type(
      screen.getByRole('spinbutton', { name: 'Discount percentage' }),
      '15'
    )
    await user.clear(
      screen.getByRole('spinbutton', { name: 'Tax rate percentage' })
    )
    await user.type(
      screen.getByRole('spinbutton', { name: 'Tax rate percentage' }),
      '8'
    )
    expect(
      screen.queryByRole('combobox', { name: 'Cost group' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Move Alpha down' }))
    await user.click(screen.getByRole('checkbox', { name: /Beta/ }))
    await user.click(
      screen.getByRole('button', { name: 'Add or update group' })
    )
    await user.click(screen.getByRole('button', { name: 'Move Alpha down' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(api.put).toHaveBeenCalledWith('/api/intelligent-routing', {
      global_channel_order: [],
      groups: [
        {
          id: 'pool',
          name: 'Pool',
          discount_percent: 15,
          tax_rate_percent: 8,
          channel_order: [12, 11],
        },
      ],
    })
  })

  it('confirms deletion before removing a resource pool group', async () => {
    mockGet({
      global_channel_order: [],
      groups: [
        {
          id: 'pool',
          name: 'Pool',
          discount_percent: 0,
          tax_rate_percent: 0,
          channel_order: [],
        },
      ],
      channels: [],
    })
    const user = userEvent.setup()
    renderPage('resource-pools')
    await user.click(await screen.findByRole('button', { name: /^Delete$/ }))
    expect(await screen.findByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /^Delete$/ }))
    expect(
      screen.queryByRole('heading', { name: /Pool/ })
    ).not.toBeInTheDocument()
  })
})
