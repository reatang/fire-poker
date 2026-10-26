import { Check, ChevronDown, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

export type ModelOption = { id: number; name: string; model: string }
export type ProviderOption = { id: number; name: string; models: ModelOption[] }

type ModelSelectorProps = {
  open: boolean
  onToggleOpen: () => void
  onClose: () => void
  // label is the "provider · model" text shown on the trigger; empty falls back to a placeholder.
  label: string
  // providers is already filtered by query.
  providers: ProviderOption[]
  selectedModelId: number | null
  query: string
  onQueryChange: (query: string) => void
  onSelectProvider: (providerId: number) => void
  onSelectModel: (providerId: number, model: ModelOption) => void
}

// ModelSelector is the composer's model picker: providers group their models, with a search box.
export function ModelSelector({ open, onToggleOpen, onClose, label, providers, selectedModelId, query, onQueryChange, onSelectProvider, onSelectModel }: ModelSelectorProps) {
  const { t } = useTranslation()

  return (
    <div className="relative">
      <button
        onClick={onToggleOpen}
        className={cn(
          'flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-colors',
          'hover:bg-accent text-muted-foreground hover:text-foreground',
          open && 'bg-accent text-foreground'
        )}
      >
        <span className="desktop-model-name truncate" title={label}>{label || t('input.model')}</span>
        <ChevronDown size={10} />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={onClose} />
          <div className="absolute bottom-full left-0 mb-1 z-20 flex max-h-80 w-64 flex-col rounded-xl border border-border bg-popover shadow-lg">
            <div className="min-h-0 overflow-y-auto py-1">
              {providers.map(provider => (
                <div key={provider.id}>
                  <button type="button" title={t('input.useLastProviderModel')} onClick={() => onSelectProvider(provider.id)} className="w-full px-3 py-1.5 text-left text-[11px] font-semibold text-muted-foreground bg-muted/50 hover:bg-accent">
                    {provider.name}
                  </button>
                  {provider.models.map(model => (
                    <button
                      key={model.id}
                      onClick={() => onSelectModel(provider.id, model)}
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
              {providers.length === 0 && (
                <div className="px-3 py-6 text-center text-[11px] text-muted-foreground">
                  {t('input.noModels')}
                </div>
              )}
            </div>
            <div className="border-t border-border p-2">
              <div className="flex items-center gap-2 rounded-lg border border-input bg-background px-2">
                <Search size={14} className="text-muted-foreground" />
                <input
                  value={query}
                  onChange={e => onQueryChange(e.target.value)}
                  placeholder={t('input.searchModel')}
                  className="h-8 min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                />
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
