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

import {
  getVisibleHeaderNavCustomLinks,
  isExternalTopNavHref,
  isSafeTopNavHref,
  MAX_CUSTOM_TOP_NAV_LINKS,
  parseHeaderNavModules,
} from '../nav-modules'

describe('custom top navigation links', () => {
  test('preserves valid links, order, and disabled state', () => {
    const config = parseHeaderNavModules({
      customLinks: [
        {
          id: 'support',
          title: ' Support ',
          url: ' /support ',
          enabled: false,
        },
        {
          id: 'status',
          title: 'Status',
          url: 'https://status.example.com',
          enabled: true,
        },
      ],
    })

    assert.deepEqual(config.customLinks, [
      {
        id: 'support',
        title: 'Support',
        url: '/support',
        enabled: false,
      },
      {
        id: 'status',
        title: 'Status',
        url: 'https://status.example.com',
        enabled: true,
      },
    ])
  })

  test('rejects unsafe and protocol-relative links', () => {
    const config = parseHeaderNavModules({
      customLinks: [
        { title: 'Script', url: 'javascript:alert(1)', enabled: true },
        { title: 'External', url: '//example.com', enabled: true },
        { title: 'Safe', url: '/safe', enabled: true },
      ],
    })

    assert.deepEqual(
      config.customLinks.map((link) => link.title),
      ['Safe']
    )
    assert.equal(isSafeTopNavHref('javascript:alert(1)'), false)
    assert.equal(isSafeTopNavHref('//example.com'), false)
  })

  test('limits stored custom links before rendering', () => {
    const config = parseHeaderNavModules({
      customLinks: Array.from(
        { length: MAX_CUSTOM_TOP_NAV_LINKS + 3 },
        (_, index) => ({
          title: `Link ${index}`,
          url: `/link-${index}`,
          enabled: true,
        })
      ),
    })

    assert.equal(config.customLinks.length, MAX_CUSTOM_TOP_NAV_LINKS)
  })

  test('opens only HTTP and HTTPS links as external destinations', () => {
    assert.equal(isExternalTopNavHref('/support'), false)
    assert.equal(isExternalTopNavHref('https://example.com'), true)
    assert.equal(isExternalTopNavHref('http://example.com'), true)
  })

  test('hides disabled links while preserving them in the configuration', () => {
    const config = parseHeaderNavModules({
      customLinks: [
        { title: 'Hidden', url: '/hidden', enabled: false },
        { title: 'Visible', url: '/visible', enabled: true },
      ],
    })

    assert.equal(config.customLinks.length, 2)
    assert.deepEqual(
      getVisibleHeaderNavCustomLinks(config).map((link) => link.title),
      ['Visible']
    )
  })
})
