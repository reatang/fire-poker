import { describe, expect, it } from 'vitest'
import zhCN from '@/i18n/locales/zh-CN'
import en from '@/i18n/locales/en'

// Flattens nested locale objects into dotted keys such as "chat.newChat".
function flattenKeys(value: unknown, prefix = ''): string[] {
  if (typeof value !== 'object' || value === null) return [prefix]
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    flattenKeys(child, prefix ? `${prefix}.${key}` : key),
  )
}

describe('locales', () => {
  it('define the same keys in every language', () => {
    const zhKeys = new Set(flattenKeys(zhCN))
    const enKeys = new Set(flattenKeys(en))
    expect([...zhKeys].filter((key) => !enKeys.has(key))).toEqual([])
    expect([...enKeys].filter((key) => !zhKeys.has(key))).toEqual([])
  })
})
