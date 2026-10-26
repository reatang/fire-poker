import { StrictMode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import { useComposerAttachments } from '@/components/chat/input/useComposerAttachments'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { inferAttachmentMeta } from '@/lib/attachments'
import { useAlertStore } from '@/alert/store'

const listeners = vi.hoisted(() => new Map<string, Set<(event: unknown) => void>>())
vi.mock('@wailsio/runtime', () => ({
  Events: {
    On: (name: string, callback: (event: unknown) => void) => {
      const callbacks = listeners.get(name) ?? new Set()
      callbacks.add(callback)
      listeners.set(name, callbacks)
      return () => { callbacks.delete(callback) }
    },
  },
  Flags: { GetFlag: () => false },
}))
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/file', () => ({
  File: { SaveTempFile: vi.fn(), SelectFile: vi.fn(), ValidateAttachment: vi.fn() },
}))

beforeEach(() => {
  listeners.clear()
  vi.mocked(FileBinding.SaveTempFile).mockReset()
  vi.mocked(FileBinding.SelectFile).mockReset()
  vi.mocked(FileBinding.ValidateAttachment).mockReset().mockResolvedValue({})
  useAlertStore.getState().reset()
})

function pasteEvent() {
  const blob = new File(['image'], 'photo.png', { type: 'image/png' })
  return {
    preventDefault: vi.fn(),
    clipboardData: { items: [{ kind: 'file', type: 'image/png', getAsFile: () => blob }] },
  } as unknown as React.ClipboardEvent<HTMLDivElement>
}

function nativeDrop(target: string) {
  for (const callback of listeners.get('files-dropped') ?? []) {
    callback({ data: { files: ['/pictures/photo.png'], target } })
  }
}

it('routes native drops to one composer and ignores unaddressed or foreign drops', async () => {
  const first = renderHook(() => useComposerAttachments(1))
  const second = renderHook(() => useComposerAttachments(2))
  act(() => {
    nativeDrop('another-window')
    nativeDrop('')
    nativeDrop(second.result.current.composerId)
  })
  await waitFor(() => expect(second.result.current.attachments).toHaveLength(1))
  expect(first.result.current.attachments).toHaveLength(0)
  expect(FileBinding.ValidateAttachment).toHaveBeenCalledTimes(1)
})

it('keeps native drop addresses valid after StrictMode effect replay', async () => {
  const hook = renderHook(() => useComposerAttachments(1), { wrapper: StrictMode })
  act(() => nativeDrop(hook.result.current.composerId))
  await waitFor(() => expect(hook.result.current.attachments).toHaveLength(1))
})

it('reserves capacity across concurrent browser uploads and native drops', async () => {
  const hook = renderHook(() => useComposerAttachments(1))
  act(() => hook.result.current.setAttachments(Array.from({ length: 9 }, (_, i) => inferAttachmentMeta(`/pictures/${i}.png`))))
  vi.mocked(FileBinding.SaveTempFile).mockResolvedValue({ file_path: '/tmp/photo.png' })
  await act(async () => {
    const upload = hook.result.current.handlePaste(pasteEvent())
    nativeDrop(hook.result.current.composerId)
    await Promise.all([upload, hook.result.current.handlePaste(pasteEvent())])
  })
  expect(hook.result.current.attachments).toHaveLength(10)
  expect(FileBinding.SaveTempFile).toHaveBeenCalledTimes(1)
  expect(FileBinding.ValidateAttachment).not.toHaveBeenCalled()
  expect(hook.result.current.isImporting).toBe(false)
})

it.each(['clear', 'session'])('discards pending uploads after a draft %s', async (change) => {
  const hook = renderHook(({ session }) => useComposerAttachments(session), { initialProps: { session: 1 } })
  let finish!: (result: { file_path: string }) => void
  vi.mocked(FileBinding.SaveTempFile).mockImplementation(() => new Promise(resolve => { finish = resolve }) as ReturnType<typeof FileBinding.SaveTempFile>)
  let pending!: Promise<void>
  const oldAddress = hook.result.current.composerId
  await act(async () => {
    pending = hook.result.current.handlePaste(pasteEvent())
    await vi.waitFor(() => expect(FileBinding.SaveTempFile).toHaveBeenCalledOnce())
  })
  expect(hook.result.current.isImporting).toBe(true)
  if (change === 'session') hook.rerender({ session: 2 })
  act(() => hook.result.current.setAttachments([]))
  await act(async () => {
    nativeDrop(oldAddress)
    finish({ file_path: '/tmp/photo.png' })
    await pending
  })
  expect(hook.result.current.attachments).toHaveLength(0)
  expect(hook.result.current.composerId).not.toBe(oldAddress)
  expect(hook.result.current.isImporting).toBe(false)
})

it('rejects native paths and picker files before adding an invalid attachment', async () => {
  vi.mocked(FileBinding.ValidateAttachment).mockRejectedValue(new Error('File exceeds 20 MB'))
  vi.mocked(FileBinding.SelectFile).mockResolvedValue({ file_path: '/pictures/photo.png' })
  const hook = renderHook(() => useComposerAttachments(1))
  await act(async () => { await hook.result.current.handleAttach() })
  act(() => nativeDrop(hook.result.current.composerId))
  await waitFor(() => expect(hook.result.current.isImporting).toBe(false))
  expect(hook.result.current.attachments).toHaveLength(0)
  expect(FileBinding.ValidateAttachment).toHaveBeenCalledTimes(2)
  expect(useAlertStore.getState().toasts[0]?.message).toBe('File exceeds 20 MB')
})

it('releases a file picker reservation after cancellation', async () => {
  vi.mocked(FileBinding.SelectFile).mockResolvedValue(null)
  const hook = renderHook(() => useComposerAttachments(1))
  await act(async () => { await hook.result.current.handleAttach() })
  expect(hook.result.current.isImporting).toBe(false)
  expect(useAlertStore.getState().toasts).toHaveLength(0)
})
