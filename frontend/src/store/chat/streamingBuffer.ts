import type { StreamChunkEvent } from '../../types'
import {
  cancelScheduledFrame,
  EMPTY_STREAMING_MESSAGE,
  EMPTY_STREAMING_PATCH,
  isStreamingStatus,
  scheduleNextFrame,
} from './helpers'
import type { ChatSet, PendingStreamingPatch } from './types'

// StreamingBuffer coalesces streamed deltas per session and flushes them once per animation
// frame, so a fast stream causes one store update per frame instead of one per chunk.
export type StreamingBuffer = {
  clearStreamingBuffer: (sessionId: number) => void
  clearStreamingSeqs: (sessionId: number) => void
  flushStreamingBuffer: (sessionId: number) => void
  appendStreamingDelta: (data: StreamChunkEvent) => void
  ensureStreamingPlaceholder: (sessionId: number) => void
}

export function createStreamingBuffer(set: ChatSet): StreamingBuffer {
  const pendingStreamingBuffers = new Map<number, PendingStreamingPatch>()
  const scheduledStreamingFlushes = new Map<number, number>()
  const latestStreamingChunkSeq = new Map<string, number>()

  const clearStreamingBuffer = (sessionId: number) => {
    const handle = scheduledStreamingFlushes.get(sessionId)
    if (handle !== undefined) {
      cancelScheduledFrame(handle)
      scheduledStreamingFlushes.delete(sessionId)
    }
    pendingStreamingBuffers.delete(sessionId)
  }

  const clearStreamingSeqs = (sessionId: number) => {
    latestStreamingChunkSeq.delete(`${sessionId}:content`)
    latestStreamingChunkSeq.delete(`${sessionId}:thinking`)
  }

  const flushStreamingBuffer = (sessionId: number) => {
    const pending = pendingStreamingBuffers.get(sessionId)
    clearStreamingBuffer(sessionId)

    if (
      !pending ||
      (!pending.contentDelta &&
        !pending.thinkingDelta &&
        pending.contentSnapshot === undefined &&
        pending.thinkingSnapshot === undefined)
    ) return

    set((state) => {
      const previous = state.streamingMessages[sessionId] ?? EMPTY_STREAMING_MESSAGE

      return {
        streamingMessages: {
          ...state.streamingMessages,
          [sessionId]: {
            content: pending.contentSnapshot ?? previous.content + pending.contentDelta,
            thinking: pending.thinkingSnapshot ?? previous.thinking + pending.thinkingDelta,
          },
        },
      }
    })
  }

  const scheduleStreamingFlush = (sessionId: number) => {
    if (scheduledStreamingFlushes.has(sessionId)) return

    const handle = scheduleNextFrame(() => {
      flushStreamingBuffer(sessionId)
    })

    scheduledStreamingFlushes.set(sessionId, handle)
  }

  const appendStreamingDelta = (data: StreamChunkEvent) => {
    const key = data.contentType === 'thinking' ? 'thinking' : 'content'
    const seqKey = `${data.sessionId}:${key}`
    if (typeof data.seq === 'number') {
      const previousSeq = latestStreamingChunkSeq.get(seqKey) ?? 0
      if (data.seq <= previousSeq) return
      latestStreamingChunkSeq.set(seqKey, data.seq)
    }

    const previous = pendingStreamingBuffers.get(data.sessionId) ?? EMPTY_STREAMING_PATCH
    if (typeof data.content === 'string') {
      pendingStreamingBuffers.set(data.sessionId, {
        ...previous,
        [`${key}Snapshot`]: data.content,
        [`${key}Delta`]: '',
      })
    } else {
      const deltaKey = `${key}Delta` as 'contentDelta' | 'thinkingDelta'
      pendingStreamingBuffers.set(data.sessionId, {
        ...previous,
        [deltaKey]: previous[deltaKey] + data.delta,
      })
    }

    scheduleStreamingFlush(data.sessionId)
  }

  const ensureStreamingPlaceholder = (sessionId: number) => {
    set((state) => {
      const conversation = state.conversations.find((item) => item.id === sessionId)
      const status = state.sessionStatuses[sessionId] ?? conversation?.status
      if (!isStreamingStatus(status)) {
        return state
      }

      const sessionMessages = state.messages[sessionId]
      if (!sessionMessages) {
        return state
      }

      const lastMessage = sessionMessages[sessionMessages.length - 1]
      if (lastMessage?.role === 'assistant' && lastMessage?.contentType !== 'tool_call') {
        return state
      }

      return {
        messages: {
          ...state.messages,
          [sessionId]: [
            ...sessionMessages,
            {
              id: -(sessionId * 1000) - sessionMessages.length - 1,
              sessionId,
              parentId: lastMessage?.role === 'user' ? lastMessage.id : null,
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
      }
    })
  }

  return {
    clearStreamingBuffer,
    clearStreamingSeqs,
    flushStreamingBuffer,
    appendStreamingDelta,
    ensureStreamingPlaceholder,
  }
}
