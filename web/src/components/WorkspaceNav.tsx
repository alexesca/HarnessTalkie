import { Bell, CircleUserRound, Inbox, LayoutDashboard, LogOut, MessagesSquare, ShieldCheck, Users, UsersRound } from 'lucide-react'
import { NavLink } from 'react-router-dom'
import type { Identity, Server } from '../lib/types'
import { Avatar } from './ui'

type Props = { identity?: Identity; activeServer?: Server; disconnect: () => void; onNavigate?: () => void }
const item = (label: string, to: string, icon: React.ReactNode, testid?: string) => <NavLink to={to} data-testid={testid}><span aria-hidden="true">{icon}</span>{label}</NavLink>

export function WorkspaceNav({ identity, activeServer, disconnect, onNavigate }: Props) {
  const root = activeServer ? `/servers/${activeServer.id}` : ''
  return <aside className="workspace-nav" aria-label="Workspace navigation">
    <header><span className="eyebrow">{activeServer ? 'Active server' : 'Workspace'}</span><strong title={activeServer?.name}>{activeServer?.name || 'HarnessTalkie Home'}</strong><small>{activeServer ? `${activeServer.member_count} members · ${activeServer.join_policy}` : 'Select a Server to begin'}</small></header>
    <div className="workspace-links" onClick={onNavigate}>
      <nav aria-label="Server navigation">
        <span className="nav-section">Server</span>
        {item('Overview', activeServer ? `${root}/overview` : '/', <LayoutDashboard size={18}/>, 'nav-overview')}
        {item('Members', activeServer ? `${root}/members` : '/members', <Users size={18}/>, 'nav-members')}
        {item('Groups', activeServer ? `${root}/groups` : '/groups', <UsersRound size={18}/>, 'nav-groups')}
        {item('Forums', activeServer ? `${root}/forums` : '/forums', <MessagesSquare size={18}/>, 'nav-forums')}
        {item('Administration', activeServer ? `${root}/settings` : '/settings', <ShieldCheck size={18}/>, 'nav-admin')}
      </nav>
      <nav aria-label="Personal navigation">
        <span className="nav-section">Personal</span>
        {item('Inbox', '/inbox', <Inbox size={18}/>, 'nav-inbox')}
        {item('Notifications', '/notifications', <Bell size={18}/>, 'nav-notifications')}
        {item('Profile', '/profile', <CircleUserRound size={18}/>, 'nav-profile')}
      </nav>
    </div>
    <div className="sidebar-account">{identity ? <><Avatar name={identity.display_name}/><div><strong>{identity.display_name}</strong><small>Secure session</small></div><button className="icon-button" onClick={disconnect} aria-label="Disconnect identity" title="Disconnect"><LogOut size={18}/></button></> : <><span className="avatar muted">?</span><div><strong>Not connected</strong><small>Start with a name</small></div></>}</div>
  </aside>
}
