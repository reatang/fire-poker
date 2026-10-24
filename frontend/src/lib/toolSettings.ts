import { Settings } from '@bindings/github.com/reatang/fire-poker/backend/service/settings'
import { useAppStore } from '@/store/appStore'

// The backend config is authoritative; this store is only the cross-window defaults mirror.
export async function loadToolDefaults(): Promise<string[]> {
  const result = await Settings.LoadBootstrap({})
  if (!result) throw new Error('Unable to load default tools')
  const ids = [...result.bootstrap.default_tool_ids]
  useAppStore.getState().setEnabledToolIds(ids)
  return ids
}

export const TOOL_I18N_KEYS: Record<string, { name: string; description: string }> = {
  datetime: {
    name: 'input.toolNames.datetime.name',
    description: 'input.toolNames.datetime.description',
  },
  file_read: {
    name: 'input.toolNames.fileRead.name',
    description: 'input.toolNames.fileRead.description',
  },
  file_write: {
    name: 'input.toolNames.fileWrite.name',
    description: 'input.toolNames.fileWrite.description',
  },
  shell: {
    name: 'input.toolNames.shell.name',
    description: 'input.toolNames.shell.description',
  },
  windows_shell: {
    name: 'input.toolNames.windowsShell.name',
    description: 'input.toolNames.windowsShell.description',
  },
  macos_shell: {
    name: 'input.toolNames.macosShell.name',
    description: 'input.toolNames.macosShell.description',
  },
  linux_shell: {
    name: 'input.toolNames.linuxShell.name',
    description: 'input.toolNames.linuxShell.description',
  },
  web_fetch: {
    name: 'input.toolNames.webFetch.name',
    description: 'input.toolNames.webFetch.description',
  },
  web_search: {
    name: 'input.toolNames.webSearch.name',
    description: 'input.toolNames.webSearch.description',
  },
  code_exec: {
    name: 'input.toolNames.codeExec.name',
    description: 'input.toolNames.codeExec.description',
  },
}
