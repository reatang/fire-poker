import { useEffect, useLayoutEffect, useRef, useState, useCallback } from 'react'

function scrollElementToBottom(el: HTMLDivElement, smooth = false) {
  const top = el.scrollHeight
  if (typeof el.scrollTo === 'function') {
    el.scrollTo({ top, behavior: smooth ? 'smooth' : 'auto' })
    return
  }
  el.scrollTop = top
}

export function useAutoScroll(deps: unknown[], resetKey?: unknown) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const autoScrollRef = useRef(true)

  const scrollToBottom = useCallback((smooth = false) => {
    const el = containerRef.current
    if (!el) return
    scrollElementToBottom(el, smooth)
    autoScrollRef.current = true
    setIsAtBottom(true)
  }, [])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const handler = () => {
      const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50
      setIsAtBottom(atBottom)
      autoScrollRef.current = atBottom
    }
    el.addEventListener('scroll', handler, { passive: true })
    return () => el.removeEventListener('scroll', handler)
  }, [])

  useLayoutEffect(() => {
    if (resetKey === undefined) return
    autoScrollRef.current = true
    const el = containerRef.current
    if (!el) return
    scrollElementToBottom(el)
    setIsAtBottom(true)
  }, [resetKey])

  useLayoutEffect(() => {
    if (autoScrollRef.current) {
      const el = containerRef.current
      if (!el) return
      scrollElementToBottom(el)
    }
  }, deps) // eslint-disable-line react-hooks/exhaustive-deps -- callers pass the list that should trigger a scroll.

  return { containerRef, isAtBottom, scrollToBottom, autoScrollRef }
}
