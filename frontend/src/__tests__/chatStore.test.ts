import { vi } from 'vitest'
import { describe, it, expect, beforeEach } from 'vitest'
import { useChatStore } from '../store/chatStore'
import { Agent as AgentBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { Provider as ProviderBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/provider'

const eventHandlers = new Map<string, (event: { data: unknown }) => void>()
const rafCallbacks = new Map<number, FrameRequestCallback>()
let rafId = 0

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  // Keep the real Create helpers: generated event bindings use them at import time.
  ...(await importOriginal<typeof import('@wailsio/runtime')>()),
  Events: {
    On: vi.fn((name: string, handler: (event: { data: unknown }) => void) => {
      eventHandlers.set(name, handler)
      return () => eventHandlers.delete(name)
    }),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/agent', () => ({
  Agent: {
    GetSessionToolSettings: vi.fn().mockResolvedValue({ tool_ids: [] }),
    SaveSessionToolSettings: vi.fn(async (input: {tool_ids: string[]}) => ({ tool_ids: input.tool_ids })),
    CreateSession: vi.fn().mockResolvedValue({ session_id: 1, title: 'New Chat' }),
    ForkSession: vi.fn(),
    DeleteSession: vi.fn().mockResolvedValue(undefined),
    RenameSession: vi.fn().mockResolvedValue(undefined),
    ToggleStarSession: vi.fn().mockResolvedValue(undefined),
    ListSessions: vi.fn().mockResolvedValue({ sessions: [], has_more: false, next_cursor: 0 }),
    LoadSessionMessages: vi.fn().mockResolvedValue({ messages: [], total: 0, has_more: false }),
    MarkSessionRead: vi.fn().mockResolvedValue(undefined),
    SendMessage: vi.fn().mockResolvedValue(undefined),
    StopGeneration: vi.fn().mockResolvedValue(undefined),
    RespondToConfirm: vi.fn().mockResolvedValue(undefined),
    GenerateTitle: vi.fn().mockResolvedValue({ title: 'New Chat' }),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/provider', () => ({
  Provider: {
    ProviderAndModelList: vi.fn().mockResolvedValue({ provider_models: [] }),
  },
}))

beforeEach(() => {
  eventHandlers.clear()
  rafCallbacks.clear()
  rafId = 0
  ;(globalThis as { __firePokerChatListenersInitialized?: boolean }).__firePokerChatListenersInitialized = false
  vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => {
    rafId += 1
    rafCallbacks.set(rafId, callback)
    return rafId
  }))
  vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => {
    rafCallbacks.delete(id)
  }))
  useChatStore.setState({
    conversations: [],
    messages: {},
    currentConversationId: null,
    streamingMessages: {},
    pendingTitleGenerations: {},
    sessionRunConfigs: {},
    pendingConfirms: {},
    sessionsLoading: false,
    hasMoreSessions: true,
    sessionsCursor: 0,
  })
})

describe('chatStore', () => {
  it('replaces optimistic temporary attachment paths with the persisted session paths', async () => {
    vi.mocked(AgentBinding.SendMessage).mockResolvedValueOnce({ attachments: [{
      path: '/data/sessions/1/attachments/photo.png', name: 'photo.png', mime: 'image/png', kind: 'image',
    }] })
    await useChatStore.getState().sendMessage({
      sessionId: 1, content: 'describe', providerId: 1, modelName: 'm', enabledUserTools: [],
      attachments: [{ path: '/old/temp/photo.png', name: 'photo.png', mime: 'image/png', kind: 'image' }],
    })
    const question = useChatStore.getState().messages[1].find(message => message.role === 'user')
    expect(question?.attachments?.[0].path).toBe('/data/sessions/1/attachments/photo.png')
  })
  it('reloads the persisted partial answer after stopping and clears streaming state', async () => {
    vi.mocked(AgentBinding.StopGeneration).mockResolvedValue({})
    vi.mocked(AgentBinding.LoadSessionMessages).mockResolvedValueOnce({
      messages: [{ id: 12, session_id: 1, parent_id: null, role: 'assistant', content_type: 'text', content: '保留的半截回答', model_name: 'test', agent_name: '', tokens_in: 0, tokens_out: 0, extra: 'interrupted', attachments: '', created: new Date().toISOString() }],
      total: 1, has_more: false,
    })
    useChatStore.setState({ sessionStatuses: { 1: 'loading' }, streamingMessages: { 1: { content: '保留的半截回答', thinking: '' } }, pendingConfirms: { 1: { requestId: 'pending', toolName: 'shell', args: '{}', purpose: 'test' } } })
    await useChatStore.getState().stopGeneration(1)
    expect(useChatStore.getState().messages[1][0].extra).toBe('interrupted')
    expect(useChatStore.getState().streamingMessages[1]).toBeUndefined()
    expect(useChatStore.getState().pendingConfirms[1]).toBeNull()
    expect(useChatStore.getState().sessionStatuses[1]).toBe('idle')
  })
  it('createConversation adds a conversation and returns its id', async () => {
    const id = await useChatStore.getState().createConversation()
    const convs = useChatStore.getState().conversations
    expect(convs).toHaveLength(1)
    expect(convs[0].id).toBe(id)
    expect(convs[0].title).toBe('New Chat')
    expect(convs[0].status).toBe('idle')
  })

  it('deleteConversation removes the conversation', async () => {
    const id = await useChatStore.getState().createConversation()
    await useChatStore.getState().deleteConversation(id)
    expect(useChatStore.getState().conversations).toHaveLength(0)
  })

  it('renameConversation updates title', async () => {
    const id = await useChatStore.getState().createConversation()
    await useChatStore.getState().renameConversation(id, 'My Chat')
    const conv = useChatStore.getState().conversations.find(c => c.id === id)
    expect(conv?.title).toBe('My Chat')
  })

  it('generateTitleDraft returns a suggested title without mutating the conversation title', async () => {
    const id = await useChatStore.getState().createConversation()
    const generateTitleMock = vi.mocked(AgentBinding.GenerateTitle)
    generateTitleMock.mockResolvedValueOnce({ title: 'Suggested Title' })

    await useChatStore.getState().sendMessage({
      sessionId: id,
      content: 'hello world',
      providerId: 7,
      modelName: 'gpt-test',
      enabledUserTools: [],
      attachments: [],
    })

    const draftTitle = await useChatStore.getState().generateTitleDraft(id)

    expect(draftTitle).toBe('Suggested Title')
    expect(generateTitleMock).toHaveBeenLastCalledWith({
      session_id: id,
      provider_id: 7,
      model_name: 'gpt-test',
    })
    expect(useChatStore.getState().conversations.find(c => c.id === id)?.title).toBe('New Chat')
  })

  it('toggleStar updates starred field', async () => {
    const id = await useChatStore.getState().createConversation()
    await useChatStore.getState().toggleStar(id, true)
    expect(useChatStore.getState().conversations[0].starred).toBe(true)
    await useChatStore.getState().toggleStar(id, false)
    expect(useChatStore.getState().conversations[0].starred).toBe(false)
  })

  it('markAsRead resets unread status to idle and persists it', async () => {
    const id = await useChatStore.getState().createConversation()
    const markSessionReadMock = vi.mocked(AgentBinding.MarkSessionRead)

    useChatStore.setState(s => ({
      conversations: s.conversations.map(c => c.id === id ? { ...c, status: 'done-unread' as const } : c)
    }))
    markSessionReadMock.mockClear()

    await useChatStore.getState().markAsRead(id)

    const conv = useChatStore.getState().conversations.find(c => c.id === id)
    expect(conv?.status).toBe('idle')
    expect(markSessionReadMock).toHaveBeenCalledWith({ session_id: id })
  })

  it('createConversation initializes an empty message list', async () => {
    const id = await useChatStore.getState().createConversation()
    expect(useChatStore.getState().messages[id]).toEqual([])
  })

  it('setCurrentConversation does not reload messages that are already initialized', async () => {
    const id = await useChatStore.getState().createConversation()
    const loadMessagesMock = vi.mocked(AgentBinding.LoadSessionMessages)

    loadMessagesMock.mockClear()
    useChatStore.getState().setCurrentConversation(id)

    expect(loadMessagesMock).not.toHaveBeenCalled()
  })

  it('sendMessage adds optimistic user and assistant messages immediately', async () => {
    const id = await useChatStore.getState().createConversation()
    const generateTitleMock = vi.mocked(AgentBinding.GenerateTitle)
    generateTitleMock.mockClear()

    await useChatStore.getState().sendMessage({
      sessionId: id,
      content: 'hello world',
      providerId: 7,
      modelName: 'gpt-test',
      enabledUserTools: [],
      attachments: [],
    })

    const sessionMessages = useChatStore.getState().messages[id]
    expect(sessionMessages).toHaveLength(2)
    expect(sessionMessages[0]).toMatchObject({
      sessionId: id,
      role: 'user',
      contentType: 'text',
      content: 'hello world',
    })
    expect(sessionMessages[1]).toMatchObject({
      sessionId: id,
      role: 'assistant',
      contentType: 'text',
      content: '',
      modelName: 'gpt-test',
    })
    expect(useChatStore.getState().streamingMessages[id]).toEqual({
      content: '',
      thinking: '',
    })
    expect(generateTitleMock).not.toHaveBeenCalled()
  })

  it('generates a title once after the first response completes and leaves it unchanged on later turns', async () => {
    const id = await useChatStore.getState().createConversation()
    const generateTitleMock = vi.mocked(AgentBinding.GenerateTitle)
    generateTitleMock.mockClear()

    await useChatStore.getState().sendMessage({
      sessionId: id,
      content: 'hello world',
      providerId: 7,
      modelName: 'gpt-test',
      enabledUserTools: [],
      attachments: [],
    })

    useChatStore.getState().initEventListeners()

    expect(generateTitleMock).not.toHaveBeenCalled()

    eventHandlers.get('agent:session:status')?.({
      data: {
        sessionId: id,
        status: 'waiting-unread',
      },
    })

    expect(generateTitleMock).not.toHaveBeenCalled()

    eventHandlers.get('agent:session:status')?.({
      data: {
        sessionId: id,
        status: 'done-unread',
      },
    })

    expect(generateTitleMock).toHaveBeenCalledTimes(1)
    expect(generateTitleMock).toHaveBeenCalledWith({
      session_id: id,
      provider_id: 7,
      model_name: 'gpt-test',
    })

    await useChatStore.getState().sendMessage({
      sessionId: id,
      content: 'follow-up question',
      providerId: 7,
      modelName: 'gpt-test',
      enabledUserTools: [],
      attachments: [],
    })
    eventHandlers.get('agent:session:status')?.({ data: { sessionId: id, status: 'done-unread' } })
    eventHandlers.get('agent:session:status')?.({ data: { sessionId: id, status: 'done-unread' } })
    expect(generateTitleMock).toHaveBeenCalledTimes(1)
  })

  it.each(['error', 'interrupted', 'stopped'] as const)('does not defer title generation to later turns after the first response is %s', async (outcome) => {
    const id = await useChatStore.getState().createConversation()
    const generateTitleMock = vi.mocked(AgentBinding.GenerateTitle)
    generateTitleMock.mockClear()
    useChatStore.getState().initEventListeners()
    const params = {
      sessionId: id,
      content: 'first question',
      providerId: 7,
      modelName: 'gpt-test',
      enabledUserTools: [],
      attachments: [],
    }
    await useChatStore.getState().sendMessage(params)
    vi.mocked(AgentBinding.LoadSessionMessages).mockResolvedValueOnce({
      messages: [{ id: 1, session_id: id, parent_id: null, role: 'user', content_type: 'text', content: 'first question', model_name: '', agent_name: '', tokens_in: 0, tokens_out: 0, extra: '', attachments: '', created: new Date().toISOString() }],
      total: 1, has_more: false,
    })
    if (outcome === 'error') {
      eventHandlers.get('agent:session:status')?.({ data: { sessionId: id, status: 'error-unread' } })
    } else if (outcome === 'interrupted') {
      eventHandlers.get('agent:stream:done')?.({ data: { sessionId: id, interrupted: true } })
      // Wait for the persisted first question to finish loading before the next turn.
      await Promise.resolve()
    } else {
      await useChatStore.getState().stopGeneration(id)
    }
    expect(generateTitleMock).not.toHaveBeenCalled()
    expect(useChatStore.getState().pendingTitleGenerations[id]).toBeUndefined()

    await useChatStore.getState().sendMessage({ ...params, content: 'follow-up question' })
    eventHandlers.get('agent:session:status')?.({ data: { sessionId: id, status: 'done-unread' } })
    expect(generateTitleMock).not.toHaveBeenCalled()
  })

  it('initEventListeners appends stream chunks from Wails event data', () => {
    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 1,
        delta: 'hello',
        contentType: 'text',
      },
    })
    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 1,
        delta: ' world',
        contentType: 'text',
      },
    })
    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 1,
        delta: 'thinking',
        contentType: 'thinking',
      },
    })

    expect(useChatStore.getState().streamingMessages[1]).toBeUndefined()

    rafCallbacks.forEach((callback) => callback(0))
    rafCallbacks.clear()

    expect(useChatStore.getState().streamingMessages[1]).toEqual({
      content: 'hello world',
      thinking: 'thinking',
    })
  })

  it('allows event listeners to be reinitialized after cleanup', () => {
    const off = useChatStore.getState().initEventListeners()
    expect(eventHandlers.has('agent:stream:chunk')).toBe(true)

    off()
    expect(eventHandlers.has('agent:stream:chunk')).toBe(false)

    useChatStore.getState().initEventListeners()
    expect(eventHandlers.has('agent:stream:chunk')).toBe(true)
  })

  it('batches multiple stream chunks into one visible update per frame', () => {
    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 2,
        delta: 'hello',
        contentType: 'text',
      },
    })
    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 2,
        delta: ' world',
        contentType: 'text',
      },
    })

    expect(useChatStore.getState().streamingMessages[2]).toBeUndefined()
    expect(vi.mocked(requestAnimationFrame)).toHaveBeenCalledTimes(1)

    rafCallbacks.forEach((callback) => callback(0))
    rafCallbacks.clear()

    expect(useChatStore.getState().streamingMessages[2]).toEqual({
      content: 'hello world',
      thinking: '',
    })
  })

  it('preserves every chunk when Wails delivers batched stream events', () => {
    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:stream:chunk')?.({
      data: [
        {
          sessionId: 8,
          delta: 'A',
          contentType: 'text',
        },
        {
          sessionId: 8,
          delta: 'B',
          contentType: 'text',
        },
        {
          sessionId: 8,
          delta: 'C',
          contentType: 'text',
        },
      ],
    })

    rafCallbacks.forEach((callback) => callback(0))
    rafCallbacks.clear()

    expect(useChatStore.getState().streamingMessages[8]).toEqual({
      content: 'ABC',
      thinking: '',
    })
  })

  it('ignores stale chunk snapshots when stream events arrive out of order', () => {
    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 9,
        seq: 2,
        delta: 'B',
        content: 'AB',
        contentType: 'thinking',
      },
    })
    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 9,
        seq: 1,
        delta: 'A',
        content: 'A',
        contentType: 'thinking',
      },
    })

    rafCallbacks.forEach((callback) => callback(0))
    rafCallbacks.clear()

    expect(useChatStore.getState().streamingMessages[9]).toEqual({
      content: '',
      thinking: 'AB',
    })
  })

  it('flushes pending chunks before stream completion cleanup', async () => {
    const loadMessagesMock = vi.mocked(AgentBinding.LoadSessionMessages)
    loadMessagesMock.mockClear()

    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:stream:chunk')?.({
      data: {
        sessionId: 3,
        delta: 'final chunk',
        contentType: 'text',
      },
    })

    expect(useChatStore.getState().streamingMessages[3]).toBeUndefined()

    eventHandlers.get('agent:stream:done')?.({
      data: {
        sessionId: 3,
        usage: { input: 1, output: 1 },
      },
    })

    expect(useChatStore.getState().streamingMessages[3]).toBeUndefined()
    expect(loadMessagesMock).toHaveBeenCalledWith({
      session_id: 3,
      offset: 0,
      limit: 100,
    })
  })

  it('updates the latest streaming tool result instead of appending duplicates', () => {
    useChatStore.getState().initEventListeners()
    useChatStore.setState({
      messages: {
        5: [
          {
            id: 11,
            sessionId: 5,
            parentId: null,
            role: 'assistant',
            contentType: 'tool_call',
            content: '{"name":"shell","args":{"command":"echo hi"}}',
            modelName: '',
            agentName: '',
            tokensIn: 0,
            tokensOut: 0,
            extra: 'run shell',
            createdAt: new Date().toISOString(),
          },
          {
            id: -21,
            sessionId: 5,
            parentId: null,
            role: 'tool',
            contentType: 'tool_result',
            content: 'partial',
            modelName: '',
            agentName: 'shell',
            tokensIn: 0,
            tokensOut: 0,
            extra: '',
            createdAt: new Date().toISOString(),
          },
          {
            id: -22,
            sessionId: 5,
            parentId: null,
            role: 'assistant',
            contentType: 'text',
            content: '',
            modelName: '',
            agentName: '',
            tokensIn: 0,
            tokensOut: 0,
            extra: '',
            createdAt: new Date().toISOString(),
          },
        ],
      },
    })

    eventHandlers.get('agent:stream:tool_result')?.({
      data: {
        sessionId: 5,
        toolName: 'shell',
        result: 'partial\nmore',
      },
    })

    const sessionMessages = useChatStore.getState().messages[5]
    const toolResults = sessionMessages.filter((message) => message.contentType === 'tool_result')
    expect(toolResults).toHaveLength(1)
    expect(toolResults[0].content).toBe('partial\nmore')
  })

  it('keeps the active conversation read when done-unread status arrives', async () => {
    const id = await useChatStore.getState().createConversation()
    const markSessionReadMock = vi.mocked(AgentBinding.MarkSessionRead)

    useChatStore.setState({
      currentConversationId: id,
      conversations: useChatStore.getState().conversations.map(c =>
        c.id === id ? { ...c, status: 'loading' as const } : c
      ),
    })

    useChatStore.getState().initEventListeners()
    markSessionReadMock.mockClear()

    eventHandlers.get('agent:session:status')?.({
      data: {
        sessionId: id,
        status: 'done-unread',
      },
    })

    const conv = useChatStore.getState().conversations.find(c => c.id === id)
    expect(conv?.status).toBe('idle')
    expect(markSessionReadMock).toHaveBeenCalledWith({ session_id: id })
  })

  it('keeps waiting-unread for the active conversation so confirm UI can stay visible', async () => {
    const id = await useChatStore.getState().createConversation()
    const markSessionReadMock = vi.mocked(AgentBinding.MarkSessionRead)
    const generateTitleMock = vi.mocked(AgentBinding.GenerateTitle)

    useChatStore.setState({
      currentConversationId: id,
      conversations: useChatStore.getState().conversations.map(c =>
        c.id === id ? { ...c, status: 'loading' as const } : c
      ),
      pendingTitleGenerations: {
        [id]: {
          providerId: 7,
          modelName: 'gpt-test',
        },
      },
    })

    useChatStore.getState().initEventListeners()
    markSessionReadMock.mockClear()
    generateTitleMock.mockClear()

    eventHandlers.get('agent:session:status')?.({
      data: {
        sessionId: id,
        status: 'waiting-unread',
      },
    })

    const conv = useChatStore.getState().conversations.find(c => c.id === id)
    expect(conv?.status).toBe('waiting-unread')
    expect(markSessionReadMock).not.toHaveBeenCalled()
    expect(generateTitleMock).not.toHaveBeenCalled()
  })

  it('refreshes the conversation list after a task session is spawned', () => {
    const listSessionsMock = vi.mocked(AgentBinding.ListSessions)
    listSessionsMock.mockClear()

    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:session:spawned')?.({
      data: {
        sessionId: 42,
        title: 'Install CLI from official docs',
        kind: 'task',
        userMessage: 'install feishu cli',
      },
    })

    expect(useChatStore.getState().conversations[0]).toMatchObject({
      id: 42,
      title: 'Install CLI from official docs',
      kind: 'task',
      status: 'loading',
    })
    expect(useChatStore.getState().messages[42]?.[0]).toMatchObject({
      role: 'user',
      contentType: 'text',
      content: 'install feishu cli',
    })
    expect(useChatStore.getState().messages[42]?.[1]).toMatchObject({
      role: 'assistant',
      contentType: 'text',
      content: '',
    })
    expect(listSessionsMock).toHaveBeenCalledWith({
      cursor: 0,
      limit: 20,
      starred_only: false,
      include_hidden: true,
    })
  })

  it('adds an assistant placeholder when an open session enters loading', async () => {
    const id = await useChatStore.getState().createConversation()

    useChatStore.setState({
      currentConversationId: id,
      conversations: useChatStore.getState().conversations.map(c =>
        c.id === id ? { ...c, status: 'idle' as const } : c
      ),
      sessionStatuses: {
        [id]: 'idle',
      },
      messages: {
        [id]: [{
          id: 100,
          sessionId: id,
          parentId: null,
          role: 'user',
          contentType: 'text',
          content: 'install this cli',
          modelName: '',
          agentName: '',
          tokensIn: 0,
          tokensOut: 0,
          extra: '',
          createdAt: '2026-05-26T00:00:00Z',
        }],
      },
    })

    useChatStore.getState().initEventListeners()

    eventHandlers.get('agent:session:status')?.({
      data: {
        sessionId: id,
        status: 'loading',
      },
    })

    const messages = useChatStore.getState().messages[id] ?? []
    expect(messages).toHaveLength(2)
    expect(messages[1]).toMatchObject({
      role: 'assistant',
      contentType: 'text',
      content: '',
    })
  })
})

describe('stream error handling', () => {
  it('calls loadMessages after stream:error event', () => {
    const loadMessagesMock = vi.mocked(AgentBinding.LoadSessionMessages)
    loadMessagesMock.mockClear()

    const off = useChatStore.getState().initEventListeners()

    useChatStore.setState({
      conversations: [{ id: 1, title: 'chat', kind: 'user' as const, createdAt: '', updatedAt: '', starred: false, status: 'loading' }],
      messages: { 1: [] },
      streamingMessages: { 1: { content: 'partial', thinking: '' } },
    })

    const handler = eventHandlers.get('agent:stream:error')
    expect(handler).toBeDefined()
    handler!({ data: { sessionId: 1, error: 'rate limit' } })

    expect(loadMessagesMock).toHaveBeenCalledWith(
      expect.objectContaining({ session_id: 1 })
    )
    expect(useChatStore.getState().streamingMessages[1]).toBeUndefined()

    off()
  })
})

describe('sendMessage error handling', () => {
  it('replaces temp assistant message with error bubble when SendMessage throws', async () => {
    vi.mocked(AgentBinding.SendMessage).mockRejectedValueOnce(
      new Error(JSON.stringify({ msg: 'Failed to send message', detail: 'api key invalid' }))
    )

    useChatStore.setState({
      conversations: [{ id: 1, title: 'chat', kind: 'user' as const, createdAt: '', updatedAt: '', starred: false, status: 'idle' }],
      messages: { 1: [] },
      streamingMessages: {},
    })

    await useChatStore.getState().sendMessage({
      sessionId: 1,
      content: 'hello',
      providerId: 7,
      modelName: 'gpt-4',
      enabledUserTools: [],
      attachments: [],
    })

    const messages = useChatStore.getState().messages[1] ?? []
    const errorMsg = messages.find((m) => m.contentType === 'error')
    expect(errorMsg).toBeDefined()
    const parsed = JSON.parse(errorMsg!.content) as { msg: string; detail: string }
    expect(parsed.msg).toBe('Failed to send message')
    expect(parsed.detail).toBe('api key invalid')

    const status = useChatStore.getState().conversations.find((c) => c.id === 1)?.status
    expect(status).toBe('idle')
  })
})


describe('regeneration state', () => {
  const original = [
    {id:101, sessionId:1, parentId:null, role:'user' as const, contentType:'text' as const, content:'question', modelName:'', agentName:'', tokensIn:0, tokensOut:0, extra:'', createdAt:''},
    {id:102, sessionId:1, parentId:null, role:'assistant' as const, contentType:'text' as const, content:'old answer', modelName:'m', agentName:'', tokensIn:0, tokensOut:0, extra:'', createdAt:''},
  ]
  const params = {sessionId:1, regenerateMessageId:102, content:'question', providerId:7, modelName:'m', enabledUserTools:[], attachments:[]}
  it('keeps one question while retrying and forwards the response ID', async () => {
    vi.mocked(AgentBinding.SendMessage).mockResolvedValueOnce({ attachments: [] })
    useChatStore.setState({messages:{1:original},sessionStatuses:{1:'idle'}})
    await useChatStore.getState().sendMessage(params)
    const messages = useChatStore.getState().messages[1]
    expect(messages.filter(m=>m.role==='user')).toHaveLength(1)
    expect(messages[0].id).toBe(101)
    expect(messages.some(m=>m.content==='old answer')).toBe(false)
    expect(AgentBinding.SendMessage).toHaveBeenLastCalledWith(expect.objectContaining({regenerate_message_id:102,content:'question'}))
  })
  it('restores the original answer if the retry cannot start', async () => {
    vi.mocked(AgentBinding.SendMessage).mockRejectedValueOnce(new Error('unavailable'))
    useChatStore.setState({messages:{1:original},sessionStatuses:{1:'idle'}})
    await expect(useChatStore.getState().sendMessage(params)).rejects.toThrow('unavailable')
    expect(useChatStore.getState().messages[1]).toEqual(original)
    expect(useChatStore.getState().sessionStatuses[1]).toBe('idle')
    expect(useChatStore.getState().streamingMessages[1]).toBeUndefined()
  })
  it('retries with the provider that produced the answer when several offer the same model', async () => {
    const providerModel = (id: number) => ({
      provider: { id, icon: '', provider_type: 'openai_compatibility', provider_name: `P${id}`, base_url: `http://p${id}`, enabled: true, api_key: '', is_default: false },
      models: [{ id: id * 10, provider_id: id, model: 'm', owned_by: '', object: '', enable: true, alias: null, is_custom: false, is_default: false }],
    })
    vi.mocked(ProviderBinding.ProviderAndModelList).mockResolvedValueOnce({ provider_models: [providerModel(3), providerModel(7)] } as never)
    vi.mocked(AgentBinding.SendMessage).mockResolvedValueOnce({ attachments: [] })
    useChatStore.setState({
      messages: { 1: original },
      sessionStatuses: { 1: 'idle' },
      sessionRunConfigs: { 1: { providerId: 7, modelName: 'm' } },
    })

    await useChatStore.getState().regenerateMessage(1, 102)

    expect(AgentBinding.SendMessage).toHaveBeenLastCalledWith(expect.objectContaining({
      regenerate_message_id: 102,
      provider_id: 7,
      model_name: 'm',
    }))
  })
})


describe('conversation forks', () => {
  const session = {id:900,title:'Source · 分叉',kind:'user',tags:[],starred:false,status:'idle',created:'2026-10-23',updated:'2026-10-23'}
  const raw = (id:number) => ({id,session_id:900,parent_id:null,role:'assistant',content_type:'text',content:'copied '+id,model_name:'m',agent_name:'',tokens_in:0,tokens_out:0,extra:'',attachments:'',created:''})
  it('loads all pages before switching to the fork and keeps the source intact', async () => {
    const source = [{id:7,sessionId:1,parentId:null,role:'user' as const,contentType:'text' as const,content:'source',modelName:'',agentName:'',tokensIn:0,tokensOut:0,extra:'',createdAt:''}]
    useChatStore.setState({currentConversationId:1,messages:{1:source},sessionStatuses:{}})
    vi.mocked(AgentBinding.ForkSession).mockResolvedValueOnce({session})
    vi.mocked(AgentBinding.LoadSessionMessages)
      .mockResolvedValueOnce({messages:Array.from({length:100},(_,i)=>raw(1000+i)),total:102,has_more:true})
      .mockResolvedValueOnce({messages:[raw(1100),raw(1101)],total:102,has_more:false})
    await useChatStore.getState().forkConversation(1,7)
    expect(AgentBinding.ForkSession).toHaveBeenLastCalledWith({session_id:1,message_id:7})
    expect(AgentBinding.LoadSessionMessages).toHaveBeenLastCalledWith({session_id:900,offset:100,limit:100})
    expect(useChatStore.getState().currentConversationId).toBe(900)
    expect(useChatStore.getState().messages[900]).toHaveLength(102)
    expect(useChatStore.getState().messages[1]).toBe(source)
    expect(useChatStore.getState().conversations[0].title).toBe('Source · 分叉')
  })
  it('keeps the current conversation if creating the fork fails', async () => {
    useChatStore.setState({currentConversationId:1})
    vi.mocked(AgentBinding.ForkSession).mockRejectedValueOnce(new Error('cannot fork'))
    await expect(useChatStore.getState().forkConversation(1,7)).rejects.toThrow('cannot fork')
    expect(useChatStore.getState().currentConversationId).toBe(1)
    expect(useChatStore.getState().conversations).toHaveLength(0)
  })
})
