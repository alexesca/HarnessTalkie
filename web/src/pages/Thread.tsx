import { useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useSession } from '../session'
import { messageForError } from '../lib/rpc'
import { useLoad } from '../lib/use-load'
import type { Comment, Thread as ThreadData } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page } from '../components/ui'

function CommentBranch({ comment, depth = 0 }: { comment: Comment; depth?: number }) {
  return <li className="comment" style={{ '--depth': Math.min(depth, 5) } as React.CSSProperties}><div><strong>{comment.author_id}</strong><time>{new Date(comment.created_at).toLocaleString()}</time><p>{comment.content}</p></div>{comment.children?.length ? <ol>{comment.children.map(child => <CommentBranch key={child.id} comment={child} depth={depth + 1}/>)}</ol> : null}</li>
}

export default function Thread() {
  const { postId = '' } = useParams()
  const navigate = useNavigate()
  const { call } = useSession()
  const [lookup, setLookup] = useState(postId)
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  const { data, loading, error, refresh } = useLoad(signal => postId ? call<ThreadData>('GetThread', { post: postId, response: { mode: 'compact' } }, signal) : Promise.reject(new Error('Choose a discussion')), [call, postId])
  async function comment(event: FormEvent) { event.preventDefault(); if (!postId || !draft.trim()) return; try { await call('Comment', { post_or_comment: postId, content: draft.trim(), mentions: [] }); setDraft(''); await refresh(); setNotice('Reply published') } catch (reason) { setNotice(messageForError(reason)) } }
  async function act(method: 'FollowThread' | 'React') { try { await call(method, method === 'React' ? { target: postId, reaction: 'like' } : { post: postId }); await refresh(); setNotice(method === 'React' ? 'Reaction added' : 'Following this discussion') } catch (reason) { setNotice(messageForError(reason)) } }
  return <Page eyebrow="Discussion thread" title={data?.post.title || 'Open a discussion'} description={data?.post.content} actions={<Badge>{data?.post.visibility || 'forum'}</Badge>}>
    <form className="thread-lookup" onSubmit={event => { event.preventDefault(); navigate(`/posts/${lookup}`) }}><label className="sr-only" htmlFor="thread-id">Post identifier</label><input id="thread-id" data-testid="thread-id" value={lookup} onChange={event => setLookup(event.target.value)} placeholder="Open a discussion reference"/><button className="secondary">Open</button></form>
    {loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : data ? <div className="thread-layout"><article className="thread-main" data-testid="thread-visible"><div className="post-author"><span>Started by {data.post.author_id}</span><time>{new Date(data.post.created_at).toLocaleString()}</time></div><p className="thread-content">{data.post.content}</p><div className="thread-actions"><button data-testid="thread-follow" className="secondary" onClick={() => void act('FollowThread')}>Follow · {data.followers?.length || 0}</button><button data-testid="react" className="secondary" onClick={() => void act('React')}>Appreciate · {data.reactions?.like || 0}</button></div>{notice && <Notice>{notice}</Notice>}<section><h2>{data.comments.length} replies</h2>{data.comments.length ? <ol className="comment-tree" data-testid="comments">{data.comments.map(commentItem => <CommentBranch key={commentItem.id} comment={commentItem}/>)}</ol> : <Empty title="No replies yet">Add context or ask a follow-up.</Empty>}</section><form className="reply-box" onSubmit={comment}><label htmlFor="comment-content">Add to the discussion</label><textarea id="comment-content" data-testid="comment-content" value={draft} onChange={event => setDraft(event.target.value)} placeholder="Write a thoughtful reply…"/><button data-testid="comment-send">Publish reply</button></form></article><aside className="thread-context"><h2>Thread context</h2><dl><div><dt>Scope</dt><dd>{data.post.group_id ? 'Group' : 'Server'}</dd></div><div><dt>Visibility</dt><dd>{data.post.visibility}</dd></div><div><dt>Reactions</dt><dd>{Object.values(data.reactions || {}).reduce((sum, count) => sum + count, 0)}</dd></div></dl></aside></div> : null}
  </Page>
}
