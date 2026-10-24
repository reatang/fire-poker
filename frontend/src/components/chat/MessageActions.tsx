import { useEffect, useRef, useState } from 'react'
import { Check, Copy, GitBranch, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Clipboard } from '@wailsio/runtime'
import { useAlertStore } from '@/alert/store'

export function AssistantIdentity() {
  return <div className="desktop-assistant-label"><img src="/brand/fire-poker.svg" alt="" /><span>Fire Poker</span></div>
}

export function MessageActions({ content, onRegenerate, onFork, forkOnly = false, disabled = false }: { content: string; onRegenerate?: () => Promise<void>; onFork?: () => Promise<void>; forkOnly?: boolean; disabled?: boolean }) {
  const { i18n } = useTranslation()
  const zh = i18n.language.startsWith('zh')
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
    } catch { setError(zh ? '复制失败，请重试' : 'Copy failed. Please retry.') }
  }
  const regenerate = async () => {
    if (!onRegenerate || busy || disabled) return
    setBusy(true); setError('')
    try { await onRegenerate() } catch (e) {
      const message = e instanceof Error ? e.message : String(e)
      useAlertStore.getState().pushAlert({kind:'error', placement:'toast', title:zh ? '重新生成失败' : 'Regeneration failed', message})
    }
    finally { setBusy(false) }
  }
  const fork = async () => {
    if (!onFork || busy || disabled) return
    setBusy(true)
    try { await onFork() } catch (e) {
      useAlertStore.getState().pushAlert({kind:'error',placement:'toast',title:zh ? '分叉失败' : 'Fork failed',message:e instanceof Error ? e.message : String(e)})
    } finally { setBusy(false) }
  }
  return <div className="message-actions">
    {!forkOnly && <button type="button" onClick={() => void copy()} disabled={!content} aria-label={zh ? '复制回复' : 'Copy response'} title={zh ? '复制回复' : 'Copy response'}>
      {copied ? <Check size={14}/> : <Copy size={14}/>}<span>{copied ? (zh ? '已复制' : 'Copied') : (zh ? '复制' : 'Copy')}</span>
    </button>}
    {!forkOnly && !onFork && <button type="button" onClick={() => void regenerate()} disabled={disabled || busy || !onRegenerate} aria-label={zh ? '重新生成' : 'Regenerate'} title={!onRegenerate ? (zh ? '仅最新一轮回复可重新生成' : 'Only the latest response can be regenerated') : (zh ? '重新生成最新回复' : 'Regenerate latest response')}>
      <RotateCcw size={14} className={busy ? 'animate-spin' : ''}/><span>{zh ? '重新生成' : 'Regenerate'}</span>
    </button>}
    {onFork && <button type="button" onClick={() => void fork()} disabled={disabled || busy} aria-label={zh ? '从此处分叉' : 'Fork from here'} title={zh ? '复制截至此消息的全部内容，在新会话中继续' : 'Copy conversation up to this message into a new chat'}><GitBranch size={14}/><span>{busy ? (zh ? '正在分叉…' : 'Forking…') : (zh ? '分叉' : 'Fork')}</span></button>}
    {error && <span role="alert" className="text-destructive">{error}</span>}
  </div>
}
