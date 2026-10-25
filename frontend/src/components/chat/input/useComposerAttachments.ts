import { useEffect, useRef, useState } from 'react'
import { Events } from '@wailsio/runtime'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { SelectFileInput, SaveTempFileInput } from '@bindings/github.com/reatang/fire-poker/backend/service/file/file_dto'
import { ATTACHMENT_MAX_COUNT, ATTACHMENT_MAX_BYTES, blobToBase64, inferAttachmentMeta } from '@/lib/attachments'
import type { Attachment } from '@/types'

// useComposerAttachments owns the composer's pending attachments: the file picker, pasted files,
// browser drops and macOS native window drops, all capped at ATTACHMENT_MAX_COUNT.
export function useComposerAttachments() {
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const attachmentsRef = useRef<Attachment[]>([])
  attachmentsRef.current = attachments
  const [isDraggingOver, setIsDraggingOver] = useState(false)

  useEffect(() => {
    const offEnter = Events.On(Events.Types.Mac.WindowFileDraggingEntered, () => {
      setIsDraggingOver(true)
    })
    const offExit = Events.On(Events.Types.Mac.WindowFileDraggingExited, () => {
      setIsDraggingOver(false)
    })
    const offPerformed = Events.On(Events.Types.Mac.WindowFileDraggingPerformed, () => {
      setIsDraggingOver(false)
    })
    const offDrop = Events.On('files-dropped', (event) => {
      const payload = event.data as { files?: string[] } | null
      const files = payload?.files ?? []
      const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
      const newAtts = files.slice(0, remaining).map(path => inferAttachmentMeta(path))
      if (newAtts.length > 0) {
        setAttachments(prev => [...prev, ...newAtts])
      }
    })

    return () => {
      offEnter()
      offExit()
      offPerformed()
      offDrop()
    }
  }, [])

  const handleAttach = async () => {
    if (attachments.length >= ATTACHMENT_MAX_COUNT) return
    const result = await FileBinding.SelectFile(new SelectFileInput())
    if (!result?.file_path) return
    setAttachments(prev => [...prev, inferAttachmentMeta(result.file_path)])
  }

  const handlePaste = async (event: React.ClipboardEvent<HTMLDivElement>) => {
    const items = Array.from(event.clipboardData.items)
    const fileItems = items.filter(item => item.kind === 'file')
    if (fileItems.length === 0) return

    event.preventDefault()

    const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
    const toProcess = fileItems.slice(0, remaining).map(item => {
      const blob = item.getAsFile()
      if (!blob) return null
      if (blob.size > ATTACHMENT_MAX_BYTES) return null
      const name = blob.name || `pasted-${Date.now()}.${item.type.split('/')[1] ?? 'bin'}`
      return { blob, name, mime: item.type || 'application/octet-stream' }
    }).filter((x): x is NonNullable<typeof x> => x !== null)

    const results = await Promise.allSettled(
      toProcess.map(async ({ blob, name, mime }) => {
        const data = await blobToBase64(blob)
        const result = await FileBinding.SaveTempFile(new SaveTempFileInput({ name, data, mime }))
        if (!result?.file_path) throw new Error('no path')
        const att = inferAttachmentMeta(result.file_path)
        return { ...att, name }
      })
    )

    const newAtts = results
      .filter((r): r is PromiseFulfilledResult<Attachment> => r.status === 'fulfilled')
      .map(r => r.value)

    if (newAtts.length > 0) {
      setAttachments(prev => [...prev, ...newAtts])
    }
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    setIsDraggingOver(true)
  }

  const handleDragLeave = (event: React.DragEvent<HTMLDivElement>) => {
    if ((event.currentTarget as HTMLDivElement).contains(event.relatedTarget as Node)) return
    setIsDraggingOver(false)
  }

  const handleDrop = async (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setIsDraggingOver(false)

    const dropped = Array.from(event.dataTransfer.files)
    const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
    const toProcess = dropped.slice(0, remaining).filter(file => file.size <= ATTACHMENT_MAX_BYTES)

    const results = await Promise.allSettled(
      toProcess.map(async (file) => {
        const data = await blobToBase64(file)
        const result = await FileBinding.SaveTempFile(
          new SaveTempFileInput({ name: file.name, data, mime: file.type || 'application/octet-stream' })
        )
        if (!result?.file_path) throw new Error('no path')
        const att = inferAttachmentMeta(result.file_path)
        return { ...att, name: file.name }
      })
    )

    const newAtts = results
      .filter((r): r is PromiseFulfilledResult<Attachment> => r.status === 'fulfilled')
      .map(r => r.value)

    if (newAtts.length > 0) {
      setAttachments(prev => [...prev, ...newAtts])
    }
  }

  return {
    attachments,
    setAttachments,
    isDraggingOver,
    handleAttach,
    handlePaste,
    handleDragOver,
    handleDragLeave,
    handleDrop,
  }
}
