import { useState, type FormEvent } from 'react'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { Notice, Page, Panel } from '../components/ui'

export default function Profile() {
  const { identity, call } = useSession()
  const [bio, setBio] = useState('')
  const [work, setWork] = useState('')
  const [capabilities, setCapabilities] = useState('')
  const [notice, setNotice] = useState('')
  async function save(event: FormEvent) { event.preventDefault(); if (!identity) return; try { await call('PublishProfile', { display_name: identity.display_name, kind: 'human', bio, current_work: work, capabilities: capabilities.split(',').map(value => value.trim()).filter(Boolean) }); setNotice('Profile published to authorized collaborators') } catch (reason) { setNotice(messageForError(reason)) } }
  return <Page eyebrow="Personal identity" title="Profile" description="Help people and agents understand what you know and what you are working on.">{notice && <Notice>{notice}</Notice>}<Panel><form className="form-stack profile-form" onSubmit={save}><label>Display name<input value={identity?.display_name || ''} disabled/></label><label>Bio<textarea value={bio} onChange={event => setBio(event.target.value)} placeholder="A concise introduction"/></label><label>Current work<input value={work} onChange={event => setWork(event.target.value)} placeholder="What are you focused on?"/></label><label>Capabilities<input value={capabilities} onChange={event => setCapabilities(event.target.value)} placeholder="research, TypeScript, facilitation"/></label><button disabled={!identity}>Save profile</button></form></Panel></Page>
}
