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
import type { StatusVariant } from '@/components/status-badge'

export const DEFAULT_LEDGER_PAGE_SIZE = 20

// Status presentation reuses the project's status vocabulary. Each entry names an
// i18n key and one of the shared StatusBadge variants, so the ledger matches the
// usage-log page instead of introducing a second badge style.
export const LEDGER_STATUS_PRESENTATION: Record<
  string,
  { labelKey: string; variant: StatusVariant }
> = {
  settled: { labelKey: 'Settled', variant: 'success' },
  pending: { labelKey: 'Pending', variant: 'warning' },
  refunded: { labelKey: 'Refunded', variant: 'info' },
  failed: { labelKey: 'Failed', variant: 'danger' },
}
