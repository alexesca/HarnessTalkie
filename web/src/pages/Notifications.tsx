import { useSession } from '../session'
import { useLoad } from '../lib/use-load'
import type { Notification } from '../lib/types'
import { Badge, Empty, Loading, Notice, Page } from '../components/ui'

export default function Notifications() {
  const { identity, call } = useSession()
  const { data = [], loading, error, refresh } = useLoad(signal => identity ? call<Notification[]>('ListNotifications', { limit: 100 }, signal) : Promise.resolve([]), [call, identity?.id])
  async function markAll() { const ids = data.filter(item => !item.read).map(item => item.id); if (ids.length) { await call('MarkNotificationsRead', { notification_ids: ids }); await refresh() } }
  return <Page eyebrow="Activity inbox" title="Notifications" description="Mentions, replies, invitations, approvals, and administrative events." actions={<button className="secondary" onClick={() => void markAll()}>Mark all read</button>}>{loading ? <Loading/> : error ? <Notice tone="danger">{error}</Notice> : data.length ? <div className="notification-list" data-testid="notifications-visible">{data.map(item => <article className={item.read ? 'read' : ''} key={item.id}><i/><div><h2>{item.type.replaceAll('_', ' ')}</h2><p>{item.summary || 'New collaboration activity'}</p><small>{item.created_at ? new Date(item.created_at).toLocaleString() : 'Just now'}</small></div><Badge tone={item.read ? 'neutral' : 'success'}>{item.read ? 'read' : 'new notification'}</Badge></article>)}</div> : <Empty title="You’re all caught up">Important collaboration activity will appear here durably.</Empty>}</Page>
}
