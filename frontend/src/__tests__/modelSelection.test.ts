import { beforeEach, describe, expect, it } from 'vitest'
import { preferredModel, preferredProviderModel } from '@/lib/modelSelection'
import { useModelSelectionStore as useAppStore } from '@/store/modelSelectionStore'

const providers = [
  { provider: { id: 1, enabled: true, is_default: true }, models: [{ id: 11, model: 'a-default', enable: true, is_default: true }, { id: 12, model: 'a-custom', enable: true, is_default: false }] },
  { provider: { id: 2, enabled: true, is_default: false }, models: [{ id: 21, model: 'b', enable: true, is_default: false }] },
]

beforeEach(() => { useAppStore.setState({ lastProviderId: null, lastModelsByProvider: {} }) })

describe('remember model per provider', () => {
  it('persists separate choices and restores the last provider after rehydration', async () => {
    useAppStore.getState().rememberModel(1, 'a-custom')
    useAppStore.getState().rememberModel(2, 'b')
    const saved = localStorage.getItem('fire-poker-model-selection')
    expect(saved).toContain('a-custom')
    useAppStore.setState({ lastProviderId: null, lastModelsByProvider: {} })
    localStorage.setItem('fire-poker-model-selection', saved!)
    await useAppStore.persist.rehydrate()
    const preferences = useAppStore.getState()
    expect(preferredModel(providers, preferences)?.model).toBe('b')
    expect(preferredProviderModel(providers[0], preferences)?.model).toBe('a-custom')
  })

  it('survives changed model IDs and falls back when a saved model or provider is unavailable', () => {
    const preferences = { lastProviderId: 1, lastModelsByProvider: { '1': 'a-custom' } }
    expect(preferredModel([{ ...providers[0], models: providers[0].models.map(m => ({ ...m, id: m.id + 100 })) }], preferences)?.id).toBe(112)
    expect(preferredModel([{ ...providers[0], models: [providers[0].models[0]] }], preferences)?.model).toBe('a-default')
    expect(preferredModel([{ ...providers[0], provider: { ...providers[0].provider, enabled: false } }, providers[1]], preferences)?.model).toBe('b')
  })
})
