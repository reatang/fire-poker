import { useState, useEffect, useRef } from 'react'
import type { CSSProperties } from 'react'
import { Events } from '@wailsio/runtime'
import { useEditor, EditorContent as EditorContentBase } from '@tiptap/react'
import { Extension, InputRule } from '@tiptap/core'
import type { Editor, JSONContent } from '@tiptap/core'
import type { Node as ProseMirrorNode } from '@tiptap/pm/model'
import { TextSelection } from '@tiptap/pm/state'

const EditorContent = EditorContentBase as unknown as React.FC<{ editor: ReturnType<typeof useEditor> }>
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Suggestion from '@tiptap/suggestion'
import { Markdown } from 'tiptap-markdown'
import { buildSlashSuggestion } from './slashSuggestion'
import {
  Send, Square, Paperclip, Wrench, ChevronDown, Check, Search,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useChatStore } from '@/store/chatStore'
import { useAppStore } from '@/store/appStore'
import { Provider as ProviderBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/provider'
import { Plugin as PluginBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/plugin'
import { Window } from '@bindings/github.com/reatang/fire-poker/backend/service/window'
import type { ProviderModel } from '@bindings/github.com/reatang/fire-poker/backend/models/view_model/models'
import type { Attachment } from '@/types'
import { useShallow } from 'zustand/react/shallow'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { SelectFileInput, SaveTempFileInput } from '@bindings/github.com/reatang/fire-poker/backend/service/file/file_dto'
import { AttachmentChips } from './AttachmentChips'
import { ATTACHMENT_MAX_COUNT, ATTACHMENT_MAX_BYTES, inferAttachmentMeta } from '@/lib/attachments'
import { preferredModel, preferredProviderModel } from '@/lib/modelSelection'
import { useModelSelectionStore } from '@/store/modelSelectionStore'
import { loadToolDefaults, TOOL_I18N_KEYS } from '@/lib/toolSettings'

const SlashCommand = Extension.create({
  name: 'slashCommand',
  addProseMirrorPlugins() {
    return [
      Suggestion({
        editor: this.editor as any,
        ...buildSlashSuggestion(),
        command: ({ editor, range, props }: { editor: any; range: { from: number; to: number }; props: { name: string } }) => {
          editor.chain().focus().deleteRange(range).insertContent(`/${props.name} `).run()
        },
      }),
    ]
  },
})

type ChatToolOption = {
  id: string
  name: string
  description: string
  category: string
  enabled: boolean
}

async function blobToBase64(blob: Blob): Promise<string> {
  const buf = await blob.arrayBuffer()
  const bytes = new Uint8Array(buf)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

const SingleLineCodeBlock = Extension.create({
  name: 'singleLineCodeBlock',

  addInputRules() {
    return [
      new InputRule({
        find: /^```\s?([^`]+?)\s?```$/,
        handler: ({ range, match, commands }) => {
          const code = match[1]?.trim() ?? ''

          commands.insertContentAt(range, {
            type: 'codeBlock',
            content: code ? [{ type: 'text', text: code }] : [],
          })
        },
      }),
    ]
  },
})

const createCodeBlockContent = (code: string): JSONContent => ({
  type: 'codeBlock',
  content: code ? [{ type: 'text', text: code }] : [],
})

const getNodeTextWithBreaks = (node: ProseMirrorNode) => (
  node.textBetween(0, node.content.size, '\n', '\n')
)

const convertFencedCodeBlock = (editor: Editor) => {
  let converted = false

  editor.state.doc.descendants((node, pos) => {
    if (converted || node.type.name !== 'paragraph') return false

    const match = getNodeTextWithBreaks(node).match(/^```[^\n]*\n([\s\S]*?)\n```$/)
    if (!match) return true

    editor.commands.insertContentAt(
      { from: pos, to: pos + node.nodeSize },
      createCodeBlockContent(match[1].trim())
    )
    converted = true

    return false
  })

  if (converted) return true

  const topLevelNodes: Array<{
    node: ProseMirrorNode
    pos: number
  }> = []

  editor.state.doc.forEach((node, offset) => {
    topLevelNodes.push({ node, pos: offset + 1 })
  })

  for (let start = 0; start < topLevelNodes.length; start += 1) {
    const startText = getNodeTextWithBreaks(topLevelNodes[start].node).trim()
    if (topLevelNodes[start].node.type.name !== 'paragraph' || !/^```[^\n]*$/.test(startText)) {
      continue
    }

    for (let end = start + 1; end < topLevelNodes.length; end += 1) {
      const endText = getNodeTextWithBreaks(topLevelNodes[end].node).trim()

      if (topLevelNodes[end].node.type.name !== 'paragraph' || endText !== '```') continue

      const code = topLevelNodes
        .slice(start + 1, end)
        .map(item => getNodeTextWithBreaks(item.node))
        .join('\n')
        .trim()

      editor.commands.insertContentAt(
        { from: topLevelNodes[start].pos, to: topLevelNodes[end].pos + topLevelNodes[end].node.nodeSize },
        createCodeBlockContent(code)
      )

      return true
    }
  }

  return false
}

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
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const attachmentsRef = useRef<Attachment[]>([])
  attachmentsRef.current = attachments
  const [isDraggingOver, setIsDraggingOver] = useState(false)
  const targetSessionId = sessionId ?? currentConversationId
  const defaultToolIds = useAppStore(state => state.enabledToolIds)
  const sessionToolIds = useChatStore(state => targetSessionId ? state.sessionToolIds[targetSessionId] : undefined)
  const selectedToolIds = targetSessionId ? (sessionToolIds ?? []) : (draftToolIds ?? defaultToolIds)
  const tools = toolOptions.map(tool => ({ ...tool, enabled: selectedToolIds.includes(tool.id) }))

  const enabledToolsCount = tools.filter(x => x.enabled).length

  const getToolCategoryLabel = (category: string) => {
    if (category === 'builtin') return t('input.toolCategories.builtin')
    if (category === 'mcp') return t('input.toolCategories.mcp')
    if (category === 'user') return t('input.toolCategories.user')
    return t('input.toolCategories.other')
  }

  const getToolDisplayName = (tool: ChatToolOption) => {
    const key = TOOL_I18N_KEYS[tool.id]
    return key ? t(key.name) : tool.name
  }

  const getToolDisplayDescription = (tool: ChatToolOption) => {
    const key = TOOL_I18N_KEYS[tool.id]
    return key ? t(key.description) : tool.description
  }

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
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

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
  }, [targetSessionId, editor])

  useEffect(() => {
    const offEnter = Events.On(Events.Types.Mac.WindowFileDraggingEntered, () => {
      setIsDraggingOver(true)
    })
    const offExit = Events.On(Events.Types.Mac.WindowFileDraggingExited, () => {
      setIsDraggingOver(false)
    })
    const offPerformed = Events.On(Events.Types.Mac.WindowFileDraggingPerformed, () => {
      setIsDraggingOver(false)
    })
    const offDrop = Events.On('files-dropped', (event) => {
      const payload = event.data as { files?: string[] } | null
      const files = payload?.files ?? []
      const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
      const newAtts = files.slice(0, remaining).map(path => inferAttachmentMeta(path))
      if (newAtts.length > 0) {
        setAttachments(prev => [...prev, ...newAtts])
      }
    })

    return () => {
      offEnter()
      offExit()
      offPerformed()
      offDrop()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleAttach = async () => {
    if (attachments.length >= ATTACHMENT_MAX_COUNT) return
    const result = await FileBinding.SelectFile(new SelectFileInput())
    if (!result?.file_path) return
    setAttachments(prev => [...prev, inferAttachmentMeta(result.file_path)])
  }

  const handleSend = async () => {
    if (!editor || isStreaming || toolsSaving) return
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
      baseUrl: selectedProvider.provider.base_url,
      apiKey: selectedProvider.provider.api_key,
      modelName: selectedModel.model,
      providerType: selectedProvider.provider.provider_type,
      enabledUserTools,
      attachments: outboundAttachments,
    })
  }

  const handleStop = () => {
    if (targetSessionId) {
      void stopGeneration(targetSessionId)
    }
  }

  const handlePaste = async (event: React.ClipboardEvent<HTMLDivElement>) => {
    const items = Array.from(event.clipboardData.items)
    const fileItems = items.filter(item => item.kind === 'file')
    if (fileItems.length === 0) return

    event.preventDefault()

    const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
    const toProcess = fileItems.slice(0, remaining).map(item => {
      const blob = item.getAsFile()
      if (!blob) return null
      if (blob.size > ATTACHMENT_MAX_BYTES) return null
      const name = blob.name || `pasted-${Date.now()}.${item.type.split('/')[1] ?? 'bin'}`
      return { blob, name, mime: item.type || 'application/octet-stream' }
    }).filter((x): x is NonNullable<typeof x> => x !== null)

    const results = await Promise.allSettled(
      toProcess.map(async ({ blob, name, mime }) => {
        const data = await blobToBase64(blob)
        const result = await FileBinding.SaveTempFile(new SaveTempFileInput({ name, data, mime }))
        if (!result?.file_path) throw new Error('no path')
        const att = inferAttachmentMeta(result.file_path)
        return { ...att, name }
      })
    )

    const newAtts = results
      .filter((r): r is PromiseFulfilledResult<Attachment> => r.status === 'fulfilled')
      .map(r => r.value)

    if (newAtts.length > 0) {
      setAttachments(prev => [...prev, ...newAtts])
    }
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    setIsDraggingOver(true)
  }

  const handleDragLeave = (event: React.DragEvent<HTMLDivElement>) => {
    if ((event.currentTarget as HTMLDivElement).contains(event.relatedTarget as Node)) return
    setIsDraggingOver(false)
  }

  const handleDrop = async (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setIsDraggingOver(false)

    const dropped = Array.from(event.dataTransfer.files)
    const remaining = ATTACHMENT_MAX_COUNT - attachmentsRef.current.length
    const toProcess = dropped.slice(0, remaining).filter(file => file.size <= ATTACHMENT_MAX_BYTES)

    const results = await Promise.allSettled(
      toProcess.map(async (file) => {
        const data = await blobToBase64(file)
        const result = await FileBinding.SaveTempFile(
          new SaveTempFileInput({ name: file.name, data, mime: file.type || 'application/octet-stream' })
        )
        if (!result?.file_path) throw new Error('no path')
        const att = inferAttachmentMeta(result.file_path)
        return { ...att, name: file.name }
      })
    )

    const newAtts = results
      .filter((r): r is PromiseFulfilledResult<Attachment> => r.status === 'fulfilled')
      .map(r => r.value)

    if (newAtts.length > 0) {
      setAttachments(prev => [...prev, ...newAtts])
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
      className={cn('chat-input-area shrink-0', isQuick ? 'px-3 pb-3 pt-1' : 'desktop-composer-area pb-4 pt-2')}
      data-file-drop-target
      onPaste={handlePaste}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <div className={cn('mx-auto w-full', isQuick ? 'max-w-none px-0' : 'desktop-composer-width px-4')}>
        <div className={cn(
          'border border-border bg-background shadow-sm focus-within:border-primary/40 transition-colors relative',
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

          {/* Tools */}
          <div className="relative">
            <button
              title={toolsLocked ? t('settingsPage.general.tools.locked') : undefined}
              onClick={() => { setToolsOpen(v => !v); setModelOpen(false) }}
              className={cn(
                'flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-colors',
                'hover:bg-accent text-muted-foreground hover:text-foreground',
                toolsOpen && 'bg-accent text-foreground'
              )}
            >
              <Wrench size={14} />
              <span>
                {enabledToolsCount > 0
                  ? t('input.toolsSelected', { count: enabledToolsCount })
                  : t('input.tools')}
              </span>
              <ChevronDown size={10} />
            </button>
            {toolsOpen && (
              <>
                <div className="fixed inset-0 z-10" onClick={() => setToolsOpen(false)} />
                <div className="absolute bottom-full left-0 mb-1 z-20 w-64 rounded-xl border border-border bg-popover shadow-lg">
                  <div className="max-h-72 overflow-y-auto py-1">
                    {toolsLocked && <p className="px-3 py-2 text-xs text-muted-foreground">{t('settingsPage.general.tools.locked')}</p>}
                    {Object.entries(groupedTools).map(([category, categoryTools]) => (
                      <div key={category}>
                        <div className="px-3 py-1.5 text-[11px] font-semibold uppercase text-muted-foreground bg-muted/50">
                          {getToolCategoryLabel(category)}
                        </div>
                        {categoryTools.map(tool => (
                          <button
                            key={tool.id}
                            disabled={toolsLocked || toolsSaving || (!!targetSessionId && sessionToolIds === undefined)}
                            aria-pressed={tool.enabled}
                            onClick={() => { void toggleTool(tool.id) }}
                            className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-left"
                          >
                            <div className={cn(
                              'w-4 h-4 rounded border flex items-center justify-center shrink-0',
                              tool.enabled ? 'bg-primary border-primary' : 'border-border'
                            )}>
                              {tool.enabled && <Check size={10} className="text-primary-foreground" />}
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="text-foreground text-xs font-medium">{getToolDisplayName(tool)}</div>
                              <div className="text-muted-foreground text-xs truncate">{getToolDisplayDescription(tool)}</div>
                            </div>
                          </button>
                        ))}
                      </div>
                    ))}
                  </div>
                  <div className="border-t border-border p-2">
                    <button
                      type="button"
                      onClick={() => {
                        setToolsOpen(false)
                        void Window.OpenSettings({ tab: 'plugins' })
                      }}
                      className="w-full rounded-lg border border-border px-3 py-2 text-left text-xs text-foreground transition-colors hover:bg-accent"
                    >
                      {t('input.manageToolsPlugins')}
                    </button>
                  </div>
                </div>
              </>
            )}
          </div>

          {/* Model selector */}
          <div className="relative">
            <button
              onClick={() => {
                setModelOpen(v => {
                  if (v) setModelQuery('')
                  return !v
                })
                setToolsOpen(false)
              }}
              className={cn(
                'flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-colors',
                'hover:bg-accent text-muted-foreground hover:text-foreground',
                modelOpen && 'bg-accent text-foreground'
              )}
            >
              <span className="desktop-model-name truncate" title={[selectedProviderName, selectedModelObj?.name].filter(Boolean).join(' · ')}>{[selectedProviderName, selectedModelObj?.name].filter(Boolean).join(' · ') || t('input.model')}</span>
              <ChevronDown size={10} />
            </button>
            {modelOpen && (
              <>
                <div className="fixed inset-0 z-10" onClick={closeModelSelector} />
                <div className="absolute bottom-full left-0 mb-1 z-20 flex max-h-80 w-64 flex-col rounded-xl border border-border bg-popover shadow-lg">
                  <div className="min-h-0 overflow-y-auto py-1">
                    {filteredProviders.map(provider => (
                      <div key={provider.id}>
                        <button type="button" title="使用该提供商上次选择的模型" onClick={() => {
                          const pm = providerModels.find(p => p.provider.id === provider.id)
                          const model = pm && preferredProviderModel(pm, useModelSelectionStore.getState())
                          if (model) { setSelectedModelId(model.id); useModelSelectionStore.getState().rememberModel(provider.id, model.model); closeModelSelector() }
                        }} className="w-full px-3 py-1.5 text-left text-[11px] font-semibold text-muted-foreground bg-muted/50 hover:bg-accent">
                          {provider.name}
                        </button>
                        {provider.models.map(model => (
                          <button
                            key={model.id}
                            onClick={() => { setSelectedModelId(model.id); useModelSelectionStore.getState().rememberModel(provider.id, model.model); closeModelSelector() }}
                            className={cn(
                              'w-full flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent text-left',
                              selectedModelId === model.id && 'text-primary font-medium'
                            )}
                          >
                            {model.name}
                            {selectedModelId === model.id && <Check size={12} className="ml-auto" />}
                          </button>
                        ))}
                      </div>
                    ))}
                    {filteredProviders.length === 0 && (
                      <div className="px-3 py-6 text-center text-[11px] text-muted-foreground">
                        {t('input.noModels')}
                      </div>
                    )}
                  </div>
                  <div className="border-t border-border p-2">
                    <div className="flex items-center gap-2 rounded-lg border border-input bg-background px-2">
                      <Search size={14} className="text-muted-foreground" />
                      <input
                        value={modelQuery}
                        onChange={e => setModelQuery(e.target.value)}
                        placeholder={t('input.searchModel')}
                        className="h-8 min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                      />
                    </div>
                  </div>
                </div>
              </>
            )}
          </div>

          <div className="flex-1" />

          {/* Send / Stop button */}
          <button
            aria-label={isStreaming ? t('input.stop') : t('input.send')}
            onClick={() => {
              void (isStreaming ? Promise.resolve(handleStop()) : handleSend())
            }}
            disabled={!isStreaming && (toolsSaving || (isEmpty && attachments.length === 0))}
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
