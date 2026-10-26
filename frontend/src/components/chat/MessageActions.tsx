import { useEffect, useRef, useState } from 'react'
import { Check, Copy, GitBranch, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Clipboard } from '@wailsio/runtime'
import { useAlertStore } from '@/alert/store'
import { parseUnknownIError } from '@/lib/ierror'

export function AssistantIdentity() {
  return <div className="desktop-assistant-label"><img src="/brand/fire-poker.svg" alt="" /><span>Fire Poker</span></div>
}

export function MessageActions({ content, onRegenerate, onFork, forkOnly = false, disabled = false }: { content: string; onRegenerate?: () => Promise<void>; onFork?: () => Promise<void>; forkOnly?: boolean; disabled?: boolean }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const timer = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => clearTimeout(timer.current), [])
  const copy = async () => {
    try {
      await Clipboard.SetText(content)
      setCopied(true); setError(''); clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 1800)
    } catch { setError(t('messageActions.copyFailed')) }
  }
  const regenerate = async () => {
    if (!onRegenerate || busy || disabled) return
    setBusy(true); setError('')
    try { await onRegenerate() } catch (e) {
      // Backend errors arrive as serialized IError JSON; show only the localized message.
      useAlertStore.getState().pushAlert({kind:'error', placement:'toast', title:t('messageActions.regenerateFailed'), message:parseUnknownIError(e).msg})
    }
    finally { setBusy(false) }
  }
  const fork = async () => {
    if (!onFork || busy || disabled) return
    setBusy(true)
    try { await onFork() } catch (e) {
      useAlertStore.getState().pushAlert({kind:'error',placement:'toast',title:t('messageActions.forkFailed'),message:parseUnknownIError(e).msg})
    } finally { setBusy(false) }
  }
  return <div className="message-actions">
    {!forkOnly && <button type="button" onClick={() => void copy()} disabled={!content} aria-label={t('messageActions.copyResponse')} title={t('messageActions.copyResponse')}>
      {copied ? <Check size={14}/> : <Copy size={14}/>}<span>{t(copied ? 'messageActions.copied' : 'messageActions.copy')}</span>
    </button>}
    {!forkOnly && !onFork && <button type="button" onClick={() => void regenerate()} disabled={disabled || busy || !onRegenerate} aria-label={t('messageActions.regenerate')} title={t(onRegenerate ? 'messageActions.regenerateLatest' : 'messageActions.regenerateLatestOnly')}>
      <RotateCcw size={14} className={busy ? 'animate-spin' : ''}/><span>{t('messageActions.regenerate')}</span>
    </button>}
    {onFork && <button type="button" onClick={() => void fork()} disabled={disabled || busy} aria-label={t('messageActions.forkFromHere')} title={t('messageActions.forkHint')}><GitBranch size={14}/><span>{t(busy ? 'messageActions.forking' : 'messageActions.fork')}</span></button>}
    {error && <span role="alert" className="text-destructive">{error}</span>}
  </div>
}
