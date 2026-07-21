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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { normalizeStoredMessage } from '../lib/storage'

describe('AI console legacy storage compatibility', () => {
  test('converts legacy multimodal messages without losing image content', () => {
    const message = normalizeStoredMessage({
      id: 'legacy-message',
      role: 'user',
      content: [
        { type: 'text', text: 'describe this' },
        { type: 'image_url', image_url: { url: 'data:image/png;base64,abc' } },
      ],
      createAt: 123,
      status: 'complete',
    })

    assert.equal(message.key, 'legacy-message')
    assert.equal(message.requestText, 'describe this')
    assert.equal(message.images?.[0]?.dataUrl, 'data:image/png;base64,abc')
    assert.equal(message.createdAt, 123)
  })

  test('marks an interrupted empty assistant response as an error', () => {
    const message = normalizeStoredMessage({
      id: 'interrupted',
      role: 'assistant',
      content: '',
      status: 'loading',
    })

    assert.equal(message.status, 'error')
  })
})
