import type { StoreApi } from 'zustand'
import type { Attachment, Conversation, ConversationStatus, Message } from '../../types'

export type StreamingMessageState = { content: string; thinking: string }
export type PendingStreamingPatch = {
  contentDelta: string
  thinkingDelta: string
  contentSnapshot?: string
  thinkingSnapshot?: string
}
// Only the provider identity is kept in the store; the backend resolves its credentials.
export type ModelRunConfig = {
  providerId: number
  modelName: string
}
type PendingTitleGeneration = ModelRunConfig
type TitleGenerationConfig = ModelRunConfig

export interface ChatStore {
  conversations: Conversation[]
  messages: Record<number, Message[]>
  sessionStatuses: Record<number, ConversationStatus | undefined>
  currentConversationId: number | null
  streamingMessages: Record<number, StreamingMessageState>
  pendingTitleGenerations: Record<number, PendingTitleGeneration | undefined>
  sessionRunConfigs: Record<number, PendingTitleGeneration | undefined>
  sessionToolIds: Record<number, string[] | undefined>
  sessionToolLocks: Record<number, boolean | undefined>
  loadSessionTools: (sessionId: number) => Promise<string[]>
  saveSessionTools: (sessionId: number, ids: string[]) => Promise<void>
  pendingConfirms: Record<number, {
    requestId: string
    toolName: string
    args: string
    purpose: string
  } | null>
  sessionsLoading: boolean
  hasMoreSessions: boolean
  sessionsCursor: number
  loadSessions: (reset?: boolean) => Promise<void>
  loadMessages: (sessionId: number) => Promise<void>
  createConversation: (params?: { title?: string; tags?: string[] }) => Promise<number>
  forkConversation: (sessionId: number, messageId: number) => Promise<void>
  deleteConversation: (id: number) => Promise<void>
  renameConversation: (id: number, title: string) => Promise<void>
  toggleStar: (id: number, starred: boolean) => Promise<void>
  markAsRead: (id: number) => Promise<void>
  setCurrentConversation: (id: number | null) => void
  setConversationStatus: (id: number, status: ConversationStatus) => void
  regenerateMessage: (sessionId: number, responseId: number) => Promise<void>
  sendMessage: (params: {
    regenerateMessageId?: number
    sessionId: number
    content: string
    providerId: number
    modelName: string
    enabledUserTools: string[]
    attachments: Attachment[]
    systemPrompt?: string
    skillName?: string
  }) => Promise<void>
  stopGeneration: (sessionId: number) => Promise<void>
  respondToConfirm: (
    sessionId: number,
    approved: boolean,
    message: string,
    action?: 'approve' | 'reject' | 'comment'
  ) => Promise<void>
  generateTitle: (sessionId: number, config: ModelRunConfig) => Promise<void>
  generateTitleDraft: (sessionId: number, fallbackConfig?: TitleGenerationConfig) => Promise<string | null>
  initEventListeners: () => () => void
}

export type ChatSet = StoreApi<ChatStore>['setState']
export type ChatGet = StoreApi<ChatStore>['getState']

type TurnKeys = 'regenerateMessage' | 'sendMessage' | 'stopGeneration' | 'respondToConfirm' | 'generateTitle' | 'generateTitleDraft'
export type TurnSlice = Pick<ChatStore, TurnKeys>
export type SessionSlice = Omit<ChatStore, TurnKeys | 'initEventListeners'>
