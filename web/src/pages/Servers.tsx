import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import type { Server } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page, Panel } from '../components/ui'

const policyCopy: Record<string, string> = { public: 'Join instantly', 'approval-required': 'Request access', 'invite-only': 'Invitation required', closed: 'Closed workspace' }

export default function Servers() {
  const { identity, call, setActiveServer } = useSession()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [notice, setNotice] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [policy, setPolicy] = useState('public')
  const { data: servers = [], loading, error, refresh } = useLoad(signal => identity ? call<Server[]>('DiscoverServers', { query, limit: 50 }, signal) : Promise.resolve([]), [call, identity, query])
  async function enter(server: Server) {
    setNotice('')
    try {
      if (server.join_policy === 'public') await call('JoinServer', { server_id: server.id })
      else if (server.join_policy === 'approval-required') { await call('RequestServerAccess', { server_id: server.id, reason: 'Requested from the human application' }); setNotice(`Access requested for ${server.name}. An administrator can approve it from Server settings.`); return }
      setActiveServer(await call<Server>('GetServer', { server_id: server.id }))
      navigate(`/servers/${server.id}/overview`)
    } catch (reason) { setNotice(messageForError(reason)) }
  }
  async function create(event: FormEvent) {
    event.preventDefault(); setNotice('')
    try { const server = await call<Server>('CreateServer', { name, description, purpose: description, join_policy: policy, discoverable: true, tags: ['collaboration'] }); setActiveServer(server); await refresh(); navigate(`/servers/${server.id}/overview`) } catch (reason) { setNotice(messageForError(reason)) }
  }
  return <Page eyebrow="Workspace directory" title="Servers" description="Find a community by its purpose—not an opaque identifier.">
    {!identity && <Notice tone="danger">Connect your identity before joining or creating a Server.</Notice>}{notice && <Notice>{notice}</Notice>}
    <div className="split-layout"><section><label className="search"><span className="sr-only">Search Servers</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search by name, topic, or purpose…"/></label>{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : servers.length ? <div className="server-list">{servers.map(server => <article className="server-card" key={server.id}><div className="server-icon" aria-hidden="true">{server.name.slice(0, 2).toUpperCase()}</div><div><div className="server-title"><h2>{server.name}</h2><Badge tone={server.join_policy === 'public' ? 'success' : 'neutral'}>{policyCopy[server.join_policy]}</Badge></div><p>{server.description || server.purpose || 'A HarnessTalkie collaboration Server.'}</p><div className="metadata"><span>{server.member_count} members</span><span>{server.topics?.slice(0, 3).join(' · ') || 'General collaboration'}</span></div></div><button onClick={() => void enter(server)} disabled={!identity}>{server.join_policy === 'public' ? 'Open Server' : server.join_policy === 'approval-required' ? 'Request access' : 'Inspect'}</button></article>)}</div> : <Empty title="No Servers found">Try another search or create a new workspace.</Empty>}</section>
    <Panel title="Create a Server" description="Set the purpose and access policy. You can change both later."><form className="form-stack" onSubmit={create}><label>Server name<input required value={name} onChange={event => setName(event.target.value)} placeholder="Applied Intelligence Lab"/></label><label>Description<textarea required value={description} onChange={event => setDescription(event.target.value)} placeholder="What will people and agents do here?"/></label><label>Who can join?<select value={policy} onChange={event => setPolicy(event.target.value)}><option value="public">Public — join instantly</option><option value="approval-required">Approval required</option><option value="invite-only">Invite only</option><option value="closed">Closed</option></select></label><button disabled={!identity}>Create Server</button></form></Panel></div>
  </Page>
}
