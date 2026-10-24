import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Theme, FontSize, Language } from '../types'
import i18n from '../i18n'
import { migrateBrandStorage } from './migrateBrandStorage'

migrateBrandStorage('fire-poker-app-settings', 'lemontea-app-settings')

interface AppStore {
  theme: Theme
  fontSize: FontSize
  language: Language
  enabledToolIds: string[] // Global defaults only; session overrides live in chatStore and SQLite.
  setTheme: (theme: Theme) => void
  setFontSize: (size: FontSize) => void
  setLanguage: (lang: Language) => void
  setEnabledToolIds: (ids: string[]) => void
}

export const useAppStore = create<AppStore>()(
  persist(
    (set, get) => ({
      theme: 'auto',
      fontSize: 'xl',
      language: 'zh-CN',
      enabledToolIds: [] as string[],
      setTheme: (theme) => set({ theme }),
      setFontSize: (fontSize) => set({ fontSize }),
      setLanguage: (language) => {
        i18n.changeLanguage(language)
        set({ language })
      },
      setEnabledToolIds: (enabledToolIds) => {
        if (JSON.stringify(get().enabledToolIds) !== JSON.stringify(enabledToolIds)) {
          set({ enabledToolIds })
        }
      },
    }),
    {
      name: 'fire-poker-app-settings',
      onRehydrateStorage: () => (state) => {
        if (state?.language) {
          i18n.changeLanguage(state.language)
        }
      },
    }
  )
)
