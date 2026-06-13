export type SelfServiceConfig = {
  enabled: boolean
  lookback_hours: number
  max_refunds_per_claim: number
  daily_refund_limit: number
  display_limit: number
  refund_percent: number
  exclude_models: string
  scan_limit: number
}

export type SelfServiceUpgradeRule = {
  id: number
  threshold_quota: number
  target_group: string
  description: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export type SelfServiceUpgradeOffer = {
  rule_id: number
  target_group: string
  description: string
  threshold_quota: number
}

export type SelfServiceIssueRecord = {
  log_id: number
  created_at: number
  model_name: string
  quota: number
  refunded_quota: number
  request_id: string
  status: string
}

export type SelfServiceSelfInfo = {
  enabled: boolean
  user_id: number
  username: string
  current_group: string
  quota: number
  used_quota: number
  total_quota: number
  total_refunded_quota: number
  config: SelfServiceConfig
  upgrade_offer: SelfServiceUpgradeOffer | null
  upgrade_rules: SelfServiceUpgradeRule[]
}

export type SelfServiceCheckResult = SelfServiceSelfInfo & {
  scanned_count: number
  candidate_count: number
  new_candidate_count: number
  refunded_count: number
  refunded_quota: number
  remaining_daily: number
  records: SelfServiceIssueRecord[]
  message: string
}

export type SelfServiceHistory = {
  id: number
  user_id: number
  username: string
  log_id: number
  model_name: string
  original_quota: number
  refunded_quota: number
  request_id: string
  status: string
  message: string
  created_at: number
}

export type SelfServiceClaimAttempt = {
  id: number
  user_id: number
  username: string
  scanned_count: number
  candidate_count: number
  new_candidate_count: number
  refunded_count: number
  refunded_quota: number
  status: string
  message: string
  created_at: number
}

export type SelfServiceUpgradeHistory = {
  id: number
  user_id: number
  username: string
  from_group: string
  to_group: string
  total_quota: number
  rule_id: number
  status: string
  message: string
  created_at: number
}

export type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type PageResponse<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type SelfServiceAdminData = {
  config: SelfServiceConfig
  rules: SelfServiceUpgradeRule[]
  stats: Record<string, number>
}
