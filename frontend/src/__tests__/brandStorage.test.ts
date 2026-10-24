import { beforeEach, expect, it } from 'vitest'
import { migrateBrandStorage } from '../store/migrateBrandStorage'

beforeEach(() => localStorage.clear())

it('preserves provider model selections and settings when migrating legacy keys', () => {
  for (const suffix of ['app-settings', 'model-selection']) {
    const value = JSON.stringify({ state: { language: 'en', lastModelsByProvider: { 5: 'model-a' } }, version: 0 })
    localStorage.setItem(`lemontea-${suffix}`, value)
    migrateBrandStorage(`fire-poker-${suffix}`, `lemontea-${suffix}`)
    expect(localStorage.getItem(`fire-poker-${suffix}`)).toBe(value)
    expect(localStorage.getItem(`lemontea-${suffix}`)).toBe(value)
    localStorage.setItem(`fire-poker-${suffix}`, 'new-selection')
    migrateBrandStorage(`fire-poker-${suffix}`, `lemontea-${suffix}`)
    expect(localStorage.getItem(`fire-poker-${suffix}`)).toBe('new-selection')
  }
})
