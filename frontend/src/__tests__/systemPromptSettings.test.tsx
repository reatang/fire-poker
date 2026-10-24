import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SystemPromptSettings } from '@/components/chat/SystemPromptSettings'
import { Agent } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'

vi.mock('@/store/chatStore', () => ({ useChatStore: (selector: (state: unknown) => unknown) => selector({ sessionStatuses: {} }) }))
vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/agent', () => ({ Agent: {
  GetSessionPrompt: vi.fn(), ListSystemPrompts: vi.fn(), GetSystemPrompt: vi.fn(), SaveSystemPrompt: vi.fn(), UpdateSessionPrompt: vi.fn(),
} }))

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(Agent.GetSessionPrompt).mockResolvedValue({ system: '默认规则', name: '默认系统提示词', default_system: '默认规则', is_default: true, locked: false })
  vi.mocked(Agent.ListSystemPrompts).mockResolvedValue({ names: ['写作助手'] })
  vi.mocked(Agent.GetSystemPrompt).mockResolvedValue({ name: '写作助手', system: '你是写作教练' })
  vi.mocked(Agent.UpdateSessionPrompt).mockResolvedValue({ system: '你是写作教练', name: '写作助手', default_system: '默认规则', is_default: false, locked: false })
})

describe('session prompt settings', () => {
  it('shows the active name and only permits viewing once chat has started', async () => {
    vi.mocked(Agent.GetSessionPrompt).mockResolvedValue({ system: '固定规则', name: '架构顾问', default_system: '默认规则', is_default: false, locked: true })
    const user = userEvent.setup()
    render(<SystemPromptSettings sessionId={9} hasMessages />)
    expect(await screen.findByText('架构顾问')).toBeInTheDocument()
    const button = screen.getByRole('button', { name: '系统提示词' })
    expect(button).toHaveTextContent('系统提示词')
    await user.click(button)
    expect(screen.getByRole('dialog').parentElement?.parentElement).toBe(document.body)
    await screen.findByDisplayValue('固定规则')
    expect(screen.getByRole('textbox', { name: '提示词内容' })).toHaveAttribute('readonly')
    expect(screen.queryByRole('button', { name: '应用到会话' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '恢复默认' })).not.toBeInTheDocument()
    expect(Agent.UpdateSessionPrompt).not.toHaveBeenCalled()
  })
  it('loads library text into the editor, and applies only after explicit action', async () => {
    const user = userEvent.setup()
    render(<SystemPromptSettings sessionId={7} />)
    await user.click(screen.getByRole('button', { name: '系统提示词' }))
    await screen.findByDisplayValue('默认规则')
    await user.selectOptions(screen.getByRole('combobox'), '写作助手')
    await screen.findByDisplayValue('你是写作教练')
    expect(Agent.UpdateSessionPrompt).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: '应用到会话' }))
    await waitFor(() => expect(Agent.UpdateSessionPrompt).toHaveBeenCalledWith({ session_id: 7, system: '你是写作教练', name: '写作助手', reset: false }))
    expect(await screen.findByRole('status')).toHaveTextContent('已应用到当前会话')
  })

  it('keeps a failed apply visible and displays the failure', async () => {
    vi.mocked(Agent.UpdateSessionPrompt).mockRejectedValue(new Error('请先停止生成'))
    const user = userEvent.setup()
    render(<SystemPromptSettings sessionId={8} />)
    await user.click(screen.getByRole('button', { name: '系统提示词' }))
    await screen.findByDisplayValue('默认规则')
    await user.click(screen.getByRole('button', { name: '应用到会话' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('请先停止生成')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })
})
