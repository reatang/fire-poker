package ierror

import "github.com/reatang/fire-poker/backend/pkg/i18n"

type errorCode string

// ErrorCode lets callers pass the Err* constants around, e.g. as a fallback in error mappers.
type ErrorCode = errorCode

func (e errorCode) Msg() string {
	return i18n.TCurrent(string(e), nil)
}

const (
	// Agent error codes
	ErrAgentSendMessage        errorCode = "ierror.agent.send_message"
	ErrAgentNoConfirm          errorCode = "ierror.agent.no_confirm"
	ErrAgentCreateSession      errorCode = "ierror.agent.create_session"
	ErrAgentListSessions       errorCode = "ierror.agent.list_sessions"
	ErrAgentListMessages       errorCode = "ierror.agent.list_messages"
	ErrAgentCountMessages      errorCode = "ierror.agent.count_messages"
	ErrAgentMarkRead           errorCode = "ierror.agent.mark_read"
	ErrAgentRename             errorCode = "ierror.agent.rename"
	ErrAgentDelete             errorCode = "ierror.agent.delete"
	ErrAgentToggleStar         errorCode = "ierror.agent.toggle_star"
	ErrAgentTooManyAttachments errorCode = "ierror.agent.too_many_attachments"
	ErrAgentAttachmentPath     errorCode = "ierror.agent.attachment_path_empty"
	ErrAgentAttachmentNotFound errorCode = "ierror.agent.attachment_not_found"
	ErrAgentAttachmentSize     errorCode = "ierror.agent.attachment_too_large"
	ErrAgentStreamError        errorCode = "ierror.agent.stream_error"
	ErrAgentSaveOutput         errorCode = "ierror.agent.save_output"
	ErrAgentSessionBusy        errorCode = "ierror.agent.session_busy"
	ErrAgentPromptLocked       errorCode = "ierror.agent.system_prompt_locked"
	ErrAgentToolsLocked        errorCode = "ierror.agent.session_tools_locked"
	ErrAgentProviderUnavail    errorCode = "ierror.agent.provider_unavailable"
	ErrAgentStopBeforePrompt   errorCode = "ierror.agent.stop_before_prompt"
	ErrAgentPromptEmpty        errorCode = "ierror.agent.prompt_empty"
	ErrAgentPromptNameTooLong  errorCode = "ierror.agent.prompt_name_too_long"
	ErrAgentPromptInvalidName  errorCode = "ierror.agent.prompt_invalid_name"
	ErrAgentPromptInvalid      errorCode = "ierror.agent.prompt_invalid"
	ErrAgentPromptLibrary      errorCode = "ierror.agent.prompt_library"
	ErrAgentUpdatePrompt       errorCode = "ierror.agent.update_prompt"
	ErrAgentForkTarget         errorCode = "ierror.agent.fork_target"
	ErrAgentFork               errorCode = "ierror.agent.fork"
	ErrNotificationCreate      errorCode = "ierror.notification.create"
	ErrNotificationList        errorCode = "ierror.notification.list"
	ErrNotificationResolve     errorCode = "ierror.notification.resolve"
	ErrNotificationDismiss     errorCode = "ierror.notification.dismiss"
	ErrMemoryList              errorCode = "ierror.memory.list"
	ErrMemoryGet               errorCode = "ierror.memory.get"
	ErrMemoryCreate            errorCode = "ierror.memory.create"
	ErrMemoryUpdate            errorCode = "ierror.memory.update"
	ErrMemoryForget            errorCode = "ierror.memory.forget"
	ErrMemoryRestore           errorCode = "ierror.memory.restore"
	ErrMemoryStats             errorCode = "ierror.memory.stats"
	ErrMemorySettings          errorCode = "ierror.memory.settings"

	// File error codes
	ErrFileSelectFolder errorCode = "ierror.file.select_folder"
	ErrFileSelectFile   errorCode = "ierror.file.select_file"
	ErrFileOpen         errorCode = "ierror.file.open"
	ErrFileSaveTempFile errorCode = "ierror.file.save_temp_file"

	// Settings error codes
	ErrSettingsLoadConfig   errorCode = "ierror.settings.load_config"
	ErrSettingsSaveConfig   errorCode = "ierror.settings.save_config"
	ErrSettingsReadConfig   errorCode = "ierror.settings.read_config"
	ErrSettingsParseConfig  errorCode = "ierror.settings.parse_config"
	ErrSettingsListDataDirs errorCode = "ierror.settings.list_data_dirs"

	// Provider error codes
	ErrProviderCreate        errorCode = "ierror.provider.create"
	ErrProviderCreateModels  errorCode = "ierror.provider.create_models"
	ErrProviderListProviders errorCode = "ierror.provider.list_providers"
	ErrProviderListModels    errorCode = "ierror.provider.list_models"
	ErrProviderDelete        errorCode = "ierror.provider.delete"
	ErrProviderUpdate        errorCode = "ierror.provider.update"
	ErrProviderDeleteModel   errorCode = "ierror.provider.delete_model"
	ErrProviderInvalidModel  errorCode = "ierror.provider.invalid_model_id"
	ErrProviderSetDefault    errorCode = "ierror.provider.set_default"
	ErrProviderFetchModels   errorCode = "ierror.provider.fetch_model_list"

	// Onboarding error codes
	ErrOnboardingReadInit  errorCode = "ierror.onboarding.read_init"
	ErrOnboardingWriteInit errorCode = "ierror.onboarding.write_init"
	ErrOnboardingComplete  errorCode = "ierror.onboarding.complete"

	// Runtime error codes
	ErrRuntimeUnsupportedOS    errorCode = "ierror.runtime.unsupported_os"
	ErrRuntimeFetchSums        errorCode = "ierror.runtime.fetch_sums"
	ErrRuntimeChecksumMismatch errorCode = "ierror.runtime.checksum_mismatch"
	ErrRuntimeDownload         errorCode = "ierror.runtime.download"
	ErrRuntimeExtract          errorCode = "ierror.runtime.extract"
	ErrRuntimeWriteState       errorCode = "ierror.runtime.write_state"
	ErrRuntimeReadState        errorCode = "ierror.runtime.read_state"

	// CLI plugin error codes
	ErrCliRuntimeUnavailable     errorCode = "ierror.cli.runtime_unavailable"
	ErrCliInstallFailed          errorCode = "ierror.cli.install_failed"
	ErrCliResetDataFailed        errorCode = "ierror.cli.reset_data_failed"
	ErrCliManifestInvalid        errorCode = "ierror.cli.manifest_invalid"
	ErrCliManifestSaveFailed     errorCode = "ierror.cli.manifest_save_failed"
	ErrCliManifestGenerateFailed errorCode = "ierror.cli.manifest_generate_failed"
	ErrCliToolNotFound           errorCode = "ierror.cli.tool_not_found"

	// CLI login error codes
	ErrCliLoginSessionConflict errorCode = "cli.login.session_conflict"
	ErrCliLoginNotFound        errorCode = "cli.login.not_found"
	ErrCliLoginNoCommand       errorCode = "cli.login.no_command"
	ErrCliLoginStartFailed     errorCode = "cli.login.start_failed"

	// Terminal error codes
	ErrTerminalList       errorCode = "ierror.terminal.list"
	ErrTerminalReadOutput errorCode = "ierror.terminal.read_output"
	ErrTerminalWriteInput errorCode = "ierror.terminal.write_input"
	ErrTerminalResize     errorCode = "ierror.terminal.resize"

	// Skills error codes
	ErrSkillsLoadFailed     errorCode = "ierror.skills.load_failed"
	ErrSkillsNotFound       errorCode = "ierror.skills.not_found"
	ErrSkillsInvalidName    errorCode = "ierror.skills.invalid_name"
	ErrSkillsInvalidContent errorCode = "ierror.skills.invalid_content"
	ErrSkillsNameTaken      errorCode = "ierror.skills.name_taken"
	ErrSkillsBuiltinLocked  errorCode = "ierror.skills.builtin_locked"
	ErrSkillsWriteFailed    errorCode = "ierror.skills.write_failed"
	ErrSkillsDeleteFailed   errorCode = "ierror.skills.delete_failed"
)
