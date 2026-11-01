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
function dialogSubmitButton() {
  const footer = document.querySelector('[data-slot=dialog-footer]')
  if (!footer) throw new Error('dialog footer not found')
  return within(footer as HTMLElement).getByRole('button', { name: 'Add' })
}

beforeEach(() => {
  mutateAsync.mockResolvedValue({ success: true, message: '' })
})

afterEach(() => {
  vi.clearAllMocks()
})

test('renders the enabled switch and the stored contact row', () => {
  renderSection()

  expect(screen.getByRole('switch', { name: 'Enabled' })).toBeChecked()
  expect(
    screen.getByRole('switch', { name: 'Expand by default' })
  ).not.toBeChecked()
  expect(screen.getByRole('cell', { name: 'Support' })).toBeVisible()
  expect(screen.getByText('support@example.com')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Save Settings' })).toBeDisabled()
})

test('toggling the default expansion switch marks the form dirty', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('switch', { name: 'Expand by default' }))

  expect(
    screen.getByRole('switch', { name: 'Expand by default' })
  ).toBeChecked()
  expect(screen.getByRole('button', { name: 'Save Settings' })).toBeEnabled()
})

test('submitting the dialog with an empty name shows a field error', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  expect(await screen.findByRole('dialog')).toBeVisible()

  await user.click(dialogSubmitButton())

  expect(await screen.findByText('Name is required')).toBeVisible()
  expect(mutateAsync).not.toHaveBeenCalled()
})

test('submitting the dialog without a contact ID or link shows a field error', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  const dialog = await screen.findByRole('dialog')

  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Billing')
  await user.click(dialogSubmitButton())

  expect(
    await screen.findByText('Contact ID or link is required')
  ).toBeVisible()
  expect(dialog).toBeVisible()
  expect(mutateAsync).not.toHaveBeenCalled()
})

test('adding a contact with a name and contact ID appends a row and enables saving', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Billing')
  await user.type(
    screen.getByRole('textbox', { name: 'Contact ID' }),
    'billing@example.com'
  )
  await user.click(dialogSubmitButton())

  expect(await screen.findByRole('cell', { name: 'Billing' })).toBeVisible()
  expect(screen.getByText('billing@example.com')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Save Settings' })).toBeEnabled()
})

test('saving persists the contacts as a JSON value with contact_id fields', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.click(screen.getByRole('button', { name: 'Add Contact' }))
  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Billing')
  await user.type(
    screen.getByRole('textbox', { name: 'Contact ID' }),
    'billing@example.com'
  )
  await user.click(dialogSubmitButton())
  await user.click(screen.getByRole('button', { name: 'Save Settings' }))

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
