import { beforeEach, expect, it, vi } from 'vitest'
import { Browser, Clipboard, Flags } from '@wailsio/runtime'
import { openExternalURL, writeClipboardText } from '@/lib/runtimeInterop'

vi.mock('@wailsio/runtime', () => ({
  Browser: { OpenURL: vi.fn() },
  Clipboard: { SetText: vi.fn() },
  Flags: { GetFlag: vi.fn() },
}))

const browserCopy = vi.fn()
beforeEach(() => {
  vi.mocked(Browser.OpenURL).mockReset().mockResolvedValue(undefined)
  vi.mocked(Clipboard.SetText).mockReset().mockResolvedValue(undefined)
  vi.mocked(Flags.GetFlag).mockReset()
  browserCopy.mockReset().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: browserCopy } })
})

it.each([true, false])('uses native clipboard even when file drops are %s', async (flag) => {
  vi.mocked(Flags.GetFlag).mockReturnValue(flag)
  await writeClipboardText('native text')
  expect(Clipboard.SetText).toHaveBeenCalledWith('native text')
  expect(browserCopy).not.toHaveBeenCalled()
})

it('uses browser clipboard without a native runtime', async () => {
  vi.mocked(Flags.GetFlag).mockImplementation(() => { throw new Error('No runtime') })
  await writeClipboardText('browser text')
  expect(browserCopy).toHaveBeenCalledWith('browser text')
  expect(Clipboard.SetText).not.toHaveBeenCalled()
})

it('falls back when native clipboard fails and propagates failure when both fail', async () => {
  vi.mocked(Flags.GetFlag).mockReturnValue(false)
  vi.mocked(Clipboard.SetText).mockRejectedValue(new Error('native denied'))
  await writeClipboardText('fallback')
  expect(browserCopy).toHaveBeenCalledWith('fallback')
  browserCopy.mockRejectedValue(new Error('browser denied'))
  await expect(writeClipboardText('failure')).rejects.toThrow('browser denied')
})

it('opens desktop links once through Wails', async () => {
  vi.mocked(Flags.GetFlag).mockReturnValue(false)
  const open = vi.spyOn(window, 'open').mockImplementation(() => null)
  await openExternalURL('https://example.com/help')
  expect(Browser.OpenURL).toHaveBeenCalledExactlyOnceWith('https://example.com/help')
  expect(open).not.toHaveBeenCalled()
  open.mockRestore()
})

it('opens browser preview links using window.open', async () => {
  const open = vi.spyOn(window, 'open').mockImplementation(() => null)
  await openExternalURL('https://example.com/help')
  expect(open).toHaveBeenCalledExactlyOnceWith('https://example.com/help', '_blank', 'noopener,noreferrer')
  expect(Browser.OpenURL).not.toHaveBeenCalled()
  open.mockRestore()
})

it('ignores URLs which cannot be opened as external links', async () => {
  vi.mocked(Flags.GetFlag).mockReturnValue(true)
  await openExternalURL('javascript:alert(1)')
  await openExternalURL('file:///tmp/private')
  expect(Browser.OpenURL).not.toHaveBeenCalled()
})
