import { useMemo, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import { memberLabel, type Member, type Post } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page, Panel } from '../components/ui'

export default function Forums() {
  const { activeServer, call } = useSession()
  const [query, setQuery] = useState('')
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [notice, setNotice] = useState('')
  const { data, loading, error, refresh } = useLoad(async signal => { if (!activeServer) return { posts: [], members: [] }; const [posts, members] = await Promise.all([call<Post[]>('SearchPosts', { server_id: activeServer.id, query, limit: 100 }, signal), call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 200 }, signal).catch(() => [])]); return { posts, members } }, [call, activeServer?.id, query])
  const posts = data?.posts || []
  const memberMap = useMemo(() => new Map((data?.members || []).map(member => [member.identity_id, member])), [data?.members])
  async function create(event: FormEvent) { event.preventDefault(); if (!activeServer) return; try { const post = await call<Post>('CreatePost', { server_id: activeServer.id, title, content, visibility: 'server-wide' }); setTitle(''); setContent(''); await refresh(); setNotice(`Published “${post.title}”`) } catch (reason) { setNotice(messageForError(reason)) } }
  return <Page eyebrow={activeServer?.name || 'Discussions'} title="Forums" description="Durable threads for questions, decisions, and async work.">
    {!activeServer ? <Empty title="Choose a Server first">Discussions respect Server and group visibility.</Empty> : <div className="forum-layout"><section>{notice && <Notice>{notice}</Notice>}<label className="search"><span className="sr-only">Search discussions</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search titles and discussion content…"/></label>{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : <div className="post-feed" data-testid="posts">{posts.length ? posts.map(post => <Link to={`/posts/${post.id}`} className="post-row" key={post.id}><div><div className="post-meta"><Badge>{post.group_id ? 'group' : 'server'}</Badge><span>{memberLabel(memberMap, post.author_id)}</span><time>{new Date(post.created_at).toLocaleDateString()}</time></div><h2>{post.title}</h2><p>{post.content}</p></div><span className="arrow" aria-hidden="true">→</span></Link>) : <Empty title="No discussions found">Start a thread or try a broader search.</Empty>}</div>}</section><Panel title="Start a discussion" description="Give the thread a clear topic so collaborators can find it later."><form className="form-stack" onSubmit={create}><label>Topic<input data-testid="post-title" required value={title} onChange={event => setTitle(event.target.value)} placeholder="What should we decide?"/></label><label>Context<textarea data-testid="post-content" required value={content} onChange={event => setContent(event.target.value)} placeholder="Share the facts, question, or proposal…"/></label><label>Visibility<select defaultValue="server-wide"><option value="server-wide">Everyone in this Server</option></select></label><button data-testid="post-create">Publish discussion</button></form></Panel></div>}
  </Page>
}
