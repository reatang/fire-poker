import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SettingsApp } from '@/components/settings/SettingsApp'
import i18n from '@/i18n'
import { getSettingsInitialState, useSettingsStore } from '@/store/settingsStore'

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/agent', () => ({
  Agent: { ListSystemPrompts: vi.fn().mockResolvedValue({ names: ['Writing assistant'] }) },
}))

vi.mock('@/hooks/useSettingsBootstrap', () => ({
  useSettingsBootstrap: vi.fn(),
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/window', () => ({
  Window: {
    OpenAddProvider: vi.fn(),
    OpenAddSkill: vi.fn(),
    OpenSettings: vi.fn(),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/plugin', () => ({
  Plugin: {
    ListExtensions: vi.fn().mockResolvedValue({ extensions: [] }),
    GetExtensionDetail: vi.fn(),
    ToggleExtension: vi.fn(),
    ReloadExtension: vi.fn(),
    DeleteExtension: vi.fn(),
    SaveExtensionConfig: vi.fn(),
    ImportExtension: vi.fn(),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/file', () => ({
  File: {
    SelectFolder: vi.fn(),
  },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/provider', () => ({
  Provider: {
    ListProviders: vi.fn().mockResolvedValue({ providers: [] }),
    EditProvider: vi.fn(),
    SetDefault: vi.fn(),
    DeleteProvider: vi.fn(),
  },
}))

describe('SettingsApp', () => {
  beforeEach(async () => {
    window.history.replaceState({}, '', '/?entry=settings')
    useSettingsStore.setState(getSettingsInitialState())
    await i18n.changeLanguage('en')
  })

  it('opens the About panel when the settings URL contains tab=about', () => {
    window.history.replaceState({}, '', '/?entry=settings&tab=about')

    render(<SettingsApp />)

    expect(
      screen.getByText('An agent client in classic mode')
    ).toBeInTheDocument()
  })

  it('opens prompt management from its settings URL', async () => {
    window.history.replaceState({}, '', '/?entry=settings&tab=prompts')

    render(<SettingsApp />)

    expect(screen.getByRole('heading', { name: 'System prompts' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Writing assistant' })).toBeInTheDocument()
  })
})
