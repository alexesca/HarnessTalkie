import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type PropsWithChildren } from 'react'
import { rpc } from './lib/rpc'
import type { Identity, Server } from './lib/types'

type Session = {
  identity?: Identity
  activeServer?: Server
  servers: Server[]
  serversLoading: boolean
  serversError: string
  connect: (name: string, token?: string) => Promise<Identity>
  setActiveServer: (server?: Server) => void
  refreshServers: () => Promise<Server[]>
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
  const [servers, setServers] = useState<Server[]>([])
  const [serversLoading, setServersLoading] = useState(() => Boolean(storedIdentity()))
  const [serversError, setServersError] = useState('')
  const loadedToken = useRef('')
  const setActiveServer = useCallback((server?: Server) => {
    updateActiveServer(server)
    if (server) localStorage.setItem('ht.active-server.v2', JSON.stringify(server))
    else localStorage.removeItem('ht.active-server.v2')
  }, [])
  const loadServers = useCallback(async (token?: string) => {
    if (!token) { setServers([]); setServersLoading(false); setServersError(''); setActiveServer(undefined); return [] }
    loadedToken.current = token
    setServersLoading(true); setServersError('')
    try {
      const next = await rpc<Server[]>('ListServers', { limit: 200 }, token) || []
      setServers(next)
      const saved = storedServer()
      const selected = next.find(server => server.id === saved?.id) || next[0]
      setActiveServer(selected)
      return next
    } catch (reason) {
      setServers([]); setActiveServer(undefined)
      setServersError(reason instanceof Error ? reason.message : 'Unable to load your Servers')
      throw reason
    } finally { setServersLoading(false) }
  }, [setActiveServer])
  const refreshServers = useCallback(() => loadServers(identity?.session_token), [identity?.session_token, loadServers])
  useEffect(() => { if (identity?.session_token && loadedToken.current !== identity.session_token) void loadServers(identity.session_token).catch(() => {}) }, [identity?.session_token, loadServers])
  const connect = useCallback(async (name: string, token = '') => {
    const next = await rpc<Identity>('CreateOrLoadIdentity', { identity: name.trim() || 'human' }, token.trim() || identity?.session_token)
    sessionStorage.setItem('ht.session.v2', JSON.stringify(next))
    setIdentity(next)
    await loadServers(next.session_token)
    return next
  }, [identity?.session_token, loadServers])
  const disconnect = useCallback(() => { sessionStorage.removeItem('ht.session.v2'); loadedToken.current = ''; setIdentity(undefined); setServers([]); setServersError(''); setServersLoading(false); setActiveServer(undefined) }, [setActiveServer])
  const call = useCallback(<T,>(method: string, params: unknown = {}, signal?: AbortSignal) => rpc<T>(method, params, identity?.session_token, signal), [identity?.session_token])
  const value = useMemo(() => ({ identity, activeServer, servers, serversLoading, serversError, connect, setActiveServer, refreshServers, call, disconnect }), [identity, activeServer, servers, serversLoading, serversError, connect, setActiveServer, refreshServers, call, disconnect])
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession() {
  const value = useContext(SessionContext)
  if (!value) throw new Error('SessionProvider is missing')
  return value
}
