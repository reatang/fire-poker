import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { SlidersHorizontal, X } from 'lucide-react'
import { Agent } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { useChatStore } from '@/store/chatStore'
import { parseUnknownIError } from '@/lib/ierror'

// promptLabel shows the stored prompt name, falling back to a localized default/custom label.
function promptLabel(t: TFunction, config: { name: string; is_default: boolean }): string {
  return config.name || t(config.is_default ? 'systemPrompt.defaultName' : 'systemPrompt.customName')
}

export function SystemPromptSettings({ sessionId, hasMessages = false }: { sessionId: number | null; hasMessages?: boolean }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [target, setTarget] = useState<number | null>(null)
  const [system, setSystem] = useState('')
  const [name, setName] = useState('')
  const [names, setNames] = useState<string[]>([])
  const [selected, setSelected] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [currentName, setCurrentName] = useState(() => t('systemPrompt.defaultName'))
  const [locked, setLocked] = useState(false)
  const readOnly = locked || hasMessages
  const dialog = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const status = useChatStore(s => sessionId ? s.sessionStatuses[sessionId] : undefined)
  const generating = status === 'loading' || status === 'waiting-unread'

  useEffect(() => {
    let active = true
    setLocked(hasMessages)
    setCurrentName(t(sessionId ? 'systemPrompt.loadingName' : 'systemPrompt.defaultName'))
    if (sessionId) void Agent.GetSessionPrompt({ session_id: sessionId }).then(config => {
      if (active && config) { setCurrentName(promptLabel(t, config)); setLocked(config.locked) }
    }).catch(() => { if (active) setCurrentName(t('systemPrompt.loadFailed')) })
    return () => { active = false }
  }, [sessionId, hasMessages, t])

  useEffect(() => {
    if (!open) return
    dialog.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busy) { setOpen(false); trigger.current?.focus() }
      if (e.key !== 'Tab') return
      const controls = dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled)')
      if (!controls?.length) return
      const first = controls[0], last = controls[controls.length - 1]
      if (e.shiftKey && (document.activeElement === first || document.activeElement === dialog.current)) { e.preventDefault(); last.focus() }
      else if (!e.shiftKey && (document.activeElement === last || document.activeElement === dialog.current)) { e.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, busy])

  const run = async (action: () => Promise<void>) => {
    setBusy(true); setError(''); setNotice('')
    // Backend errors arrive as serialized IError JSON; show only the localized message.
    try { await action() } catch (e) { setError(parseUnknownIError(e).msg) }
    finally { setBusy(false) }
  }

  const show = () => void run(async () => {
    let id = sessionId
    if (!id) {
      id = await useChatStore.getState().createConversation()
      if (!id) throw new Error(t('systemPrompt.createSessionFailed'))
      useChatStore.getState().setCurrentConversation(id)
    }
    setTarget(id); setSystem(''); setName(''); setOpen(true); setLoading(true)
    try {
      const [config, library] = await Promise.all([Agent.GetSessionPrompt({ session_id: id }), Agent.ListSystemPrompts()])
      if (!config) throw new Error(t('systemPrompt.readFailed'))
      setSystem(config.system); setName(config.name); setSelected(''); setNames(library?.names ?? [])
      setLocked(config.locked); setCurrentName(promptLabel(t, config))
      setNotice(t(config.is_default ? 'systemPrompt.usingDefault' : 'systemPrompt.loadedCurrent'))
    } finally { setLoading(false) }
  })

  const apply = (reset = false) => void run(async () => {
    if (!target || readOnly) return
    const result = await Agent.UpdateSessionPrompt({ session_id: target, system, name, reset })
    if (!result) throw new Error(t('systemPrompt.saveFailed'))
    setSystem(result.system); setName(result.name)
    setLocked(result.locked); setCurrentName(promptLabel(t, result))
    setNotice(t(reset ? 'systemPrompt.restored' : 'systemPrompt.applied'))
  })

  return <>
    <div className="desktop-prompt-row flex min-w-0 items-center gap-3">
    <button ref={trigger} onClick={show} disabled={busy || generating} title={t(readOnly ? 'systemPrompt.triggerLocked' : 'systemPrompt.triggerEdit')} aria-label={t('systemPrompt.button')} className="flex shrink-0 items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white shadow-sm transition-colors hover:bg-indigo-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 disabled:opacity-60">
      <SlidersHorizontal size={15} /><span>{t('systemPrompt.button')}</span>
    </button>
    <span className="truncate text-sm font-medium text-foreground" title={currentName}>{currentName}</span>
    {readOnly && <span className="shrink-0 text-xs text-muted-foreground">{t('systemPrompt.locked')}</span>}
    </div>
    {open && createPortal(<div className="desktop-prompt-overlay fixed inset-0 z-[150] flex items-center justify-center bg-black/30 p-5" onMouseDown={e => { if (e.target === e.currentTarget && !busy) { setOpen(false); trigger.current?.focus() } }}>
      <div ref={dialog} role="dialog" aria-modal="true" aria-labelledby="system-prompt-title" tabIndex={-1} className="desktop-prompt-dialog flex max-h-[90vh] w-full max-w-3xl flex-col gap-4 overflow-y-auto rounded-2xl border border-border bg-background p-6 shadow-xl outline-none">
        <div className="flex items-start justify-between gap-4">
          <div><h2 id="system-prompt-title" className="text-lg font-semibold">{t('systemPrompt.title')}</h2><p className="mt-1 text-sm text-muted-foreground">{t(readOnly ? 'systemPrompt.lockedHint' : 'systemPrompt.editHint')}</p></div>
          <button aria-label={t('systemPrompt.close')} disabled={busy} onClick={() => { setOpen(false); trigger.current?.focus() }} className="rounded p-1 hover:bg-accent"><X size={18} /></button>
        </div>
        <label className="text-sm">{t('systemPrompt.library')}
          <select value={selected} disabled={busy || loading || readOnly} onChange={e => {
            const value = e.target.value; setSelected(value)
            if (value) void run(async () => { const p = await Agent.GetSystemPrompt({ name: value, system: '' }); if (p) { setSystem(p.system); setName(p.name); setNotice(t('systemPrompt.loadedFromLibrary')) } })
          }} className="mt-2 w-full rounded-lg border border-border bg-background px-3 py-2">
            <option value="">{t('systemPrompt.libraryPlaceholder')}</option>{names.map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>
        <label className="text-sm">{t('systemPrompt.name')}
          <input value={name} maxLength={80} readOnly={readOnly} disabled={busy || loading} onChange={e => setName(e.target.value)} placeholder={t('systemPrompt.namePlaceholder')} className="mt-2 w-full rounded-lg border border-border bg-background px-3 py-2" />
        </label>
        <label className="text-sm">{t('systemPrompt.content')}
          <textarea value={loading ? '' : system} readOnly={readOnly} disabled={busy || loading} onChange={e => setSystem(e.target.value)} rows={12} placeholder={t(loading ? 'systemPrompt.loading' : 'systemPrompt.contentPlaceholder')} className="mt-2 min-h-48 w-full resize-y rounded-lg border border-border bg-background p-3 text-sm leading-relaxed outline-none focus:ring-2 focus:ring-ring" />
        </label>
        <p className="text-xs text-muted-foreground">{t('systemPrompt.scopeNote')}</p>
        {notice && <p role="status" className="text-sm text-muted-foreground">{notice}</p>}
        {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
        {!readOnly && <div className="flex flex-wrap items-center gap-2">
          <button disabled={busy || loading} onClick={() => apply(true)} className="rounded-lg border border-border px-3 py-2 text-sm hover:bg-accent disabled:opacity-50">{t('systemPrompt.reset')}</button>
          <button disabled={busy || loading || !name.trim() || !system.trim()} onClick={() => void run(async () => {
            await Agent.SaveSystemPrompt({ name: name.trim(), system }); const library = await Agent.ListSystemPrompts(); setNames(library?.names ?? []); setName(name.trim()); setNotice(t('systemPrompt.savedToLibrary'))
          })} className="rounded-lg border border-border px-3 py-2 text-sm hover:bg-accent disabled:opacity-50">{t('systemPrompt.saveToLibrary')}</button>
          <button disabled={busy || loading || !system.trim()} onClick={() => apply()} className="ml-auto rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground disabled:opacity-50">{busy ? t('systemPrompt.working') : t('systemPrompt.apply')}</button>
        </div>}
      </div>
    </div>, document.body)}
  </>
}
