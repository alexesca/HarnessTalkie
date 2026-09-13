import { useDeferredValue, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useSession } from '../session'
import { useLoad } from '../lib/use-load'
import { useEventStream } from '../lib/use-event-stream'
import type { Member } from '../lib/types'
import { Avatar, Badge, Empty, Loading, Notice, Page } from '../components/ui'

export default function Members() {
  const { activeServer, call } = useSession()
  const [query, setQuery] = useState('')
  const deferred = useDeferredValue(query.toLowerCase())
  const { data = [], loading, error, refresh } = useLoad(signal => activeServer ? call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 200 }, signal) : Promise.resolve([]), [call, activeServer?.id])
  const { event } = useEventStream(Boolean(activeServer))
  useEffect(() => { if (event && event.type !== 'heartbeat') void refresh() }, [event, refresh])
  const members = useMemo(() => !deferred ? data : data.filter(member => [member.display_name, member.handle, member.kind, member.harness, member.current_work, ...(member.capabilities || [])].join(' ').toLowerCase().includes(deferred)), [data, deferred])
  return <Page eyebrow={activeServer?.name || 'Directory'} title="Members & agents" description="Find the right collaborator by capability, current work, or role.">
    {!activeServer ? <Empty title="Choose a Server first">The directory only exposes participants you are authorized to discover.</Empty> : <><div className="directory-toolbar"><label className="search"><span className="sr-only">Search members and agents</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search skills, names, work, or harness…"/></label><span>{members.length} visible collaborators</span></div>{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : <div className="member-grid" data-testid="server-members-visible">{members.length ? members.map(member => <article className="member-card" key={member.identity_id}><div className="member-head"><Avatar name={member.display_name || member.handle || '?'} kind={member.kind} online={member.online}/><div><h2>{member.display_name || member.handle || 'Collaborator'}</h2><p>@{member.handle || 'participant'} · {member.online ? 'Online' : 'Away'}</p></div><Badge tone={member.kind === 'agent' ? 'agent' : 'neutral'}>{member.kind || 'human'}</Badge></div><p className="member-bio">{member.bio || member.current_work || 'Ready to collaborate.'}</p><div className="tag-row">{(member.capabilities || []).slice(0, 4).map(capability => <span key={capability}>{capability}</span>)}</div><dl><div><dt>Role</dt><dd>{member.role}</dd></div>{member.harness && <div><dt>Harness</dt><dd>{member.harness}</dd></div>}{member.current_work && <div><dt>Current work</dt><dd>{member.current_work}</dd></div>}</dl><Link className="button secondary full" to={`/dm/${member.identity_id}`}>Message {member.display_name || member.handle}</Link></article>) : <Empty title="No collaborators match">Try a broader skill or name.</Empty>}</div>}</>}
  </Page>
}
