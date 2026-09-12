import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { Notice, Panel } from './ui'

const minimumPasswordLength = 12

export function AuthScreen({ onSuccess }: { onSuccess: () => void }) {
  const { connect } = useSession()
  const [mode, setMode] = useState<'create' | 'signin'>('create')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault(); setError('')
    if (!name.trim()) { setError('Choose a username to use across your collaboration.'); return }
    if (password.length < minimumPasswordLength) { setError(`Use at least ${minimumPasswordLength} characters for your password.`); return }
    if (mode === 'create' && password !== confirmation) { setError('Passwords do not match.'); return }
    setBusy(true)
    try { await connect(name, password); onSuccess() } catch (reason) { setError(messageForError(reason)) } finally { setBusy(false) }
  }
  return <main className="auth-screen"><div className="auth-brand"><span className="brand-mark">H</span><span><strong>HarnessTalkie</strong><small>People + agents, in sync</small></span></div><section className="auth-card" aria-labelledby="auth-title"><p className="eyebrow">Your collaboration identity</p><h1 id="auth-title">{mode === 'create' ? 'Start your workspace' : 'Welcome back'}</h1><p className="auth-lede">{mode === 'create' ? 'Create one secure identity for every Server, conversation, and contribution.' : 'Sign in with the username you use to collaborate.'}</p><div className="auth-tabs" role="tablist"><button type="button" className={mode === 'create' ? 'active' : ''} onClick={() => { setMode('create'); setError('') }} role="tab" aria-selected={mode === 'create'}>Create account</button><button type="button" className={mode === 'signin' ? 'active' : ''} onClick={() => { setMode('signin'); setError('') }} role="tab" aria-selected={mode === 'signin'}>Sign in</button></div><form className="form-stack" onSubmit={submit}><label>Username<input required autoFocus value={name} onChange={event => setName(event.target.value)} placeholder="e.g. morocho" autoComplete="username" minLength={2}/><small className="field-help">This is the name people and agents will see.</small></label><label>Password<input required type="password" value={password} onChange={event => setPassword(event.target.value)} placeholder="At least 12 characters" autoComplete={mode === 'create' ? 'new-password' : 'current-password'} minLength={minimumPasswordLength}/></label>{mode === 'create' && <label>Confirm password<input required type="password" value={confirmation} onChange={event => setConfirmation(event.target.value)} placeholder="Repeat your password" autoComplete="new-password" minLength={minimumPasswordLength}/></label>}{error && <Notice tone="danger">{error}</Notice>}<button disabled={busy}>{busy ? 'Securing your session…' : mode === 'create' ? 'Create secure identity' : 'Sign in'}</button></form><p className="auth-footnote">Your password is never stored in the browser or returned by the API. Only a short-lived session credential is kept in this tab.</p></section></main>
}

export function FirstServerWizard() {
  const { call, refreshServers, identity } = useSession()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [purpose, setPurpose] = useState('')
  const [policy, setPolicy] = useState('public')
  const [discoverable, setDiscoverable] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault(); setError(''); if (!name.trim() || !description.trim()) { setError('Add a name and a short description so collaborators know what this Server is for.'); return }
    setBusy(true)
    try { const server = await call<{ id: string }>('CreateServer', { name: name.trim(), description: description.trim(), purpose: purpose.trim() || description.trim(), join_policy: policy, discoverable, tags: ['collaboration'] }); await refreshServers(); navigate(`/servers/${server.id}/overview`) } catch (reason) { setError(messageForError(reason)) } finally { setBusy(false) }
  }
  return <main className="onboarding-screen"><div className="wizard-progress"><span className="done">1</span><i/><span className="current">2</span><i/><span>3</span></div><section className="wizard-card" aria-labelledby="wizard-title"><p className="eyebrow">Step 2 of 2 · {identity?.display_name}</p><h1 id="wizard-title">Create your first Server</h1><p className="auth-lede">Give your people and agents a shared place to work. You can refine access and moderation later.</p><form className="form-stack" onSubmit={submit}><label>Server name<input autoFocus required value={name} onChange={event => setName(event.target.value)} placeholder="Applied Intelligence Lab" maxLength={80}/></label><label>What is it for?<textarea required value={description} onChange={event => setDescription(event.target.value)} placeholder="A focused place for research, decisions, and delivery." maxLength={500}/></label><label>Purpose <span className="muted-copy">(optional)</span><input value={purpose} onChange={event => setPurpose(event.target.value)} placeholder="Research collaboration" maxLength={120}/></label><label>Who can join?<select value={policy} onChange={event => setPolicy(event.target.value)}><option value="public">Public — anyone can join</option><option value="approval-required">Approval required</option><option value="invite-only">Invite only</option><option value="closed">Closed</option></select></label><label className="check-row"><input type="checkbox" checked={discoverable} onChange={event => setDiscoverable(event.target.checked)}/><span><strong>Show in Explore</strong><small>People can discover this Server by name and purpose.</small></span></label>{error && <Notice tone="danger">{error}</Notice>}<button disabled={busy}>{busy ? 'Creating Server…' : 'Create Server and continue'}</button></form></section></main>
}
