import { z } from 'zod'

export const quickRefundSchema = z
  .object({
    startTime: z.string().min(1),
    endTime: z.string().min(1),
    channelIds: z.array(z.number()).min(1),
    modelNames: z.array(z.string()),
    ratio: z.coerce.number().int().min(1).max(100),
    reason: z.string().trim().min(1).max(500),
  })
  .refine((value) => new Date(value.startTime) <= new Date(value.endTime), {
    path: ['endTime'],
    message: 'End time must be after start time',
  })

export type QuickRefundFormValues = z.infer<typeof quickRefundSchema>
