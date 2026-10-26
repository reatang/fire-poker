import { lazy, Suspense, type ComponentType, type LazyExoticComponent } from 'react'
import { AppSettingsSyncProvider } from '@/components/providers/AppSettingsSyncProvider'
import { AlertEventProvider } from '@/components/providers/AlertEventProvider'
import { ThemeProvider } from '@/components/providers/ThemeProvider'
import { FontSizeProvider } from '@/components/providers/FontSizeProvider'
import { AlertViewport } from '@/components/alert/AlertViewport'
import { useNotificationsSubscription } from '@/hooks/useNotificationsSubscription'
import { useCliInstallSubscription } from '@/hooks/useCliInstallSubscription'
import { useDisableInputAssistance } from '@/hooks/useDisableInputAssistance'

// Each window loads only its own entry chunk; the ?entry= query selects it.
const OnboardingApp = lazy(() => import('@/components/onboarding/OnboardingApp').then((m) => ({ default: m.OnboardingApp })))
const MainLayout = lazy(() => import('@/components/layout/MainLayout').then((m) => ({ default: m.MainLayout })))

const ENTRIES: Record<string, LazyExoticComponent<ComponentType>> = {
  settings: lazy(() => import('@/components/settings/SettingsApp').then((m) => ({ default: m.SettingsApp }))),
  add_provider: lazy(() => import('@/components/settings/providers/AddProviderApp').then((m) => ({ default: m.AddProviderApp }))),
  add_skill: lazy(() => import('@/components/settings/skills/AddSkillApp').then((m) => ({ default: m.AddSkillApp }))),
  add_memory: lazy(() => import('@/components/settings/memory/AddMemoryApp').then((m) => ({ default: m.AddMemoryApp }))),
  quick_chat: lazy(() => import('@/components/quick-chat/QuickChatApp').then((m) => ({ default: m.QuickChatApp }))),
}

function App() {
  useNotificationsSubscription()
  useCliInstallSubscription()
  useDisableInputAssistance()
  const entry = new URLSearchParams(window.location.search).get('entry') ?? ''

  // Onboarding runs before settings exist, so it stays outside the settings-driven providers.
  if (entry === 'onboarding') {
    return <Suspense fallback={null}><OnboardingApp /></Suspense>
  }

  const Entry = ENTRIES[entry] ?? MainLayout
  return (
    <AppSettingsSyncProvider>
      <AlertEventProvider>
        <ThemeProvider>
          <FontSizeProvider>
            <Suspense fallback={null}>
              <Entry />
            </Suspense>
            <AlertViewport />
          </FontSizeProvider>
        </ThemeProvider>
      </AlertEventProvider>
    </AppSettingsSyncProvider>
  )
}

export default App
