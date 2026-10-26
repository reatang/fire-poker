import i18n from '../../i18n'
import { preferredModel } from '@/lib/modelSelection'
import { useModelSelectionStore } from '../modelSelectionStore'
import { Agent as AgentBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { EMPTY_STREAMING_MESSAGE, isStreamingStatus, parseIError } from './helpers'
import type { StreamingBuffer } from './streamingBuffer'
import type { ChatGet, ChatSet, TurnSlice } from './types'

// createTurnSlice runs one conversation turn: send, regenerate, stop, tool approval and titles.
export function createTurnSlice(set: ChatSet, get: ChatGet, buffer: StreamingBuffer): TurnSlice {
  const { clearStreamingBuffer, clearStreamingSeqs, flushStreamingBuffer } = buffer
  return {
    regenerateMessage: async (sessionId, responseId) => {
      const state = get()
      if (isStreamingStatus(state.sessionStatuses[sessionId] ?? state.conversations.find(c => c.id === sessionId)?.status)) throw new Error(i18n.t('chatErrors.waitForReply'))
      const messages = state.messages[sessionId] ?? []
      const question = [...messages].reverse().find(m => m.role === 'user' && m.contentType === 'text')
      const response = messages.find(m => m.id === responseId && m.role === 'assistant')
      if (!question || !response || messages.indexOf(response) < messages.indexOf(question)) throw new Error(i18n.t('chatErrors.latestOnly'))
      const { Provider } = await import('@bindings/github.com/reatang/fire-poker/backend/service/provider')
      const result = await Provider.ProviderAndModelList({})
      const current = get()
      if (isStreamingStatus(current.sessionStatuses[sessionId] ?? current.conversations.find(c => c.id === sessionId)?.status)
        || current.messages[sessionId] !== messages) throw new Error(i18n.t('chatErrors.conversationChanged'))
      const providers = (result?.provider_models ?? []).filter(p => p.provider.enabled)
      const previous = state.sessionRunConfigs[sessionId]
      const original = providers.flatMap(p => p.models.filter(m => m.enable && m.model === response.modelName).map(m => ({p, m})))
      const matching = original.find(x => x.p.provider.id === previous?.providerId) ?? original[0]
      const selected = matching?.m ?? preferredModel(providers, useModelSelectionStore.getState())
      const provider = matching?.p ?? providers.find(p => p.models.some(m => m.id === selected?.id))
      if (!provider || !selected) throw new Error(i18n.t('chatErrors.noModel'))
      await get().sendMessage({sessionId, regenerateMessageId: responseId, content: question.content,
        attachments: question.attachments ?? [], providerId: provider.provider.id, modelName: selected.model,
        enabledUserTools: await get().loadSessionTools(sessionId)})
    },

    sendMessage: async (params) => {
      const originalMessages = get().messages[params.sessionId] ?? []
      const questionIndex = originalMessages.map(m => m.role === 'user' && m.contentType === 'text').lastIndexOf(true)
      const retainedMessages = params.regenerateMessageId ? originalMessages.slice(0, questionIndex + 1) : originalMessages
      const now = new Date().toISOString()
      const tempUserId = -Date.now()
      const tempAssistantId = tempUserId - 1
      const isFirstMessage = (get().messages[params.sessionId]?.length ?? 0) === 0
      clearStreamingBuffer(params.sessionId)
      clearStreamingSeqs(params.sessionId)

      set((state) => ({
        conversations: state.conversations.map((conversation) =>
          conversation.id === params.sessionId
            ? {
                ...conversation,
                status: 'loading',
                updatedAt: now,
              }
            : conversation
        ),
        messages: {
          ...state.messages,
          [params.sessionId]: [
            ...retainedMessages,
            ...(params.regenerateMessageId ? [] : [{
              id: tempUserId,
              sessionId: params.sessionId,
              parentId: null,
              role: 'user' as const,
              contentType: 'text' as const,
              content: params.content,
              modelName: '',
              agentName: '',
              tokensIn: 0,
              tokensOut: 0,
              extra: '',
              attachments: params.attachments.length > 0 ? params.attachments : undefined,
              createdAt: now,
            }]),
            {
              id: tempAssistantId,
              sessionId: params.sessionId,
              parentId: tempUserId,
              role: 'assistant',
              contentType: 'text',
              content: '',
              modelName: params.modelName,
              agentName: '',
              tokensIn: 0,
              tokensOut: 0,
              extra: '',
              createdAt: now,
            },
          ],
        },
        pendingConfirms: {
          ...state.pendingConfirms,
          [params.sessionId]: null,
        },
        sessionStatuses: {
          ...state.sessionStatuses,
          [params.sessionId]: 'loading',
        },
        sessionRunConfigs: {
          ...state.sessionRunConfigs,
          [params.sessionId]: {
            providerId: params.providerId,
            modelName: params.modelName,
          },
        },
        streamingMessages: {
          ...state.streamingMessages,
          [params.sessionId]: EMPTY_STREAMING_MESSAGE,
        },
        pendingTitleGenerations: {
          ...state.pendingTitleGenerations,
          [params.sessionId]: isFirstMessage
            ? {
                providerId: params.providerId,
                modelName: params.modelName,
              }
            : state.pendingTitleGenerations[params.sessionId],
        },
      }))

      try {
        await AgentBinding.SendMessage({
          session_id: params.sessionId,
          ...(params.regenerateMessageId ? { regenerate_message_id: params.regenerateMessageId } : {}),
          content: params.content,
          system_prompt: params.systemPrompt ?? '',
          skill_name: params.skillName ?? '',
          provider_id: params.providerId,
          model_name: params.modelName,
          enabled_user_tools: params.enabledUserTools,
          attachments: params.attachments.map(att => ({
            path: att.path,
            name: att.name,
            mime: att.mime,
          })),
        })
      } catch (err: unknown) {
        const parsed = parseIError(err)
        set((state) => {
          const nextStreaming = { ...state.streamingMessages }
          delete nextStreaming[params.sessionId]
          return {
            messages: {
              ...state.messages,
              [params.sessionId]: params.regenerateMessageId ? originalMessages : (state.messages[params.sessionId] ?? []).map((m) =>
                m.id === tempAssistantId
                  ? { ...m, contentType: 'error' as const, content: JSON.stringify(parsed) }
                  : m
              ),
            },
            conversations: state.conversations.map((c) =>
              c.id === params.sessionId ? { ...c, status: 'idle' } : c
            ),
            streamingMessages: nextStreaming,
            sessionStatuses: {
              ...state.sessionStatuses,
              [params.sessionId]: 'idle',
            },
            pendingTitleGenerations: {
              ...state.pendingTitleGenerations,
              [params.sessionId]: undefined,
            },
          }
        })
        if (params.regenerateMessageId) throw err
      }
    },

    stopGeneration: async (sessionId) => {
      await AgentBinding.StopGeneration({ session_id: sessionId })
      flushStreamingBuffer(sessionId)
      clearStreamingSeqs(sessionId)
      set(state => {
        const next = { ...state.streamingMessages }
        delete next[sessionId]
        return { streamingMessages: next, sessionStatuses: { ...state.sessionStatuses, [sessionId]: 'idle' }, pendingConfirms: { ...state.pendingConfirms, [sessionId]: null } }
      })
      await get().loadMessages(sessionId)
    },

    respondToConfirm: async (sessionId, approved, message, action = approved ? 'approve' : 'reject') => {
      await AgentBinding.RespondToConfirm({
        session_id: sessionId,
        approved,
        message,
        action,
      })
      set((state) => ({
        pendingConfirms: { ...state.pendingConfirms, [sessionId]: null },
      }))
    },

    generateTitle: async (sessionId, config) => {
      const result = await AgentBinding.GenerateTitle({
        session_id: sessionId,
        provider_id: config.providerId,
        model_name: config.modelName,
      })
      if (!result?.title) return

      set((state) => ({
        conversations: state.conversations.map((conversation) =>
          conversation.id === sessionId ? { ...conversation, title: result.title } : conversation
        ),
      }))
    },

    generateTitleDraft: async (sessionId, fallbackConfig) => {
      const runConfig = get().sessionRunConfigs[sessionId] ?? fallbackConfig
      if (!runConfig) return null

      const result = await AgentBinding.GenerateTitle({
        session_id: sessionId,
        provider_id: runConfig.providerId,
        model_name: runConfig.modelName,
      })

      return result?.title?.trim() || null
    },
  }
}
