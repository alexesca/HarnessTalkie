import { useEffect, useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import type { Member, Message } from '../lib/types'
import { Avatar, Empty, Loading, Notice, Page } from '../components/ui'

export default function Inbox() {
  const { participantId = '' } = useParams()
  const { identity, activeServer, call } = useSession()
  const [selected, setSelected] = useState(participantId)
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  useEffect(() => { if (participantId) setSelected(participantId) }, [participantId])
  const { data, loading, error, refresh } = useLoad(async signal => {
    if (!identity) return { members: [], messages: [] }
    const membersPromise = activeServer ? call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 100 }, signal) : Promise.resolve([])
    const messagesPromise = selected ? call<Message[]>('GetDMHistory', { server_id: activeServer?.id, with: selected }, signal) : Promise.resolve([])
    const [members, messages] = await Promise.all([membersPromise, messagesPromise])
    return { members: members.filter(member => member.identity_id !== identity.id), messages }
  }, [call, identity?.id, activeServer?.id, selected])
  async function send(event: FormEvent) {
    event.preventDefault(); if (!selected || !draft.trim()) return
    setNotice('')
    try { await call('SendDM', { server_id: activeServer?.id, to: selected, content: draft.trim() }); setDraft(''); await refresh(); setNotice('Message delivered') } catch (reason) { setNotice(messageForError(reason)) }
  }
  const peer = data?.members.find(member => member.identity_id === selected)
  return <Page eyebrow="Private conversations" title="Inbox" description="Durable direct messages that resume after a disconnect.">
    {!identity ? <Empty title="Connect to open your inbox">Your session token remains in this browser tab only.</Empty> : loading && !data ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : <div className="inbox-layout"><aside className="conversation-list" aria-label="Conversations"><h2>Collaborators</h2>{data?.members.map(member => <button className={selected === member.identity_id ? 'selected' : ''} onClick={() => setSelected(member.identity_id)} key={member.identity_id}><Avatar name={member.display_name || member.handle || '?'} kind={member.kind} online={member.online}/><span><strong>{member.display_name || member.handle}</strong><small>{member.kind === 'agent' ? member.harness || 'Agent' : member.role}</small></span></button>)}{!data?.members.length && <p className="muted-copy">Join a Server to find collaborators.</p>}</aside><section className="message-pane">{selected ? <><header><Avatar name={peer?.display_name || peer?.handle || 'C'} kind={peer?.kind} online={peer?.online}/><div><h2>{peer?.display_name || peer?.handle || 'Direct message'}</h2><p>{peer?.online ? 'Online now' : 'Messages will be delivered when they return'}</p></div></header><div className="message-history" data-testid="dm-messages" aria-live="polite">{data?.messages.length ? data.messages.map(message => <div className={message.sender_id === identity.id ? 'message own' : 'message'} key={message.id}><strong>{message.sender_id === identity.id ? 'You' : peer?.display_name || 'Collaborator'}</strong><p>{message.content}</p><time>{new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></div>) : <Empty title="Start the conversation">Share context, ask a question, or say hello.</Empty>}</div><form className="composer" onSubmit={send}><label className="sr-only" htmlFor="dm-recipient">Recipient</label><input id="dm-recipient" data-testid="dm-recipient" value={selected} onChange={event => setSelected(event.target.value)} className="technical-field" aria-describedby="recipient-help"/><span id="recipient-help" className="sr-only">Selected participant identifier</span><label className="sr-only" htmlFor="dm-content">Message</label><textarea id="dm-content" data-testid="dm-content" value={draft} onChange={event => setDraft(event.target.value)} placeholder={`Message ${peer?.display_name || 'collaborator'}…`} onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }}/><button data-testid="dm-send">Send</button></form>{notice && <span className="composer-status" role="status">{notice}</span>}</> : <Empty title="Choose a collaborator">Select a person or agent to open a private conversation.</Empty>}</section></div>}
  </Page>
}
