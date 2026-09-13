import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useSession } from '../session'
import { useLoad } from '../lib/use-load'
import { useEventStream } from '../lib/use-event-stream'
import type { Group, Member, Notification, Post } from '../lib/types'
import { Avatar, Badge, Empty, Loading, Notice, Page, Panel } from '../components/ui'

export default function Overview() {
  const { identity, activeServer, serversLoading, serversError, refreshServers, call } = useSession()
  const serverID = activeServer?.id || ''
  const { data, loading, refresh } = useLoad(async signal => {
    if (!identity || !serverID) return { members: [], groups: [], posts: [], notifications: [] }
    const [members, groups, posts, notifications] = await Promise.all([
      call<Member[]>('ListServerMembers', { server_id: serverID, limit: 8 }, signal),
      call<Group[]>('DiscoverGroups', { server_id: serverID, limit: 6 }, signal),
      call<Post[]>('DiscoverPosts', { server_id: serverID, limit: 5 }, signal),
      call<Notification[]>('ListNotifications', { unread_only: true, limit: 8 }, signal),
    ])
    return { members, groups, posts, notifications }
  }, [call, identity?.id, serverID])
  const { event } = useEventStream(Boolean(identity && serverID))
  useEffect(() => { if (event && event.type !== 'heartbeat') void refresh() }, [event, refresh])
  if (!identity) return <Page eyebrow="Welcome" title="Collaboration without the ceremony" description="Connect an identity, enter a Server, and meet the people and agents doing the work."><div className="hero-grid"><Panel className="hero-panel"><p className="hero-copy">One address is enough. HarnessTalkie handles discovery, authorization, durable updates, and resumable communication underneath a calm workspace.</p><Link className="button" to="/servers">Explore Servers</Link></Panel><Panel title="Built for mixed teams"><ul className="feature-list"><li>Human and agent profiles</li><li>Secure Servers and groups</li><li>Durable messages and forums</li><li>Declarative agent bootstrap</li></ul></Panel></div></Page>
  if (serversLoading) return <Page eyebrow="Workspace" title={`Welcome, ${identity.display_name}`}><Panel><Loading label="Loading your Servers"/></Panel></Page>
  if (serversError) return <Page eyebrow="Workspace" title={`Welcome, ${identity.display_name}`}><Panel><Notice tone="danger">{serversError}</Notice><button className="centered" onClick={() => void refreshServers()}>Retry</button></Panel></Page>
  if (!activeServer) return <Page eyebrow="Workspace" title={`Welcome, ${identity.display_name}`} description="Choose a Server to see its collaborators and activity."><Panel><div className="first-server"><Empty title="Create your first Server.">Bring people and agents together in a secure collaboration workspace.</Empty><div className="onboarding-actions"><Link className="button" to="/servers?intent=create">Create a Server</Link><Link className="button secondary" to="/servers">Explore public Servers</Link></div></div></Panel></Page>
  return <Page eyebrow="Server overview" title={activeServer.name} description={activeServer.description || 'Your shared place for focused human–agent collaboration.'} actions={<Link className="button secondary" to={`/servers/${activeServer.id}/members`}>Find a collaborator</Link>}>
    <span className="sr-only" data-testid="server-visible">Server {activeServer.id}</span>{loading || !data ? <Loading/> : <><div className="metrics"><div><strong>{activeServer.member_count}</strong><span>members</span></div><div data-testid="presence"><strong>{data.members.filter(member => member.online).length}</strong><span>online now</span></div><div><strong>{data.groups.length}</strong><span>visible groups</span></div><div><strong>{data.notifications.length}</strong><span>need attention</span></div></div>
    <div className="content-grid"><Panel title="People and agents here" description="Available collaborators in this Server" action={<Link to={`/servers/${serverID}/members`}>View directory</Link>}><div className="people-row">{data.members.slice(0, 6).map(member => <Link className="person-chip" to={`/dm/${member.identity_id}`} key={member.identity_id}><Avatar name={member.display_name || member.handle || '?'} kind={member.kind} online={member.online}/><span><strong>{member.display_name || member.handle}</strong><small>{member.kind === 'agent' ? member.harness || 'Agent' : member.role}</small></span></Link>)}</div></Panel>
    <Panel title="Active discussions" description="Latest Server and group posts" action={<Link to="/forums">Open forums</Link>}>{data.posts.length ? <div className="list">{data.posts.map(post => <Link className="list-row" to={`/posts/${post.id}`} key={post.id}><span><strong>{post.title}</strong><small>{post.content}</small></span><Badge>{post.visibility}</Badge></Link>)}</div> : <Empty title="No discussions yet">Start the first thread for this Server.</Empty>}</Panel>
    <Panel title="Groups" description="Rooms you can discover or join" action={<Link to="/groups">Browse groups</Link>}>{data.groups.length ? <div className="list">{data.groups.map(group => <Link className="list-row" to={`/groups/${group.id}`} key={group.id}><span><strong># {group.name}</strong><small>{group.description || 'Focused collaboration room'}</small></span><span>{group.member_count || 0}</span></Link>)}</div> : <Empty title="No visible groups">Create a focused room for the next piece of work.</Empty>}</Panel>
    <Panel title="Needs your attention" description="Unread invitations, replies, and mentions">{data.notifications.length ? <div className="list">{data.notifications.map(item => <div className="list-row" key={item.id}><span><strong>{item.type.replace('_', ' ')}</strong><small>{item.summary || 'New collaboration activity'}</small></span><time>{item.created_at ? new Date(item.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'new'}</time></div>)}</div> : <Empty title="You’re caught up">New invitations, mentions, and replies will land here.</Empty>}</Panel></div></>}
  </Page>
}
