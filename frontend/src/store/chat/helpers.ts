import type { ConversationStatus, Message, StreamChunkEvent } from '../../types'
import type { PendingStreamingPatch, StreamingMessageState } from './types'

// Generated event payloads type enum-like fields as plain strings; narrow them at the boundary.
export function asStreamChunkEvent(raw: Omit<StreamChunkEvent, 'contentType'> & { contentType: string }): StreamChunkEvent {
  return { ...raw, contentType: raw.contentType === 'thinking' ? 'thinking' : 'text' }
}

export function eventItems<T>(event: { data: T | T[] }): T[] {
  return Array.isArray(event.data) ? event.data : [event.data]
}

export const EMPTY_STREAMING_MESSAGE: StreamingMessageState = { content: '', thinking: '' }
export const EMPTY_STREAMING_PATCH: PendingStreamingPatch = { contentDelta: '', thinkingDelta: '' }

export function scheduleNextFrame(callback: FrameRequestCallback): number {
  if (typeof globalThis.requestAnimationFrame === 'function') {
    return globalThis.requestAnimationFrame(callback)
  }

  return globalThis.setTimeout(() => callback(Date.now()), 16) as unknown as number
}

export function cancelScheduledFrame(handle: number) {
  if (typeof globalThis.cancelAnimationFrame === 'function') {
    globalThis.cancelAnimationFrame(handle)
    return
  }

  globalThis.clearTimeout(handle)
}

export function parseIError(err: unknown): { msg: string; detail: string } {
  try {
    const message = err instanceof Error ? err.message : String(err)
    const parsed = JSON.parse(message) as { msg?: string; detail?: string }
    return {
      msg: parsed.msg ?? message,
      detail: parsed.detail ?? message,
    }
  } catch {
    return { msg: String(err), detail: String(err) }
  }
}

export function isStreamingStatus(status: ConversationStatus | undefined): boolean {
  return status === 'loading' || status === 'waiting-unread'
}

export function findLatestStreamToolResultIndex(messages: Message[], toolName: string): number {
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i]
    if (message.contentType === 'tool_result' && message.agentName === toolName) {
      return i
    }
    if (message.contentType === 'tool_call') {
      break
    }
  }
  return -1
}
