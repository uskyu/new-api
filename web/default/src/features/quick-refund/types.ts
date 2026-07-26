export type ApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}

export type RefundChannel = {
  id: number
  name: string
}

export type RefundOptions = {
  channels: RefundChannel[]
  models: string[]
}

export type RefundPayload = {
  start_time: number
  end_time: number
  channel_ids: number[]
  model_names: string[]
  ratio: number
  reason: string
}

export type RefundBreakdown = {
  items: number
  source_quota: number
  refund_quota: number
}

export type RefundPreview = {
  matched_items: number
  source_quota: number
  refund_quota: number
  skipped: number
  by_channel: Record<string, RefundBreakdown>
  by_model: Record<string, RefundBreakdown>
}

export type RefundItem = {
  id: number
  source_log_id: number
  user_id: number
  channel_id: number
  model_name: string
  source_quota: number
  refund_quota: number
  status: string
  message: string
}

export type RefundBatch = {
  id: number
  start_time: number
  end_time: number
  channel_ids: string
  model_names: string
  ratio: number
  reason: string
  operator_id: number
  status: string
  total_items: number
  success_items: number
  skipped_items: number
  failed_items: number
  total_quota: number
  refunded_quota: number
  error: string
  created_at: number
  updated_at: number
  items?: RefundItem[]
  item_page: number
  item_page_size: number
  item_total: number
}

export type RefundBatchPage = {
  page: number
  page_size: number
  total: number
  items: RefundBatch[]
}
