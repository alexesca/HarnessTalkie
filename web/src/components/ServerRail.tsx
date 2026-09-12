import { Compass, Home, Plus } from 'lucide-react'
import { NavLink, useNavigate } from 'react-router-dom'
import type { Server } from '../lib/types'

const colors = ['#28534b', '#344c70', '#5b3e62', '#66502f', '#365b68', '#583f42']

export function serverInitials(name: string) {
  const words = name.trim().split(/\s+/).filter(Boolean)
  return (words.length > 1 ? `${words[0][0]}${words[1][0]}` : words[0]?.slice(0, 2) || '?').toUpperCase()
}

function colorFor(id: string) {
  const hash = [...id].reduce((value, character) => ((value * 31) + character.charCodeAt(0)) >>> 0, 0)
  return colors[hash % colors.length]
}

type Props = { servers: Server[]; activeServer?: Server; onSelect: (server: Server) => void; onNavigate?: () => void }

export function ServerRail({ servers, activeServer, onSelect, onNavigate }: Props) {
  const navigate = useNavigate()
  function select(server: Server) { onSelect(server); navigate(`/servers/${server.id}/overview`); onNavigate?.() }
  return <aside className="server-rail" aria-label="Servers">
    <NavLink className="rail-action rail-home" to="/" end onClick={onNavigate} aria-label="HarnessTalkie Home"><span className="rail-tile"><Home size={23}/></span><span>Home</span></NavLink>
    <div className="rail-divider"/>
    <div className="joined-servers" data-testid="joined-servers">
      {servers.map(server => <button key={server.id} type="button" className={`server-rail-item${activeServer?.id === server.id ? ' active' : ''}`} onClick={() => select(server)} aria-label={server.name} title={server.name} aria-current={activeServer?.id === server.id ? 'page' : undefined}>
        <span className="rail-tile server-tile" style={{ backgroundColor: colorFor(server.id) }}>{serverInitials(server.name)}</span><span className="rail-label">{server.name}</span>
      </button>)}
    </div>
    <div className="rail-divider"/>
    <NavLink className="rail-action" to="/servers" onClick={onNavigate} aria-label="Explore Servers"><span className="rail-tile"><Compass size={22}/></span><span>Explore</span></NavLink>
    <NavLink className="rail-action" to="/servers?intent=create" onClick={onNavigate} aria-label="Create Server"><span className="rail-tile"><Plus size={24}/></span><span>Create</span></NavLink>
  </aside>
}
