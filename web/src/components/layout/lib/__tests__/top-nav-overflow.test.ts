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

import type { TopNavLink } from '../../types'
import { splitTopNavLinks } from '../top-nav-overflow'

describe('top navigation overflow', () => {
  test('keeps configured order across visible and overflow menus', () => {
    const links: TopNavLink[] = Array.from({ length: 7 }, (_, index) => ({
      title: `Link ${index + 1}`,
      href: `/link-${index + 1}`,
    }))

    const result = splitTopNavLinks(links, 4)

    assert.deepEqual(
      result.visible.map((link) => link.title),
      ['Link 1', 'Link 2', 'Link 3', 'Link 4']
    )
    assert.deepEqual(
      result.overflow.map((link) => link.title),
      ['Link 5', 'Link 6', 'Link 7']
    )
  })
})
