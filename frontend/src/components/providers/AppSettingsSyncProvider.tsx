import { useEffect } from 'react'
import i18n from '@/i18n'
import { useAppStore } from '@/store/appStore'
import type { FontSize, Language, Theme } from '@/types'

type PersistedAppState = {
  state?: {
    theme?: Theme
    fontSize?: FontSize
    language?: Language
    enabledToolIds?: string[]
  }
}

export function AppSettingsSyncProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key !== 'fire-poker-app-settings' || !event.newValue) {
        return
      }

      try {
        const parsed = JSON.parse(event.newValue) as PersistedAppState
        const nextState = parsed.state
        if (!nextState) {
          return
        }

        if (nextState.language && nextState.language !== i18n.language) {
          void i18n.changeLanguage(nextState.language)
        }

        const state = useAppStore.getState()
        const updated = {
          theme: nextState.theme ?? state.theme,
          fontSize: nextState.fontSize ?? state.fontSize,
          language: nextState.language ?? state.language,
          enabledToolIds: nextState.enabledToolIds ?? state.enabledToolIds,
        }
        if (updated.theme !== state.theme || updated.fontSize !== state.fontSize
          || updated.language !== state.language
          || JSON.stringify(updated.enabledToolIds) !== JSON.stringify(state.enabledToolIds)) {
          useAppStore.setState(updated)
        }
      } catch {
        // Ignore malformed cross-window storage payloads.
      }
    }

    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [])

  return <>{children}</>
}
