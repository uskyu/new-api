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
import type {
  ChatCompletionMessage,
  GroupOption,
  Message,
  ModelOption,
} from '@/features/playground/types'

export type { GroupOption, ModelOption } from '@/features/playground/types'

export type AiConsoleReasoningEffort = '' | 'low' | 'medium' | 'high'

export type AiConsoleImage = {
  id: string
  dataUrl: string
  name: string
}

export type AiConsoleFile = {
  id: string
  filename: string
  markdown: string
  warnings: string[]
  size: number
  type: string
}

export type AiConsoleMessage = Message & {
  requestText?: string
  images?: AiConsoleImage[]
  files?: Array<Pick<AiConsoleFile, 'filename' | 'size' | 'warnings'>>
}

export type AiConsoleSession = {
  id: string
  title: string
  model: string
  group: string
  reasoningEffort?: AiConsoleReasoningEffort
  customTitle: boolean
  updatedAt: number
  lastMessagePreview: string
}

export type AiConsolePreferences = {
  selectedModel?: string
  selectedGroup?: string
  reasoningEffort?: AiConsoleReasoningEffort
}

export type AiConsoleRequest = {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: true
  reasoning_effort?: Exclude<AiConsoleReasoningEffort, ''>
}

export type AiConsoleOptions = {
  groups: GroupOption[]
  models: ModelOption[]
  isLoading: boolean
}

export type AiConsoleFileParseResult = {
  filename: string
  markdown: string
  warnings?: string[]
}
