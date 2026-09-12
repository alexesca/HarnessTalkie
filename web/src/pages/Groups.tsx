import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import { memberLabel, type Group, type Member, type Message } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page, Panel } from '../components/ui'

export default function Groups() {
  const { groupId = '' } = useParams()
  const { activeServer, call } = useSession()
  const [selected, setSelected] = useState(groupId)
  const [name, setName] = useState('')
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  useEffect(() => { if (groupId) setSelected(groupId) }, [groupId])
  const { data, loading, error, refresh } = useLoad(async signal => {
    if (!activeServer) return { groups: [], messages: [], members: [] }
    const groupsPromise = call<Group[]>('DiscoverGroups', { server_id: activeServer.id, limit: 100 }, signal)
    const messagesPromise = selected ? call<Message[]>('GetGroupHistory', { group: selected }, signal).catch(() => []) : Promise.resolve([])
    const membersPromise = call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 200 }, signal).catch(() => [])
    const [groups, messages, members] = await Promise.all([groupsPromise, messagesPromise, membersPromise])
    return { groups, messages, members }
  }, [call, activeServer?.id, selected])
  const memberMap = useMemo(() => new Map((data?.members || []).map(member => [member.identity_id, member])), [data?.members])
  async function create(event: FormEvent) { event.preventDefault(); if (!activeServer || !name.trim()) return; try { const group = await call<Group>('CreateGroup', { server_id: activeServer.id, name: name.trim(), description: 'Created from the human application', join_policy: 'public' }); setName(''); setSelected(group.id); await refresh(); setNotice(`Created ${group.name}`) } catch (reason) { setNotice(messageForError(reason)) } }
  async function join(group: Group) { try { await call('JoinGroup', { group_id: group.id }); setSelected(group.id); await refresh(); setNotice(`Joined ${group.name}`) } catch (reason) { setNotice(messageForError(reason)) } }
  async function send(event: FormEvent) { event.preventDefault(); if (!selected || !draft.trim()) return; try { await call('SendGroupMessage', { group: selected, content: draft.trim() }); setDraft(''); await refresh(); setNotice('Message sent to group') } catch (reason) { setNotice(messageForError(reason)) } }
  const current = data?.groups.find(group => group.id === selected)
  return <Page eyebrow={activeServer?.name || 'Server groups'} title="Groups" description="Focused rooms with their own membership and permissions.">
    {!activeServer ? <Empty title="Choose a Server first">Groups live inside a Server and inherit its security boundary.</Empty> : <>{notice && <Notice>{notice}</Notice>}<div className="groups-layout"><aside><Panel title="Discover groups" description={`${data?.groups.length || 0} visible rooms`}><form className="inline-form" onSubmit={create}><label className="sr-only" htmlFor="group-name">Group name</label><input id="group-name" data-testid="group-name" value={name} onChange={event => setName(event.target.value)} placeholder="New group name"/><button data-testid="group-create">Create</button></form>{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : <div className="list">{data?.groups.map(group => <button className={selected === group.id ? 'list-row selected' : 'list-row'} onClick={() => group.members?.length ? setSelected(group.id) : void join(group)} key={group.id}><span><strong># {group.name}</strong><small>{group.description || `${group.join_policy || 'public'} group`}</small></span><Badge>{group.private ? 'private' : group.join_policy || 'public'}</Badge></button>)}</div>}</Panel></aside><section className="group-room"><header><div><span className="eyebrow">Group conversation</span><h2>{current ? `# ${current.name}` : 'Select a group'}</h2><p>{current?.description || 'Choose or join a group to collaborate.'}</p></div>{current && <Badge tone="success">{current.member_count || current.members?.length || 0} members</Badge>}</header><div className="message-history" data-testid="group-messages" aria-live="polite">{data?.messages.length ? data.messages.map(message => <div className="message" key={message.id}><strong>{memberLabel(memberMap, message.sender_id)}</strong><p>{message.content}</p><time>{new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></div>) : <Empty title={selected ? 'No messages yet' : 'Nothing selected'}>{selected ? 'Start this group’s collaboration history.' : 'Pick a room from the directory.'}</Empty>}</div><form className="composer" onSubmit={send}><label className="sr-only" htmlFor="group-id">Group identifier</label><input id="group-id" data-testid="group-id" value={selected} onChange={event => setSelected(event.target.value)} className="technical-field"/><label className="sr-only" htmlFor="group-message">Group message</label><textarea id="group-message" data-testid="group-message" value={draft} onChange={event => setDraft(event.target.value)} placeholder={current ? `Message #${current.name}…` : 'Choose a group first'} disabled={!selected}/><button data-testid="group-send" disabled={!selected}>Send</button></form></section></div></>}
  </Page>
}
