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
import { FileText, ImagePlus, Send, Square, X } from 'lucide-react'
import { nanoid } from 'nanoid'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInput,
  PromptInputButton,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { parseAiConsoleFile } from '../api'
import type {
  AiConsoleFile,
  AiConsoleImage,
  AiConsoleReasoningEffort,
  GroupOption,
  ModelOption,
} from '../types'

const MAX_IMAGES = 5
const MAX_IMAGE_BYTES = 10 << 20

type ConsoleInputProps = {
  disabled: boolean
  isGenerating: boolean
  isLoadingOptions: boolean
  models: ModelOption[]
  groups: GroupOption[]
  model: string
  group: string
  reasoningEffort: AiConsoleReasoningEffort
  onModelChange: (value: string) => void
  onGroupChange: (value: string) => void
  onReasoningEffortChange: (value: AiConsoleReasoningEffort) => void
  onSend: (
    text: string,
    images: AiConsoleImage[],
    files: AiConsoleFile[]
  ) => void
  onStop: () => void
}

function readImage(file: File): Promise<AiConsoleImage> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () =>
        resolve({
          id: nanoid(),
          dataUrl: String(reader.result || ''),
          name: file.name || 'image',
        }),
      { once: true }
    )
    reader.addEventListener('error', () => reject(reader.error), {
      once: true,
    })
    reader.readAsDataURL(file)
  })
}

export function ConsoleInput(props: ConsoleInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [images, setImages] = useState<AiConsoleImage[]>([])
  const [files, setFiles] = useState<AiConsoleFile[]>([])
  const [isParsing, setIsParsing] = useState(false)
  const imageInputRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const addImages = async (imageFiles: File[]) => {
    const capacity = Math.max(0, MAX_IMAGES - images.length)
    if (capacity === 0) {
      toast.error(
        t('Up to {{count}} images are supported', { count: MAX_IMAGES })
      )
      return
    }
    const accepted = imageFiles
      .filter((file) => {
        if (file.size <= MAX_IMAGE_BYTES) return true
        toast.error(t('Each image must not exceed 10 MB'))
        return false
      })
      .slice(0, capacity)
    const nextImages = await Promise.all(accepted.map(readImage))
    setImages((current) => [...current, ...nextImages])
  }

  const addDocuments = async (documentFiles: File[]) => {
    if (documentFiles.length === 0) return
    setIsParsing(true)
    try {
      for (const file of documentFiles) {
        try {
          const parsed = await parseAiConsoleFile(file)
          setFiles((current) => [
            ...current,
            {
              id: nanoid(),
              filename: parsed.filename,
              markdown: parsed.markdown,
              warnings: parsed.warnings || [],
              size: file.size,
              type: file.type,
            },
          ])
        } catch (error) {
          toast.error(
            error instanceof Error ? t(error.message) : t('File parsing failed')
          )
        }
      }
    } finally {
      setIsParsing(false)
    }
  }

  const handlePaste = (event: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const clipboardFiles = [...event.clipboardData.files]
    if (clipboardFiles.length === 0) return
    event.preventDefault()
    const imageFiles = clipboardFiles.filter((file) =>
      file.type.startsWith('image/')
    )
    const documentFiles = clipboardFiles.filter(
      (file) => !file.type.startsWith('image/')
    )
    void addImages(imageFiles)
    void addDocuments(documentFiles)
  }

  const submit = (_message: PromptInputMessage) => {
    if (props.disabled) return
    if (!text.trim() && images.length === 0 && files.length === 0) return
    props.onSend(text, images, files)
    setText('')
    setImages([])
    setFiles([])
  }

  const canSubmit =
    !props.disabled &&
    !isParsing &&
    Boolean(props.model) &&
    Boolean(text.trim() || images.length > 0 || files.length > 0)

  return (
    <div className='mx-auto w-full max-w-4xl px-2 pb-2 sm:px-4 sm:pb-4'>
      <PromptInput
        className='relative'
        groupClassName='bg-background border-border rounded-xl shadow-lg overflow-hidden'
        onSubmit={submit}
      >
        {images.length > 0 || files.length > 0 ? (
          <div className='flex max-h-36 flex-wrap gap-2 overflow-y-auto border-b p-3'>
            {images.map((image) => (
              <div
                className='group relative size-16 overflow-hidden rounded-lg border'
                key={image.id}
              >
                <img
                  alt={image.name}
                  className='size-full object-cover'
                  src={image.dataUrl}
                />
                <Button
                  aria-label={t('Delete image')}
                  className='absolute top-1 right-1 size-6 bg-black/70 text-white hover:bg-black'
                  onClick={() =>
                    setImages((current) =>
                      current.filter((item) => item.id !== image.id)
                    )
                  }
                  size='icon-sm'
                  type='button'
                >
                  <X className='size-3' />
                </Button>
              </div>
            ))}
            {files.map((file) => (
              <div
                className='bg-muted flex h-16 max-w-full items-center gap-2 rounded-lg border px-3'
                key={file.id}
              >
                <FileText className='text-muted-foreground size-4 shrink-0' />
                <div className='min-w-0'>
                  <div className='max-w-52 truncate text-sm font-medium'>
                    {file.filename}
                  </div>
                  <div className='text-muted-foreground max-w-52 truncate text-xs'>
                    {file.warnings.map((warning) => t(warning)).join('; ') ||
                      t('Ready')}
                  </div>
                </div>
                <Button
                  aria-label={t('Delete file')}
                  onClick={() =>
                    setFiles((current) =>
                      current.filter((item) => item.id !== file.id)
                    )
                  }
                  size='icon-sm'
                  type='button'
                  variant='ghost'
                >
                  <X className='size-3.5' />
                </Button>
              </div>
            ))}
          </div>
        ) : null}

        <PromptInputTextarea
          autoComplete='off'
          className='min-h-20 px-4 pt-4 pb-3 leading-6 md:min-h-24'
          disabled={props.disabled}
          onChange={(event) => setText(event.target.value)}
          onPaste={handlePaste}
          placeholder={t('Ask anything')}
          value={text}
        />

        <PromptInputFooter className='border-t px-2.5 py-2'>
          <div className='flex w-full flex-wrap items-center gap-1.5'>
            <input
              accept='image/*'
              className='hidden'
              multiple
              onChange={(event) => {
                void addImages([...(event.target.files || [])])
                event.target.value = ''
              }}
              ref={imageInputRef}
              type='file'
            />
            <input
              accept='.txt,.md,.csv,.json,.log,.pdf,.docx,.xlsx,.pptx'
              className='hidden'
              multiple
              onChange={(event) => {
                void addDocuments([...(event.target.files || [])])
                event.target.value = ''
              }}
              ref={fileInputRef}
              type='file'
            />

            <Tooltip>
              <TooltipTrigger
                render={
                  <PromptInputButton
                    aria-label={t('Upload image')}
                    disabled={props.disabled}
                    onClick={() => imageInputRef.current?.click()}
                    type='button'
                  />
                }
              >
                <ImagePlus className='size-4' />
              </TooltipTrigger>
              <TooltipContent>{t('Upload image')}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <PromptInputButton
                    aria-label={t('Upload file')}
                    disabled={props.disabled || isParsing}
                    onClick={() => fileInputRef.current?.click()}
                    type='button'
                  />
                }
              >
                {isParsing ? <Spinner /> : <FileText className='size-4' />}
              </TooltipTrigger>
              <TooltipContent>{t('Upload file')}</TooltipContent>
            </Tooltip>

            <NativeSelect
              aria-label={t('Reasoning effort')}
              className='w-28'
              disabled={props.disabled}
              onChange={(event) =>
                props.onReasoningEffortChange(
                  event.target.value as AiConsoleReasoningEffort
                )
              }
              size='sm'
              value={props.reasoningEffort}
            >
              <NativeSelectOption value=''>
                {t('Reasoning off')}
              </NativeSelectOption>
              <NativeSelectOption value='low'>{t('Low')}</NativeSelectOption>
              <NativeSelectOption value='medium'>
                {t('Medium')}
              </NativeSelectOption>
              <NativeSelectOption value='high'>{t('High')}</NativeSelectOption>
            </NativeSelect>

            <div className='ml-auto flex min-w-0 items-center gap-1.5'>
              <ModelGroupSelector
                disabled={props.disabled || props.isLoadingOptions}
                groups={props.groups}
                models={props.models}
                onGroupChange={props.onGroupChange}
                onModelChange={props.onModelChange}
                selectedGroup={props.group}
                selectedModel={props.model}
              />
              {props.isGenerating ? (
                <PromptInputButton
                  aria-label={t('Stop')}
                  onClick={props.onStop}
                  type='button'
                  variant='secondary'
                >
                  <Square className='size-4 fill-current' />
                </PromptInputButton>
              ) : (
                <PromptInputButton
                  aria-label={t('Send')}
                  disabled={!canSubmit}
                  type='submit'
                >
                  <Send className='size-4' />
                </PromptInputButton>
              )}
            </div>
          </div>
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}
