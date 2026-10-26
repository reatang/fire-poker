import { useCallback, useEffect, useRef, useState } from 'react'
import { Events, Flags } from '@wailsio/runtime'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { Agent as AgentBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { SelectFileInput, SaveTempFileInput } from '@bindings/github.com/reatang/fire-poker/backend/service/file/file_dto'
import { ATTACHMENT_MAX_COUNT, ATTACHMENT_MAX_BYTES, attachmentFromStored, blobToBase64, inferAttachmentMeta } from '@/lib/attachments'
import { useAlertStore } from '@/alert/store'
import { parseUnknownIError } from '@/lib/ierror'
import i18n from '@/i18n'
import type { Attachment } from '@/types'

function newDraft(key: number | null | undefined) {
  // getRandomValues also works in WebKit webviews using a custom URL scheme.
  const id = Array.from(crypto.getRandomValues(new Uint32Array(4)), value => value.toString(16).padStart(8, '0')).join('')
  return { key, id, scopeId: id, pending: 0, sessionId: key ?? undefined, sessionPromise: null as Promise<number> | null }
}

type SessionOptions = {
  createSession: () => Promise<number>
  onSessionCreated: (sessionId: number) => void
}

// Each draft has its own drop address and reservations for pending bridge calls.
export function useComposerAttachments(draftKey?: number | null, options?: SessionOptions) {
  const [attachments, updateAttachments] = useState<Attachment[]>([])
  const attachmentsRef = useRef<Attachment[]>([])
  const draftRef = useRef(newDraft(draftKey))
  const lifecycleRef = useRef(0)
  const optionsRef = useRef(options)
  optionsRef.current = options
  if (draftRef.current.key !== draftKey) {
    if (draftRef.current.key == null && draftRef.current.sessionId === draftKey) {
      draftRef.current.key = draftKey
    } else {
      draftRef.current = newDraft(draftKey)
    }
  }
  const [isDraggingOver, setIsDraggingOver] = useState(false)
  const [pendingImports, setPendingImports] = useState(0)

  const setAttachments = useCallback((value: React.SetStateAction<Attachment[]>) => {
    const next = typeof value === 'function' ? value(attachmentsRef.current) : value
    if (next.length === 0) {
      draftRef.current = { ...newDraft(draftRef.current.key), scopeId: draftRef.current.scopeId }
      setPendingImports(0)
    }
    attachmentsRef.current = next
    updateAttachments(next)
  }, [])

  useEffect(() => () => {
    lifecycleRef.current += 1
  }, [])

  const importAttachments = useCallback(async (tasks: Array<() => Promise<Attachment>>) => {
    const draft = draftRef.current
    const lifecycle = lifecycleRef.current
    const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length - draft.pending
    const accepted = tasks.slice(0, Math.max(0, remaining))
    if (accepted.length === 0) return
    draft.pending += accepted.length
    setPendingImports(draft.pending)
    const active = () => draftRef.current === draft && lifecycleRef.current === lifecycle
    const ensureSession = async () => {
      if (!active()) throw new CancelledSelection()
      if (draft.sessionId) return draft.sessionId
      if (!draft.sessionPromise) {
        const sessionOptions = optionsRef.current
        if (!sessionOptions) throw new Error(i18n.t('input.attachFailed'))
        draft.sessionPromise = sessionOptions.createSession().then(sessionId => {
          if (!active()) throw new CancelledSelection()
          if (!sessionId) throw new Error(i18n.t('input.attachFailed'))
          draft.sessionId = sessionId
          sessionOptions.onSessionCreated(sessionId)
          return sessionId
        }).catch(error => { draft.sessionPromise = null; throw error })
      }
      return draft.sessionPromise
    }
    const results = await Promise.allSettled(accepted.map(async task => {
      const attachment = await task()
      const sessionId = await ensureSession()
      const stored = await AgentBinding.StoreAttachments({ session_id: sessionId, attachments: [attachment] })
      const persisted = stored?.attachments[0]
      if (!persisted) throw new Error(i18n.t('input.attachFailed'))
      return attachmentFromStored(persisted)
    }))
    draft.pending -= accepted.length
    if (draftRef.current !== draft || lifecycleRef.current !== lifecycle) return
    setPendingImports(draft.pending)

    const added = results.flatMap(result => result.status === 'fulfilled' ? [result.value] : [])
    if (added.length > 0) {
      const next = [...attachmentsRef.current, ...added].slice(0, ATTACHMENT_MAX_COUNT)
      attachmentsRef.current = next
      updateAttachments(next)
    }
    const failed = results.find(result => result.status === 'rejected' && !(result.reason instanceof CancelledSelection))
    if (failed?.status === 'rejected') {
      useAlertStore.getState().pushAlert({
        kind: 'error', placement: 'toast', title: i18n.t('input.attach'),
        message: parseUnknownIError(failed.reason).msg,
      })
    }
  }, [])

  const importPaths = useCallback((paths: string[]) => importAttachments(paths.map(path => async () => {
    await FileBinding.ValidateAttachment({ path })
    return inferAttachmentMeta(path)
  })), [importAttachments])

  useEffect(() => Events.On('files-dropped', (event) => {
    const payload = event.data as { files?: string[]; target?: string } | null
    if (payload?.target !== draftRef.current.id) return
    setIsDraggingOver(false)
    void importPaths(payload.files ?? [])
  }), [importPaths])

  const handleAttach = () => importAttachments([async () => {
    const result = await FileBinding.SelectFile(new SelectFileInput())
    if (!result?.file_path) throw new CancelledSelection()
    await FileBinding.ValidateAttachment({ path: result.file_path })
    return inferAttachmentMeta(result.file_path)
  }])

  const importFiles = (files: File[]) => importAttachments(files.map(file => async () => {
    if (file.size > ATTACHMENT_MAX_BYTES) {
      throw new Error(i18n.t('input.attachLimitSize', { mb: ATTACHMENT_MAX_BYTES / 1024 / 1024 }))
    }
    const name = file.name || `pasted-${Date.now()}.${file.type.split('/')[1] ?? 'bin'}`
    const data = await blobToBase64(file)
    const result = await FileBinding.SaveTempFile(new SaveTempFileInput({
      name, data, mime: file.type || 'application/octet-stream',
    }))
    if (!result?.file_path) throw new Error(i18n.t('input.attachFailed'))
    return { ...inferAttachmentMeta(result.file_path), name }
  }))

  const handlePaste = async (event: React.ClipboardEvent<HTMLDivElement>) => {
    const items = Array.from(event.clipboardData.items).filter(item => item.kind === 'file')
    if (items.length === 0) return
    event.preventDefault()
    const files = items.map(item => item.getAsFile()).filter((file): file is File => file !== null)
    await importFiles(files)
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    setIsDraggingOver(true)
  }

  const handleDragLeave = (event: React.DragEvent<HTMLDivElement>) => {
    if (event.currentTarget.contains(event.relatedTarget as Node)) return
    setIsDraggingOver(false)
  }

  const handleDrop = async (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setIsDraggingOver(false)
    // Wails resolves original paths and addresses the event to this draft.
    try {
      if (Flags.GetFlag('enableFileDrop') === true) return
    } catch {
      // Browser previews have no native window flags.
    }
    await importFiles(Array.from(event.dataTransfer.files))
  }

  return {
    attachments, setAttachments, composerId: draftRef.current.id, draftScopeId: draftRef.current.scopeId,
    isDraggingOver, isImporting: pendingImports > 0,
    handleAttach, handlePaste, handleDragOver, handleDragLeave, handleDrop,
  }
}

class CancelledSelection extends Error {}
