import { useContext, useEffect, useRef, useState } from 'react'
import { FileText, Plus, RefreshCw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Agent } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { SettingsMenuContext } from '@/components/settings/SettingsShell'
import { ConfirmDialog } from '@/components/settings/common/ConfirmDialog'
import { SettingsContentLayout } from '@/components/settings/common/SettingsContentLayout'
import { SettingsPanelHeader } from '@/components/settings/common/SettingsPanelHeader'
import { SettingsSectionLayout } from '@/components/settings/common/SettingsSectionLayout'
import { cn } from '@/lib/utils'
import { parseUnknownIError } from '@/lib/ierror'

type Prompt = { name: string; system: string }
const EMPTY_PROMPT: Prompt = { name: '', system: '' }
const MAX_BYTES = 128 * 1024

export function SystemPromptsSection() {
  const { t } = useTranslation()
  const { isH5, submenuCollapsed, onCollapseSubmenu } = useContext(SettingsMenuContext)
  const [names, setNames] = useState<string[]>([])
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [draft, setDraft] = useState<Prompt>(EMPTY_PROMPT)
  const [saved, setSaved] = useState<Prompt>(EMPTY_PROMPT)
  const [editing, setEditing] = useState(false)
  const [loading, setLoading] = useState(true)
  const [libraryReady, setLibraryReady] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [pendingSelection, setPendingSelection] = useState<{ name: string | null } | null>(null)
  const mounted = useRef(true)
  const dirty = draft.name !== saved.name || draft.system !== saved.system
  const contentBytes = new TextEncoder().encode(draft.system).length
  const duplicate = selected === null && names.some(name => name.toLowerCase() === draft.name.trim().toLowerCase())
  const canSave = libraryReady && dirty && !!draft.name.trim() && !!draft.system.trim() && !duplicate && contentBytes <= MAX_BYTES
  const buttonClass = 'rounded-xl border border-border px-4 py-2 text-sm transition-colors hover:bg-accent disabled:opacity-50'

  const refresh = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await Agent.ListSystemPrompts()
      if (!result) throw new Error(t('settingsPage.prompts.loadFailed'))
      setNames(result.names)
      setLibraryReady(true)
    } catch (e) { setError(parseUnknownIError(e).msg) }
    finally { setLoading(false) }
  }

  useEffect(() => {
    let active = true
    void Agent.ListSystemPrompts().then(result => {
      if (!active) return
      if (!result) throw new Error(t('settingsPage.prompts.loadFailed'))
      setNames(result.names)
      setLibraryReady(true)
    }).catch(e => { if (active) setError(parseUnknownIError(e).msg) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [t])

  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false }
  }, [])

  useEffect(() => {
    if (!dirty) return
    const beforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', beforeUnload)
    return () => window.removeEventListener('beforeunload', beforeUnload)
  }, [dirty])

  const select = async (name: string | null) => {
    setError(''); setNotice('')
    if (name === null) {
      setSelected(null); setDraft(EMPTY_PROMPT); setSaved(EMPTY_PROMPT); setEditing(true)
      onCollapseSubmenu()
      return
    }
    setBusy(true)
    try {
      const result = await Agent.GetSystemPrompt({ name, system: '' })
      if (!mounted.current) return
      if (!result) throw new Error(t('settingsPage.prompts.loadFailed'))
      setSelected(result.name); setDraft(result); setSaved(result); setEditing(true)
      onCollapseSubmenu()
    } catch (e) { if (mounted.current) setError(parseUnknownIError(e).msg) }
    finally { if (mounted.current) setBusy(false) }
  }

  const navigate = (name: string | null) => {
    if (name !== null && name === selected) { onCollapseSubmenu(); return }
    if (dirty) setPendingSelection({ name })
    else void select(name)
  }

  const save = async () => {
    if (!canSave || busy) return
    setBusy(true); setError(''); setNotice('')
    try {
      const result = await Agent.SaveSystemPrompt({ name: draft.name.trim(), system: draft.system })
      if (!result) throw new Error(t('systemPrompt.saveFailed'))
      setSelected(result.name); setDraft(result); setSaved(result)
      setNames(current => [...new Set([...current, result.name])].sort((a, b) => a.localeCompare(b)))
      setNotice(t('settingsPage.prompts.saved'))
    } catch (e) { setError(parseUnknownIError(e).msg) }
    finally { setBusy(false) }
  }

  const remove = async () => {
    if (!selected || busy) return
    setBusy(true); setError(''); setNotice('')
    try {
      await Agent.DeleteSystemPrompt({ name: selected, system: '' })
      setNames(current => current.filter(name => name !== selected))
      setSelected(null); setDraft(EMPTY_PROMPT); setSaved(EMPTY_PROMPT); setEditing(false)
      setNotice(t('settingsPage.prompts.deleted'))
    } catch (e) { setError(parseUnknownIError(e).msg) }
    finally { setBusy(false); setConfirmDelete(false) }
  }

  const filtered = names.filter(name => name.toLowerCase().includes(query.trim().toLowerCase()))

  return <>
    <SettingsSectionLayout sidebarClassName="w-64 xl:w-80" sidebar={
      <div className="flex h-full min-h-0 flex-col gap-4 p-2">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-semibold">{t('settingsPage.prompts.library')}</h3>
          <div className="flex gap-1">
            <button type="button" aria-label={t('settingsPage.prompts.refresh')} disabled={busy || loading} onClick={() => void refresh()} className="rounded-lg p-2 hover:bg-accent disabled:opacity-50"><RefreshCw size={16} /></button>
            <button type="button" aria-label={t('settingsPage.prompts.create')} disabled={busy || loading || !libraryReady} onClick={() => navigate(null)} className="rounded-lg p-2 hover:bg-accent disabled:opacity-50"><Plus size={16} /></button>
          </div>
        </div>
        <label className="flex items-center gap-2 rounded-xl border border-border px-3 py-2 text-muted-foreground">
          <Search size={15} className="shrink-0" />
          <input aria-label={t('settingsPage.prompts.search')} placeholder={t('settingsPage.prompts.search')} value={query} onChange={e => setQuery(e.target.value)} className="min-w-0 w-full bg-transparent text-sm text-foreground outline-none" />
        </label>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {loading ? <p role="status" className="px-2 py-6 text-sm text-muted-foreground">{t('systemPrompt.loading')}</p>
            : filtered.length === 0 ? <p className="px-2 py-6 text-sm text-muted-foreground">{t(query.trim() ? 'settingsPage.prompts.noResults' : 'settingsPage.prompts.empty')}</p>
              : <ul className="space-y-1">{filtered.map(name => <li key={name}>
                <button type="button" disabled={busy} onClick={() => navigate(name)} aria-pressed={selected === name} className={cn('flex w-full items-center gap-2 rounded-xl px-3 py-3 text-left text-sm disabled:opacity-50', selected === name ? 'bg-primary/10 text-primary' : 'hover:bg-accent')}>
                  <FileText size={16} className="shrink-0" /><span className="truncate" title={name}>{name}</span>
                </button>
              </li>)}</ul>}
        </div>
        {error && isH5 && !submenuCollapsed && <p role="alert" className="text-sm text-destructive">{error}</p>}
      </div>
    }>
      <SettingsContentLayout
        header={<div className="pb-6"><SettingsPanelHeader title={t('settingsPage.primary.prompts')} description={t('settingsPage.prompts.description')} /></div>}
        footprint={editing ? <div className="flex flex-wrap items-center justify-between gap-3">
          <div>{selected && <button type="button" disabled={busy} onClick={() => setConfirmDelete(true)} className={cn(buttonClass, 'text-destructive')}>{t('settingsPage.prompts.delete')}</button>}</div>
          <div className="flex gap-2">
            <button type="button" disabled={busy || !dirty} onClick={() => { setDraft(saved); setError(''); setNotice('') }} className={buttonClass}>{t('settingsPage.prompts.revert')}</button>
            <button type="button" disabled={busy || loading || !canSave} onClick={() => void save()} className="rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50">{t(busy ? 'systemPrompt.working' : 'settingsPage.prompts.save')}</button>
          </div>
        </div> : undefined}
      >
        {editing ? <div className="space-y-5 pb-4">
          <label className="block text-sm font-medium">{t('systemPrompt.name')}
            <input value={draft.name} maxLength={80} readOnly={selected !== null} disabled={busy} onChange={e => { setDraft({ ...draft, name: e.target.value }); setNotice('') }} placeholder={t('systemPrompt.namePlaceholder')} className="mt-2 w-full rounded-xl border border-border bg-background px-3 py-2.5 outline-none focus:ring-2 focus:ring-ring read-only:text-muted-foreground" />
          </label>
          {duplicate && <p role="alert" className="text-sm text-destructive">{t('settingsPage.prompts.duplicate')}</p>}
          <label className="block text-sm font-medium">{t('systemPrompt.content')}
            <textarea value={draft.system} disabled={busy} onChange={e => { setDraft({ ...draft, system: e.target.value }); setNotice('') }} rows={16} placeholder={t('systemPrompt.contentPlaceholder')} className="mt-2 min-h-64 w-full resize-y rounded-xl border border-border bg-background p-4 font-mono text-sm font-normal leading-relaxed outline-none focus:ring-2 focus:ring-ring" />
          </label>
          <p className={cn('text-xs', contentBytes > MAX_BYTES ? 'text-destructive' : 'text-muted-foreground')}>{t('settingsPage.prompts.size', { size: (contentBytes / 1024).toFixed(1) })}</p>
        </div> : <div className="flex flex-col items-center gap-4 py-16 text-center text-muted-foreground">
          <FileText size={36} />
          <p className="text-sm">{t('settingsPage.prompts.selectHint')}</p>
          <button type="button" disabled={busy || loading || !libraryReady} onClick={() => navigate(null)} className={buttonClass}>{t('settingsPage.prompts.create')}</button>
        </div>}
        {notice && <p role="status" className="pb-4 text-sm text-muted-foreground">{notice}</p>}
        {error && <p role="alert" className="pb-4 text-sm text-destructive">{error}</p>}
      </SettingsContentLayout>
    </SettingsSectionLayout>
    <ConfirmDialog open={confirmDelete} title={t('settingsPage.prompts.deleteConfirm')} description={t('settingsPage.prompts.deleteHint', { name: selected })} confirmLabel={t('settingsPage.prompts.delete')} cancelLabel={t('settingsPage.prompts.cancel')} confirmTone="danger" busy={busy} onConfirm={() => void remove()} onCancel={() => { if (!busy) setConfirmDelete(false) }} />
    <ConfirmDialog open={pendingSelection !== null} title={t('settingsPage.prompts.discardTitle')} description={t('settingsPage.prompts.discardHint')} confirmLabel={t('settingsPage.prompts.discard')} cancelLabel={t('settingsPage.prompts.cancel')} onConfirm={() => { if (pendingSelection) void select(pendingSelection.name); setPendingSelection(null) }} onCancel={() => setPendingSelection(null)} />
  </>
}
