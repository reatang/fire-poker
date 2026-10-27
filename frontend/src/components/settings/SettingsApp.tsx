import { useCallback, useEffect } from 'react'
import type { ReactNode } from 'react'
import { Keyboard, Languages, MonitorSmartphone } from 'lucide-react'
import { AboutSettingsView } from '@/components/settings/about/AboutSettingsView'
import { DataManagementView } from '@/components/settings/data/DataManagementView'
import { SettingsShell } from '@/components/settings/SettingsShell'
import { SettingsSectionLayout } from '@/components/settings/common/SettingsSectionLayout'
import { SettingsSubmenuList } from '@/components/settings/common/SettingsSubmenuList'
import { GeneralSettingsPanel } from '@/components/settings/general/GeneralSettingsPanel'
import { MemoryList, MemorySettingsView, useMemoryController } from '@/components/settings/memory/MemorySettingsView'
import { PluginsSettingsSection } from '@/components/settings/plugins/PluginsSettingsSection'
import { usePluginSettings } from '@/components/settings/plugins/usePluginSettings'
import { useRuntimeDownloadReminder } from '@/components/settings/plugins/useRuntimeDownloadReminder'
import { ProviderDetailView } from '@/components/settings/providers/ProviderDetailView'
import { ProviderList } from '@/components/settings/providers/ProviderList'
import { SkillsList } from '@/components/settings/skills/SkillsList'
import { SkillsDetailPane } from '@/components/settings/skills/SkillsDetailPane'
import { SystemPromptsSection } from '@/components/settings/prompts/SystemPromptsSection'
import { useSettingsBootstrap } from '@/hooks/useSettingsBootstrap'
import { useSettingsStore } from '@/store/settingsStore'
import { useTranslation } from 'react-i18next'
import { Window } from '@bindings/github.com/reatang/fire-poker/backend/service/window'
import { Provider as ProviderBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/provider'
import { EditProviderInput } from '@bindings/github.com/reatang/fire-poker/backend/service/provider/provider_dto/models'
import type { ProviderWrapper } from '@bindings/github.com/reatang/fire-poker/backend/service/provider/provider_dto/models'
import type { ProviderItem } from '@/types/settings'

function mapProviderItem(w: ProviderWrapper): ProviderItem {
  return {
    id: w.providers.id,
    provider_name: w.providers.provider_name,
    provider_type: w.providers.provider_type,
    base_url: w.providers.base_url,
    api_key: w.providers.api_key,
    enabled: w.providers.enabled,
    is_default: w.providers.is_default,
    model_count: w.models.length,
    icon: w.providers.icon,
    models: w.models.map((m) => ({
      id: m.id,
      provider_id: m.provider_id,
      model: m.model,
      owned_by: m.owned_by,
      object: m.object,
      enable: m.enable,
      alias: m.alias,
      is_custom: m.is_custom,
      is_default: m.is_default,
    })),
  }
}

export function SettingsApp() {
  useSettingsBootstrap()
  const { t } = useTranslation()

  const setProviders = useSettingsStore((state) => state.setProviders)
  const updateProvider = useSettingsStore((state) => state.updateProvider)
  const deleteProvider = useSettingsStore((state) => state.deleteProvider)
  const setDefaultProvider = useSettingsStore((state) => state.setDefaultProvider)
  const plugins = usePluginSettings()
  const { refreshExtensions } = plugins

  const refreshProviders = useCallback(async () => {
    try {
      const result = await ProviderBinding.ListProviders({})
      if (result?.providers) {
        setProviders(result.providers.map(mapProviderItem))
      }
    } catch {
      // ignore
    }
  }, [setProviders])

  useEffect(() => {
    const handleFocus = () => {
      void refreshProviders()
      void refreshExtensions()
    }
    window.addEventListener('focus', handleFocus)
    return () => window.removeEventListener('focus', handleFocus)
  }, [refreshExtensions, refreshProviders])

  const handleToggleEnable = useCallback(async (item: ProviderItem) => {
    const updated = { ...item, enabled: !item.enabled }
    updateProvider(updated)
    try {
      await ProviderBinding.EditProvider(new EditProviderInput({
        provider_id: item.id,
        provider_name: item.provider_name,
        base_url: item.base_url,
        api_key: item.api_key,
        enable: !item.enabled,
        default_model: null,
        models: [],
      }))
    } catch {
      updateProvider(item)
    }
  }, [updateProvider])

  const handleSetDefault = useCallback(async (id: number) => {
    try {
      await ProviderBinding.SetDefault({ provider_id: id, model_id: null })
      setDefaultProvider(id)
    } catch {
      // ignore
    }
  }, [setDefaultProvider])

  const handleDeleteFromList = useCallback(async (id: number) => {
    try {
      await ProviderBinding.DeleteProvider({ provider_id: id })
      deleteProvider(id)
    } catch {
      // ignore
    }
  }, [deleteProvider])

  const activeTab = useSettingsStore((state) => state.activeTab)
  const generalTab = useSettingsStore((state) => state.generalTab)
  const setActiveTab = useSettingsStore((state) => state.setActiveTab)
  const setGeneralTab = useSettingsStore((state) => state.setGeneralTab)
  const version = useSettingsStore((state) => state.version)
  const providers = useSettingsStore((state) => state.providers)
  const selectedProviderId = useSettingsStore((state) => state.selectedProviderId)
  const setSelectedProviderId = useSettingsStore((state) => state.setSelectedProviderId)
  const memoryController = useMemoryController(activeTab === 'memory')

  const generalItems = [
    { key: 'display' as const, label: t('settingsPage.general.display.title'), icon: <MonitorSmartphone size={16} /> },
    { key: 'locale' as const, label: t('settingsPage.general.locale.title'), icon: <Languages size={16} /> },
    { key: 'shortcuts' as const, label: t('settingsPage.general.shortcuts.title'), icon: <Keyboard size={16} /> },
  ]

  useEffect(() => {
    const tab = new URLSearchParams(window.location.search).get('tab')
    if (tab === 'general' || tab === 'providers' || tab === 'prompts' || tab === 'plugins' || tab === 'skills' || tab === 'memory' || tab === 'data' || tab === 'about') {
      setActiveTab(tab)
    }
  }, [setActiveTab])

  useEffect(() => {
    void refreshExtensions()
  }, [refreshExtensions])

  useRuntimeDownloadReminder(activeTab === 'plugins')

  let content: ReactNode

  if (activeTab === 'data') {
    content = (
      <SettingsSectionLayout>
        <DataManagementView />
      </SettingsSectionLayout>
    )
  } else if (activeTab === 'about') {
    content = (
      <SettingsSectionLayout>
        <AboutSettingsView version={version} />
      </SettingsSectionLayout>
    )
  } else if (activeTab === 'providers') {
    const selectedProvider = providers.find((provider) => provider.id === selectedProviderId) ?? providers[0] ?? null

    content = (
      <SettingsSectionLayout
        sidebarClassName="w-80"
        sidebar={(
          <ProviderList
            items={providers}
            selectedId={selectedProvider?.id ?? null}
            onSelect={setSelectedProviderId}
            onCreate={() => { void Window.OpenAddProvider({}) }}
            onToggleEnable={handleToggleEnable}
            onSetDefault={handleSetDefault}
            onDelete={handleDeleteFromList}
          />
        )}
      >
        <ProviderDetailView
          provider={selectedProvider}
          onUpdated={refreshProviders}
          onDeleted={deleteProvider}
          onSetDefault={handleSetDefault}
        />
      </SettingsSectionLayout>
    )
  } else if (activeTab === 'prompts') {
    content = <SystemPromptsSection />
  } else if (activeTab === 'plugins') {
    content = <PluginsSettingsSection plugins={plugins} />
  } else if (activeTab === 'skills') {
    content = (
      <SettingsSectionLayout
        sidebarClassName="w-80"
        sidebar={<SkillsList />}
      >
        <SkillsDetailPane />
      </SettingsSectionLayout>
    )
  } else if (activeTab === 'memory') {
    content = (
      <SettingsSectionLayout
        sidebarClassName="w-56 lg:w-64 xl:w-80"
        sidebar={<MemoryList controller={memoryController} />}
      >
        <MemorySettingsView controller={memoryController} />
      </SettingsSectionLayout>
    )
  } else {
    content = (
      <SettingsSectionLayout
        sidebar={
          <SettingsSubmenuList
            title={t('settingsPage.primary.general')}
            items={generalItems}
            value={generalTab}
            onChange={setGeneralTab}
          />
        }
      >
        <GeneralSettingsPanel />
      </SettingsSectionLayout>
    )
  }

  return <SettingsShell>{content}</SettingsShell>
}

