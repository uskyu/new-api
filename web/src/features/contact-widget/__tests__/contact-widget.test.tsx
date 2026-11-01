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
import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { ContactWidget } from '../components/contact-widget'
import type { ContactWidgetResponse } from '../types'

const { getContactWidgetMock } = vi.hoisted(() => ({
  getContactWidgetMock: vi.fn(),
}))

vi.mock('../api', () => ({ getContactWidget: getContactWidgetMock }))

const ENABLED_OPTION = JSON.stringify({
  enabled: true,
  default_open: true,
  title: 'Contact Us',
  description: 'Weekdays 9:00 - 18:00',
  contacts: [
    {
      id: 1,
      name: 'WeChat Support',
      icon: 'wechat',
      contact_id: 'newapi_wechat',
      link: '',
    },
    {
      id: 2,
      name: 'Telegram Group',
      icon: 'telegram',
      contact_id: '',
      link: 'https://example.com/chat',
    },
  ],
})

const UPDATED_OPTION = JSON.stringify({
  enabled: true,
  default_open: true,
  title: 'Contact Us',
  description: '',
  contacts: [
    {
      id: 3,
      name: 'Discord Server',
      icon: 'discord',
      contact_id: 'newapi_discord',
      link: '',
    },
  ],
})

function respondWith(data: string) {
  getContactWidgetMock.mockResolvedValue({
    success: true,
    message: '',
    data,
  } satisfies ContactWidgetResponse)
}

function contactRow(name: string): HTMLElement {
  const row = screen.getByText(name).closest('li')
  if (!row) throw new Error(`missing contact row for ${name}`)
  return row as HTMLElement
}

beforeEach(() => {
  window.localStorage.clear()
  respondWith(ENABLED_OPTION)
})

afterEach(() => {
  getContactWidgetMock.mockReset()
})

test('renders nothing while the widget is disabled', async () => {
  respondWith(JSON.stringify({ ...JSON.parse(ENABLED_OPTION), enabled: false }))
  render(<ContactWidget />)
  await act(async () => {})

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Open contact widget' })
  ).not.toBeInTheDocument()
})

test('renders nothing when no contacts are configured', async () => {
  respondWith(JSON.stringify({ ...JSON.parse(ENABLED_OPTION), contacts: [] }))
  render(<ContactWidget />)
  await act(async () => {})

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Open contact widget' })
  ).not.toBeInTheDocument()
})

test('shows the expanded contact card by default without the round button', async () => {
  render(<ContactWidget />)

  const panel = await screen.findByRole('dialog', { name: 'Contact Us' })
  expect(within(panel).getByText('Weekdays 9:00 - 18:00')).toBeVisible()
  expect(within(panel).getByText('WeChat Support')).toBeVisible()
  expect(within(panel).getByText('Telegram Group')).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Open contact widget' })
  ).not.toBeInTheDocument()
})

test('starts collapsed when the admin disables the default expansion', async () => {
  respondWith(
    JSON.stringify({ ...JSON.parse(ENABLED_OPTION), default_open: false })
  )
  const user = userEvent.setup()
  render(<ContactWidget />)

  const toggle = await screen.findByRole('button', {
    name: 'Open contact widget',
  })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

  await user.click(toggle)
  expect(screen.getByRole('dialog', { name: 'Contact Us' })).toBeVisible()
})

test('closes the card over the round button and restores it on click', async () => {
  const user = userEvent.setup()
  render(<ContactWidget />)

  await screen.findByRole('dialog', { name: 'Contact Us' })
  await user.click(screen.getByRole('button', { name: 'Close' }))

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  const expand = screen.getByRole('button', { name: 'Open contact widget' })
  expect(expand).toHaveAttribute('aria-expanded', 'false')

  await user.click(expand)
  expect(screen.getByRole('dialog', { name: 'Contact Us' })).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Open contact widget' })
  ).not.toBeInTheDocument()
})

test('shows a copy button only for contacts that have a contact ID', async () => {
  render(<ContactWidget />)
  await screen.findByRole('dialog', { name: 'Contact Us' })

  expect(
    within(contactRow('WeChat Support')).getByRole('button', {
      name: 'Copy to clipboard',
    })
  ).toBeInTheDocument()
  expect(
    within(contactRow('Telegram Group')).queryByRole('button', {
      name: 'Copy to clipboard',
    })
  ).not.toBeInTheDocument()
})

test('opens contact links in a new tab without opener access', async () => {
  render(<ContactWidget />)
  await screen.findByRole('dialog', { name: 'Contact Us' })

  const link = within(contactRow('Telegram Group')).getByRole('link', {
    name: 'Open Telegram Group link',
  })
  expect(link).toHaveAttribute('href', 'https://example.com/chat')
  expect(link).toHaveAttribute('target', '_blank')
  expect(link).toHaveAttribute('rel', 'noopener noreferrer')
})

test('collapses the panel on Escape and returns focus to the round toggle', async () => {
  const user = userEvent.setup()
  render(<ContactWidget />)

  await screen.findByRole('dialog', { name: 'Contact Us' })
  await user.keyboard('{Escape}')

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  const toggle = screen.getByRole('button', { name: 'Open contact widget' })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(toggle).toHaveFocus()
})

test('keeps the panel open when clicking inside it and collapses on an outside click', async () => {
  const user = userEvent.setup()
  render(<ContactWidget />)
  await screen.findByRole('dialog', { name: 'Contact Us' })

  await user.click(screen.getByRole('dialog', { name: 'Contact Us' }))
  expect(screen.getByRole('dialog')).toBeInTheDocument()

  await user.click(document.body)
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('paints the cached config before the fresh response arrives', async () => {
  let resolveFetch: (response: ContactWidgetResponse) => void = () => undefined
  getContactWidgetMock.mockReturnValue(
    new Promise<ContactWidgetResponse>((resolve) => {
      resolveFetch = resolve
    })
  )
  window.localStorage.setItem('contact_widget', ENABLED_OPTION)

  render(<ContactWidget />)
  expect(
    within(screen.getByRole('dialog')).getByText('WeChat Support')
  ).toBeVisible()

  await act(async () => {
    resolveFetch({ success: true, message: '', data: UPDATED_OPTION })
  })
  expect(screen.queryByText('WeChat Support')).not.toBeInTheDocument()
  expect(screen.getByText('Discord Server')).toBeVisible()
})
