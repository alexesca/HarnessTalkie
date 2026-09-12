import type { PropsWithChildren, ReactNode } from 'react'

export function Page({ eyebrow, title, description, actions, children }: PropsWithChildren<{ eyebrow?: string; title: string; description?: string; actions?: ReactNode }>) {
  return <div className="page"><header className="page-header"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1>{description && <p className="page-description">{description}</p>}</div>{actions && <div className="page-actions">{actions}</div>}</header>{children}</div>
}
export function Panel({ title, description, action, children, className = '' }: PropsWithChildren<{ title?: string; description?: string; action?: ReactNode; className?: string }>) {
  return <section className={`panel ${className}`}><header className="panel-header"><div>{title && <h2>{title}</h2>}{description && <p>{description}</p>}</div>{action}</header>{children}</section>
}
export function Empty({ title, children }: PropsWithChildren<{ title: string }>) { return <div className="empty"><span aria-hidden="true">◇</span><h3>{title}</h3>{children && <p>{children}</p>}</div> }
export function Notice({ children, tone = 'info' }: PropsWithChildren<{ tone?: 'info' | 'danger' | 'success' }>) { return <div className={`notice ${tone}`} role={tone === 'danger' ? 'alert' : 'status'}>{children}</div> }
export function Loading({ label = 'Loading collaboration data' }: { label?: string }) { return <div className="skeletons" role="status" aria-label={label}><i/><i/><i/></div> }
export function Avatar({ name, kind = 'human', online }: { name: string; kind?: string; online?: boolean }) { return <span className={`avatar ${kind}`} aria-label={`${kind}${online ? ', online' : ''}`}>{name.slice(0, 1).toUpperCase()}<i className={online ? 'online' : ''}/></span> }
export function Badge({ children, tone = 'neutral' }: PropsWithChildren<{ tone?: string }>) { return <span className={`badge ${tone}`}>{children}</span> }
