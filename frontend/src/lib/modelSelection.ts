// Persist canonical model names per provider; database model IDs can change
// when a provider refreshes its model list.
type ModelOption = { id: number; model: string; enable: boolean; is_default: boolean }
type ProviderOptions = { provider: { id: number; enabled: boolean; is_default: boolean }; models: ModelOption[] }
type Preferences = { lastProviderId: number | null; lastModelsByProvider: Record<string, string> }

export function preferredProviderModel<T extends ModelOption>(provider: { provider: { id: number }; models: T[] }, preferences: Preferences): T | undefined {
  const models = provider.models.filter(m => m.enable)
  const saved = preferences.lastModelsByProvider[provider.provider.id]
  return models.find(m => m.model === saved) ?? models.find(m => m.is_default) ?? models[0]
}

export function preferredModel<T extends ProviderOptions>(providers: T[], preferences: Preferences) {
  const enabled = providers.filter(p => p.provider.enabled && p.models.some(m => m.enable))
  const provider = enabled.find(p => p.provider.id === preferences.lastProviderId)
    ?? enabled.find(p => p.provider.is_default) ?? enabled[0]
  return provider ? preferredProviderModel(provider, preferences) : undefined
}
