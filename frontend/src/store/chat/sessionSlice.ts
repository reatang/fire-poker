import i18n from '../../i18n'
import { Agent as AgentBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import type { Attachment, Conversation, ConversationStatus, Message } from '../../types'
import type { StreamingBuffer } from './streamingBuffer'
import type { ChatGet, ChatSet, SessionSlice } from './types'

// createSessionSlice owns the conversation list, per-session tool selection and message loading.
export function createSessionSlice(set: ChatSet, get: ChatGet, buffer: StreamingBuffer): SessionSlice {
  const { clearStreamingBuffer, clearStreamingSeqs } = buffer
  return {
    conversations: [],
    messages: {},
    sessionStatuses: {},
    currentConversationId: null,
    streamingMessages: {},
    pendingTitleGenerations: {},
    sessionRunConfigs: {},
    sessionToolIds: {},
    sessionToolLocks: {},
    loadSessionTools: async (sessionId) => {
      const result = await AgentBinding.GetSessionToolSettings({ session_id: sessionId })
      if (!result) throw new Error('Unable to load session tools')
      const ids = [...result.tool_ids]
      set(state => ({
        sessionToolIds: { ...state.sessionToolIds, [sessionId]: ids },
        sessionToolLocks: { ...state.sessionToolLocks, [sessionId]: result.locked === true },
      }))
      return ids
    },
    saveSessionTools: async (sessionId, ids) => {
      const result = await AgentBinding.SaveSessionToolSettings({ session_id: sessionId, tool_ids: ids })
      if (!result) throw new Error('Unable to save session tools')
      set(state => ({
        sessionToolIds: { ...state.sessionToolIds, [sessionId]: [...result.tool_ids] },
        sessionToolLocks: { ...state.sessionToolLocks, [sessionId]: result.locked === true },
      }))
    },
    pendingConfirms: {},
    sessionsLoading: false,
    hasMoreSessions: true,
    sessionsCursor: 0,

    loadSessions: async (reset = false) => {
      if (get().sessionsLoading) return
      set({ sessionsLoading: true })

      try {
        const cursor = reset ? 0 : get().sessionsCursor
        const result = await AgentBinding.ListSessions({
          cursor,
          limit: 20,
          starred_only: false,
          include_hidden: true,
        })
        if (!result) return

        const nextConversations: Conversation[] = (result.sessions ?? []).map((session) => ({
          id: session.id,
          title: session.title,
          kind: ((session.tags ?? []).includes('task') || session.kind === 'task' ? 'task' : 'user') as Conversation['kind'],
          tags: session.tags ?? (session.kind === 'task' ? ['task'] : []),
          createdAt: session.created,
          updatedAt: session.updated,
          starred: session.starred,
          status: session.status as ConversationStatus,
        }))

        set((state) => ({
          conversations: reset ? nextConversations : [...state.conversations, ...nextConversations],
          sessionStatuses: {
            ...state.sessionStatuses,
            ...Object.fromEntries(nextConversations.map((conversation) => [conversation.id, conversation.status])),
          },
          hasMoreSessions: result.has_more,
          sessionsCursor: result.next_cursor,
        }))
      } finally {
        set({ sessionsLoading: false })
      }
    },

    loadMessages: async (sessionId) => {
      let result = await AgentBinding.LoadSessionMessages({
        session_id: sessionId,
        offset: 0,
        limit: 100,
      })
      if (!result) return

      const allMessages = [...(result.messages ?? [])]
      while (result.has_more) {
        const next = await AgentBinding.LoadSessionMessages({session_id:sessionId,offset:allMessages.length,limit:100})
        if (!next?.messages?.length) throw new Error(i18n.t('chatErrors.messagesIncomplete'))
        allMessages.push(...next.messages)
        result = next
      }
      const messages: Message[] = allMessages.map((message) => {
        let attachments: Attachment[] | undefined
        const raw = message.attachments
        if (raw && raw.trim() !== '') {
          try {
            const parsed = JSON.parse(raw) as Attachment[]
            if (Array.isArray(parsed) && parsed.length > 0) attachments = parsed
          } catch {
            // ignore malformed payload
          }
        }
        return {
          id: message.id,
          sessionId: message.session_id,
          parentId: message.parent_id ?? null,
          role: message.role as Message['role'],
          contentType: message.content_type as Message['contentType'],
          content: message.content,
          modelName: message.model_name,
          agentName: message.agent_name,
          tokensIn: message.tokens_in,
          tokensOut: message.tokens_out,
          extra: message.extra,
          attachments,
          createdAt: message.created,
        }
      })

      // For task sessions still running, append a temp assistant placeholder
      // so the streaming message container is valid when the user opens the session
      const sessionStatus =
        get().sessionStatuses[sessionId] ??
        get().conversations.find((c) => c.id === sessionId)?.status
      const lastMsg = messages[messages.length - 1]
      if (
        (sessionStatus === 'loading' || sessionStatus === 'waiting-unread') &&
        (!lastMsg || lastMsg.role !== 'assistant' || lastMsg.contentType === 'tool_call')
      ) {
        messages.push({
          id: -(sessionId * 1000),
          sessionId,
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
        })
      }

      set((state) => ({
        messages: { ...state.messages, [sessionId]: messages },
      }))
    },

    forkConversation: async (sessionId, messageId) => {
      const result = await AgentBinding.ForkSession({session_id:sessionId,message_id:messageId})
      if (!result?.session) throw new Error(i18n.t('chatErrors.forkNotCreated'))
      const fork = result.session
      set(state => ({
        conversations: [{id:fork.id,title:fork.title,kind:'user',tags:fork.tags ?? [],starred:false,status:'idle',createdAt:fork.created,updatedAt:fork.updated},...state.conversations],
        sessionStatuses: {...state.sessionStatuses,[fork.id]:'idle'},
        sessionRunConfigs: {...state.sessionRunConfigs,[fork.id]:state.sessionRunConfigs[sessionId]},
      }))
      // Switch only after the complete persisted prefix has loaded successfully.
      // On load failure, the created fork remains discoverable in the sidebar.
      await get().loadMessages(fork.id)
      set({currentConversationId:fork.id})
    },

    createConversation: async (params) => {
      const result = await AgentBinding.CreateSession({ title: params?.title ?? '', tags: params?.tags ?? [] })
      if (!result) return 0

      const tags = result.tags ?? params?.tags ?? []
      const conversation: Conversation = {
        id: result.session_id,
        title: result.title,
        kind: tags.includes('task') ? 'task' : 'user',
        tags,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        starred: false,
        status: 'idle',
      }

      set((state) => ({
        conversations: [conversation, ...state.conversations],
        messages: { ...state.messages, [result.session_id]: [] },
        pendingTitleGenerations: {
          ...state.pendingTitleGenerations,
          [result.session_id]: undefined,
        },
        sessionRunConfigs: {
          ...state.sessionRunConfigs,
          [result.session_id]: undefined,
        },
        sessionStatuses: {
          ...state.sessionStatuses,
          [result.session_id]: 'idle',
        },
      }))
      return result.session_id
    },

    deleteConversation: async (id) => {
      await AgentBinding.DeleteSession({ session_id: id })
      clearStreamingBuffer(id)
      clearStreamingSeqs(id)
      set((state) => {
        const nextMessages = { ...state.messages }
        const nextPendingTitleGenerations = { ...state.pendingTitleGenerations }
        const nextSessionRunConfigs = { ...state.sessionRunConfigs }
   const nextSessionToolIds = { ...state.sessionToolIds }
   const nextSessionToolLocks = { ...state.sessionToolLocks }
        const nextPendingConfirms = { ...state.pendingConfirms }
        const nextStreamingMessages = { ...state.streamingMessages }
        const nextSessionStatuses = { ...state.sessionStatuses }

        delete nextMessages[id]
        delete nextPendingTitleGenerations[id]
        delete nextSessionRunConfigs[id]
   delete nextSessionToolIds[id]
   delete nextSessionToolLocks[id]
        delete nextPendingConfirms[id]
        delete nextStreamingMessages[id]
        delete nextSessionStatuses[id]

        return {
          conversations: state.conversations.filter((conversation) => conversation.id !== id),
          currentConversationId: state.currentConversationId === id ? null : state.currentConversationId,
          messages: nextMessages,
          pendingTitleGenerations: nextPendingTitleGenerations,
          sessionRunConfigs: nextSessionRunConfigs,
   sessionToolIds: nextSessionToolIds,
   sessionToolLocks: nextSessionToolLocks,
          pendingConfirms: nextPendingConfirms,
          streamingMessages: nextStreamingMessages,
          sessionStatuses: nextSessionStatuses,
        }
      })
    },

    renameConversation: async (id, title) => {
      await AgentBinding.RenameSession({ session_id: id, title })
      set((state) => ({
        conversations: state.conversations.map((conversation) =>
          conversation.id === id
            ? { ...conversation, title, updatedAt: new Date().toISOString() }
            : conversation
        ),
      }))
    },

    toggleStar: async (id, starred) => {
      await AgentBinding.ToggleStarSession({ session_id: id, starred })
      set((state) => ({
        conversations: state.conversations.map((conversation) =>
          conversation.id === id ? { ...conversation, starred } : conversation
        ),
      }))
    },

    markAsRead: async (id) => {
      const conversation = get().conversations.find((item) => item.id === id)
      if (!conversation || conversation.status === 'idle' || conversation.status === 'loading') return

      set((state) => ({
        conversations: state.conversations.map((item) =>
          item.id === id ? { ...item, status: 'idle' } : item
        ),
      }))

      await AgentBinding.MarkSessionRead({ session_id: id })
    },

    setCurrentConversation: (id) => {
      if (id) void get().markAsRead(id)
      if (id && get().messages[id] === undefined) {
        void get().loadMessages(id)
      }
      set({ currentConversationId: id })
    },

    setConversationStatus: (id, status) =>
      set((state) => ({
        conversations: state.conversations.map((conversation) =>
          conversation.id === id ? { ...conversation, status } : conversation
        ),
      })),
  }
}
