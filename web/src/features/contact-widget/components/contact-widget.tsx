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
import { ExternalLink, MessageSquare, X } from 'lucide-react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { isHttpUrl } from '@/lib/content-format'
import { cn } from '@/lib/utils'

import { useContactWidget } from '../hooks/use-contact-widget'
import { ContactIcon } from './contact-icon'

const PANEL_TRANSITION = { duration: 0.16 }

/**
 * Floating contact widget for the public landing page. The round button and
 * the contact card are mutually exclusive: opening the card places it exactly
 * over the button's spot, and closing the card brings the button back. The
 * card starts expanded or collapsed according to the admin configuration.
 * Renders nothing until the widget is enabled with contacts.
 */
export function ContactWidget() {
  const { t } = useTranslation()
  const { config } = useContactWidget()
  // The visitor's explicit toggle wins over the configured default.
  const [userToggled, setUserToggled] = useState<boolean | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const toggleRef = useRef<HTMLButtonElement>(null)
  const wasOpenRef = useRef(false)
  const panelId = useId()
  const shouldReduceMotion = useReducedMotion()

  const isOpen = userToggled ?? config?.defaultOpen ?? true

  useEffect(() => {
    if (!isOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setUserToggled(false)
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [isOpen])

  useEffect(() => {
    if (!isOpen) return
    const onPointerDown = (event: PointerEvent) => {
      if (containerRef.current?.contains(event.target as Node)) return
      setUserToggled(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [isOpen])

  // Return focus to the round button once the panel collapses so keyboard
  // users keep their place instead of dropping to the document body.
  useEffect(() => {
    if (wasOpenRef.current && !isOpen) {
      toggleRef.current?.focus()
    }
    wasOpenRef.current = isOpen
  }, [isOpen])

  if (!config?.enabled || config.contacts.length === 0) return null

  const renderContactActions = (
    contactId: string,
    link: string,
    name: string
  ) => (
    <div className='flex shrink-0 items-center gap-1'>
      {contactId !== '' && <CopyButton value={contactId} />}
      {isHttpUrl(link) && (
        <a
          href={link}
          target='_blank'
          rel='noopener noreferrer'
          aria-label={t('Open {{name}} link', { name })}
          className='text-muted-foreground hover:text-foreground inline-flex shrink-0 transition-colors'
        >
          <ExternalLink className='size-4' />
        </a>
      )}
    </div>
  )

  const panelBody = (
    <>
      <div className='mb-2 flex items-start justify-between gap-1 sm:mb-3 sm:gap-2'>
        <div className='min-w-0'>
          <h2 className='truncate text-sm font-semibold sm:text-base'>
            {config.title}
          </h2>
          {config.description !== '' && (
            <p className='text-muted-foreground mt-0.5 line-clamp-2 text-xs max-sm:hidden'>
              {config.description}
            </p>
          )}
        </div>
        <Button
          variant='ghost'
          size='icon'
          className='size-7 shrink-0 sm:size-8'
          aria-label={t('Close')}
          onClick={() => setUserToggled(false)}
        >
          <X className='size-3.5' />
        </Button>
      </div>
      <ul className='flex flex-col gap-1.5 sm:gap-2'>
        {config.contacts.map((contact) => (
          <li
            key={contact.id}
            className='flex items-center gap-2 rounded-lg border p-1.5 sm:gap-3 sm:rounded-xl sm:p-2.5'
          >
            <span className='bg-muted flex size-7 shrink-0 items-center justify-center rounded-md sm:size-9 sm:rounded-lg'>
              <ContactIcon icon={contact.icon} name={contact.name} size={16} />
            </span>
            <div className='min-w-0 flex-1'>
              <p className='truncate text-xs font-medium sm:text-sm'>
                {contact.name}
              </p>
              {contact.contactId !== '' && (
                <p className='text-muted-foreground truncate font-mono text-[10px] sm:text-xs'>
                  {contact.contactId}
                </p>
              )}
            </div>
            {renderContactActions(
              contact.contactId,
              contact.link,
              contact.name
            )}
          </li>
        ))}
      </ul>
    </>
  )

  // The card covers the round button's spot and stays a compact corner card on
  // every screen size; on phones it is deliberately small so it does not block
  // the page, and long contact lists scroll inside the card.
  const panelClassName = cn(
    'absolute right-0 bottom-0 max-h-[40dvh] w-[min(12rem,calc(100vw-2rem))] overflow-y-auto rounded-xl border bg-popover p-3 text-popover-foreground shadow-lg sm:max-h-[calc(100dvh-8rem)] sm:w-80 sm:rounded-2xl sm:p-4'
  )

  return (
    <div
      ref={containerRef}
      className='fixed right-4 bottom-[calc(1rem+env(safe-area-inset-bottom))] z-50 sm:right-6 sm:bottom-[calc(1.5rem+env(safe-area-inset-bottom))]'
    >
      {isOpen && shouldReduceMotion ? (
        <div
          id={panelId}
          role='dialog'
          aria-label={config.title}
          className={panelClassName}
        >
          {panelBody}
        </div>
      ) : (
        <AnimatePresence>
          {isOpen && (
            <motion.div
              id={panelId}
              role='dialog'
              aria-label={config.title}
              className={panelClassName}
              initial={{ opacity: 0, y: 8, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 8, scale: 0.96 }}
              transition={PANEL_TRANSITION}
            >
              {panelBody}
            </motion.div>
          )}
        </AnimatePresence>
      )}
      {!isOpen && (
        <Button
          ref={toggleRef}
          size='icon'
          className='size-12 rounded-full shadow-lg'
          aria-expanded={isOpen}
          aria-controls={panelId}
          aria-label={t('Open contact widget')}
          onClick={() => setUserToggled(true)}
        >
          <MessageSquare className='size-5' />
        </Button>
      )}
    </div>
  )
}
