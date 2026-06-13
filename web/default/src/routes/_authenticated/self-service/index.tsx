import { createFileRoute } from '@tanstack/react-router'
import { SelfService } from '@/features/self-service'

export const Route = createFileRoute('/_authenticated/self-service/')({
  component: SelfService,
})
