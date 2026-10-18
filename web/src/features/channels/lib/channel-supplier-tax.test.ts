import { describe, expect, it } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from './channel-form'

const form = {
  ...CHANNEL_FORM_DEFAULT_VALUES,
  name: 'Supplier A',
  models: 'gpt-4o',
  key: 'test-key',
  supplier_tax_rate_percent: 6.5,
}

describe('channel supplier tax configuration', () => {
  it('serializes a channel tax rate into settings on create and edit', () => {
    expect(channelFormSchema.safeParse(form).success).toBe(true)
    const created = transformFormDataToCreatePayload(form)
    const edited = transformFormDataToUpdatePayload(form, 18)
    expect(
      JSON.parse(created.channel.settings ?? '{}').supplier_tax_rate_percent
    ).toBe(6.5)
    expect(JSON.parse(edited.settings ?? '{}').supplier_tax_rate_percent).toBe(
      6.5
    )
  })

  it('rejects negative, excessive and non-finite tax rates', () => {
    for (const rate of [-0.1, 100.1, Infinity, Number.NaN]) {
      expect(
        channelFormSchema.safeParse({
          ...form,
          supplier_tax_rate_percent: rate,
        }).success
      ).toBe(false)
    }
  })
})
