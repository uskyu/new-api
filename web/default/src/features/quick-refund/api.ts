import { api } from '@/lib/api'
import type {
  ApiResponse,
  RefundBatch,
  RefundBatchPage,
  RefundOptions,
  RefundPayload,
  RefundPreview,
} from './types'

export async function getRefundOptions(
  startTime: number,
  endTime: number
): Promise<ApiResponse<RefundOptions>> {
  const response = await api.get('/api/refund/admin/options', {
    params: { start_time: startTime, end_time: endTime },
  })
  return response.data
}

export async function previewRefund(
  payload: RefundPayload
): Promise<ApiResponse<RefundPreview>> {
  const response = await api.post('/api/refund/admin/preview', payload)
  return response.data
}

export async function createRefundBatch(
  payload: RefundPayload & { idempotency_key: string }
): Promise<ApiResponse<RefundBatch>> {
  const response = await api.post('/api/refund/admin/batches', payload)
  return response.data
}

export async function getRefundBatches(
  page = 1,
  pageSize = 20
): Promise<ApiResponse<RefundBatchPage>> {
  const response = await api.get('/api/refund/admin/batches', {
    params: { p: page, page_size: pageSize },
  })
  return response.data
}

export async function getRefundBatch(
  id: number,
  itemPage = 1,
  itemPageSize = 20
): Promise<ApiResponse<RefundBatch>> {
  const response = await api.get(`/api/refund/admin/batches/${id}`, {
    params: { item_page: itemPage, item_page_size: itemPageSize },
  })
  return response.data
}
