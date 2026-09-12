import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import type { Member, Message, TypingIndicator } from '../lib/types'
import { Avatar, Empty, Loading, Notice, Page } from '../components/ui'

export default function Inbox() {
  const { participantId = '' } = useParams()
  const { identity, activeServer, call } = useSession()
  const [selected, setSelected] = useState(participantId)
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  const [typing, setTyping] = useState<TypingIndicator[]>([])
  const typingTimer = useRef<number | undefined>(undefined)
  useEffect(() => { if (participantId) setSelected(participantId) }, [participantId])
  const { data, loading, error, refresh } = useLoad(async signal => {
    if (!identity || !activeServer) return { members: [], messages: [], incoming: [] }
    const membersPromise = call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 100 }, signal)
    const incomingPromise = call<Message[]>('ReceiveDMs', { server_id: activeServer.id }, signal)
    const messagesPromise = selected ? call<Message[]>('GetDMHistory', { server_id: activeServer.id, with: selected }, signal) : Promise.resolve([])
    const [serverMembers, incoming, messages] = await Promise.all([membersPromise, incomingPromise, messagesPromise])
    return { members: (serverMembers || []).filter(member => member.identity_id !== identity.id), messages: messages || [], incoming: incoming || [] }
  }, [call, identity?.id, activeServer?.id, selected], 3000)
  useEffect(() => {
    if (!selected && data?.incoming.length) setSelected(data.incoming[data.incoming.length - 1].sender_id)
  }, [data?.incoming, selected])
  useEffect(() => {
    if (!activeServer || !selected) { setTyping([]); return }
    let cancelled = false
    const poll = async () => { try { const next = await call<TypingIndicator[]>('GetTyping', { server_id: activeServer.id, with: selected }); if (!cancelled) setTyping(next || []) } catch { if (!cancelled) setTyping([]) } }
    void poll(); const timer = window.setInterval(() => void poll(), 1000)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [call, activeServer?.id, selected])
  function signalTyping(value: string) {
    setDraft(value)
    if (!activeServer || !selected) return
    void call('SetTyping', { server_id: activeServer.id, with: selected, typing: Boolean(value.trim()) }).catch(() => {})
    if (typingTimer.current) window.clearTimeout(typingTimer.current)
    if (value.trim()) typingTimer.current = window.setTimeout(() => { void call('SetTyping', { server_id: activeServer.id, with: selected, typing: false }).catch(() => {}) }, 3500)
  }
  async function send(event: FormEvent) {
    event.preventDefault(); if (!selected || !draft.trim()) return
    setNotice('')
    try { await call('SetTyping', { server_id: activeServer?.id, with: selected, typing: false }).catch(() => {}); await call('SendDM', { server_id: activeServer?.id, to: selected, content: draft.trim() }); setDraft(''); await refresh(); setNotice('Message delivered') } catch (reason) { setNotice(messageForError(reason)) }
  }
  const peer = data?.members.find(member => member.identity_id === selected)
  return <Page eyebrow="Private conversations" title="Inbox" description="Durable direct messages that resume after a disconnect." actions={<button className="secondary" onClick={() => void refresh()}>Refresh inbox</button>}>
    {!identity ? <Empty title="Sign in to open your inbox">Your conversations become available after authentication.</Empty> : !activeServer ? <Empty title="Choose a Server first">Private conversations are available only between members of the same Server.</Empty> : loading && !data ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : <div className="inbox-layout"><aside className="conversation-list" aria-label="Conversations"><h2>{activeServer.name} members</h2>{data?.members.map(member => <button className={selected === member.identity_id ? 'selected' : ''} onClick={() => setSelected(member.identity_id)} key={member.identity_id}><Avatar name={member.display_name || member.handle || '?'} kind={member.kind} online={member.online}/><span><strong>{member.display_name || member.handle}</strong><small>{member.kind === 'agent' ? member.harness || 'Agent' : member.role}</small></span></button>)}{!data?.members.length && <p className="muted-copy">No other members have joined this Server yet.</p>}</aside><section className="message-pane">{selected ? <><header><Avatar name={peer?.display_name || peer?.handle || 'C'} kind={peer?.kind} online={peer?.online}/><div><h2>{peer?.display_name || peer?.handle || 'Direct message'}</h2><p>{peer?.online ? <><span className="live-dot">●</span> Online now</> : 'Messages will be delivered when they return'}</p></div></header><div className="message-history" data-testid="dm-messages" aria-live="polite">{data?.messages.length ? data.messages.map(message => <div className={message.sender_id === identity.id ? 'message own' : 'message'} key={message.id}><strong>{message.sender_id === identity.id ? 'You' : peer?.display_name || 'Collaborator'}</strong><p>{message.content}</p><time>{new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></div>) : <Empty title="Start the conversation">Share context, ask a question, or say hello.</Empty>}</div>{typing.length > 0 && <div className="typing-indicator" role="status">{peer?.display_name || peer?.handle || 'Collaborator'} is typing</div>}<form className="composer" onSubmit={send}><label className="sr-only" htmlFor="dm-recipient">Recipient</label><input id="dm-recipient" data-testid="dm-recipient" value={selected} onChange={event => setSelected(event.target.value)} className="technical-field" aria-describedby="recipient-help"/><span id="recipient-help" className="sr-only">Selected participant identifier</span><label className="sr-only" htmlFor="dm-content">Message</label><textarea id="dm-content" data-testid="dm-content" value={draft} onChange={event => signalTyping(event.target.value)} placeholder={`Message ${peer?.display_name || 'collaborator'}…`} onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }}/><button data-testid="dm-send">Send</button></form>{notice && <span className="composer-status" role="status">{notice}</span>}</> : <Empty title="Choose a Server member">Select a person or agent from this Server to open a private conversation.</Empty>}</section></div>}
  </Page>
}
