import { useTranslation } from 'react-i18next'
import { useAlertStore } from '@/alert/store'
import { SettingsSectionLayout } from '@/components/settings/common/SettingsSectionLayout'
import { CliInstallModal } from '@/components/settings/plugins/CliInstallModal'
import { CliLoginDialog } from '@/components/settings/plugins/CliLoginDialog'
import { PluginToolDetailView } from '@/components/settings/plugins/PluginToolDetailView'
import { PluginToolList } from '@/components/settings/plugins/PluginToolList'
import { ToolSettingsView } from '@/components/settings/plugins/ToolSettingsView'
import { Plugin as PluginBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin'
import { LoginCliInput } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin/plugin_dto/models'
import { SYSTEM_TOOLS_ID } from '@/types/settings'
import type { PluginSettings } from './usePluginSettings'

// PluginsSettingsSection renders the plugins tab from state owned by usePluginSettings.
export function PluginsSettingsSection({ plugins }: { plugins: PluginSettings }) {
  const { t } = useTranslation()
  const pushAlert = useAlertStore((state) => state.pushAlert)
  const {
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
  } = plugins

  return (
    <>
      <SettingsSectionLayout
        sidebarClassName="w-80"
        sidebar={(
          <PluginToolList
            items={extensions}
            selectedId={systemToolsSelected ? SYSTEM_TOOLS_ID : (selectedExtension?.id ?? null)}
            onSelect={setSelectedExtensionId}
            onCreate={handleCreateExtension}
            onToggleEnable={handleToggleExtension}
            onReload={handleReloadExtension}
            onDelete={handleDeleteExtension}
          />
        )}
      >
        {systemToolsSelected ? <ToolSettingsView /> : <PluginToolDetailView
          extension={selectedExtension}
          configText={selectedExtensionConfig}
          loading={pluginLoading}
          onReload={() => {
            if (selectedExtension) {
              void handleReloadExtension(selectedExtension.id)
            }
          }}
          onLoginCli={() => { void handleLoginCli() }}
          onSave={(configText) => { void handleSaveExtensionConfig(configText) }}
          onResetData={() => { void handleResetCliData() }}
          onRegenerateCliManifest={() => { void handleRegenerateCliManifest() }}
        />}
      </SettingsSectionLayout>
      <CliInstallModal
        open={cliModalOpen}
        onClose={() => setCliModalOpen(false)}
        onNpmSubmit={(npmPackage, name) => { void handleNpmSubmit(npmPackage, name) }}
        onSmartSubmit={(content) => { void handleSmartSubmit(content) }}
      />
      {loginDialog ? (
        <CliLoginDialog
          open={true}
          extensionId={loginDialog.id}
          title={loginDialog.title}
          onStart={() => PluginBinding.LoginCli(new LoginCliInput({ id: loginDialog.id })).then(() => undefined)}
          onClose={() => setLoginDialog(null)}
          onCancel={() => { void handleCancelLogin() }}
          onStartError={handleLoginStartError}
          onDone={(exitCode: number) => {
            pushAlert({
              kind: exitCode === 0 ? 'success' : 'error',
              placement: 'toast',
              title: selectedExtension?.name ?? loginDialog.title,
              message: exitCode === 0
                ? t('settingsPage.plugins.cliLoginDoneSuccess')
                : t('settingsPage.plugins.cliLoginDoneFailureShort', { code: exitCode }),
              detail: null,
              autoClose: true,
              durationMs: 4000,
            })
          }}
        />
      ) : null}
    </>
  )
}
