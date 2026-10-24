import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Settings } from '@bindings/github.com/reatang/fire-poker/backend/service/settings'
import { Plugin } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin'
import { SettingsContentLayout } from '@/components/settings/common/SettingsContentLayout'
import { SettingsPanelHeader } from '@/components/settings/common/SettingsPanelHeader'
import { SettingsActionBar } from '@/components/settings/common/SettingsActionBar'
import { SettingsDirtyGuard } from '@/components/settings/common/SettingsDirtyGuard'
import { loadToolDefaults, TOOL_I18N_KEYS } from '@/lib/toolSettings'

type ToolOption = { id: string; name: string; description: string; category: string }

export function ToolSettingsView() {
  const { t } = useTranslation()
  const [tools, setTools] = useState<ToolOption[]>([])
  const [saved, setSaved] = useState<string[]>([])
  const [draft, setDraft] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const dirty = [...saved].sort().join('\0') !== [...draft].sort().join('\0')

  useEffect(() => {
    let cancelled = false
    void Promise.all([Plugin.ListAvailableTools({}), loadToolDefaults()]).then(([result, ids]) => {
      if (cancelled) return
      setTools(result?.tools ?? [])
      setSaved(ids)
      setDraft(ids)
    }).catch(() => {
      if (!cancelled) setError('settingsPage.general.tools.loadError')
    }).finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  const save = async () => {
    setSaving(true)
    try {
      const result = await Settings.SaveToolSettings({ default_tool_ids: draft })
      if (!result) throw new Error('No saved settings returned')
      const ids = await loadToolDefaults()
      if ([...ids].sort().join('\0') !== [...draft].sort().join('\0')) {
        throw new Error('Persisted tool settings do not match the requested selection')
      }
      setSaved(ids)
      setDraft(ids)
      setError('')
    } catch { setError('settingsPage.general.tools.saveError') }
    finally { setSaving(false) }
  }

  return (
    <SettingsDirtyGuard dirty={dirty}>
      <SettingsContentLayout
        header={<div className="pb-6"><SettingsPanelHeader title={t('settingsPage.plugins.systemTools.title')} description={t('settingsPage.general.tools.description')} /></div>}
        footprint={<SettingsActionBar primaryLabel={t('settingsPage.actions.apply')} primaryDisabled={loading || saving || !dirty}
          onPrimaryClick={() => { void save() }} secondaryLabel={t('settingsPage.actions.reset')} secondaryDisabled={loading || saving}
          onSecondaryClick={() => setDraft([...saved])} />}
      >
        <div className="space-y-3">
          {error && <p role="alert" className="text-sm text-destructive">{t(error)}</p>}
          {!loading && !tools.length && !error && <p className="text-sm text-muted-foreground">{t('settingsPage.general.tools.empty')}</p>}
          {tools.map(tool => {
            const labels = TOOL_I18N_KEYS[tool.id]
            return <label key={tool.id} className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-card p-4">
              <input type="checkbox" checked={draft.includes(tool.id)} disabled={loading || saving}
                onChange={() => setDraft(ids => ids.includes(tool.id) ? ids.filter(id => id !== tool.id) : [...ids, tool.id])}
                className="mt-1 accent-primary" />
              <span><span className="block text-sm font-medium">{labels ? t(labels.name) : tool.name}</span>
                <span className="mt-1 block text-xs text-muted-foreground">{labels ? t(labels.description) : tool.description}</span></span>
            </label>
          })}
        </div>
      </SettingsContentLayout>
    </SettingsDirtyGuard>
  )
}
