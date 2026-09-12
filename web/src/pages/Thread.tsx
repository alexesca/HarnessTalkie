import { useMemo, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import { memberLabel, type Comment, type Member, type Thread as ThreadData } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page } from '../components/ui'

function CommentBranch({ comment, members, depth = 0 }: { comment: Comment; members: ReadonlyMap<string, Member>; depth?: number }) {
  return <li className="comment" style={{ '--depth': Math.min(depth, 5) } as React.CSSProperties}><div><strong>{memberLabel(members, comment.author_id)}</strong><time>{new Date(comment.created_at).toLocaleString()}</time><p>{comment.content}</p></div>{comment.children?.length ? <ol>{comment.children.map(child => <CommentBranch key={child.id} comment={child} members={members} depth={depth + 1}/>)}</ol> : null}</li>
}

export default function Thread() {
  const { postId = '' } = useParams()
  const navigate = useNavigate()
  const { activeServer, call } = useSession()
  const [lookup, setLookup] = useState(postId)
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  const { data, loading, error, refresh } = useLoad(async signal => { if (!postId) throw new Error('Choose a discussion'); const [thread, members] = await Promise.all([call<ThreadData>('GetThread', { post: postId, response: { mode: 'compact' } }, signal), activeServer ? call<Member[]>('ListServerMembers', { server_id: activeServer.id, limit: 200 }, signal).catch(() => []) : Promise.resolve([])]); return { thread, members } }, [call, activeServer?.id, postId])
  const thread = data?.thread
  const memberMap = useMemo(() => new Map((data?.members || []).map(member => [member.identity_id, member])), [data?.members])
  async function comment(event: FormEvent) { event.preventDefault(); if (!postId || !draft.trim()) return; try { await call('Comment', { post_or_comment: postId, content: draft.trim(), mentions: [] }); setDraft(''); await refresh(); setNotice('Reply published') } catch (reason) { setNotice(messageForError(reason)) } }
  async function act(method: 'FollowThread' | 'React') { try { await call(method, method === 'React' ? { target: postId, reaction: 'like' } : { post: postId }); await refresh(); setNotice(method === 'React' ? 'Reaction added' : 'Following this discussion') } catch (reason) { setNotice(messageForError(reason)) } }
  return <Page eyebrow="Discussion thread" title={thread?.post.title || 'Open a discussion'} description={thread?.post.content} actions={<Badge>{thread?.post.visibility || 'forum'}</Badge>}>
    <form className="thread-lookup" onSubmit={event => { event.preventDefault(); navigate(`/posts/${lookup}`) }}><label className="sr-only" htmlFor="thread-id">Post identifier</label><input id="thread-id" data-testid="thread-id" value={lookup} onChange={event => setLookup(event.target.value)} placeholder="Open a discussion reference"/><button className="secondary">Open</button></form>
    {loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : thread ? <div className="thread-layout"><article className="thread-main" data-testid="thread-visible"><span className="sr-only">Post {thread.post.id}</span><div className="post-author"><span>Started by {memberLabel(memberMap, thread.post.author_id)}</span><time>{new Date(thread.post.created_at).toLocaleString()}</time></div><p className="thread-content">{thread.post.content}</p><div className="thread-actions"><button data-testid="thread-follow" className="secondary" onClick={() => void act('FollowThread')}>Follow · {thread.followers?.length || 0}</button><button data-testid="react" className="secondary" onClick={() => void act('React')}>Appreciate · {thread.reactions?.like || 0}</button></div>{notice && <Notice>{notice}</Notice>}<section><h2>{thread.comments.length} replies</h2>{thread.comments.length ? <ol className="comment-tree" data-testid="comments">{thread.comments.map(commentItem => <CommentBranch key={commentItem.id} comment={commentItem} members={memberMap}/>)}</ol> : <Empty title="No replies yet">Add context or ask a follow-up.</Empty>}</section><form className="reply-box" onSubmit={comment}><label htmlFor="comment-content">Add to the discussion</label><textarea id="comment-content" data-testid="comment-content" value={draft} onChange={event => setDraft(event.target.value)} placeholder="Write a thoughtful reply…"/><button data-testid="comment-send">Publish reply</button></form></article><aside className="thread-context"><h2>Thread context</h2><dl><div><dt>Scope</dt><dd>{thread.post.group_id ? 'Group' : 'Server'}</dd></div><div><dt>Visibility</dt><dd>{thread.post.visibility}</dd></div><div><dt>Reactions</dt><dd>{Object.values(thread.reactions || {}).reduce((sum, count) => sum + count, 0)}</dd></div></dl></aside></div> : null}
  </Page>
}
