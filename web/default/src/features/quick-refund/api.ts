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

export async function getRefundBatches(): Promise<
  ApiResponse<RefundBatchPage>
> {
  const response = await api.get('/api/refund/admin/batches', {
    params: { p: 1, page_size: 20 },
  })
  return response.data
}

export async function getRefundBatch(
  id: number
): Promise<ApiResponse<RefundBatch>> {
  const response = await api.get(`/api/refund/admin/batches/${id}`)
  return response.data
}
