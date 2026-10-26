import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Check, Copy, File as FileIcon, Folder, FolderOpen } from 'lucide-react'
import { Settings } from '@bindings/github.com/reatang/fire-poker/backend/service/settings'
import { File } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { SettingsContentLayout } from '@/components/settings/common/SettingsContentLayout'
import { SettingsPanelHeader } from '@/components/settings/common/SettingsPanelHeader'
import { parseUnknownIError } from '@/lib/ierror'
import { writeClipboardText } from '@/lib/runtimeInterop'

type DataDirectoryItem = {
  id: string
  path: string
  revealDir: string
  isFile: boolean
  exists: boolean
}

const COPIED_RESET_MS = 1500

export function DataManagementView() {
  const { t } = useTranslation()
  const [items, setItems] = useState<DataDirectoryItem[]>([])
  const [loadError, setLoadError] = useState<string>('')
  const [copiedId, setCopiedId] = useState<string>('')

  useEffect(() => {
    let cancelled = false
    Settings.ListDataDirectories({})
      .then((result) => {
        if (cancelled) {
          return
        }
        setItems((result?.items ?? []).map((item) => ({
          id: item.id,
          path: item.path,
          revealDir: item.reveal_dir,
          isFile: item.is_file,
          exists: item.exists,
        })))
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoadError(parseUnknownIError(err).msg)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!copiedId) {
      return
    }
    const timer = window.setTimeout(() => setCopiedId(''), COPIED_RESET_MS)
    return () => window.clearTimeout(timer)
  }, [copiedId])

  const copyPath = async (item: DataDirectoryItem) => {
    try {
      await writeClipboardText(item.path)
      setCopiedId(item.id)
    } catch {
      setCopiedId('')
    }
  }

  return (
    <SettingsContentLayout
      header={
        <SettingsPanelHeader
          title={t('settingsPage.data.title')}
          description={t('settingsPage.data.description')}
        />
      }
    >
      <div className="space-y-3 py-6">
        {loadError && (
          <p role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {t('settingsPage.data.loadFailed')}: {loadError}
          </p>
        )}
        {items.map((item) => {
          const Icon = item.isFile ? FileIcon : Folder
          return (
            <section
              key={item.id}
              aria-label={t(`settingsPage.data.items.${item.id}.label`)}
              className="rounded-2xl border border-border/60 bg-card/40 p-4"
            >
              <div className="flex items-start gap-3">
                <Icon size={18} className="mt-0.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-sm font-medium text-foreground">{t(`settingsPage.data.items.${item.id}.label`)}</h2>
                    {!item.exists && (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                        {t('settingsPage.data.notCreated')}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground">{t(`settingsPage.data.items.${item.id}.description`)}</p>
                </div>
              </div>
              <div className="mt-3 flex items-center gap-2">
                <input
                  value={item.path}
                  readOnly
                  aria-label={t('settingsPage.data.pathLabel')}
                  onFocus={(event) => event.currentTarget.select()}
                  className="min-w-0 flex-1 truncate rounded-xl border border-border bg-muted/40 px-3 py-2 font-mono text-xs text-foreground"
                />
                <button
                  type="button"
                  title={copiedId === item.id ? t('settingsPage.data.copied') : t('settingsPage.data.copy')}
                  aria-label={copiedId === item.id ? t('settingsPage.data.copied') : t('settingsPage.data.copy')}
                  onClick={() => void copyPath(item)}
                  className="inline-flex h-9 shrink-0 items-center justify-center rounded-xl border border-border px-3 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                >
                  {copiedId === item.id ? <Check size={15} /> : <Copy size={15} />}
                </button>
                <button
                  type="button"
                  title={t('settingsPage.data.open')}
                  aria-label={t('settingsPage.data.open')}
                  disabled={!item.exists}
                  onClick={() => void File.OpenFile({ path: item.revealDir })}
                  className="inline-flex h-9 shrink-0 items-center justify-center rounded-xl border border-border px-3 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent"
                >
                  <FolderOpen size={15} />
                </button>
              </div>
            </section>
          )
        })}
      </div>
    </SettingsContentLayout>
  )
}
