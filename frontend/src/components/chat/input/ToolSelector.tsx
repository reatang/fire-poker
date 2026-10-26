import { Check, ChevronDown, Wrench } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { TOOL_I18N_KEYS } from '@/lib/toolSettings'

export type ChatToolOption = {
  id: string
  name: string
  description: string
  category: string
  enabled: boolean
}

type ToolSelectorProps = {
  open: boolean
  onToggleOpen: () => void
  onClose: () => void
  // locked: the conversation has started, so the selection is frozen.
  locked: boolean
  // busy: a save is in flight or the session's selection has not loaded yet.
  busy: boolean
  enabledCount: number
  groupedTools: Record<string, ChatToolOption[]>
  onToggleTool: (id: string) => void
  onManage: () => void
}

// ToolSelector is the composer's tool picker: a toggle button plus a grouped checklist popover.
export function ToolSelector({ open, onToggleOpen, onClose, locked, busy, enabledCount, groupedTools, onToggleTool, onManage }: ToolSelectorProps) {
  const { t } = useTranslation()

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

  return (
    <div className="relative">
      <button
        title={locked ? t('settingsPage.general.tools.locked') : undefined}
        onClick={onToggleOpen}
        className={cn(
          'flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-colors',
          'hover:bg-accent text-muted-foreground hover:text-foreground',
          open && 'bg-accent text-foreground'
        )}
      >
        <Wrench size={14} />
        <span>
          {enabledCount > 0
            ? t('input.toolsSelected', { count: enabledCount })
            : t('input.tools')}
        </span>
        <ChevronDown size={10} />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={onClose} />
          <div className="absolute bottom-full left-0 mb-1 z-20 w-64 rounded-xl border border-border bg-popover shadow-lg">
            <div className="max-h-72 overflow-y-auto py-1">
              {locked && <p className="px-3 py-2 text-xs text-muted-foreground">{t('settingsPage.general.tools.locked')}</p>}
              {Object.entries(groupedTools).map(([category, categoryTools]) => (
                <div key={category}>
                  <div className="px-3 py-1.5 text-[11px] font-semibold uppercase text-muted-foreground bg-muted/50">
                    {getToolCategoryLabel(category)}
                  </div>
                  {categoryTools.map(tool => (
                    <button
                      key={tool.id}
                      disabled={locked || busy}
                      aria-pressed={tool.enabled}
                      onClick={() => onToggleTool(tool.id)}
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
                onClick={onManage}
                className="w-full rounded-lg border border-border px-3 py-2 text-left text-xs text-foreground transition-colors hover:bg-accent"
              >
                {t('input.manageToolsPlugins')}
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
