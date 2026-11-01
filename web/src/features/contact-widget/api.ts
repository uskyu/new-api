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
import { api } from '@/lib/api'

import type { ContactWidgetResponse } from './types'

// ============================================================================
// Contact Widget APIs
// ============================================================================

/**
 * Get the admin-configured contact widget.
 * Returns the raw option JSON string (empty when not configured).
 */
export async function getContactWidget(): Promise<ContactWidgetResponse> {
  // Same ETag revalidation contract as the home page content endpoint: the
  // global no-store header is dropped so the browser can revalidate, while
  // the server keeps admin edits immediately visible.
  const res = await api.get('/api/contact_widget', {
    headers: { 'Cache-Control': null },
  })
  return res.data
}
