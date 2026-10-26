import { Events } from '@wailsio/runtime'
import { Agent as AgentBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import * as ChatEvents from '@bindings/events'
import type {
  ConfirmRequestEvent,
  Conversation,
  ConversationStatus,
  Message,
  SessionStatusEvent,
  StreamDoneEvent,
  StreamErrorEvent,
  ToolCallEvent,
  ToolResultEvent,
} from '../../types'
import { asStreamChunkEvent, eventItems, findLatestStreamToolResultIndex } from './helpers'
import type { StreamingBuffer } from './streamingBuffer'
import type { ChatGet, ChatSet, ChatStore } from './types'

// createInitEventListeners subscribes the store to backend chat events once per page and returns
// the unsubscribe function.
export function createInitEventListeners(set: ChatSet, get: ChatGet, buffer: StreamingBuffer): ChatStore['initEventListeners'] {
  const { clearStreamingSeqs, flushStreamingBuffer, appendStreamingDelta, ensureStreamingPlaceholder } = buffer
  return () => {
    if ((globalThis as { __firePokerChatListenersInitialized?: boolean }).__firePokerChatListenersInitialized) {
      return () => {}
    }
    ;(globalThis as { __firePokerChatListenersInitialized?: boolean }).__firePokerChatListenersInitialized = true

    const offs: Array<() => void> = []

    offs.push(Events.On(ChatEvents.AgentStreamChunk, (event) => {
      for (const raw of eventItems(event)) {
        if (!raw) continue
        const data = asStreamChunkEvent(raw)

        ensureStreamingPlaceholder(data.sessionId)
        appendStreamingDelta(data)
      }
    }))

    offs.push(Events.On(ChatEvents.AgentStreamConfirmRequest, (event: { data: ConfirmRequestEvent | ConfirmRequestEvent[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue

        set((state) => ({
          pendingConfirms: {
            ...state.pendingConfirms,
            [data.sessionId]: {
              requestId: data.requestId,
              toolName: data.toolName,
              args: data.args,
              purpose: data.purpose,
            },
          },
        }))
      }
    }))

    offs.push(Events.On(ChatEvents.AgentStreamToolCall, (event: { data: ToolCallEvent | ToolCallEvent[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue

        let parsedArgs: Record<string, unknown>
        try {
          parsedArgs = JSON.parse(data.args) as Record<string, unknown>
        } catch {
          parsedArgs = {}
        }
        const payload = JSON.stringify({ name: data.toolName, args: parsedArgs })
        const tempId = -Date.now() - (Math.random() * 1000 | 0)

        set((state) => {
          const sessionMessages = state.messages[data.sessionId] ?? []
          const insertIndex = sessionMessages.length > 0 ? sessionMessages.length - 1 : 0

          const toolCallMessage: Message = {
            id: tempId,
            sessionId: data.sessionId,
            parentId: null,
            role: 'assistant',
            contentType: 'tool_call',
            content: payload,
            modelName: '',
            agentName: '',
            tokensIn: 0,
            tokensOut: 0,
            extra: data.purpose,
            createdAt: new Date().toISOString(),
          }

          const newMessages = [...sessionMessages]
          newMessages.splice(insertIndex, 0, toolCallMessage)

          return {
            messages: {
              ...state.messages,
              [data.sessionId]: newMessages,
            },
          }
        })
      }
    }))

    offs.push(Events.On(ChatEvents.AgentStreamToolResult, (event: { data: ToolResultEvent | ToolResultEvent[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue

        set((state) => {
          const sessionMessages = state.messages[data.sessionId] ?? []
          const insertIndex = sessionMessages.length > 0 ? sessionMessages.length - 1 : 0
          const latestToolResultIndex = findLatestStreamToolResultIndex(sessionMessages, data.toolName)

          if (latestToolResultIndex >= 0) {
            const nextMessages = [...sessionMessages]
            nextMessages[latestToolResultIndex] = {
              ...nextMessages[latestToolResultIndex],
              content: data.result,
              createdAt: new Date().toISOString(),
            }
            return {
              messages: {
                ...state.messages,
                [data.sessionId]: nextMessages,
              },
            }
          }

          const tempId = -Date.now() - (Math.random() * 1000 | 0)

          const toolResultMessage: Message = {
            id: tempId,
            sessionId: data.sessionId,
            parentId: null,
            role: 'tool',
            contentType: 'tool_result',
            content: data.result,
            modelName: '',
            agentName: data.toolName,
            tokensIn: 0,
            tokensOut: 0,
            extra: '',
            createdAt: new Date().toISOString(),
          }

          const newMessages = [...sessionMessages]
          newMessages.splice(insertIndex, 0, toolResultMessage)

          return {
            messages: {
              ...state.messages,
              [data.sessionId]: newMessages,
            },
          }
        })
      }
    }))

    offs.push(Events.On(ChatEvents.AgentStreamDone, (event: { data: StreamDoneEvent | StreamDoneEvent[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue

        flushStreamingBuffer(data.sessionId)
        clearStreamingSeqs(data.sessionId)

        set((state) => {
          const nextStreaming = { ...state.streamingMessages }
          delete nextStreaming[data.sessionId]
          return {
            streamingMessages: nextStreaming,
            pendingTitleGenerations: {
              ...state.pendingTitleGenerations,
              [data.sessionId]: undefined,
            },
            sessionStatuses: {
              ...state.sessionStatuses,
              [data.sessionId]: 'idle',
            },
          }
        })

        void get().loadMessages(data.sessionId)
      }
    }))

    offs.push(Events.On(ChatEvents.AgentStreamError, (event: { data: StreamErrorEvent | StreamErrorEvent[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue

        flushStreamingBuffer(data.sessionId)
        clearStreamingSeqs(data.sessionId)

        set((state) => {
          const nextStreaming = { ...state.streamingMessages }
          delete nextStreaming[data.sessionId]
          return {
            streamingMessages: nextStreaming,
            sessionStatuses: {
              ...state.sessionStatuses,
              [data.sessionId]: 'error-unread',
            },
          }
        })

        void get().loadMessages(data.sessionId)
      }
    }))

    offs.push(Events.On(ChatEvents.AgentSessionSpawned, (event: { data: { sessionId: number; title: string; kind: string; tags?: string[]; userMessage?: string } | { sessionId: number; title: string; kind: string; tags?: string[]; userMessage?: string }[] }) => {
      for (const data of eventItems(event)) {
        if (!data) continue
        if (get().conversations.some((c) => c.id === data.sessionId)) continue

        const now = new Date().toISOString()
        const tempUserId = -Date.now()
        const tempAssistantId = tempUserId - 1
        const tags = data.tags ?? (data.kind === 'task' ? ['task'] : [])
        const newConversation: Conversation = {
          id: data.sessionId,
          title: data.title,
          kind: tags.includes('task') ? 'task' : 'user',
          tags,
          createdAt: now,
          updatedAt: now,
          starred: false,
          status: 'loading',
        }

        set((state) => ({
          conversations: [newConversation, ...state.conversations],
          messages: data.userMessage
            ? {
                ...state.messages,
                [data.sessionId]: [
                  {
                    id: tempUserId,
                    sessionId: data.sessionId,
                    parentId: null,
                    role: 'user',
                    contentType: 'text',
                    content: data.userMessage,
                    modelName: '',
                    agentName: '',
                    tokensIn: 0,
                    tokensOut: 0,
                    extra: '',
                    createdAt: now,
                  },
                  {
                    id: tempAssistantId,
                    sessionId: data.sessionId,
                    parentId: tempUserId,
                    role: 'assistant',
                    contentType: 'text',
                    content: '',
                    modelName: '',
                    agentName: '',
                    tokensIn: 0,
                    tokensOut: 0,
                    extra: '',
                    createdAt: now,
                  },
                ],
              }
            : state.messages,
          sessionStatuses: {
            ...state.sessionStatuses,
            [data.sessionId]: 'loading',
          },
        }))

        void get().loadSessions(true)
      }
    }))

    offs.push(Events.On(ChatEvents.AgentSessionStatus, (event) => {
      for (const raw of eventItems(event)) {
        if (!raw) continue
        const data: SessionStatusEvent = { sessionId: raw.sessionId, status: raw.status as ConversationStatus }

        // Unknown session (e.g. a task spawned from another window) — reload list
        const known = get().conversations.some((c) => c.id === data.sessionId)
        if (!known) {
          void get().loadSessions(true)
          continue
        }

        const shouldFinalizeCurrentConversation =
          data.status === 'done-unread' || data.status === 'error-unread'

        const pendingTitleGeneration = get().pendingTitleGenerations[data.sessionId]
        if (shouldFinalizeCurrentConversation && pendingTitleGeneration) {
          set((state) => ({
            pendingTitleGenerations: {
              ...state.pendingTitleGenerations,
              [data.sessionId]: undefined,
            },
          }))
          if (data.status === 'done-unread') {
            void get().generateTitle(data.sessionId, pendingTitleGeneration)
          }
        }

        if (data.sessionId === get().currentConversationId && shouldFinalizeCurrentConversation) {
          set((state) => ({
            sessionStatuses: {
              ...state.sessionStatuses,
              [data.sessionId]: 'idle',
            },
            conversations: state.conversations.map((conversation) =>
              conversation.id === data.sessionId ? { ...conversation, status: 'idle' } : conversation
            ),
          }))
          void AgentBinding.MarkSessionRead({ session_id: data.sessionId })
          continue
        }

        set((state) => ({
          sessionStatuses: {
            ...state.sessionStatuses,
            [data.sessionId]: data.status,
          },
          conversations: state.conversations.map((conversation) =>
            conversation.id === data.sessionId ? { ...conversation, status: data.status } : conversation
          ),
        }))

        ensureStreamingPlaceholder(data.sessionId)
      }
    }))

    return () => {
      offs.forEach((off) => off())
      ;(globalThis as { __firePokerChatListenersInitialized?: boolean }).__firePokerChatListenersInitialized = false
    }
  }
}
