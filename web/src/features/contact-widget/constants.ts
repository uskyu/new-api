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
import {
  Globe,
  Headphones,
  LifeBuoy,
  Link2,
  Mail,
  MessageSquare,
  Phone,
  QrCode,
} from 'lucide-react'
import type { ComponentType } from 'react'
import {
  SiAlipay,
  SiAndroid,
  SiApple,
  SiBilibili,
  SiDiscord,
  SiFacebook,
  SiGithub,
  SiGmail,
  SiGooglechat,
  SiInstagram,
  SiKakaotalk,
  SiLine,
  SiMessenger,
  SiNotion,
  SiPaypal,
  SiProtonmail,
  SiQq,
  SiReddit,
  SiSignal,
  SiSinaweibo,
  SiSnapchat,
  SiTelegram,
  SiTiktok,
  SiViber,
  SiWechat,
  SiWhatsapp,
  SiX,
  SiYoutube,
  SiZhihu,
  SiZoom,
} from 'react-icons/si'

type ContactIconComponent = ComponentType<{
  size?: number | string
  className?: string
}>

export type ContactIconPreset = {
  key: string
  /** i18n key; brand names double as their own translation key. */
  labelKey: string
  Icon: ContactIconComponent
}

// Curated presets covering mainstream contact channels. Contact entries may
// still use any other value: an http(s) image URL renders the image, and
// anything else falls back to a letter badge so unusual apps stay supported.
export const CONTACT_ICON_PRESETS: ContactIconPreset[] = [
  { key: 'wechat', labelKey: 'WeChat', Icon: SiWechat },
  { key: 'qq', labelKey: 'QQ', Icon: SiQq },
  { key: 'telegram', labelKey: 'Telegram', Icon: SiTelegram },
  { key: 'discord', labelKey: 'Discord', Icon: SiDiscord },
  { key: 'whatsapp', labelKey: 'WhatsApp', Icon: SiWhatsapp },
  { key: 'instagram', labelKey: 'Instagram', Icon: SiInstagram },
  { key: 'x', labelKey: 'X', Icon: SiX },
  { key: 'facebook', labelKey: 'Facebook', Icon: SiFacebook },
  { key: 'messenger', labelKey: 'Messenger', Icon: SiMessenger },
  { key: 'line', labelKey: 'Line', Icon: SiLine },
  { key: 'kakaotalk', labelKey: 'KakaoTalk', Icon: SiKakaotalk },
  { key: 'youtube', labelKey: 'YouTube', Icon: SiYoutube },
  { key: 'github', labelKey: 'GitHub', Icon: SiGithub },
  { key: 'tiktok', labelKey: 'TikTok', Icon: SiTiktok },
  { key: 'bilibili', labelKey: 'Bilibili', Icon: SiBilibili },
  { key: 'zhihu', labelKey: 'Zhihu', Icon: SiZhihu },
  { key: 'weibo', labelKey: 'Weibo', Icon: SiSinaweibo },
  { key: 'reddit', labelKey: 'Reddit', Icon: SiReddit },
  { key: 'snapchat', labelKey: 'Snapchat', Icon: SiSnapchat },
  { key: 'signal', labelKey: 'Signal', Icon: SiSignal },
  { key: 'viber', labelKey: 'Viber', Icon: SiViber },
  { key: 'zoom', labelKey: 'Zoom', Icon: SiZoom },
  { key: 'notion', labelKey: 'Notion', Icon: SiNotion },
  { key: 'gmail', labelKey: 'Gmail', Icon: SiGmail },
  { key: 'googlechat', labelKey: 'Google Chat', Icon: SiGooglechat },
  { key: 'protonmail', labelKey: 'ProtonMail', Icon: SiProtonmail },
  { key: 'paypal', labelKey: 'PayPal', Icon: SiPaypal },
  { key: 'alipay', labelKey: 'Alipay', Icon: SiAlipay },
  { key: 'apple', labelKey: 'Apple', Icon: SiApple },
  { key: 'android', labelKey: 'Android', Icon: SiAndroid },
  { key: 'mail', labelKey: 'Email', Icon: Mail },
  { key: 'phone', labelKey: 'Phone', Icon: Phone },
  { key: 'website', labelKey: 'Website', Icon: Globe },
  { key: 'message', labelKey: 'Message', Icon: MessageSquare },
  { key: 'support', labelKey: 'Customer Service', Icon: Headphones },
  { key: 'help', labelKey: 'Help', Icon: LifeBuoy },
  { key: 'link', labelKey: 'Link', Icon: Link2 },
  { key: 'qrcode', labelKey: 'QR Code', Icon: QrCode },
]

export function findContactIconPreset(
  key: string
): ContactIconPreset | undefined {
  return CONTACT_ICON_PRESETS.find((preset) => preset.key === key)
}
