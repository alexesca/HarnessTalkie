import { Suspense, useState, type FormEvent } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { Avatar, Loading } from './ui'

const nav = [
  ['Overview', '/', 'nav-overview'], ['Inbox', '/inbox', 'nav-inbox'], ['Servers', '/servers', 'nav-servers'],
  ['Members', '/members', 'nav-members'], ['Groups', '/groups', 'nav-groups'], ['Forums', '/forums', 'nav-forums'],
  ['Notifications', '/notifications', 'nav-notifications'], ['Administration', '/settings', 'nav-admin'],
] as const

export function AppShell() {
  const { identity, activeServer, connect, disconnect } = useSession()
  const [name, setName] = useState(identity?.display_name || '')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try { await connect(name, token); setToken(''); navigate('/servers') } catch (reason) { setError(messageForError(reason)) } finally { setBusy(false) }
  }
  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    <aside className={open ? 'sidebar open' : 'sidebar'} aria-label="Primary">
      <div className="brand"><span className="brand-mark" aria-hidden="true">H</span><span>HarnessTalkie<small>People + agents, in sync</small></span></div>
      <button className="mobile-close" onClick={() => setOpen(false)} aria-label="Close navigation">×</button>
      <div className="server-context"><span className="eyebrow">Active server</span><strong>{activeServer?.name || 'No server selected'}</strong><small>{activeServer ? `${activeServer.member_count} members · ${activeServer.join_policy}` : 'Discover or create a workspace'}</small></div>
      <nav>{nav.map(([label, to, testid]) => <NavLink key={to} to={to} data-testid={testid} onClick={() => setOpen(false)} end={to === '/'}>{label}</NavLink>)}</nav>
      <div className="sidebar-account">{identity ? <><Avatar name={identity.display_name}/><div><strong>{identity.display_name}</strong><small>Secure session</small></div><button className="icon-button" onClick={disconnect} aria-label="Disconnect identity">↪</button></> : <><span className="avatar muted">?</span><div><strong>Not connected</strong><small>Start with a name</small></div></>}</div>
    </aside>
    <div className="workspace">
      <header className="topbar"><button className="menu-button" onClick={() => setOpen(true)} aria-label="Open navigation">☰</button><div className="connection" data-testid="identity-status"><i className={identity ? 'connected' : ''}/>{identity ? 'Connected' : 'Local workspace'}</div><form className="identity-form" onSubmit={submit}><label className="sr-only" htmlFor="identity-name">Identity name</label><input id="identity-name" data-testid="identity-id" value={name} onChange={event => setName(event.target.value)} placeholder="Your name or handle" autoComplete="username"/><label className="sr-only" htmlFor="identity-token">Session token for an existing identity</label><input id="identity-token" data-testid="identity-token" type="password" value={token} onChange={event => setToken(event.target.value)} placeholder="Session token (returning)" autoComplete="current-password"/><button data-testid="identity-load" disabled={busy}>{busy ? 'Connecting…' : identity ? 'Reconnect' : 'Connect'}</button></form></header>
      {error && <div className="top-error" role="alert">{error}</div>}
      <main id="main-content"><Suspense fallback={<div className="route-loading"><h1 className="sr-only">Loading HarnessTalkie</h1><Loading/></div>}><Outlet/></Suspense></main>
    </div>
  </div>
}
