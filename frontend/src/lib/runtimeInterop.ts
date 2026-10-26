import { Browser, Clipboard, Flags } from '@wailsio/runtime'
import { useAlertStore } from '@/alert/store'
import { parseUnknownIError } from '@/lib/ierror'
import i18n from '@/i18n'

export function reportInteropError(error: unknown): void {
  useAlertStore.getState().pushAlert({
    kind: 'error', placement: 'toast', title: i18n.t('alert.actionFailed'),
    message: parseUnknownIError(error).msg,
  })
}

function hasNativeRuntime(): boolean {
  try {
    // Every native window receives this flag, including windows with drops disabled.
    return typeof Flags.GetFlag('enableFileDrop') === 'boolean'
  } catch {
    return false
  }
}

export async function writeClipboardText(text: string): Promise<void> {
  if (hasNativeRuntime()) {
    try {
      await Clipboard.SetText(text)
      return
    } catch (error) {
      if (!navigator.clipboard) throw error
    }
  }
  await navigator.clipboard.writeText(text)
}

export async function openExternalURL(url: string): Promise<void> {
  const parsed = new URL(url, window.location.href)
  if (!['http:', 'https:', 'mailto:'].includes(parsed.protocol)) return
  if (hasNativeRuntime()) {
    await Browser.OpenURL(parsed.href)
  } else {
    window.open(parsed.href, '_blank', 'noopener,noreferrer')
  }
}
