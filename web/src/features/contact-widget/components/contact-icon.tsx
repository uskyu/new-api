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
import { isHttpUrl } from '@/lib/content-format'

import { findContactIconPreset } from '../constants'

const DEFAULT_SIZE = 28

/**
 * Renders a contact entry icon: a preset brand icon, an http(s) image URL,
 * or a letter badge built from the contact name. All variants are decorative
 * because the contact name is always rendered next to them.
 */
export function ContactIcon(props: {
  icon: string
  name: string
  size?: number
}) {
  const size = props.size ?? DEFAULT_SIZE
  const preset = findContactIconPreset(props.icon)

  if (preset) {
    const PresetIcon = preset.Icon
    return <PresetIcon size={size} className='shrink-0' aria-hidden />
  }

  if (isHttpUrl(props.icon)) {
    return (
      <img
        src={props.icon}
        width={size}
        height={size}
        className='shrink-0 object-contain'
        alt=''
        aria-hidden='true'
      />
    )
  }

  const initial = props.name.trim().charAt(0).toUpperCase()
  return (
    <span
      aria-hidden='true'
      className='bg-muted text-muted-foreground flex shrink-0 items-center justify-center rounded-full font-medium'
      style={{
        width: size,
        height: size,
        fontSize: Math.max(10, Math.round(size * 0.45)),
      }}
    >
      {initial}
    </span>
  )
}
