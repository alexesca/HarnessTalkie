import { useCallback, useEffect, useState } from 'react'
import { messageForError } from './rpc'

export function useLoad<T>(load: (signal: AbortSignal) => Promise<T>, dependencies: readonly unknown[]) {
  const [data, setData] = useState<T>()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const refresh = useCallback(async (signal = new AbortController().signal) => {
    setLoading(true)
    setError('')
    try { setData(await load(signal)) } catch (reason) {
      if (!(reason instanceof DOMException && reason.name === 'AbortError')) setError(messageForError(reason))
    } finally { if (!signal.aborted) setLoading(false) }
  // The caller supplies the precise request identity just as it would for an effect.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, dependencies)
  useEffect(() => {
    const controller = new AbortController()
    void refresh(controller.signal)
    return () => controller.abort()
  }, [refresh])
  return { data, error, loading, refresh: () => refresh() }
}
