import { createFileRoute } from '@tanstack/react-router'
import { VideoGenerationService } from '@/features/service-checkout'

export const Route = createFileRoute('/services/video-generation/')({
  component: VideoGenerationService,
})
