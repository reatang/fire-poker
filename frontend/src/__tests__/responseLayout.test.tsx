import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { ChatMessages } from '@/components/chat/ChatMessages'
import { MessageActions } from '@/components/chat/MessageActions'
import { useChatStore } from '@/store/chatStore'
import i18n from '@/i18n'
import type { Message } from '@/types'
import { Clipboard } from '@wailsio/runtime'

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...await importOriginal<typeof import('@wailsio/runtime')>(),
  Clipboard: { SetText: vi.fn() },
}))

vi.mock('@/components/chat/TerminalSessionPanels', () => ({ TerminalSessionPanels: () => null }))
vi.mock('@/hooks/useAutoScroll', () => ({ useAutoScroll: () => ({containerRef:{current:null},isAtBottom:true,scrollToBottom:vi.fn()}) }))

const msg = (id: number, role: Message['role'], contentType: Message['contentType'], content: string): Message => ({id,sessionId:1,parentId:null,role,contentType,content,modelName:'test',agentName:'',tokensIn:0,tokensOut:0,extra:'',createdAt:new Date().toISOString()})
beforeEach(async () => {
  await i18n.changeLanguage('zh-CN')
  vi.stubGlobal('requestAnimationFrame', () => 1)
  vi.stubGlobal('cancelAnimationFrame', () => {})
  useChatStore.setState({currentConversationId:1,conversations:[],messages:{},sessionStatuses:{},streamingMessages:{},pendingConfirms:{},initEventListeners:()=>()=>{}})
})

it('keeps one identity above thinking and text when streaming becomes persisted history', async () => {
  const user = userEvent.setup()
  useChatStore.setState({messages:{1:[msg(1,'user','text','Question'),msg(-2,'assistant','text','')]},sessionStatuses:{1:'loading'},streamingMessages:{1:{content:'Answer',thinking:'Reasoning'}}})
  const {container}=render(<ChatMessages />)
  const assertOrder = () => {
    const identity = container.querySelector('.desktop-assistant-label')!
    const reasoning = screen.getByText('Reasoning')
    expect(container.querySelectorAll('.desktop-assistant-label')).toHaveLength(1)
    expect(identity.compareDocumentPosition(reasoning) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(reasoning.compareDocumentPosition(screen.getByText('Answer')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  }
  await user.click(screen.getByRole('button',{name:String(i18n.t('chat.thinking'))}))
  assertOrder()
  act(() => useChatStore.setState({messages:{1:[msg(1,'user','text','Question'),msg(2,'assistant','thinking','Reasoning'),msg(3,'assistant','text','Answer')]},sessionStatuses:{1:'idle'},streamingMessages:{}}))
  const turn = container.querySelector('.assistant-turn')!
  expect(turn.querySelector('.desktop-assistant-label')).toBeTruthy()
  // The completed thinking toggle and answer share the same turn wrapper.
  expect(turn.textContent).toContain('思考')
  expect(turn.textContent).toContain('Answer')
  await user.click(screen.getByRole('button',{name:String(i18n.t('chat.thought'))}))
  assertOrder()
  const identity = turn.querySelector('.desktop-assistant-label')!
  expect(identity.compareDocumentPosition(screen.getByText('Answer')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(screen.getByRole('button',{name:'复制回复'})).toBeEnabled()
  expect(screen.getByRole('button',{name:'重新生成'})).toBeEnabled()
})

it('copies only answer content and invokes regeneration once', async () => {
  const user=userEvent.setup()
  const copy=vi.mocked(Clipboard.SetText).mockResolvedValue()
  const regenerate=vi.fn().mockResolvedValue(undefined)
  render(<MessageActions content={'## Answer\n\nText'} onRegenerate={regenerate}/>)
  await user.click(screen.getByRole('button',{name:'复制回复'}))
  expect(copy).toHaveBeenCalledWith('## Answer\n\nText')
  expect(screen.getByText('已复制')).toBeInTheDocument()
  await user.click(screen.getByRole('button',{name:'重新生成'}))
  expect(regenerate).toHaveBeenCalledTimes(1)
})

it('shows copy failures and disables regeneration while another run is active', async () => {
  const user=userEvent.setup()
  vi.mocked(Clipboard.SetText).mockRejectedValue(new Error('denied'))
  render(<MessageActions content="answer" disabled onRegenerate={vi.fn()}/>)
  expect(screen.getByRole('button',{name:'重新生成'})).toBeDisabled()
  await user.click(screen.getByRole('button',{name:'复制回复'}))
  expect(screen.getByRole('alert')).toHaveTextContent('复制失败')
})

it('replaces unavailable regeneration with a fork action on earlier replies', async () => {
  const user=userEvent.setup()
  const fork=vi.fn().mockResolvedValue(undefined)
  render(<MessageActions content="Earlier answer" onFork={fork}/>)
  expect(screen.queryByRole('button',{name:'重新生成'})).not.toBeInTheDocument()
  expect(screen.getByRole('button',{name:'复制回复'})).toBeEnabled()
  await user.click(screen.getByRole('button',{name:'从此处分叉'}))
  expect(fork).toHaveBeenCalledTimes(1)
})

it('offers a fork below user messages as well', async () => {
  const { MessageItem }=await import('@/components/chat/MessageItem')
  const user=userEvent.setup();const fork=vi.fn().mockResolvedValue(undefined)
  render(<MessageItem message={msg(1,'user','text','Earlier question')} onFork={fork}/>)
  await user.click(screen.getByRole('button',{name:'从此处分叉'}))
  expect(fork).toHaveBeenCalledTimes(1)
  expect(screen.queryByRole('button',{name:'重新生成'})).not.toBeInTheDocument()
})
