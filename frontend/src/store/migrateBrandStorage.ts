// Copy once before Zustand hydrates; retain the original as a recovery copy.
export function migrateBrandStorage(key: string, legacyKey: string) {
  try {
    if (localStorage.getItem(key) !== null) return
    const saved = localStorage.getItem(legacyKey)
    if (saved !== null) localStorage.setItem(key, saved)
  } catch {
    // Storage can be unavailable; the stores can still use their defaults.
  }
}
