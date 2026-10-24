import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { migrateBrandStorage } from './migrateBrandStorage'

migrateBrandStorage('fire-poker-model-selection', 'lemontea-model-selection')

interface ModelSelection {
  lastProviderId: number | null
  lastModelsByProvider: Record<string, string>
  rememberModel: (providerId: number, model: string) => void
}

export const useModelSelectionStore = create<ModelSelection>()(persist(set => ({
  lastProviderId: null,
  lastModelsByProvider: {},
  rememberModel: (providerId, model) => set(state => ({
    lastProviderId: providerId,
    lastModelsByProvider: { ...state.lastModelsByProvider, [providerId]: model },
  })),
}), { name: 'fire-poker-model-selection' }))
