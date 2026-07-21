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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'

import { getUserGroups, getUserModels } from '@/features/playground/api'

import type { AiConsoleOptions } from '../types'

type UseAiConsoleOptionsProps = {
  group: string
  model: string
  onGroupChange: (group: string) => void
  onModelChange: (model: string) => void
}

export function useAiConsoleOptions(
  props: UseAiConsoleOptionsProps
): AiConsoleOptions {
  const groupsQuery = useQuery({
    queryKey: ['ai-console-groups'],
    queryFn: getUserGroups,
  })
  const modelsQuery = useQuery({
    queryKey: ['ai-console-models', props.group],
    queryFn: () => getUserModels(props.group),
    enabled: props.group !== '',
  })

  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data])
  const models = useMemo(() => modelsQuery.data ?? [], [modelsQuery.data])

  useEffect(() => {
    if (groups.length === 0) return
    if (groups.some((group) => group.value === props.group)) return
    props.onGroupChange(groups[0].value)
  }, [groups, props])

  useEffect(() => {
    if (models.length === 0) return
    if (models.some((model) => model.value === props.model)) return
    props.onModelChange(models[0].value)
  }, [models, props])

  return {
    groups,
    models,
    isLoading: groupsQuery.isLoading || modelsQuery.isLoading,
  }
}
