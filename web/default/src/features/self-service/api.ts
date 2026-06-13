import { api } from '@/lib/api'
import type {
  ApiResponse,
  PageResponse,
  SelfServiceAdminData,
  SelfServiceCheckResult,
  SelfServiceClaimAttempt,
  SelfServiceConfig,
  SelfServiceHistory,
  SelfServiceSelfInfo,
  SelfServiceUpgradeHistory,
  SelfServiceUpgradeRule,
} from './types'

export async function getSelfServiceInfo(): Promise<
  ApiResponse<SelfServiceSelfInfo>
> {
  const res = await api.get('/api/self-service/self/')
  return res.data
}

export async function checkSelfService(): Promise<
  ApiResponse<SelfServiceCheckResult>
> {
  const res = await api.post('/api/self-service/self/check')
  return res.data
}

export async function upgradeSelfServiceGroup(): Promise<
  ApiResponse<{ current_group: string }>
> {
  const res = await api.post('/api/self-service/self/upgrade')
  return res.data
}

export async function getSelfServiceAdminConfig(): Promise<
  ApiResponse<SelfServiceAdminData>
> {
  const res = await api.get('/api/self-service/admin/config')
  return res.data
}

export async function updateSelfServiceConfig(
  config: SelfServiceConfig
): Promise<ApiResponse<SelfServiceConfig>> {
  const res = await api.put('/api/self-service/admin/config', config)
  return res.data
}

export async function updateSelfServiceRules(
  rules: Partial<SelfServiceUpgradeRule>[]
): Promise<ApiResponse<SelfServiceUpgradeRule[]>> {
  const res = await api.put('/api/self-service/admin/upgrade-rules', { rules })
  return res.data
}

export async function getSelfServiceClaimAttempts(params: {
  p?: number
  page_size?: number
  user_id?: string
  status?: string
}): Promise<ApiResponse<PageResponse<SelfServiceClaimAttempt>>> {
  const res = await api.get('/api/self-service/admin/claim-attempts', {
    params,
  })
  return res.data
}

export async function getSelfServiceRefundHistories(params: {
  p?: number
  page_size?: number
  user_id?: string
  status?: string
}): Promise<ApiResponse<PageResponse<SelfServiceHistory>>> {
  const res = await api.get('/api/self-service/admin/refund-histories', {
    params,
  })
  return res.data
}

export async function getSelfServiceUpgradeHistories(params: {
  p?: number
  page_size?: number
  user_id?: string
}): Promise<ApiResponse<PageResponse<SelfServiceUpgradeHistory>>> {
  const res = await api.get('/api/self-service/admin/upgrade-histories', {
    params,
  })
  return res.data
}
