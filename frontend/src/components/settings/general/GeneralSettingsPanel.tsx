import { Settings } from '@bindings/github.com/reatang/fire-poker/backend/service/settings'
import { DisplaySettingsView } from '@/components/settings/general/DisplaySettingsView'
import { LocaleSettingsView } from '@/components/settings/general/LocaleSettingsView'
import { ShortcutSettingsView } from '@/components/settings/general/ShortcutSettingsView'
import { useAppStore } from '@/store/appStore'
import { useSettingsStore } from '@/store/settingsStore'

export function GeneralSettingsPanel() {
  const generalTab = useSettingsStore((state) => state.generalTab)
  const fontSize = useSettingsStore((state) => state.fontSize)
  const quickChatShortcut = useSettingsStore((state) => state.quickChatShortcut)
  const displayDraft = useSettingsStore((state) => state.displayDraft)
  const localeDraft = useSettingsStore((state) => state.localeDraft)
  const shortcutDraft = useSettingsStore((state) => state.shortcutDraft)
  const localeDirty = useSettingsStore((state) => state.localeDirty)
  const shortcutDirty = useSettingsStore((state) => state.shortcutDirty)
  const shortcutValidation = useSettingsStore((state) => state.shortcutValidation)
  const languages = useSettingsStore((state) => state.languages)
  const regions = useSettingsStore((state) => state.regions)
  const setDisplayDraft = useSettingsStore((state) => state.setDisplayDraft)
  const setLocaleDraft = useSettingsStore((state) => state.setLocaleDraft)
  const setShortcutDraft = useSettingsStore((state) => state.setShortcutDraft)
  const setShortcutValidation = useSettingsStore((state) => state.setShortcutValidation)
  const applyDisplaySettings = useSettingsStore((state) => state.applyDisplaySettings)
  const applyLocaleSettings = useSettingsStore((state) => state.applyLocaleSettings)
  const applyShortcutSettings = useSettingsStore((state) => state.applyShortcutSettings)
  const setFontSize = useAppStore((state) => state.setFontSize)
  const setLanguage = useAppStore((state) => state.setLanguage)

  if (generalTab === 'locale') {
    return (
      <LocaleSettingsView
        locale={localeDraft.locale}
        language={localeDraft.language}
        languages={languages}
        regions={regions}
        dirty={localeDirty}
        onLocaleChange={(value) => setLocaleDraft({ locale: value })}
        onLanguageChange={(value) => setLocaleDraft({ language: value })}
        onApply={async () => {
          await Settings.SaveLocaleSettings({ locale: localeDraft.locale, language: localeDraft.language })
          applyLocaleSettings({ locale: localeDraft.locale, language: localeDraft.language })
          setLanguage(localeDraft.language)
        }}
      />
    )
  }

  if (generalTab === 'shortcuts') {
    return (
      <ShortcutSettingsView
        quickChatShortcut={shortcutDraft.quickChatShortcut}
        dirty={shortcutDirty}
        validation={shortcutValidation}
        onShortcutChange={async (shortcut) => {
          setShortcutDraft({ quickChatShortcut: shortcut })
          setShortcutValidation({ status: 'idle', message: '' })
          try {
            const result = await Settings.ValidateShortcutSettings({ quick_chat_shortcut: shortcut })
            if (result) {
              const nextShortcut = result.quick_chat_shortcut || shortcut
              setShortcutDraft({ quickChatShortcut: nextShortcut })
              setShortcutValidation({
                status: result.status as typeof shortcutValidation.status,
                message: result.message ?? '',
              })
            }
          } catch {
            setShortcutValidation({ status: 'available', message: '' })
          }
        }}
        onApply={async () => {
          try {
            const result = await Settings.SaveShortcutSettings({ quick_chat_shortcut: shortcutDraft.quickChatShortcut })
            if (result) {
              setShortcutValidation({
                status: result.status as typeof shortcutValidation.status,
                message: result.message ?? '',
              })
              if (result.status === 'available') {
                applyShortcutSettings(result.quick_chat_shortcut || shortcutDraft.quickChatShortcut)
              }
            }
          } catch {
            applyShortcutSettings(shortcutDraft.quickChatShortcut)
          }
        }}
        onReset={() => {
          setShortcutDraft({ quickChatShortcut: quickChatShortcut })
          setShortcutValidation({ status: 'idle', message: '' })
        }}
      />
    )
  }

  return (
    <DisplaySettingsView
      value={fontSize}
      draft={displayDraft.fontSize}
      onDraftChange={(value) => setDisplayDraft({ fontSize: value })}
      onApply={async () => {
        await Settings.SaveDisplaySettings({ font_size: displayDraft.fontSize })
        applyDisplaySettings(displayDraft.fontSize)
        setFontSize(displayDraft.fontSize)
      }}
      onReset={() => setDisplayDraft({ fontSize })}
    />
  )
}
