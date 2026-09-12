import { createContext, useCallback, useContext, useMemo, useState, type PropsWithChildren } from 'react'
import { rpc } from './lib/rpc'
import type { Identity, Server } from './lib/types'

type Session = {
  identity?: Identity
  activeServer?: Server
  connect: (name: string) => Promise<Identity>
  setActiveServer: (server?: Server) => void
  call: <T>(method: string, params?: unknown, signal?: AbortSignal) => Promise<T>
  disconnect: () => void
}

const SessionContext = createContext<Session | null>(null)
const storedIdentity = () => {
  try { return JSON.parse(sessionStorage.getItem('ht.session.v2') || 'null') as Identity | null } catch { return null }
}
const storedServer = () => {
  try { return JSON.parse(localStorage.getItem('ht.active-server.v2') || 'null') as Server | null } catch { return null }
}

export function SessionProvider({ children }: PropsWithChildren) {
  const [identity, setIdentity] = useState<Identity | undefined>(() => storedIdentity() || undefined)
  const [activeServer, updateActiveServer] = useState<Server | undefined>(() => storedServer() || undefined)
  const connect = useCallback(async (name: string) => {
    const next = await rpc<Identity>('CreateOrLoadIdentity', { identity: name.trim() || 'human' }, identity?.session_token)
    sessionStorage.setItem('ht.session.v2', JSON.stringify(next))
    setIdentity(next)
    return next
  }, [identity?.session_token])
  const disconnect = useCallback(() => { sessionStorage.removeItem('ht.session.v2'); setIdentity(undefined) }, [])
  const setActiveServer = useCallback((server?: Server) => {
    updateActiveServer(server)
    if (server) localStorage.setItem('ht.active-server.v2', JSON.stringify(server))
    else localStorage.removeItem('ht.active-server.v2')
  }, [])
  const call = useCallback(<T,>(method: string, params: unknown = {}, signal?: AbortSignal) => rpc<T>(method, params, identity?.session_token, signal), [identity?.session_token])
  const value = useMemo(() => ({ identity, activeServer, connect, setActiveServer, call, disconnect }), [identity, activeServer, connect, setActiveServer, call, disconnect])
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession() {
  const value = useContext(SessionContext)
  if (!value) throw new Error('SessionProvider is missing')
  return value
}
