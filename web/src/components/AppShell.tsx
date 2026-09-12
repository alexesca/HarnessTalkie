import { Suspense, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Menu, X } from 'lucide-react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import type { Server } from '../lib/types'
import { Loading } from './ui'
import { ServerRail } from './ServerRail'
import { WorkspaceNav } from './WorkspaceNav'
import { AuthScreen, FirstServerWizard } from './Onboarding'

export function AppShell() {
  const { identity, activeServer, servers, serversLoading, serversError, disconnect, setActiveServer, refreshServers, call } = useSession()
  const [routeError, setRouteError] = useState('')
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
  if (!identity) return <div className="app-shell unauthenticated"><a className="skip-link" href="#main-content">Skip to content</a><AuthScreen onSuccess={() => navigate('/')}/></div>
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
      <header className="topbar"><button ref={menuButton} className="menu-button" onClick={() => setOpen(true)} aria-label="Open navigation" aria-expanded={open}><Menu size={22}/></button><div className="connection" data-testid="identity-status"><i className="connected"/>Connected as {identity.display_name}</div><span className="topbar-reference" title={identity.id}>{identity.reference || identity.id}</span></header>
      <main id="main-content">{routeError ? <div className="route-loading"><div className="notice danger" role="alert">{routeError}</div></div> : routeSyncing ? <div className="route-loading"><Loading label="Opening Server"/></div> : serversLoading && location.pathname === '/' ? <div className="route-loading"><Loading label="Loading your Servers"/></div> : !servers.length && location.pathname === '/' ? <FirstServerWizard/> : <Suspense fallback={<div className="route-loading"><h1 className="sr-only">Loading HarnessTalkie</h1><Loading/></div>}><Outlet/></Suspense>}</main>
    </div>
  </div>
}
