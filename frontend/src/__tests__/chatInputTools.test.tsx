import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nextProvider } from 'react-i18next'
import i18n from '@/i18n'
import { ChatInput } from '@/components/chat/ChatInput'
import { useChatStore } from '@/store/chatStore'
import { useAppStore } from '@/store/appStore'
import { Agent } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'

vi.mock('@wailsio/runtime', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wailsio/runtime')>()
  return {
    ...actual,
    Events: {
      On: vi.fn(() => vi.fn()),
      Types: {
        Mac: {
          WindowFileDraggingEntered: 'mac:WindowFileDraggingEntered',
          WindowFileDraggingExited: 'mac:WindowFileDraggingExited',
          WindowFileDraggingPerformed: 'mac:WindowFileDraggingPerformed',
        },
      },
    },
  }
})

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/provider', () => ({
  Provider: {
    ProviderAndModelList: vi.fn().mockResolvedValue({
      provider_models: [
        {
          provider: {
            id: 1,
            provider_name: 'P',
            enabled: true,
            is_default: true,
            base_url: 'http://x',
            api_key: 'k',
            provider_type: 'aliyun',
          },
          models: [{ id: 1, model: 'm', alias: 'm', is_default: true, enable: true }],
        },
      ],
    }),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/plugin', () => ({
  Plugin: {
    ListAvailableTools: vi.fn().mockResolvedValue({
      tools: [
        { id: 'web_fetch', name: 'web_fetch', description: 'Fetch a URL and return its content', category: 'builtin' },
        { id: 'mcp:filesystem:1.0.0', name: 'filesystem', description: 'Filesystem tools', category: 'mcp' },
      ],
    }),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/settings', () => ({
  Settings: { LoadBootstrap: vi.fn().mockResolvedValue({ bootstrap: { default_tool_ids: ['web_fetch'] } }) },
}))
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/agent', () => ({
  Agent: {
    LoadSessionMessages: vi.fn().mockResolvedValue({messages:[],has_more:false,total:0}),
    MarkSessionRead: vi.fn().mockResolvedValue({}),
    GetSessionToolSettings: vi.fn().mockResolvedValue({ tool_ids: ['web_fetch'] }),
    SaveSessionToolSettings: vi.fn(async (input: {tool_ids: string[]}) => ({ tool_ids: input.tool_ids })),
  },
}))

const openSettings = vi.fn()
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/window', () => ({
  Window: {
    OpenSettings: (...args: unknown[]) => openSettings(...args),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/file', () => ({
  File: {
    ImagePreview: vi.fn().mockResolvedValue(null),
    SelectFile: vi.fn(),
    SaveTempFile: vi.fn(),
  },
}))

function renderInput() {
  return render(<I18nextProvider i18n={i18n}><ChatInput /></I18nextProvider>)
}

describe('Chat input tools', () => {
  beforeEach(async () => {
    openSettings.mockReset()
    vi.mocked(Agent.SaveSessionToolSettings).mockClear()
    vi.mocked(Agent.GetSessionToolSettings).mockResolvedValue({tool_ids:['web_fetch'],locked:false})
    useChatStore.setState({
      conversations: [{
        id: 1, title: 't', kind: 'user' as const, createdAt: '', updatedAt: '',
        starred: false, status: 'idle',
      }],
      currentConversationId: 1,
      sessionToolIds: {},
      sessionToolLocks: {},
      sessionStatuses: {},
      messages: {},
    })
    await i18n.changeLanguage('en')
  })

  it('renders aggregated mcp entries and opens plugin settings', async () => {
    const user = userEvent.setup()
    renderInput()

    await user.click(screen.getByRole('button', { name: /tools/i }))
    expect(await screen.findByText('filesystem')).toBeInTheDocument()
    expect(screen.queryByText('read_file')).not.toBeInTheDocument()
    expect(screen.queryByText('Web Search')).not.toBeInTheDocument()
    expect(screen.queryByText('Search the web')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /manage tools & plugins/i }))
    await waitFor(() => expect(openSettings).toHaveBeenCalledWith({ tab: 'plugins' }))
  })

  it('saves a session selection without changing global defaults or another session', async () => {
    const user = userEvent.setup()
    const view = renderInput()
    await user.click(screen.getByRole('button', { name: /tools/i }))
    const fetch = await screen.findByRole('button', { name: /fetch a url|web fetch/i })
    await waitFor(() => expect(fetch).toHaveAttribute('aria-pressed', 'true'))
    useChatStore.setState(state => ({sessionToolIds: {...state.sessionToolIds, 2: ['mcp:filesystem:1.0.0']}}))
    await user.click(fetch)
    await waitFor(() => expect(Agent.SaveSessionToolSettings).toHaveBeenCalledWith({session_id:1,tool_ids:[]}))
    expect(useAppStore.getState().enabledToolIds).toEqual(['web_fetch'])
    expect(useChatStore.getState().sessionToolIds[1]).toEqual([])
    expect(useChatStore.getState().sessionToolIds[2]).toEqual(['mcp:filesystem:1.0.0'])
    view.unmount()
  })

  it('restores separate selections when switching conversations', async () => {
    vi.mocked(Agent.GetSessionToolSettings)
      .mockResolvedValueOnce({tool_ids:['web_fetch'],locked:false})
      .mockResolvedValueOnce({tool_ids:[],locked:false})
      .mockResolvedValueOnce({tool_ids:['web_fetch'],locked:false})
    const user=userEvent.setup()
    renderInput()
    await user.click(screen.getByRole('button',{name:/tools/i}))
    const fetch=await screen.findByRole('button',{name:/fetch a url|web fetch/i})
    await waitFor(()=>expect(fetch).toHaveAttribute('aria-pressed','true'))
    act(()=>useChatStore.getState().setCurrentConversation(2))
    await waitFor(()=>expect(fetch).toHaveAttribute('aria-pressed','false'))
    act(()=>useChatStore.getState().setCurrentConversation(1))
    await waitFor(()=>expect(fetch).toHaveAttribute('aria-pressed','true'))
    expect(useAppStore.getState().enabledToolIds).toEqual(['web_fetch'])
  })

  it('keeps a started conversation locked before its message history has loaded', async () => {
    vi.mocked(Agent.GetSessionToolSettings).mockResolvedValue({tool_ids:['web_fetch'],locked:true})
    const user=userEvent.setup()
    renderInput()
    await user.click(screen.getByRole('button',{name:/tools/i}))
    const fetch=await screen.findByRole('button',{name:/fetch a url|web fetch/i})
    await waitFor(()=>expect(fetch).toBeDisabled())
    expect(screen.getByText(/Tools are locked after/)).toBeInTheDocument()
    await user.click(fetch)
    expect(Agent.SaveSessionToolSettings).not.toHaveBeenCalled()
    expect(useChatStore.getState().sessionToolIds[1]).toEqual(['web_fetch'])
  })

  it('locks immediately when generation begins even if cached backend metadata is unlocked', async () => {
    const user=userEvent.setup()
    renderInput()
    await user.click(screen.getByRole('button',{name:/tools/i}))
    const fetch=await screen.findByRole('button',{name:/fetch a url|web fetch/i})
    await waitFor(()=>expect(fetch).toBeEnabled())
    act(()=>useChatStore.setState({sessionStatuses:{1:'loading'}}))
    expect(fetch).toBeDisabled()
    await user.click(fetch)
    expect(Agent.SaveSessionToolSettings).not.toHaveBeenCalled()
  })
})
