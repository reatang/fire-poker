import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { formatTimestamp, groupConversationsByDate, shouldShowTimestamp } from '../lib/utils'
import type { Conversation, Message } from '../types'

const makeMsg = (daysAgo: number, id: number): Message => ({
  id,
  sessionId: 1,
  parentId: null,
  role: 'user',
  contentType: 'text',
  content: 'test',
  modelName: '',
  agentName: '',
  tokensIn: 0,
  tokensOut: 0,
  extra: '',
  createdAt: new Date(Date.now() - daysAgo * 86400000).toISOString(),
})

describe('shouldShowTimestamp', () => {
  it('always shows timestamp for first message', () => {
    const msgs = [makeMsg(0, 1)]
    expect(shouldShowTimestamp(msgs, 0)).toBe(true)
  })

  it('does not show timestamp for same-day messages', () => {
    const msgs = [makeMsg(0, 1), makeMsg(0, 2)]
    expect(shouldShowTimestamp(msgs, 1)).toBe(false)
  })

  it('shows timestamp when messages cross day boundary', () => {
    const msgs = [makeMsg(1, 1), makeMsg(0, 2)]
    expect(shouldShowTimestamp(msgs, 1)).toBe(true)
  })
})

describe('localized date labels', () => {
  const now = new Date(2026, 9, 25, 18, 0)
  beforeEach(() => { vi.useFakeTimers(); vi.setSystemTime(now) })
  afterEach(() => { vi.useRealTimers() })

  it('formats relative and absolute timestamps per language', () => {
    const at = (month: number, day: number, year = 2026) => new Date(year, month, day, 14, 5)
    expect(formatTimestamp(at(9, 25), 'zh-CN')).toBe('今天 14:05')
    expect(formatTimestamp(at(9, 24), 'zh-CN')).toBe('昨天 14:05')
    expect(formatTimestamp(at(2, 3), 'zh-CN')).toBe('3月3日 14:05')
    expect(formatTimestamp(at(2, 3, 2025), 'zh-CN')).toBe('2025年3月3日 14:05')
    expect(formatTimestamp(at(9, 25), 'en')).toBe('Today 02:05 PM')
    expect(formatTimestamp(at(9, 24), 'en')).toBe('Yesterday 02:05 PM')
    expect(formatTimestamp(at(2, 3), 'en')).toBe('Mar 3 02:05 PM')
    expect(formatTimestamp(at(2, 3, 2025), 'en')).toBe('Mar 3, 2025 02:05 PM')
  })

  it('labels conversation groups per language', () => {
    const conversation = (id: number, updated: Date): Conversation => ({
      id, title: `c${id}`, kind: 'user', createdAt: updated.toISOString(), updatedAt: updated.toISOString(), starred: false, status: 'idle',
    })
    const items = [conversation(1, now), conversation(2, new Date(2026, 9, 24, 9)), conversation(3, new Date(2026, 9, 21, 9)), conversation(4, new Date(2026, 0, 1, 9))]
    expect(groupConversationsByDate(items, 'zh-CN').map(g => g.label)).toEqual(['今天', '昨天', '过去7天', '更久以前'])
    expect(groupConversationsByDate(items, 'en').map(g => g.label)).toEqual(['Today', 'Yesterday', 'Last 7 days', 'Earlier'])
  })
})
