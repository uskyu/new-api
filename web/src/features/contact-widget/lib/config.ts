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
import * as z from 'zod'

import { isHttpUrl } from '@/lib/content-format'

import type { ContactWidgetConfig, ContactWidgetContact } from '../types'

// Mirrors the backend limits enforced in controller/contact_widget.go.
export const CONTACT_WIDGET_LIMITS = {
  title: 50,
  description: 200,
  contacts: 20,
  name: 30,
  icon: 200,
  contactId: 128,
  link: 500,
} as const

const contactWireSchema = z.object({
  id: z.number(),
  name: z.string().max(CONTACT_WIDGET_LIMITS.name),
  icon: z.string().max(CONTACT_WIDGET_LIMITS.icon),
  contact_id: z.string().max(CONTACT_WIDGET_LIMITS.contactId),
  link: z.string().max(CONTACT_WIDGET_LIMITS.link),
})

const contactWidgetWireSchema = z.object({
  enabled: z.boolean().default(false),
  default_open: z.boolean().default(true),
  title: z.string().max(CONTACT_WIDGET_LIMITS.title).default(''),
  description: z.string().max(CONTACT_WIDGET_LIMITS.description).default(''),
  contacts: z
    .array(contactWireSchema)
    .max(CONTACT_WIDGET_LIMITS.contacts)
    .default([]),
})

/**
 * Parses the raw option value served by GET /api/contact_widget.
 * Returns null when the value is empty or malformed; a disabled widget is
 * returned as-is so the settings form can show its stored state.
 */
export function parseContactWidgetConfig(
  raw: string
): ContactWidgetConfig | null {
  const trimmed = raw.trim()
  if (!trimmed) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return null
  }

  const result = contactWidgetWireSchema.safeParse(parsed)
  if (!result.success) return null

  const contacts: ContactWidgetContact[] = []
  for (const contact of result.data.contacts) {
    const name = contact.name.trim()
    const contactId = contact.contact_id.trim()
    const link = contact.link.trim()
    // Keep only actionable entries with a safe link, matching the backend rule
    // that every contact needs a contact ID or an http(s) link.
    if (contactId === '' && !isHttpUrl(link)) continue
    contacts.push({
      id: contact.id,
      name,
      icon: contact.icon.trim(),
      contactId,
      link,
    })
  }

  return {
    enabled: result.data.enabled,
    defaultOpen: result.data.default_open,
    title: result.data.title.trim(),
    description: result.data.description.trim(),
    contacts,
  }
}

/** Serializes a config back to the option value the backend validates. */
export function serializeContactWidgetConfig(
  config: ContactWidgetConfig
): string {
  return JSON.stringify({
    enabled: config.enabled,
    default_open: config.defaultOpen,
    title: config.title.trim(),
    description: config.description.trim(),
    contacts: config.contacts.map((contact) => ({
      id: contact.id,
      name: contact.name.trim(),
      icon: contact.icon.trim(),
      contact_id: contact.contactId.trim(),
      link: contact.link.trim(),
    })),
  })
}

export function createEmptyContactWidgetConfig(): ContactWidgetConfig {
  return {
    enabled: false,
    defaultOpen: true,
    title: '',
    description: '',
    contacts: [],
  }
}

export function createEmptyContact(id: number): ContactWidgetContact {
  return { id, name: '', icon: '', contactId: '', link: '' }
}
