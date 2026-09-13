import { useEffect, useState } from 'react'
import { useSession } from '../session'

export type RealtimeEvent = { type: string; sequence?: number; sender_id?: string; target_id?: string; message?: { server_id?: string } }

// Fetch is used instead of EventSource so the bearer token never appears in a
// URL. The endpoint is SSE, while this client keeps authentication in headers.
export function useEventStream(enabled: boolean) {
  const { identity } = useSession()
  const [event, setEvent] = useState<RealtimeEvent>()
  const [connected, setConnected] = useState(false)
  useEffect(() => {
    if (!enabled || !identity?.session_token) { setConnected(false); return }
    const controller = new AbortController()
    let stopped = false
    let cursor = 0
    const consume = async () => {
      while (!stopped) {
        try {
          const response = await fetch(cursor ? `/events?after=${cursor}` : '/events', { headers: { Authorization: `Bearer ${identity.session_token}` }, signal: controller.signal })
          if (!response.ok || !response.body) throw new Error('event stream unavailable')
          setConnected(true)
          const reader = response.body.getReader()
          const decoder = new TextDecoder()
          let buffer = ''
          while (!stopped) {
            const chunk = await reader.read()
            if (chunk.done) break
            buffer += decoder.decode(chunk.value, { stream: true })
            const frames = buffer.split('\n\n')
            buffer = frames.pop() || ''
            for (const frame of frames) {
              const data = frame.split('\n').find(line => line.startsWith('data:'))?.slice(5).trim()
              if (!data || data === '{}') continue
              try {
                const next = JSON.parse(data) as RealtimeEvent
                if (typeof next.sequence === 'number') cursor = Math.max(cursor, next.sequence)
                setEvent(next)
              } catch { /* Ignore malformed frames and keep the stream alive. */ }
            }
          }
        } catch (reason) {
          if (!stopped && !(reason instanceof DOMException && reason.name === 'AbortError')) setConnected(false)
        }
        if (!stopped) await new Promise(resolve => window.setTimeout(resolve, 800))
      }
    }
    void consume()
    return () => { stopped = true; controller.abort(); setConnected(false) }
  }, [enabled, identity?.session_token])
  return { event, connected }
}
