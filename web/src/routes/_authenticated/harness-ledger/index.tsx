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
import { createFileRoute, redirect } from '@tanstack/react-router'
import z from 'zod'

import { HarnessLedger } from '@/features/harness-ledger'
import { LEDGER_STATUS_VALUES } from '@/features/harness-ledger/types'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

const ledgerSearchSchema = z.object({
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(20),
  request_id: z.string().optional().catch(undefined),
  model_name: z.string().optional().catch(undefined),
  status: z.array(z.enum(LEDGER_STATUS_VALUES)).optional().catch(undefined),
  start_time: z.number().optional().catch(undefined),
  end_time: z.number().optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/harness-ledger/')({
  // The ledger shows every request the gateway booked, including internal ones the
  // platform feed deliberately excludes, so it is restricted to the root
  // administrator rather than to admins in general.
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.SUPER_ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  validateSearch: ledgerSearchSchema,
  component: HarnessLedger,
})
