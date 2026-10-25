import { create } from 'zustand'
import { createInitEventListeners } from './chat/chatEvents'
import { createSessionSlice } from './chat/sessionSlice'
import { createStreamingBuffer } from './chat/streamingBuffer'
import { createTurnSlice } from './chat/turnSlice'
import type { ChatStore } from './chat/types'

export type { ModelRunConfig } from './chat/types'

// The chat store is composed from slices that share one streaming buffer:
// sessions (list, tools, messages), turns (send/regenerate/stop/confirm/titles) and backend events.
export const useChatStore = create<ChatStore>()((set, get) => {
  const buffer = createStreamingBuffer(set)
  return {
    ...createSessionSlice(set, get, buffer),
    ...createTurnSlice(set, get, buffer),
    initEventListeners: createInitEventListeners(set, get, buffer),
  }
})
