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

import { parseInactiveDays } from '../inactive-days'

describe('custom inactive day validation', () => {
  test('accepts whole day values within the supported range', () => {
    assert.equal(parseInactiveDays('1'), 1)
    assert.equal(parseInactiveDays(' 120 '), 120)
    assert.equal(parseInactiveDays('3650'), 3650)
  })

  test('rejects values that cannot form a safe inactivity query', () => {
    for (const value of ['', '0', '-1', '1.5', '3651', '1e2']) {
      assert.equal(parseInactiveDays(value), null)
    }
  })
})
