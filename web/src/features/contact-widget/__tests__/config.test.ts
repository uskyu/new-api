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
import { describe, expect, test } from 'vitest'

import {
  parseContactWidgetConfig,
  serializeContactWidgetConfig,
} from '../lib/config'
import type { ContactWidgetConfig } from '../types'

function wireContact(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: 'Support',
    icon: 'wechat',
    contact_id: 'newapi_support',
    link: 'https://example.com/support',
    ...overrides,
  }
}

function wireOption(
  contacts: Record<string, unknown>[],
  overrides: Record<string, unknown> = {}
): string {
  return JSON.stringify({
    enabled: true,
    default_open: true,
    title: 'Contact Us',
    description: 'Reach the team',
    contacts,
    ...overrides,
  })
}

describe('parseContactWidgetConfig', () => {
  test('returns null for an empty option value', () => {
    expect(parseContactWidgetConfig('')).toBeNull()
    expect(parseContactWidgetConfig('   ')).toBeNull()
  })

  test('returns null for malformed JSON', () => {
    expect(parseContactWidgetConfig('{not json')).toBeNull()
    expect(parseContactWidgetConfig('"ContactWidget"')).toBeNull()
  })

  test('parses a complete config and maps contact_id to contactId', () => {
    const config = parseContactWidgetConfig(
      wireOption([wireContact(), wireContact({ id: 2, name: ' Sales ' })])
    )

    expect(config).toEqual({
      enabled: true,
      defaultOpen: true,
      title: 'Contact Us',
      description: 'Reach the team',
      contacts: [
        {
          id: 1,
          name: 'Support',
          icon: 'wechat',
          contactId: 'newapi_support',
          link: 'https://example.com/support',
        },
        {
          id: 2,
          name: 'Sales',
          icon: 'wechat',
          contactId: 'newapi_support',
          link: 'https://example.com/support',
        },
      ],
    })
  })

  test('maps default_open and falls back to expanded when it is absent', () => {
    expect(
      parseContactWidgetConfig(
        wireOption([wireContact()], { default_open: false })
      )?.defaultOpen
    ).toBe(false)

    const raw = JSON.parse(wireOption([wireContact()]))
    delete raw.default_open
    expect(parseContactWidgetConfig(JSON.stringify(raw))?.defaultOpen).toBe(
      true
    )
  })

  test('returns a disabled config as-is so the stored state stays visible', () => {
    const config = parseContactWidgetConfig(
      wireOption([wireContact()], { enabled: false })
    )

    expect(config?.enabled).toBe(false)
    expect(config?.contacts).toHaveLength(1)
  })

  test('drops contacts without a contact ID and without an http(s) link', () => {
    const config = parseContactWidgetConfig(
      wireOption([
        wireContact(),
        wireContact({ id: 2, contact_id: '', link: 'javascript:alert(1)' }),
        wireContact({ id: 3, contact_id: '', link: '' }),
      ])
    )

    expect(config?.contacts.map((contact) => contact.id)).toEqual([1])
  })

  test('returns null when a field exceeds its backend limit', () => {
    expect(
      parseContactWidgetConfig(
        wireOption([wireContact({ name: 'a'.repeat(31) })])
      )
    ).toBeNull()
    expect(
      parseContactWidgetConfig(wireOption([], { title: 'a'.repeat(51) }))
    ).toBeNull()
    expect(
      parseContactWidgetConfig(
        wireOption([wireContact({ link: 'x'.repeat(501) })])
      )
    ).toBeNull()
  })

  test('returns null when more contacts are stored than the backend allows', () => {
    const contacts = Array.from({ length: 21 }, (_, index) =>
      wireContact({ id: index + 1 })
    )

    expect(parseContactWidgetConfig(wireOption(contacts))).toBeNull()
  })
})

describe('serializeContactWidgetConfig', () => {
  const config: ContactWidgetConfig = {
    enabled: true,
    defaultOpen: false,
    title: ' Contact Us ',
    description: ' Reach the team ',
    contacts: [
      {
        id: 1,
        name: ' Support ',
        icon: ' wechat ',
        contactId: ' newapi_support ',
        link: ' https://example.com/support ',
      },
    ],
  }

  test('writes the backend wire field contact_id', () => {
    const serialized = JSON.parse(serializeContactWidgetConfig(config))

    expect(serialized).toMatchObject({
      enabled: true,
      default_open: false,
      contacts: [
        {
          id: 1,
          name: 'Support',
          icon: 'wechat',
          contact_id: 'newapi_support',
          link: 'https://example.com/support',
        },
      ],
    })
    expect(serialized.contacts[0]).not.toHaveProperty('contactId')
  })

  test('trims config and contact fields before storing', () => {
    const serialized = JSON.parse(serializeContactWidgetConfig(config))

    expect(serialized.title).toBe('Contact Us')
    expect(serialized.description).toBe('Reach the team')
    expect(serialized.contacts[0].name).toBe('Support')
    expect(serialized.contacts[0].link).toBe('https://example.com/support')
  })

  test('survives a serialize/parse round trip', () => {
    const trimmed: ContactWidgetConfig = {
      enabled: true,
      defaultOpen: false,
      title: 'Contact Us',
      description: 'Reach the team',
      contacts: [
        {
          id: 1,
          name: 'Support',
          icon: 'wechat',
          contactId: 'newapi_support',
          link: 'https://example.com/support',
        },
      ],
    }

    expect(
      parseContactWidgetConfig(serializeContactWidgetConfig(trimmed))
    ).toEqual(trimmed)
  })
})
