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

import { parseAiConsoleStreamEvent } from '../lib/stream'

describe('AI console stream parsing', () => {
  test('ignores text and JSON heartbeat events', () => {
    assert.deepEqual(parseAiConsoleStreamEvent('ping'), [{ type: 'heartbeat' }])
    assert.deepEqual(parseAiConsoleStreamEvent('{"type":"heartbeat"}'), [
      { type: 'heartbeat' },
    ])
  })

  test('preserves reasoning and content chunks from one event', () => {
    const events = parseAiConsoleStreamEvent(
      JSON.stringify({
        choices: [
          { delta: { reasoning_content: 'thinking', content: 'answer' } },
        ],
      })
    )

    assert.deepEqual(events, [
      { type: 'reasoning', chunk: 'thinking' },
      { type: 'content', chunk: 'answer' },
    ])
  })

  test('reports structured upstream errors', () => {
    assert.deepEqual(
      parseAiConsoleStreamEvent(
        JSON.stringify({ error: { message: 'unavailable', code: 'busy' } })
      ),
      [{ type: 'error', message: 'unavailable', code: 'busy' }]
    )
  })
})
