import { useState, useEffect } from 'react'
import type { CSSProperties } from 'react'
import { useEditor, EditorContent as EditorContentBase } from '@tiptap/react'
import { TextSelection } from '@tiptap/pm/state'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { Markdown } from 'tiptap-markdown'
import { convertFencedCodeBlock, SingleLineCodeBlock, SlashCommand } from './input/editorExtensions'
import {
  Send, Square, Paperclip,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useChatStore } from '@/store/chatStore'
import { useAppStore } from '@/store/appStore'
import { Provider as ProviderBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/provider'
import { Plugin as PluginBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin'
import { Window } from '@bindings/github.com/reatang/fire-poker/backend/service/window'
import type { ProviderModel } from '@bindings/github.com/reatang/fire-poker/backend/models/view_model/models'
import { useShallow } from 'zustand/react/shallow'
import { AttachmentChips } from './AttachmentChips'
import { ATTACHMENT_MAX_COUNT } from '@/lib/attachments'
import { preferredModel, preferredProviderModel } from '@/lib/modelSelection'
import { useModelSelectionStore } from '@/store/modelSelectionStore'
import { loadToolDefaults } from '@/lib/toolSettings'
import { ModelSelector } from './input/ModelSelector'
import { ToolSelector, type ChatToolOption } from './input/ToolSelector'
import { useComposerAttachments } from './input/useComposerAttachments'

const EditorContent = EditorContentBase as unknown as React.FC<{ editor: ReturnType<typeof useEditor> }>


type ChatInputProps = {
  sessionId?: number
  variant?: 'default' | 'quick'
}

const quickNoDragStyle = {
  '--wails-draggable': 'no-drag',
  WebkitAppRegion: 'no-drag',
} as CSSProperties & Record<'--wails-draggable', string>

export function ChatInput({ sessionId, variant = 'default' }: ChatInputProps) {
  const { t } = useTranslation()
  const isQuick = variant === 'quick'
  const {
    currentConversationId,
    isStreaming,
    toolsLocked,
    createConversation,
    sendMessage,
    stopGeneration,
    setCurrentConversation,
  } = useChatStore(useShallow((state) => {
    const resolvedSessionId = sessionId ?? state.currentConversationId
    const currentConversation = resolvedSessionId
      ? state.conversations.find((conversation) => conversation.id === resolvedSessionId)
      : undefined
    const sessionStatus = resolvedSessionId
      ? (state.sessionStatuses[resolvedSessionId] ?? currentConversation?.status)
      : undefined

    return {
      currentConversationId: state.currentConversationId,
      isStreaming: sessionStatus === 'loading' || sessionStatus === 'waiting-unread',
      toolsLocked: !!resolvedSessionId && (
        state.sessionToolLocks[resolvedSessionId] === true
        || (state.messages[resolvedSessionId]?.length ?? 0) > 0
        || sessionStatus === 'loading' || sessionStatus === 'waiting-unread'
      ),
      createConversation: state.createConversation,
      sendMessage: state.sendMessage,
      stopGeneration: state.stopGeneration,
      setCurrentConversation: state.setCurrentConversation,
    }
  }))
  const [toolOptions, setToolOptions] = useState<ChatToolOption[]>([])
  const [draftToolIds, setDraftToolIds] = useState<string[] | null>(null)
  const [toolsSaving, setToolsSaving] = useState(false)
  const [toolError, setToolError] = useState('')
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([])
  const [selectedModelId, setSelectedModelId] = useState<number | null>(null)
  const [toolsOpen, setToolsOpen] = useState(false)
  const [modelOpen, setModelOpen] = useState(false)
  const [modelQuery, setModelQuery] = useState('')
  const [isEmpty, setIsEmpty] = useState(true)
  const targetSessionId = sessionId ?? currentConversationId
  const {
    attachments,
    setAttachments,
    composerId,
    draftScopeId,
    isImporting,
    isDraggingOver,
    handleAttach,
    handlePaste,
    handleDragOver,
    handleDragLeave,
    handleDrop,
  } = useComposerAttachments(targetSessionId, {
    createSession: async () => {
      const id = await createConversation()
      if (id && draftToolIds !== null) await useChatStore.getState().saveSessionTools(id, draftToolIds)
      return id
    },
    onSessionCreated: setCurrentConversation,
  })
  const defaultToolIds = useAppStore(state => state.enabledToolIds)
  const sessionToolIds = useChatStore(state => targetSessionId ? state.sessionToolIds[targetSessionId] : undefined)
  const selectedToolIds = targetSessionId ? (sessionToolIds ?? []) : (draftToolIds ?? defaultToolIds)
  const tools = toolOptions.map(tool => ({ ...tool, enabled: selectedToolIds.includes(tool.id) }))

  const enabledToolsCount = tools.filter(x => x.enabled).length

  const exitListToParagraph = () => {
    if (!editor) return false

    const { state, view } = editor
    const { $from } = state.selection
    let listDepth: number | null = null

    for (let depth = $from.depth; depth > 0; depth -= 1) {
      const nodeName = $from.node(depth).type.name

      if (nodeName === 'bulletList' || nodeName === 'orderedList') {
        listDepth = depth
        break
      }
    }

    if (listDepth === null) return false

    const insertPos = $from.after(listDepth)
    const paragraph = state.schema.nodes.paragraph.create()
    const tr = state.tr.insert(insertPos, paragraph)

    tr.setSelection(TextSelection.near(tr.doc.resolve(insertPos + 1)))
    view.dispatch(tr.scrollIntoView())
    view.focus()

    return true
  }

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
      SingleLineCodeBlock,
      Placeholder.configure({
        placeholder: t('input.placeholder'),
      }),
      Markdown.configure({
        html: false,
        breaks: true,
        transformPastedText: true,
      }),
      SlashCommand,
    ],
    content: '',
    editorProps: {
      attributes: {
        class: 'tiptap-input focus:outline-none px-4 pt-3 pb-2 min-h-[5.25rem] max-h-48 overflow-y-auto text-foreground',
      },
      handleKeyDown: (_view, event) => {
        if (event.key === 'Enter' && event.shiftKey) {
          if (editor?.isActive('heading')) {
            event.preventDefault()
            editor.chain().focus().splitBlock().setParagraph().run()
            return true
          }

          if (editor?.isActive('listItem')) {
            event.preventDefault()
            return exitListToParagraph()
          }
        }

        if (event.key === 'Enter' && !event.shiftKey) {
          event.preventDefault()
          handleSend()
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor }) => {
      if (convertFencedCodeBlock(editor)) {
        setIsEmpty(false)
        return
      }

      setIsEmpty(editor.isEmpty)
    },
  })

  useEffect(() => {
    const load = async (isInitial: boolean) => {
      try {
        const result = await ProviderBinding.ProviderAndModelList({})
        if (!result?.provider_models) return
        setProviderModels(result.provider_models)
        await useModelSelectionStore.persist.rehydrate()
        const preferences = useModelSelectionStore.getState()
        setSelectedModelId(current => {
          const available = result.provider_models.some(pm => pm.provider.enabled && pm.models.some(m => m.id === current && m.enable))
          return !isInitial && available ? current : (preferredModel(result.provider_models, preferences)?.id ?? null)
        })
      } catch {
        // ignore
      }
    }
    void load(true)
    const handleFocus = () => void load(false)
    window.addEventListener('focus', handleFocus)
    return () => window.removeEventListener('focus', handleFocus)
  }, [])

  useEffect(() => {
    let cancelled = false
    const loadTools = async () => {
      try {
        const [result] = await Promise.all([
          PluginBinding.ListAvailableTools({}),
          loadToolDefaults(),
          targetSessionId ? useChatStore.getState().loadSessionTools(targetSessionId) : Promise.resolve(),
        ])
        if (cancelled) return
        if (!result?.tools) return
        setToolOptions(result.tools.map((tool) => ({
          id: tool.id,
          name: tool.name,
          description: tool.description,
          category: tool.category,
          enabled: false,
        })))
        setToolError('')
      } catch {
        if (!cancelled) setToolError(t('settingsPage.general.tools.loadError'))
      }
    }

    void loadTools()
    const handleFocus = () => { void loadTools() }
    window.addEventListener('focus', handleFocus)
    return () => { cancelled = true; window.removeEventListener('focus', handleFocus) }
  }, [targetSessionId, t])

  useEffect(() => {
    if (!editor) return
    editor.commands.clearContent()
    setIsEmpty(true)
    setAttachments([])
    setDraftToolIds(null)
  }, [draftScopeId, editor, setAttachments])



  const handleSend = async () => {
    if (!editor || isStreaming || toolsSaving || isImporting) return
    const content = ((editor.storage as { markdown?: { getMarkdown?: () => string } }).markdown?.getMarkdown?.() ?? '').trim()
    if (!content && attachments.length === 0) return

    const selectedProvider = providerModels.find(pm => pm.models.some(m => m.id === selectedModelId))
    const selectedModel = selectedProvider?.models.find(m => m.id === selectedModelId)
    if (!selectedProvider || !selectedModel) return
    useModelSelectionStore.getState().rememberModel(selectedProvider.provider.id, selectedModel.model)

    let nextSessionId = targetSessionId
    if (!nextSessionId) {
      nextSessionId = await createConversation()
      if (!nextSessionId) return
      setCurrentConversation(nextSessionId)
    }

    let enabledUserTools: string[]
    try {
      if (draftToolIds !== null && !targetSessionId) {
        await useChatStore.getState().saveSessionTools(nextSessionId, draftToolIds)
      }
      enabledUserTools = useChatStore.getState().sessionToolIds[nextSessionId]
        ?? await useChatStore.getState().loadSessionTools(nextSessionId)
    } catch {
      setToolError(t('settingsPage.general.tools.loadError'))
      return
    }

    const outboundAttachments = attachments
    editor.commands.clearContent()
    setIsEmpty(true)
    setAttachments([])

    await sendMessage({
      sessionId: nextSessionId,
      content,
      providerId: selectedProvider.provider.id,
      modelName: selectedModel.model,
      enabledUserTools,
      attachments: outboundAttachments,
    })
  }

  const handleStop = () => {
    if (targetSessionId) {
      void stopGeneration(targetSessionId)
    }
  }


  const closeModelSelector = () => {
    setModelOpen(false)
    setModelQuery('')
  }

  const toggleTool = async (id: string) => {
    if (toolsSaving || toolsLocked) return
    const next = selectedToolIds.includes(id) ? selectedToolIds.filter(value => value !== id) : [...selectedToolIds, id]
    if (!targetSessionId) { setDraftToolIds(next); return }
    setToolsSaving(true)
    try {
      await useChatStore.getState().saveSessionTools(targetSessionId, next)
      setToolError('')
    } catch {
      setToolError(t('settingsPage.general.tools.saveError'))
    } finally { setToolsSaving(false) }
  }

  const groupedTools = tools.reduce<Record<string, ChatToolOption[]>>((acc, tool) => {
    const key = tool.category || 'other'
    acc[key] = acc[key] ? [...acc[key], tool] : [tool]
    return acc
  }, {})

  const enabledProviders = providerModels
    .filter(pm => pm.provider.enabled)
    .map(pm => ({
      id: pm.provider.id,
      name: pm.provider.provider_name,
      models: pm.models
        .filter(m => m.enable)
        .map(m => ({ id: m.id, name: m.alias || m.model, model: m.model })),
    }))
    .filter(pm => pm.models.length > 0)

  const selectedProviderName = enabledProviders.find(p => p.models.some(m => m.id === selectedModelId))?.name
  const selectedModelObj = enabledProviders.flatMap(p => p.models).find(m => m.id === selectedModelId)

  const normalizedModelQuery = modelQuery.trim().toLowerCase()
  const filteredProviders = enabledProviders
    .map(provider => {
      const providerMatches = provider.name.toLowerCase().includes(normalizedModelQuery)
      const models = normalizedModelQuery && !providerMatches
        ? provider.models.filter(model => model.name.toLowerCase().includes(normalizedModelQuery))
        : provider.models
      return { ...provider, models }
    })
    .filter(provider => provider.models.length > 0)

  return (
    <div
      style={isQuick ? quickNoDragStyle : undefined}
      className={cn('chat-input-area group/composer shrink-0', isQuick ? 'px-3 pb-3 pt-1' : 'desktop-composer-area pb-4 pt-2')}
      data-file-drop-target
      data-composer-id={composerId}
      onPaste={handlePaste}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <div className={cn('mx-auto w-full', isQuick ? 'max-w-none px-0' : 'desktop-composer-width px-4')}>
        <div className={cn(
          'border border-border bg-background shadow-sm focus-within:border-primary/40 transition-colors relative group-[.file-drop-target-active]/composer:border-primary group-[.file-drop-target-active]/composer:bg-primary/5',
          isQuick ? 'rounded-lg' : 'desktop-composer rounded-2xl',
          isDraggingOver && "border-primary"
        )}>
          {isDraggingOver && (
            <div className={cn(
              'absolute inset-0 z-10 bg-primary/5 flex items-center justify-center pointer-events-none',
              isQuick ? 'rounded-lg' : 'rounded-2xl',
            )}>
              <span className="text-sm text-primary font-medium">{t('input.dropFiles')}</span>
            </div>
          )}
        {/* Editor */}
        <AttachmentChips
          items={attachments}
          variant="input"
          onRemove={(idx) => setAttachments(prev => prev.filter((_, i) => i !== idx))}
        />
        <EditorContent editor={editor} />
        {toolError && <p role="alert" className="px-4 pb-2 text-xs text-destructive">{toolError}</p>}

        {/* Toolbar */}
        <div className="flex items-center gap-1 px-2 pb-2">
          {/* Add file */}
          <button
            aria-label={t('input.attach')}
            onClick={handleAttach}
            disabled={attachments.length >= ATTACHMENT_MAX_COUNT}
            className="p-1.5 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-colors disabled:opacity-40"
          >
            <Paperclip size={16} />
          </button>

          <ToolSelector
            open={toolsOpen}
            onToggleOpen={() => { setToolsOpen(v => !v); setModelOpen(false) }}
            onClose={() => setToolsOpen(false)}
            locked={toolsLocked}
            busy={toolsSaving || (!!targetSessionId && sessionToolIds === undefined)}
            enabledCount={enabledToolsCount}
            groupedTools={groupedTools}
            onToggleTool={(id) => { void toggleTool(id) }}
            onManage={() => {
              setToolsOpen(false)
              void Window.OpenSettings({ tab: 'plugins' })
            }}
          />

          <ModelSelector
            open={modelOpen}
            onToggleOpen={() => {
              setModelOpen(v => {
                if (v) setModelQuery('')
                return !v
              })
              setToolsOpen(false)
            }}
            onClose={closeModelSelector}
            label={[selectedProviderName, selectedModelObj?.name].filter(Boolean).join(' · ')}
            providers={filteredProviders}
            selectedModelId={selectedModelId}
            query={modelQuery}
            onQueryChange={setModelQuery}
            onSelectProvider={(providerId) => {
              const pm = providerModels.find(p => p.provider.id === providerId)
              const model = pm && preferredProviderModel(pm, useModelSelectionStore.getState())
              if (model) { setSelectedModelId(model.id); useModelSelectionStore.getState().rememberModel(providerId, model.model); closeModelSelector() }
            }}
            onSelectModel={(providerId, model) => { setSelectedModelId(model.id); useModelSelectionStore.getState().rememberModel(providerId, model.model); closeModelSelector() }}
          />

          <div className="flex-1" />

          {/* Send / Stop button */}
          <button
            aria-label={isStreaming ? t('input.stop') : t('input.send')}
            onClick={() => {
              void (isStreaming ? Promise.resolve(handleStop()) : handleSend())
            }}
            disabled={!isStreaming && (toolsSaving || isImporting || (isEmpty && attachments.length === 0))}
            className={cn(
              'p-1.5 rounded-lg transition-colors',
              isStreaming
                ? 'bg-destructive/10 text-destructive hover:bg-destructive/20'
                : (!isEmpty || attachments.length > 0)
                ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                : 'text-muted-foreground cursor-not-allowed'
            )}
          >
            {isStreaming ? <Square size={16} /> : <Send size={16} />}
          </button>
        </div>
        </div>
      </div>
    </div>
  )
}
