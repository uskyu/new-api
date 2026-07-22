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

import { USER_ROLE } from '../constants'
import { getPromotableUserRoles } from '../lib/role-options'

describe('user promotion role options', () => {
  test('lets an administrator promote a common user only to support', () => {
    assert.deepEqual(getPromotableUserRoles(USER_ROLE.USER, USER_ROLE.ADMIN), [
      USER_ROLE.SUPPORT,
    ])
  })

  test('lets a root user choose support or admin for a common user', () => {
    assert.deepEqual(getPromotableUserRoles(USER_ROLE.USER, USER_ROLE.ROOT), [
      USER_ROLE.SUPPORT,
      USER_ROLE.ADMIN,
    ])
  })

  test('does not expose an equal-rank promotion target', () => {
    assert.deepEqual(
      getPromotableUserRoles(USER_ROLE.SUPPORT, USER_ROLE.ADMIN),
      []
    )
  })
})
