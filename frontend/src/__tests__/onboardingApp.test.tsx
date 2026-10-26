import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '@/App'

vi.mock('@/hooks/useNotificationsSubscription', () => ({ useNotificationsSubscription: vi.fn() }))

const supportedProviderListMock = vi.fn()
const exitAppMock = vi.fn()
const saveProviderAndMarkInitializedMock = vi.fn()

vi.mock('@wailsio/runtime', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wailsio/runtime')>()
  return {
    ...actual,
    Events: {
      ...actual.Events,
      On: vi.fn(() => vi.fn()),
    },
  }
})

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/config', () => ({
  Config: {
    SupportedProviderList: (...args: unknown[]) => supportedProviderListMock(...args),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/onboarding/index', () => ({
  Onboarding: {
    ExitApp: (...args: unknown[]) => exitAppMock(...args),
    SaveProviderAndMarkInitialized: (...args: unknown[]) => saveProviderAndMarkInitializedMock(...args),
  },
}))

describe('Onboarding app', () => {
  beforeEach(() => {
    supportedProviderListMock.mockReset()
    exitAppMock.mockReset()
    saveProviderAndMarkInitializedMock.mockReset()
    supportedProviderListMock.mockResolvedValue({
      supported_providers: [
        {
          type: 'deepseek',
          icon: '',
          name: 'DeepSeek',
          description: 'Provider description',
          base_url: 'https://api.deepseek.com',
        },
      ],
    })
    window.history.replaceState({}, '', '/?entry=onboarding')
  })

  it('routes to onboarding and advances from intro to provider selection', async () => {
    const user = userEvent.setup()

    render(<App />)

    // The onboarding entry is code-split, so its first paint is asynchronous.
    expect(await screen.findByText('先连接第一个模型供应商，再开始你的第一段对话')).toBeInTheDocument()
    expect(screen.getByText('欢迎')).toBeInTheDocument()
    expect(screen.getByText('选择供应商')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '开始配置' }))

    await waitFor(() => {
      expect(screen.getByText('选择一个你计划接入的模型供应商')).toBeInTheDocument()
    })
    await user.click(screen.getByRole('button', { name: 'DeepSeek' }))
    expect(screen.getByRole('button', { name: '下一步' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: '下一步' }))

    expect(screen.getByText('配置 DeepSeek')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '返回' }))
    await user.click(screen.getByRole('button', { name: '返回' }))
    await user.click(screen.getByRole('button', { name: '退出' }))

    expect(exitAppMock).toHaveBeenCalledWith({})
  })
})
