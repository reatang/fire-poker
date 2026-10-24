import { act, render } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import i18n from '@/i18n'
import { AppSettingsSyncProvider } from '@/components/providers/AppSettingsSyncProvider'
import { useAppStore } from '@/store/appStore'

afterEach(() => vi.restoreAllMocks())

it('applies tool defaults from another window once without repeating language changes or storage writes', async () => {
  await i18n.changeLanguage('en')
  useAppStore.setState({ language: 'en', enabledToolIds: ['web_fetch'] })
  render(<AppSettingsSyncProvider><div /></AppSettingsSyncProvider>)
  const languageChange = vi.spyOn(i18n, 'changeLanguage')
  const storageWrite = vi.spyOn(Storage.prototype, 'setItem')
  const event = () => new StorageEvent('storage', {
    key: 'fire-poker-app-settings',
    newValue: JSON.stringify({ state: { language: 'en', enabledToolIds: [] } }),
  })
  act(() => { window.dispatchEvent(event()) })
  expect(useAppStore.getState().enabledToolIds).toEqual([])
  storageWrite.mockClear()
  act(() => { window.dispatchEvent(event()) })
  useAppStore.getState().setEnabledToolIds([])
  expect(storageWrite).not.toHaveBeenCalled()
  expect(languageChange).not.toHaveBeenCalled()
})
