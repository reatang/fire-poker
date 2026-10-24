import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { I18nextProvider } from 'react-i18next'
import i18n from '@/i18n'
import { ToolSettingsView } from '@/components/settings/plugins/ToolSettingsView'
import { useChatStore } from '@/store/chatStore'
import { useAppStore } from '@/store/appStore'
import { Settings } from '@bindings/github.com/reatang/fire-poker/backend/service/settings'

const config = vi.hoisted(() => ({ ids: ['web_fetch'] as string[] }))
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/settings', () => ({
  Settings: {
    LoadBootstrap: vi.fn(async () => ({bootstrap:{default_tool_ids:[...config.ids]}})),
    SaveToolSettings: vi.fn(async (input:{default_tool_ids:string[]}) => {
      config.ids = [...input.default_tool_ids]
      return {default_tool_ids:[...config.ids]}
    }),
  },
}))
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/plugin', () => ({
  Plugin: {ListAvailableTools:vi.fn().mockResolvedValue({tools:[{id:'web_fetch',name:'web_fetch',description:'fetch',category:'builtin'}]})},
}))

beforeEach(async () => {
  vi.clearAllMocks()
  config.ids = ['web_fetch']
  useChatStore.setState({sessionToolIds:{1:['web_fetch'],2:[]},sessionToolLocks:{1:true}})
  await i18n.changeLanguage('en')
})

it('preserves unchecked tools when another window synchronizes the language', async () => {
  const user = userEvent.setup()
  render(<I18nextProvider i18n={i18n}><ToolSettingsView /></I18nextProvider>)
  const checkbox = await screen.findByRole('checkbox')
  await waitFor(() => expect(checkbox).toBeChecked())
  await user.click(checkbox)
  await act(async () => { await i18n.changeLanguage('en') })
  expect(checkbox).not.toBeChecked()
  expect(Settings.LoadBootstrap).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: /apply/i }))
  await waitFor(() => expect(config.ids).toEqual([]))
  expect(checkbox).not.toBeChecked()
})

it('saves global defaults separately and reloads an explicit empty selection', async () => {
  const user = userEvent.setup()
  const view = render(<I18nextProvider i18n={i18n}><ToolSettingsView /></I18nextProvider>)
  const checkbox = await screen.findByRole('checkbox')
  await waitFor(() => expect(checkbox).toBeChecked())
  await user.click(checkbox)
  await user.click(screen.getByRole('button',{name:/apply/i}))
  await waitFor(() => expect(useAppStore.getState().enabledToolIds).toEqual([]))
  expect(useChatStore.getState().sessionToolIds).toEqual({1:['web_fetch'],2:[]})
  expect(config.ids).toEqual([])
  view.unmount()
  render(<I18nextProvider i18n={i18n}><ToolSettingsView /></I18nextProvider>)
  expect(await screen.findByRole('checkbox')).not.toBeChecked()
})

it('keeps the draft and reports an error if readback still contains the old configuration', async () => {
  const user = userEvent.setup()
  render(<I18nextProvider i18n={i18n}><ToolSettingsView /></I18nextProvider>)
  const checkbox = await screen.findByRole('checkbox')
  await waitFor(() => expect(checkbox).toBeChecked())
  await user.click(checkbox)
  vi.mocked(Settings.SaveToolSettings).mockResolvedValueOnce({default_tool_ids:[]})
  await user.click(screen.getByRole('button', { name: /apply/i }))
  expect(await screen.findByRole('alert')).toBeInTheDocument()
  expect(checkbox).not.toBeChecked()
  expect(config.ids).toEqual(['web_fetch'])
  await user.click(screen.getByRole('button', { name: /apply/i }))
  await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  expect(config.ids).toEqual([])
  expect(checkbox).not.toBeChecked()
})
