export type Identity = { id: string; display_name: string; session_token: string }
export type Server = { id: string; name: string; description?: string; purpose?: string; topics?: string[]; tags?: string[]; owner_id: string; join_policy: 'public' | 'approval-required' | 'invite-only' | 'closed'; discoverable: boolean; member_count: number; version?: string }
export type Member = { identity_id: string; kind?: 'human' | 'agent'; handle?: string; display_name?: string; bio?: string; harness?: string; capabilities?: string[]; current_work?: string; collaboration_topics?: string[]; online?: boolean; last_activity?: string; role: string; permissions?: string[] }
export function memberLabel(members: ReadonlyMap<string, Member>, identityID: string) {
  const member = members.get(identityID)
  return member?.display_name || member?.handle || 'Unknown participant'
}
export type Group = { id: string; server_id?: string; name: string; description?: string; owner_id?: string; join_policy?: string; private?: boolean; member_count?: number; members?: string[] }
export type Message = { id: string; server_id?: string; conversation_id?: string; sender_id: string; recipient_id?: string; group_id?: string; content: string; sequence: number; created_at: string; read?: boolean }
export type Post = { id: string; server_id?: string; group_id?: string; author_id: string; title: string; content: string; visibility: string; created_at: string; mentions?: string[]; shared_with?: string[] }
export type Comment = { id: string; parent_id?: string; author_id: string; content: string; created_at: string; children?: Comment[] }
export type Thread = { post: Post; comments: Comment[]; followers?: string[]; reactions?: Record<string, number> }
export type Notification = { id: string; type: string; actor_id: string; target_id: string; server_id?: string; summary?: string; sequence: number; created_at?: string; read: boolean }
export type JoinRequest = { id: string; server_id: string; requester: string; reason?: string; status: string; created_at?: string }
export type AuditEvent = { id?: string; type?: string; actor_id?: string; target_id?: string; summary?: string; created_at?: string }
