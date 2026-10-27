import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Agent } from '@bindings/github.com/reatang/fire-poker/backend/service/agent'
import { SystemPromptsSection } from '@/components/settings/prompts/SystemPromptsSection'
import i18n from '@/i18n'

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/agent', () => ({ Agent: {
  ListSystemPrompts: vi.fn(), GetSystemPrompt: vi.fn(), SaveSystemPrompt: vi.fn(), DeleteSystemPrompt: vi.fn(),
} }))

beforeEach(async () => {
  vi.resetAllMocks()
  await i18n.changeLanguage('en')
  vi.mocked(Agent.ListSystemPrompts).mockResolvedValue({ names: ['Assistant', 'Writer'] })
  vi.mocked(Agent.GetSystemPrompt).mockImplementation(({ name }) => Promise.resolve({ name, system: `${name} instructions` }) as ReturnType<typeof Agent.GetSystemPrompt>)
  vi.mocked(Agent.SaveSystemPrompt).mockImplementation(input => Promise.resolve(input) as ReturnType<typeof Agent.SaveSystemPrompt>)
  vi.mocked(Agent.DeleteSystemPrompt).mockResolvedValue(undefined)
})

describe('system prompt management', () => {
  it('searches the shared library and loads a prompt without saving it', async () => {
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await screen.findByRole('button', { name: 'Assistant' })
    await user.type(screen.getByRole('textbox', { name: 'Search prompts' }), 'wri')
    expect(screen.queryByRole('button', { name: 'Assistant' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Writer' }))
    expect(await screen.findByDisplayValue('Writer instructions')).toBeInTheDocument()
    expect(Agent.SaveSystemPrompt).not.toHaveBeenCalled()
  })

  it('creates and edits prompts in the same library used by chat', async () => {
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await screen.findByRole('button', { name: 'Assistant' })
    await user.click(screen.getAllByRole('button', { name: 'New prompt' })[0])
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    await user.type(screen.getByRole('textbox', { name: 'Prompt name' }), ' Translator ')
    await user.type(screen.getByRole('textbox', { name: 'Prompt content' }), 'Translate text')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(Agent.SaveSystemPrompt).toHaveBeenCalledWith({ name: 'Translator', system: 'Translate text' })
    expect(await screen.findByRole('button', { name: 'Translator' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    await user.type(screen.getByRole('textbox', { name: 'Prompt content' }), ' accurately')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(Agent.SaveSystemPrompt).toHaveBeenLastCalledWith({ name: 'Translator', system: 'Translate text accurately' }))
  })

  it('prevents accidental overwrites when creating a duplicate name', async () => {
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await screen.findByRole('button', { name: 'Assistant' })
    await user.click(screen.getAllByRole('button', { name: 'New prompt' })[0])
    await user.type(screen.getByRole('textbox', { name: 'Prompt name' }), 'assistant')
    await user.type(screen.getByRole('textbox', { name: 'Prompt content' }), 'Different content')
    expect(screen.getByRole('alert')).toHaveTextContent('This name already exists')
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    expect(Agent.SaveSystemPrompt).not.toHaveBeenCalled()
  })

  it('asks before discarding edits and can revert changes', async () => {
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await user.click(await screen.findByRole('button', { name: 'Assistant' }))
    await user.type(await screen.findByRole('textbox', { name: 'Prompt content' }), ' changed')
    await user.click(screen.getByRole('button', { name: 'Writer' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('Discard unsaved changes?')
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    expect(screen.getByDisplayValue('Assistant instructions changed')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Revert changes' }))
    expect(screen.getByDisplayValue('Assistant instructions')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Writer' }))
    expect(await screen.findByDisplayValue('Writer instructions')).toBeInTheDocument()
  })

  it('deletes only after confirmation', async () => {
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await user.click(await screen.findByRole('button', { name: 'Assistant' }))
    await screen.findByDisplayValue('Assistant instructions')
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(Agent.DeleteSystemPrompt).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('Existing conversations will not be affected')
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(Agent.DeleteSystemPrompt).toHaveBeenCalledWith({ name: 'Assistant', system: '' }))
    expect(screen.queryByRole('button', { name: 'Assistant' })).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Prompt deleted')
  })

  it('keeps failed saves and deletes visible for retry', async () => {
    vi.mocked(Agent.SaveSystemPrompt).mockRejectedValue(new Error('Cannot save'))
    vi.mocked(Agent.DeleteSystemPrompt).mockRejectedValue(new Error('Cannot delete'))
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    await user.click(await screen.findByRole('button', { name: 'Assistant' }))
    await user.type(await screen.findByRole('textbox', { name: 'Prompt content' }), ' changed')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot save')
    expect(screen.getByDisplayValue('Assistant instructions changed')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot delete')
    expect(screen.getByRole('button', { name: 'Assistant' })).toBeInTheDocument()
  })

  it('shows a load failure and supports retry', async () => {
    vi.mocked(Agent.ListSystemPrompts).mockRejectedValueOnce(new Error('Cannot load'))
    const user = userEvent.setup()
    render(<SystemPromptsSection />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot load')
    for (const button of screen.getAllByRole('button', { name: 'New prompt' })) expect(button).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Refresh prompt library' }))
    expect(await screen.findByRole('button', { name: 'Assistant' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
