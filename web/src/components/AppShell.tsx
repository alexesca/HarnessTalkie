import { Suspense, useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { Menu, X } from 'lucide-react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import type { Server } from '../lib/types'
import { Loading } from './ui'
import { ServerRail } from './ServerRail'
import { WorkspaceNav } from './WorkspaceNav'

export function AppShell() {
  const { identity, activeServer, servers, serversLoading, serversError, connect, disconnect, setActiveServer, refreshServers, call } = useSession()
  const [name, setName] = useState(identity?.display_name || '')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [routeError, setRouteError] = useState('')
  const [busy, setBusy] = useState(false)
  const [open, setOpen] = useState(false)
  const drawer = useRef<HTMLDivElement>(null)
  const menuButton = useRef<HTMLButtonElement>(null)
  const navigate = useNavigate()
  const location = useLocation()
  const routeServerID = location.pathname.match(/^\/servers\/([^/]+)\//)?.[1]
  const routeSyncing = Boolean(identity && routeServerID && routeServerID !== activeServer?.id && !routeError)

  useEffect(() => { setOpen(false) }, [location.pathname, location.search])
  useEffect(() => {
    if (!identity || !routeServerID || routeServerID === activeServer?.id) { setRouteError(''); return }
    const controller = new AbortController(); setRouteError('')
    call<Server>('GetServer', { server_id: routeServerID }, controller.signal).then(setActiveServer).catch(reason => {
      if (!(reason instanceof DOMException && reason.name === 'AbortError')) setRouteError(messageForError(reason))
    })
    return () => controller.abort()
  }, [identity, routeServerID, activeServer?.id, call, setActiveServer])
  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    requestAnimationFrame(() => drawer.current?.querySelector<HTMLElement>('button, a')?.focus())
    return () => { (menuButton.current || previous)?.focus() }
  }, [open])

  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try { const next = await connect(name, token); setName(next.display_name); setToken(''); navigate('/') } catch (reason) { setError(messageForError(reason)) } finally { setBusy(false) }
  }
  function handleDrawerKey(event: KeyboardEvent) {
    if (event.key === 'Escape') { event.preventDefault(); setOpen(false); return }
    if (event.key !== 'Tab' || !drawer.current) return
    const focusable = [...drawer.current.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled])')]
    if (!focusable.length) return
    const first = focusable[0], last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
  }
  const close = () => setOpen(false)
  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    {open && <button className="nav-backdrop" aria-label="Close navigation" onClick={close}/>}
    <div ref={drawer} className={`navigation-shell${open ? ' open' : ''}`} onKeyDown={handleDrawerKey}>
      <ServerRail servers={servers} activeServer={activeServer} onSelect={setActiveServer} onNavigate={close}/>
      <WorkspaceNav identity={identity} activeServer={activeServer} disconnect={() => { disconnect(); navigate('/') }} onNavigate={close}/>
      <button className="mobile-close" onClick={close} aria-label="Close navigation"><X size={22}/></button>
      {identity && serversLoading && <span className="rail-status" role="status">Loading Servers…</span>}
      {identity && serversError && <button className="rail-retry" onClick={() => void refreshServers()}>Retry Servers</button>}
    </div>
    <div className="workspace">
      <header className="topbar"><button ref={menuButton} className="menu-button" onClick={() => setOpen(true)} aria-label="Open navigation" aria-expanded={open}><Menu size={22}/></button><div className="connection" data-testid="identity-status"><i className={identity ? 'connected' : ''}/>{identity ? 'Connected' : 'Local workspace'}</div><form className="identity-form" onSubmit={submit}><label className="sr-only" htmlFor="identity-name">Identity name</label><input id="identity-name" data-testid="identity-id" value={name} onChange={event => setName(event.target.value)} placeholder="Your name or handle" autoComplete="username"/><label className="sr-only" htmlFor="identity-token">Session token for an existing identity</label><input id="identity-token" data-testid="identity-token" type="password" value={token} onChange={event => setToken(event.target.value)} placeholder="Session token (returning)" autoComplete="current-password"/><button data-testid="identity-load" disabled={busy}>{busy ? 'Connecting…' : identity ? 'Reconnect' : 'Connect'}</button></form></header>
      {error && <div className="top-error" role="alert">{error}</div>}
      <main id="main-content">{routeError ? <div className="route-loading"><div className="notice danger" role="alert">{routeError}</div></div> : routeSyncing ? <div className="route-loading"><Loading label="Opening Server"/></div> : <Suspense fallback={<div className="route-loading"><h1 className="sr-only">Loading HarnessTalkie</h1><Loading/></div>}><Outlet/></Suspense>}</main>
    </div>
  </div>
}
