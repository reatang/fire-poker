import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DataManagementView } from '@/components/settings/data/DataManagementView'

const { listDataDirectories, openFile } = vi.hoisted(() => ({
  listDataDirectories: vi.fn(),
  openFile: vi.fn(),
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/settings', () => ({
  Settings: { ListDataDirectories: listDataDirectories },
}))

vi.mock('@bindings/github.com/reatang/fire-poker/backend/service/file', () => ({
  File: { OpenFile: openFile },
}))

beforeEach(() => {
  listDataDirectories.mockReset()
  openFile.mockReset().mockResolvedValue({})
})

describe('DataManagementView', () => {
  it('lists every directory read-only with its path', async () => {
    listDataDirectories.mockResolvedValue({
      items: [
        { id: 'data_root', path: '/Users/u/.fire-poker', reveal_dir: '/Users/u/.fire-poker', is_file: false, exists: true },
        { id: 'config', path: '/Users/u/.fire-poker/config.json', reveal_dir: '/Users/u/.fire-poker', is_file: true, exists: true },
        { id: 'logs', path: '/Users/u/.fire-poker/logs', reveal_dir: '/Users/u/.fire-poker/logs', is_file: false, exists: false },
      ],
    })

    render(<DataManagementView />)

    const root = await screen.findByRole('region', { name: '数据目录' })
    const rootPath = within(root).getByRole('textbox', { name: '路径' })
    expect(rootPath).toHaveValue('/Users/u/.fire-poker')
    expect(rootPath).toHaveAttribute('readonly')
    expect(screen.getByRole('region', { name: '配置文件' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '应用' })).not.toBeInTheDocument()
  })

  it('opens the containing folder and disables opening missing paths', async () => {
    const user = userEvent.setup()
    listDataDirectories.mockResolvedValue({
      items: [
        { id: 'config', path: '/Users/u/.fire-poker/config.json', reveal_dir: '/Users/u/.fire-poker', is_file: true, exists: true },
        { id: 'logs', path: '/Users/u/.fire-poker/logs', reveal_dir: '/Users/u/.fire-poker/logs', is_file: false, exists: false },
      ],
    })

    render(<DataManagementView />)

    const config = await screen.findByRole('region', { name: '配置文件' })
    await user.click(within(config).getByRole('button', { name: '在文件管理器中打开' }))
    expect(openFile).toHaveBeenCalledWith({ path: '/Users/u/.fire-poker' })

    const logs = screen.getByRole('region', { name: '日志' })
    expect(within(logs).getByText('尚未创建')).toBeInTheDocument()
    expect(within(logs).getByRole('button', { name: '在文件管理器中打开' })).toBeDisabled()
  })

  it('copies a path to the clipboard', async () => {
    const user = userEvent.setup()
    listDataDirectories.mockResolvedValue({
      items: [{ id: 'cache', path: '/Users/u/.fire-poker/cache', reveal_dir: '/Users/u/.fire-poker/cache', is_file: false, exists: true }],
    })

    render(<DataManagementView />)

    const cache = await screen.findByRole('region', { name: '缓存' })
    await user.click(within(cache).getByRole('button', { name: '复制路径' }))
    await expect(navigator.clipboard.readText()).resolves.toBe('/Users/u/.fire-poker/cache')
    expect(within(cache).getByRole('button', { name: '已复制' })).toBeInTheDocument()
  })

  it('shows an error when listing fails', async () => {
    listDataDirectories.mockRejectedValue(new Error('boom'))

    render(<DataManagementView />)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('读取数据目录失败'))
  })
})
