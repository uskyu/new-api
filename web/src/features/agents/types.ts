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
export interface AgentApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

export interface PagedAgentData<T> {
  items: T[]
  total: number
  page?: number
  page_size?: number
}

export interface AgentBootstrapStatus {
  enabled: boolean
  initialized: boolean
  migration_ready: boolean
  missing_resources?: string[]
  default_group_id: number
  default_rate: number
}

export interface AgentAdminOverview {
  agent_count: number
  group_count: number
  promo_link_count: number
  downline_user_count: number
  rebate_balance_amount: number
  rebate_total_amount: number
}

export interface AgentRebateGroup {
  id: number
  name: string
  rebate_rate: number
  status: number
  is_default: boolean
  remark?: string
}

export interface AgentPromoLink {
  id: number
  agent_user_id: number
  username?: string
  display_name?: string
  name: string
  code: string
  status: number
  landing_page?: string
  remark?: string
  created_at?: number
  updated_at?: number
}

export interface AgentProfile {
  id: number
  user_id: number
  username: string
  display_name: string
  agent_level?: number
  parent_agent_user_id?: number
  parent_agent_username?: string
  status: number
  rebate_group_id?: number
  rebate_group_name?: string
  custom_rate?: number
  effective_rate?: number
  effective_rate_source?: string
  rebate_balance_amount?: number
  rebate_frozen_amount?: number
  rebate_total_amount?: number
  remark?: string
  created_at?: number
}

export interface AgentDailyMetric {
  date: string
  agent_user_id: number
  username: string
  display_name: string
  new_user_count: number
  topup_count: number
  topup_amount: number
  topup_rebate_amount: number
  redemption_rebate_amount: number
  total_rebate_amount: number
}

export interface AgentDailyMetrics {
  start_date: string
  end_date: string
  summary: {
    new_user_count: number
    topup_count: number
    topup_amount: number
    total_rebate_amount: number
  }
  items: AgentDailyMetric[]
}

export interface SupportManagedUser {
  id: number
  username: string
  display_name: string
  quota: number
  used_quota: number
  inviter_id: number
  inviter_username?: string
  inviter_display_name?: string
  is_agent: boolean
}

export interface ChangeableAgentUser {
  id: number
  username: string
  display_name?: string
  inviter_id?: number
  inviter_username?: string
  inviter_display_name?: string
}

export interface AgentSelfSummary {
  agent_enabled: boolean
  agent_initialized: boolean
  is_agent: boolean
  profile?: AgentProfile
  recent_rebate_count: number
  recent_rebate_amount: number
  recent_adjustment_count: number
}

export interface AgentDownlineUser {
  user_id: number
  username: string
  display_name: string
  inviter_id: number
  promo_link_id: number
  promo_link_name: string
  is_agent: boolean
  topup_count: number
  topup_amount: number
  rebate_amount: number
  latest_topup_time: number
}

export interface AgentRebateRecord {
  id: number
  record_key: string
  trade_no: string
  source_type: string
  invitee_user_id: number
  pay_amount: number
  redeem_quota: number
  rebate_rate: number
  rebate_amount: number
  status: string
  settled_at: number
}

export interface AgentAdjustment {
  id: number
  agent_user_id: number
  delta_amount: number
  balance_before: number
  balance_after: number
  change_type: 'increase' | 'decrease'
  reason: string
  created_at: number
}

export type AgentWithdrawStatus = 'pending' | 'exported' | 'paid'

export interface AgentWithdrawRequest {
  id: number
  agent_user_id: number
  username: string
  display_name: string
  email: string
  amount: number
  status: AgentWithdrawStatus
  account_no_snapshot: string
  account_name_snapshot: string
  export_batch_no?: string
  external_order_no?: string
  remark?: string
  created_at: number
  processed_at?: number
}

export interface AgentWithdrawImportResult {
  processed: number
  request_ids: number[]
}

export interface AgentPageParams {
  page: number
  pageSize: number
}
