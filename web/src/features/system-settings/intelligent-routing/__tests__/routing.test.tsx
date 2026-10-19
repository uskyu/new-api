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
      pricing_group_order: [],
    })
  })

  it('saves resource pool channel selection and order without pool pricing fields', async () => {
    mockGet({
      global_channel_order: [],
      groups: [
        {
          id: 'pool',
          name: 'Pool',
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
    expect(
      screen.queryByLabelText('Discount percentage')
    ).not.toBeInTheDocument()
    expect(
      screen.queryByLabelText('Tax rate percentage')
    ).not.toBeInTheDocument()
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
          channel_order: [12, 11],
        },
      ],
      pricing_group_order: [],
    })
  })

  it('confirms deletion before removing a resource pool group', async () => {
    mockGet({
      global_channel_order: [],
      groups: [
        {
          id: 'pool',
          name: 'Pool',
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

describe('opt-in pricing group order', () => {
  it('saves only checked groups in the configured order', async () => {
    mockGet({
      global_channel_order: [11],
      groups: [],
      pricing_group_order: ['vip'],
      pricing_group_options: [
        { id: 'vip', name: 'VIP', ratio: 0.8 },
        { id: 'supplier', name: 'Supplier', ratio: 0.7 },
      ],
      channels: [
        {
          id: 11,
          name: 'Alpha',
          models: ['model'],
          status: 1,
          priority: 0,
          enabled_groups: [],
        },
      ],
    })
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage('pricing-groups')
    await user.click(await screen.findByRole('checkbox', { name: /Supplier/ }))
    await user.click(
      screen.getByRole('button', { name: 'Move pricing group supplier up' })
    )
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(api.put).toHaveBeenCalledWith('/api/intelligent-routing', {
      global_channel_order: [11],
      groups: [],
      pricing_group_order: ['supplier', 'vip'],
    })
  })
})

describe('routing token authorization', () => {
  function mockTokens(
    tokens: unknown[],
    routing: unknown,
    access: unknown[] = []
  ) {
    vi.mocked(api.get).mockImplementation(async (url) => {
      let data: unknown = routing
      if (url === '/api/intelligent-routing/token-access') {
        data = { items: access }
      } else if (url === '/api/token/?p=1&size=100') {
        data = { items: tokens }
      }
      return { data: { success: true, data } } as never
    })
  }

  it('grants an authorized token every configured routing group', async () => {
    mockTokens(
      [{ id: 7, name: 'service', status: 1, model_limits_enabled: false }],
      {
        global_channel_order: [11],
        groups: [{ id: 'pool', name: 'Pool', channel_order: [11] }],
        channels: [],
      }
    )
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage('global-priority')
    await user.click(await screen.findByRole('checkbox', { name: /service/ }))
    await user.click(
      screen.getByRole('button', { name: 'Authorize 1 token(s)' })
    )
    expect(api.put).toHaveBeenCalledWith(
      '/api/intelligent-routing/token-access/7',
      { groups: ['global', 'pool'] }
    )
  })

  it('omits the global pool while no global channel priority is saved', async () => {
    mockTokens(
      [{ id: 8, name: 'bare', status: 1, model_limits_enabled: false }],
      { global_channel_order: [], groups: [], channels: [] }
    )
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage('global-priority')
    await user.click(await screen.findByRole('checkbox', { name: /bare/ }))
    await user.click(
      screen.getByRole('button', { name: 'Authorize 1 token(s)' })
    )
    // Saving would otherwise fail with "Unknown resource pool: global".
    expect(api.put).not.toHaveBeenCalled()
  })

  it('revokes an authorized token by clearing its routing groups', async () => {
    mockTokens(
      [{ id: 7, name: 'service', status: 1, model_limits_enabled: false }],
      {
        global_channel_order: [11],
        groups: [],
        channels: [],
      },
      [{ token_id: 7, token_name: 'service', groups: ['global'] }]
    )
    vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
    const user = userEvent.setup()
    renderPage('global-priority')
    await user.click(await screen.findByRole('button', { name: 'Revoke' }))
    expect(api.put).toHaveBeenCalledWith(
      '/api/intelligent-routing/token-access/7',
      { groups: [] }
    )
  })
})
