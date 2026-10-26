import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAlertStore } from '@/alert/store'
import { useChatStore } from '@/store/chatStore'
import { useCliInstallStore } from '@/store/cliInstallStore'
import { useSettingsStore } from '@/store/settingsStore'
import { parseUnknownIError } from '@/lib/ierror'
import { Plugin as PluginBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { CancelLoginCliInput, GenerateCliManifestInput, ImportExtensionInput, InstallCliFromNpmInput, ReloadExtensionInput, ResetCliDataInput, SaveExtensionConfigInput, ToggleExtensionInput } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin/plugin_dto/models'
import { SelectFolderInput } from '@bindings/github.com/reatang/fire-poker/backend/service/file/file_dto/models'
import type { ExtensionItem as BindingExtensionItem } from '@bindings/github.com/reatang/fire-poker/backend/models/data_models/models'
import type { ExtensionItem } from '@/types/settings'
import { SYSTEM_TOOLS_ID } from '@/types/settings'

type LoginDialogState = {
  id: string
  title: string
}

export function mapExtensionItem(item: BindingExtensionItem): ExtensionItem {
  const kind: 'mcp' | 'plugin' | 'cli' = item.kind === 'plugin' || item.kind === 'cli' ? item.kind : 'mcp'
  return {
    ...item,
    kind,
    tools: item.tools ?? [],
  }
}

// usePluginSettings holds the plugins tab's state and actions. SettingsApp calls it on every render,
// not only on the plugins tab, so state survives tab switches and the window-level unmount still
// cancels an in-flight CLI login.
export function usePluginSettings() {
  const { t } = useTranslation()
  const pushAlert = useAlertStore((state) => state.pushAlert)
  const setExtensions = useSettingsStore((state) => state.setExtensions)
  const updateExtension = useSettingsStore((state) => state.updateExtension)
  const deleteExtension = useSettingsStore((state) => state.deleteExtension)
  const providers = useSettingsStore((state) => state.providers)
  const defaultProviderId = useSettingsStore((state) => state.defaultProviderId)
  const extensions = useSettingsStore((state) => state.extensions)
  const selectedExtensionId = useSettingsStore((state) => state.selectedExtensionId)
  const setSelectedExtensionId = useSettingsStore((state) => state.setSelectedExtensionId)
  const cliInstallUpsert = useCliInstallStore((state) => state.upsert)
  const createConversation = useChatStore((state) => state.createConversation)
  const sendMessage = useChatStore((state) => state.sendMessage)
  const setCurrentConversation = useChatStore((state) => state.setCurrentConversation)
  const [selectedExtensionConfig, setSelectedExtensionConfig] = useState<string>('')
  const [pluginLoading, setPluginLoading] = useState(false)
  const [cliModalOpen, setCliModalOpen] = useState(false)
  const [loginDialog, setLoginDialog] = useState<LoginDialogState | null>(null)
  const loginDialogRef = useRef<LoginDialogState | null>(null)

  const refreshExtensions = useCallback(async () => {
    try {
      const result = await PluginBinding.ListExtensions({})
      if (result?.extensions) {
        setExtensions(result.extensions.map(mapExtensionItem))
      }
    } catch {
      // ignore
    }
  }, [setExtensions])

  useEffect(() => {
    const loadDetail = async () => {
      if (!selectedExtensionId || selectedExtensionId === SYSTEM_TOOLS_ID) {
        setSelectedExtensionConfig('')
        return
      }
      try {
        const result = await PluginBinding.GetExtensionDetail({ id: selectedExtensionId })
        setSelectedExtensionConfig(result?.config_text ?? '')
      } catch {
        setSelectedExtensionConfig('')
      }
    }
    void loadDetail()
  }, [selectedExtensionId, extensions])

  useEffect(() => {
    loginDialogRef.current = loginDialog
  }, [loginDialog])

  useEffect(() => {
    return () => {
      const activeDialog = loginDialogRef.current
      if (!activeDialog) {
        return
      }
      void Promise.resolve(
        PluginBinding.CancelLoginCli(new CancelLoginCliInput({ id: activeDialog.id })),
      ).catch(() => undefined)
    }
  }, [])

  const systemToolsSelected = selectedExtensionId === SYSTEM_TOOLS_ID
  const selectedExtension = systemToolsSelected ? null : (extensions.find((extension) => extension.id === selectedExtensionId) ?? extensions[0] ?? null)

  const handleNpmSubmit = async (npmPackage: string, name: string) => {
    try {
      const out = await PluginBinding.InstallCliFromNpm(
        new InstallCliFromNpmInput({ npm_package: npmPackage, name }),
      )
      if (out?.extension) {
        await refreshExtensions()
        setSelectedExtensionId(out.extension.id)
      }
    } catch (err) {
      pushAlert({ kind: 'error', placement: 'banner', title: '', message: String(err) })
    }
  }

  const handleSmartSubmit = async (content: string) => {
    const defaultProvider = providers.find((p) => p.id === defaultProviderId) ?? providers.find((p) => p.is_default) ?? providers[0]
    const defaultModel = defaultProvider?.models.find((m) => m.is_default) ?? defaultProvider?.models[0]
    try {
      const sessionId = await createConversation({
        title: t('settingsPage.plugins.cliFromDocsTask'),
        tags: ['task'],
      })
      if (sessionId) {
        setCurrentConversation(sessionId)
        cliInstallUpsert({
          session_id: sessionId,
          npm_package: `task:${sessionId}`,
          name: t('settingsPage.plugins.cliFromDocsTask'),
          phase: 'pending',
          detail: t('settingsPage.plugins.cliTaskPending'),
        })
        await sendMessage({
          sessionId,
          content,
          systemPrompt: '',
          skillName: 'install-cli-from-docs',
          providerId: defaultProvider?.id ?? 0,
          modelName: defaultModel?.model ?? '',
          enabledUserTools: await useChatStore.getState().loadSessionTools(sessionId),
          attachments: [],
        })
      }
      pushAlert({
        kind: 'success',
        placement: 'toast',
        title: '',
        message: t('settingsPage.plugins.cliTaskCreatedToast'),
      })
    } catch (err) {
      pushAlert({ kind: 'error', placement: 'banner', title: '', message: String(err) })
    }
  }

  const handleCreateExtension = async (kind: 'mcp' | 'plugin' | 'cli') => {
    if (kind === 'cli') {
      setCliModalOpen(true)
      return
    }
    const result = await FileBinding.SelectFolder(new SelectFolderInput())
    if (!result?.folder_path) return
    const imported = await PluginBinding.ImportExtension(new ImportExtensionInput({
      kind,
      path: result.folder_path,
    }))
    if (!imported?.extension) return
    await refreshExtensions()
    setSelectedExtensionId(imported.extension.id)
  }

  const handleToggleExtension = async (item: ExtensionItem) => {
    updateExtension({ ...item, enabled: !item.enabled })
    try {
      const result = await PluginBinding.ToggleExtension(new ToggleExtensionInput({
        id: item.id,
        enabled: !item.enabled,
      }))
      if (result?.extension) {
        updateExtension(mapExtensionItem(result.extension))
      }
    } catch {
      updateExtension(item)
    }
  }

  const handleReloadExtension = async (id: string) => {
    try {
      const result = await PluginBinding.ReloadExtension(new ReloadExtensionInput({ id }))
      if (result?.extension) {
        updateExtension(mapExtensionItem(result.extension))
      }
      if (selectedExtensionId === id) {
        const detail = await PluginBinding.GetExtensionDetail({ id })
        setSelectedExtensionConfig(detail?.config_text ?? '')
      }
    } catch {
      // ignore
    }
  }

  const handleSaveExtensionConfig = async (configText: string) => {
    if (!selectedExtension) return
    setPluginLoading(true)
    try {
      const result = await PluginBinding.SaveExtensionConfig(new SaveExtensionConfigInput({
        id: selectedExtension.id,
        config_text: configText,
      }))
      if (result?.extension) {
        updateExtension(mapExtensionItem(result.extension))
        setSelectedExtensionConfig(configText)
      }
    } finally {
      setPluginLoading(false)
    }
  }

  const handleDeleteExtension = async (id: string) => {
    try {
      await PluginBinding.DeleteExtension({ id })
      deleteExtension(id)
    } catch {
      // ignore
    }
  }

  const handleResetCliData = async (): Promise<void> => {
    if (!selectedExtension || selectedExtension.kind !== 'cli') return
    try {
      await PluginBinding.ResetCliData(new ResetCliDataInput({ id: selectedExtension.id }))
      await refreshExtensions()
    } catch (err: unknown) {
      console.error('reset cli data failed:', err)
    }
  }

  const handleRegenerateCliManifest = async (): Promise<void> => {
    if (!selectedExtension || selectedExtension.kind !== 'cli') return
    setPluginLoading(true)
    try {
      const result = await PluginBinding.GenerateCliManifest(new GenerateCliManifestInput({ id: selectedExtension.id }))
      if (result?.extension) {
        updateExtension(mapExtensionItem(result.extension))
      }
      const detail = await PluginBinding.GetExtensionDetail({ id: selectedExtension.id })
      setSelectedExtensionConfig(detail?.config_text ?? '')
    } finally {
      setPluginLoading(false)
    }
  }

  const handleLoginCli = async (): Promise<void> => {
    if (!selectedExtension || selectedExtension.kind !== 'cli') return
    setLoginDialog({
      id: selectedExtension.id,
      title: t('settingsPage.plugins.cliLoginDialogTitle', { name: selectedExtension.name }),
    })
  }

  const handleLoginStartError = (err: unknown): void => {
    const parsed = parseUnknownIError(err)
    pushAlert({
      kind: 'error',
      placement: 'banner',
      title: '',
      message: parsed.msg,
      detail: parsed.detail !== parsed.msg ? parsed.detail : null,
    })
  }

  const handleCancelLogin = async (): Promise<void> => {
    if (!loginDialog) return
    try {
      await PluginBinding.CancelLoginCli(new CancelLoginCliInput({ id: loginDialog.id }))
    } finally {
      setLoginDialog(null)
    }
  }

  return {
    extensions,
    systemToolsSelected,
    selectedExtension,
    setSelectedExtensionId,
    selectedExtensionConfig,
    pluginLoading,
    cliModalOpen,
    setCliModalOpen,
    loginDialog,
    setLoginDialog,
    refreshExtensions,
    handleNpmSubmit,
    handleSmartSubmit,
    handleCreateExtension,
    handleToggleExtension,
    handleReloadExtension,
    handleSaveExtensionConfig,
    handleDeleteExtension,
    handleResetCliData,
    handleRegenerateCliManifest,
    handleLoginCli,
    handleLoginStartError,
    handleCancelLogin,
  }
}

export type PluginSettings = ReturnType<typeof usePluginSettings>
