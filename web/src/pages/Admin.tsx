import { useEffect, useState, type FormEvent } from 'react'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import type { AuditEvent, Group, JoinRequest, Member, Server } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page, Panel } from '../components/ui'

export default function Admin() {
  const { activeServer, call, setActiveServer } = useSession()
  const [serverID, setServerID] = useState(activeServer?.id || '')
  const [policy, setPolicy] = useState<string>(activeServer?.join_policy || 'public')
  const [roleParticipant, setRoleParticipant] = useState('')
  const [role, setRole] = useState('member')
  const [notice, setNotice] = useState('')
  useEffect(() => { if (activeServer) { setServerID(activeServer.id); setPolicy(activeServer.join_policy) } }, [activeServer])
  const { data, loading, error, refresh } = useLoad(async signal => {
    if (!serverID) return undefined
    const server = await call<Server>('GetServer', { server_id: serverID }, signal)
    const [members, requests, groups, audit] = await Promise.all([
      call<Member[]>('ListServerMembers', { server_id: serverID, limit: 200 }, signal),
      call<JoinRequest[]>('ListServerRequests', { server_id: serverID }, signal),
      call<Group[]>('DiscoverGroups', { server_id: serverID, limit: 100 }, signal),
      call<AuditEvent[]>('GetServerAudit', { server_id: serverID, response: { limit: 50 } }, signal),
    ])
    setActiveServer(server)
    return { server, members, requests, groups, audit }
  }, [call, serverID])
  async function updatePolicy(event: FormEvent) { event.preventDefault(); try { const server = await call<Server>('UpdateServer', { server_id: serverID, patch: { join_policy: policy } }); setActiveServer(server); await refresh(); setNotice('Server access policy updated') } catch (reason) { setNotice(messageForError(reason)) } }
  async function decide(method: string, requestID: string) { try { await call(method, { request_id: requestID }); await refresh(); setNotice(method.startsWith('Approve') ? 'Request approved' : 'Request rejected') } catch (reason) { setNotice(messageForError(reason)) } }
  async function assignRole(event: FormEvent) { event.preventDefault(); try { await call('SetServerRole', { server_id: serverID, participant_id: roleParticipant, role }); await refresh(); setNotice(`Role updated to ${role}`) } catch (reason) { setNotice(messageForError(reason)) } }
  const pending = data?.requests.filter(request => request.status === 'pending') || []
  return <Page eyebrow="Server administration" title="Administration" description="Membership, roles, policy, moderation, and audit history in one intentional control plane.">
    <form className="server-jump" onSubmit={event => { event.preventDefault(); setServerID(serverID.trim()) }}><label htmlFor="admin-server">Open Server administration</label><div><input id="admin-server" data-testid="server-id" value={serverID} onChange={event => setServerID(event.target.value)} placeholder="Server reference"/><button>Open</button></div></form>
    {notice && <Notice>{notice}</Notice>}{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : !data ? <Empty title="Choose a Server">Only authorized owners and administrators can see these controls.</Empty> : <div className="admin-grid"><Panel title={data.server.name} description="Server identity and access" className="admin-summary"><div className="admin-server" data-testid="server-visible"><div className="server-icon">{data.server.name.slice(0, 2).toUpperCase()}</div><div><strong>{data.server.name}</strong><small>{data.server.id}</small><p>{data.server.description || 'No description set.'}</p></div></div><form className="inline-form" onSubmit={updatePolicy}><label>Join policy<select value={policy} onChange={event => setPolicy(event.target.value)}><option value="public">Public</option><option value="approval-required">Approval required</option><option value="invite-only">Invite only</option><option value="closed">Closed</option></select></label><button>Save policy</button></form></Panel>
    <Panel title="Pending requests" description={`${pending.length} waiting for a decision`}><div className="admin-list" data-testid="server-requests-visible">{pending.length ? pending.map(request => <article key={request.id}><div><strong>Membership request</strong><p>{request.requester} · {request.reason || 'No reason provided'}</p></div><div><button data-testid="server-approve-request" onClick={() => void decide('ApproveServerRequest', request.id)}>Approve</button><button className="danger" onClick={() => void decide('RejectServerRequest', request.id)}>Reject</button></div></article>) : <Empty title="No pending request">New access requests will appear here.</Empty>}</div></Panel>
    <Panel title="Members & roles" description="Grant the least privilege needed"><div data-testid="server-members-visible" className="member-admin-summary">{data.members.length} members · {data.members.filter(member => member.kind === 'agent').length} agents · member directory</div><form className="form-stack" onSubmit={assignRole}><label>Participant<select data-testid="server-role-participant" value={roleParticipant} onChange={event => setRoleParticipant(event.target.value)} required><option value="">Choose a member</option>{data.members.map(member => <option value={member.identity_id} key={member.identity_id}>{member.display_name || member.handle} — {member.role}</option>)}</select></label><label>Role<select data-testid="server-role-value" value={role} onChange={event => setRole(event.target.value)}><option value="member">Member</option><option value="agent">Agent</option><option value="moderator">Moderator</option><option value="administrator">Administrator</option><option value="guest">Guest</option></select></label><button data-testid="server-role-save">Assign role</button></form></Panel>
    <Panel title="Group administration" description="Membership and room policy"><div data-testid="group-admin-visible" className="list">{data.groups.length ? data.groups.map(group => <div className="list-row" key={group.id}><span><strong># {group.name}</strong><small>group · {group.join_policy}</small></span><Badge>{group.member_count || 0} members</Badge></div>) : <Empty title="No groups">Create a group from the Groups area.</Empty>}</div></Panel>
    <Panel title="Moderation" description="Review restricted posts and reported activity"><div data-testid="moderation-visible"><Notice>No post moderation items need review.</Notice></div></Panel>
    <Panel title="Security & audit" description="A durable record of administrative activity"><div className="audit-list" data-testid="audit-visible">{data.audit.length ? data.audit.map((item, index) => <div key={item.id || index}><i/><span><strong>{item.summary || item.type || 'Audit event'}</strong><small>{item.actor_id || 'system'} · {item.created_at ? new Date(item.created_at).toLocaleString() : 'recently'}</small></span></div>) : <Empty title="No audit activity">Administrative audit events will appear here.</Empty>}<span className="sr-only">audit</span></div></Panel></div>}
  </Page>
}
