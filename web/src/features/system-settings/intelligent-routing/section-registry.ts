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
import { createSectionRegistry } from '../utils/section-registry'

const ROUTING_SECTIONS = [
  { id: 'global-priority', titleKey: '全渠道优先级设置', build: () => null },
  { id: 'resource-pools', titleKey: '资源池分组', build: () => null },
  { id: 'pricing-groups', titleKey: '计价分组优先级', build: () => null },
] as const

export type IntelligentRoutingSectionId =
  (typeof ROUTING_SECTIONS)[number]['id']
const registry = createSectionRegistry<IntelligentRoutingSectionId, undefined>({
  sections: ROUTING_SECTIONS,
  defaultSection: 'global-priority',
  basePath: '/system-settings/intelligent-routing',
  urlStyle: 'path',
})
export const INTELLIGENT_ROUTING_SECTION_IDS = registry.sectionIds
export const getIntelligentRoutingSectionNavItems = registry.getSectionNavItems
