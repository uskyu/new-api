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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { ContactWidgetSection } from '../contact-widget-section'

const { mutateAsync } = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync, isPending: false }),
}))

const storedConfig = JSON.stringify({
  enabled: true,
  default_open: false,
  title: 'Contact us',
  description: 'We reply within one business day',
  contacts: [
    {
      id: 1,
      name: 'Support',
      icon: 'wechat',
      contact_id: 'support@example.com',
      link: '',
    },
  ],
})

function renderSection(defaultValue = storedConfig) {
  return render(<ContactWidgetSection defaultValue={defaultValue} />)
}

/** The dialog chrome also renders an sr-only "Close", so scope to the footer. */
function dialogSubmitButton(name: 'Add' | 'Update') {
  const footer = document.querySelector('[data-slot=dialog-footer]')
  if (!footer) throw new Error('dialog footer not found')
  return within(footer as HTMLElement).getByRole('button', { name })
}

beforeEach(() => {
  mutateAsync.mockResolvedValue({ success: true, message: '' })
})

afterEach(() => {
  vi.clearAllMocks()
})

test('renders the switches and the stored contact row without a save button', () => {
  renderSection()

  expect(screen.getByRole('switch', { name: 'Enabled' })).toBeChecked()
  expect(
    screen.getByRole('switch', { name: 'Expand by default' })
  ).not.toBeChecked()
  expect(screen.getByRole('cell', { name: 'Support' })).toBeVisible()
  expect(screen.getByText('support@example.com')).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Save Settings' })
  ).not.toBeInTheDocument()
})

test('toggling the default expansion switch saves immediately', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('switch', { name: 'Expand by default' }))

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1))
  const request = mutateAsync.mock.calls[0][0]
  expect(request.key).toBe('ContactWidget')
  expect(JSON.parse(request.value).default_open).toBe(true)
})

test('submitting the dialog with an empty name shows a field error', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  expect(await screen.findByRole('dialog')).toBeVisible()

  await user.click(dialogSubmitButton('Add'))

  expect(await screen.findByText('Name is required')).toBeVisible()
  expect(mutateAsync).not.toHaveBeenCalled()
})

test('submitting the dialog without a contact ID or link shows a field error', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  const dialog = await screen.findByRole('dialog')

  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Billing')
  await user.click(dialogSubmitButton('Add'))

  expect(
    await screen.findByText('Contact ID or link is required')
  ).toBeVisible()
  expect(dialog).toBeVisible()
  expect(mutateAsync).not.toHaveBeenCalled()
})

test('adding a contact persists it immediately with contact_id fields', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Billing')
  await user.type(
    screen.getByRole('textbox', { name: 'Contact ID' }),
    'billing@example.com'
  )
  await user.click(dialogSubmitButton('Add'))

  expect(await screen.findByRole('cell', { name: 'Billing' })).toBeVisible()

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1))
  const request = mutateAsync.mock.calls[0][0]
  expect(request.key).toBe('ContactWidget')
  expect(request.value).toContain('contact_id')
  const parsed = JSON.parse(request.value)
  expect(parsed.enabled).toBe(true)
  expect(parsed.default_open).toBe(false)
  expect(parsed.contacts).toHaveLength(2)
  expect(parsed.contacts[1]).toMatchObject({
    name: 'Billing',
    contact_id: 'billing@example.com',
  })
})

test('editing a contact persists the change immediately', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Edit' }))
  const nameInput = await screen.findByRole('textbox', { name: 'Name' })
  await user.clear(nameInput)
  await user.type(nameInput, 'Support Team')
  await user.click(dialogSubmitButton('Update'))

  expect(await screen.findByRole('cell', { name: 'Support Team' })).toBeVisible()
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1))
  const parsed = JSON.parse(mutateAsync.mock.calls[0][0].value)
  expect(parsed.contacts).toHaveLength(1)
  expect(parsed.contacts[0]).toMatchObject({
    id: 1,
    name: 'Support Team',
    contact_id: 'support@example.com',
  })
})

test('blurring the title field saves the edited title', async () => {
  const user = userEvent.setup()
  renderSection()

  const titleInput = screen.getByRole('textbox', { name: 'Widget Title' })
  await user.clear(titleInput)
  await user.type(titleInput, 'Reach us')
  await user.tab()

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1))
  expect(JSON.parse(mutateAsync.mock.calls[0][0].value).title).toBe('Reach us')
})
