package main

// HarnessTalkie V2 is implemented as a server-scoped layer over the durable V1
// event store.  The wire types intentionally mirror WalkieBench's public V2
// contract without importing the benchmark module into the application.

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type v2Server struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Purpose           string   `json:"purpose,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	OwnerID           string   `json:"owner_id"`
	JoinPolicy        string   `json:"join_policy"`
	MemberCount       int      `json:"member_count"`
	Capabilities      []string `json:"capabilities,omitempty"`
	Rules             []string `json:"rules,omitempty"`
	Discoverable      bool     `json:"discoverable"`
	ConnectionMethods []string `json:"connection_methods,omitempty"`
	Version           string   `json:"version,omitempty"`
}

type v2MemberRecord struct {
	IdentityID  string   `json:"identity_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	JoinedAt    string   `json:"joined_at"`
}
type v2ServerRecord struct {
	Server          v2Server                   `json:"server"`
	Members         map[string]*v2MemberRecord `json:"members"`
	RolePermissions map[string]map[string]bool `json:"role_permissions"`
}
type v2ServerRequest struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Requester string `json:"requester"`
	Reason    string `json:"reason,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}
type v2ServerInvite struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	InviteeID string `json:"invitee_id"`
	InvitedBy string `json:"invited_by"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
type v2GroupRecord struct {
	ID          string            `json:"id"`
	ServerID    string            `json:"server_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	JoinPolicy  string            `json:"join_policy"`
	Private     bool              `json:"private,omitempty"`
	OwnerID     string            `json:"owner_id"`
	Members     map[string]bool   `json:"members"`
	Invites     map[string]string `json:"invites"`
	Requests    map[string]string `json:"requests"`
	Roles       map[string]string `json:"roles,omitempty"`
	Permissions map[string]map[string]bool `json:"permissions,omitempty"`
	JoinedAt    map[string]string `json:"joined_at,omitempty"`
	InviteBy    map[string]string `json:"invite_by,omitempty"`
	InviteAt    map[string]string `json:"invite_at,omitempty"`
	InviteExpiry map[string]string `json:"invite_expiry,omitempty"`
	RequestStatus map[string]string `json:"request_status,omitempty"`
	RequestReason map[string]string `json:"request_reason,omitempty"`
	RequestAt     map[string]string `json:"request_at,omitempty"`
}
type v2PostRecord struct {
	Post       Post     `json:"post"`
	ServerID   string   `json:"server_id"`
	GroupID    string   `json:"group_id,omitempty"`
	Visibility string   `json:"visibility"`
	Mentions   []string `json:"mentions,omitempty"`
}
type v2Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	ActorID   string `json:"actor_id"`
	TargetID  string `json:"target_id"`
	ServerID  string `json:"server_id,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Sequence  uint64 `json:"sequence"`
	CreatedAt string `json:"created_at,omitempty"`
	Read      bool   `json:"read"`
}
type v2ActivityRecord struct {
	ServerID string        `json:"server_id"`
	Event    ActivityEvent `json:"event"`
}

type v2MemberView struct {
	Participant
	ServerID    string   `json:"server_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	JoinedAt    string   `json:"joined_at,omitempty"`
}
type v2PostView struct {
	Post
	ServerID   string   `json:"server_id"`
	GroupID    string   `json:"group_id,omitempty"`
	Visibility string   `json:"visibility"`
	Mentions   []string `json:"mentions,omitempty"`
}
type v2ManifestResult struct {
	Identity       Identity         `json:"identity"`
	Server         v2Server         `json:"server"`
	Membership     string           `json:"membership"`
	AccessRequest  *v2ServerRequest `json:"access_request,omitempty"`
	Participants   []Participant    `json:"participants,omitempty"`
	Groups         []Group          `json:"groups,omitempty"`
	Contacts       []Contact        `json:"contacts,omitempty"`
	Follows        []string         `json:"follows,omitempty"`
	UnreadActivity int              `json:"unread_activity"`
	Cursor         uint64           `json:"cursor"`
}
type v2GroupMemberView struct {
	Participant
	GroupID     string   `json:"group_id"`
	GroupRole   string   `json:"group_role"`
	Permissions []string `json:"permissions,omitempty"`
	JoinedAt    string   `json:"joined_at,omitempty"`
}
type v2GroupRequestView struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	Requester string `json:"requester"`
	Reason    string `json:"reason,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}
type v2GroupInviteView struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	InviteeID string `json:"invitee_id"`
	InvitedBy string `json:"invited_by"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
type v2NotificationQuery struct {
	AfterSequence uint64            `json:"after_sequence,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	UnreadOnly    bool              `json:"unread_only,omitempty"`
	Response      v2ResponseOptions `json:"response,omitempty"`
}
type v2ResponseOptions struct {
	Select      []string `json:"select,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Cursor      uint64   `json:"cursor,omitempty"`
	UnreadOnly  bool     `json:"unread_only,omitempty"`
	SinceCursor uint64   `json:"since_cursor,omitempty"`
}

func v2Map(raw json.RawMessage, out any) error { return parseParams(raw, out) }
func v2Limit(n int, fallback int) int {
	if n <= 0 {
		return fallback
	}
	if n > 1000 {
		return 1000
	}
	return n
}
func v2Options(raw map[string]json.RawMessage) v2ResponseOptions {
	var o v2ResponseOptions
	_ = json.Unmarshal(raw["response"], &o)
	return o
}
func v2MapRaw(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	return m
}
func v2String(raw map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(raw[key], &s)
	return s
}
func v2Strings(raw map[string]json.RawMessage, key string) []string {
	var s []string
	_ = json.Unmarshal(raw[key], &s)
	return s
}

func v2ServerView(r *v2ServerRecord) v2Server {
	x := r.Server
	x.MemberCount = len(r.Members)
	return x
}
func (s *server) v2MemberViewLocked(sr *v2ServerRecord, id string) (v2MemberView, error) {
	m := sr.Members[id]
	x := s.db.s.Identities[id]
	if m == nil || x == nil {
		return v2MemberView{}, missing("Server member not found")
	}
	return v2MemberView{Participant: participantFrom(x, s.online(id)), ServerID: sr.Server.ID, Role: m.Role, Permissions: append([]string(nil), m.Permissions...), JoinedAt: m.JoinedAt}, nil
}
func v2GroupView(g *v2GroupRecord) Group {
	ids := make([]string, 0, len(g.Members))
	for id := range g.Members {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return Group{ID: g.ID, ServerID: g.ServerID, Name: g.Name, Description: g.Description, OwnerID: g.OwnerID, JoinPolicy: g.JoinPolicy, Private: g.Private, MemberCount: len(ids), Members: ids}
}
func v2PostViewOf(p *v2PostRecord) v2PostView {
	return v2PostView{Post: p.Post, ServerID: p.ServerID, GroupID: p.GroupID, Visibility: p.Visibility, Mentions: append([]string(nil), p.Mentions...)}
}

func v2Contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func v2RolePermissions(sr *v2ServerRecord, role string) map[string]bool {
	if sr.RolePermissions == nil {
		sr.RolePermissions = map[string]map[string]bool{}
	}
	if sr.RolePermissions[role] == nil {
		sr.RolePermissions[role] = map[string]bool{}
	}
	return sr.RolePermissions[role]
}
var v2Roles = map[string]bool{"owner": true, "administrator": true, "moderator": true, "member": true, "guest": true, "agent": true}
var v2Permissions = map[string]bool{
	"view_server": true, "view_members": true, "send_messages": true, "invite_members": true,
	"approve_members": true, "remove_members": true, "create_groups": true, "administer_groups": true,
	"create_posts": true, "moderate_posts": true, "manage_roles": true, "manage_permissions": true,
	"manage_settings": true, "view_audit": true, "administrative_visibility": true,
}
func v2DefaultPermission(role, permission string) bool {
	switch role {
	case "owner", "administrator":
		return true
	case "moderator":
		return permission == "view_server" || permission == "view_members" || permission == "send_messages" || permission == "create_posts" || permission == "moderate_posts"
	case "member", "agent":
		return permission == "view_server" || permission == "view_members" || permission == "send_messages"
	case "guest":
		return permission == "view_server" || permission == "send_messages"
	}
	return false
}
func v2IsMember(sr *v2ServerRecord, id string) bool { _, ok := sr.Members[id]; return ok }
func v2Can(sr *v2ServerRecord, caller, permission string) bool {
	m := sr.Members[caller]
	if m == nil {
		return false
	}
	if m.Role == "owner" {
		return true
	}
	if m.Permissions != nil && v2Contains(m.Permissions, permission) {
		return true
	}
	if configured, ok := v2RolePermissions(sr, m.Role)[permission]; ok {
		return configured
	}
	return v2DefaultPermission(m.Role, permission)
}
func v2Admin(sr *v2ServerRecord, caller string) bool {
	return v2Can(sr, caller, "administrative_visibility")
}

func v2InitGroup(g *v2GroupRecord) {
	if g.Members == nil { g.Members = map[string]bool{} }
	if g.Invites == nil { g.Invites = map[string]string{} }
	if g.Requests == nil { g.Requests = map[string]string{} }
	if g.Roles == nil { g.Roles = map[string]string{} }
	if g.Permissions == nil { g.Permissions = map[string]map[string]bool{} }
	if g.JoinedAt == nil { g.JoinedAt = map[string]string{} }
	if g.InviteBy == nil { g.InviteBy = map[string]string{} }
	if g.InviteAt == nil { g.InviteAt = map[string]string{} }
	if g.InviteExpiry == nil { g.InviteExpiry = map[string]string{} }
	if g.RequestStatus == nil { g.RequestStatus = map[string]string{} }
	if g.RequestReason == nil { g.RequestReason = map[string]string{} }
	if g.RequestAt == nil { g.RequestAt = map[string]string{} }
}
func v2CanGroup(g *v2GroupRecord, caller, permission string) bool {
	if caller == g.OwnerID { return true }
	role := g.Roles[caller]
	if role == "administrator" { return true }
	if configured := g.Permissions[role]; configured != nil {
		if allowed, ok := configured[permission]; ok { return allowed }
	}
	return role == "member" && (permission == "view_history" || permission == "send_messages")
}
func v2Audit(s *server, serverID, typ, actor, target, summary string) uint64 {
	seq, at := s.db.nextActivitySequence(), s.db.now()
	e := ActivityEvent{Type: typ, ID: s.db.nextID("audit"), ActorID: actor, TargetID: target, Sequence: seq, CreatedAt: at, Summary: summary}
	s.db.s.V2Activities = append(s.db.s.V2Activities, v2ActivityRecord{ServerID: serverID, Event: e})
	s.db.recordActivity(e)
	return seq
}
func (s *server) v2NotifyLocked(serverID, recipient, typ, actor, target, summary string) {
	if recipient == "" || s.db.s.Identities[recipient] == nil {
		return
	}
	seq := v2Audit(s, serverID, typ, actor, target, summary)
	n := &v2Notification{ID: s.db.nextID("notification"), Type: typ, ActorID: actor, TargetID: target, ServerID: serverID, Summary: summary, Sequence: seq, CreatedAt: s.db.s.LastTime.Format(time.RFC3339Nano)}
	s.db.s.V2Notifications[recipient] = append(s.db.s.V2Notifications[recipient], n)
}
func (s *server) v2AddMemberLocked(sr *v2ServerRecord, id, role string) {
	if sr.Members == nil {
		sr.Members = map[string]*v2MemberRecord{}
	}
	if _, ok := sr.Members[id]; ok {
		return
	}
	sr.Members[id] = &v2MemberRecord{IdentityID: id, Role: role, JoinedAt: s.db.s.LastTime.Format(time.RFC3339Nano)}
}
func v2Match(text, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(text), strings.ToLower(q))
}
func v2ApplySelection(value any, opts v2ResponseOptions) any {
	if len(opts.Select) == 0 {
		return value
	}
	b, _ := json.Marshal(value)
	var obj any
	if json.Unmarshal(b, &obj) != nil {
		return value
	}
	keep := map[string]bool{}
	for _, k := range opts.Select {
		keep[k] = true
	}
	var trim func(any) any
	trim = func(v any) any {
		switch x := v.(type) {
		case []any:
			for i := range x {
				x[i] = trim(x[i])
			}
			return x
		case map[string]any:
			for k := range x {
				alias := k
				if k == "identity_id" {
					alias = "id"
				}
				if !keep[k] && !keep[alias] {
					delete(x, k)
					continue
				}
				x[k] = trim(x[k])
			}
		}
		return v
	}
	return trim(obj)
}
func v2Page[T any](items []T, limit, cursor int) ([]T, int, bool) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(items) {
		return []T{}, cursor, false
	}
	end := cursor + v2Limit(limit, 50)
	if end > len(items) {
		end = len(items)
	}
	return items[cursor:end], end, end < len(items)
}

func (db *store) applyV2(typ string, raw []byte) error {
	if db.s.Servers == nil {
		db.s.Servers = map[string]*v2ServerRecord{}
		db.s.V2Requests = map[string]*v2ServerRequest{}
		db.s.V2Invites = map[string]*v2ServerInvite{}
		db.s.V2Groups = map[string]*v2GroupRecord{}
		db.s.V2Posts = map[string]*v2PostRecord{}
		db.s.V2Notifications = map[string][]*v2Notification{}
	}
	switch typ {
	case "v2_server":
		var x v2ServerRecord
		if json.Unmarshal(raw, &x) == nil {
			db.s.Servers[x.Server.ID] = &x
		}
	case "v2_server_request":
		var x v2ServerRequest
		if json.Unmarshal(raw, &x) == nil {
			db.s.V2Requests[x.ID] = &x
		}
	case "v2_server_invite":
		var x v2ServerInvite
		if json.Unmarshal(raw, &x) == nil {
			db.s.V2Invites[x.ID] = &x
		}
	case "v2_group":
		var x v2GroupRecord
		if json.Unmarshal(raw, &x) == nil {
			db.s.V2Groups[x.ID] = &x
		}
	case "v2_post":
		var x v2PostRecord
		if json.Unmarshal(raw, &x) == nil {
			db.s.V2Posts[x.Post.ID] = &x
			db.s.Posts[x.Post.ID] = &x.Post
		}
	case "v2_member_add":
		var x struct {
			ServerID    string `json:"server_id"`
			Participant string `json:"participant"`
			Role        string `json:"role"`
		}
		if json.Unmarshal(raw, &x) == nil {
			if sr := db.s.Servers[x.ServerID]; sr != nil {
				if x.Role == "" {
					x.Role = "agent"
				}
				if sr.Members == nil {
					sr.Members = map[string]*v2MemberRecord{}
				}
				if sr.Members[x.Participant] == nil {
					sr.Members[x.Participant] = &v2MemberRecord{IdentityID: x.Participant, Role: x.Role, JoinedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				}
			}
		}
	case "v2_member_remove":
		var x struct {
			ServerID    string `json:"server_id"`
			Participant string `json:"participant"`
		}
		if json.Unmarshal(raw, &x) == nil {
			if sr := db.s.Servers[x.ServerID]; sr != nil {
				delete(sr.Members, x.Participant)
			}
		}
	case "v2_role":
		var x struct {
			ServerID    string `json:"server_id"`
			Participant string `json:"participant"`
			Role        string `json:"role"`
		}
		if json.Unmarshal(raw, &x) == nil {
			if sr := db.s.Servers[x.ServerID]; sr != nil {
				if sr.Members[x.Participant] == nil {
					sr.Members[x.Participant] = &v2MemberRecord{IdentityID: x.Participant}
				}
				sr.Members[x.Participant].Role = x.Role
			}
		}
	case "v2_comment":
		var x struct {
			Post    string
			Comment CommentNode
		}
		if json.Unmarshal(raw, &x) == nil {
			db.s.Comments[x.Comment.ID] = &x.Comment
			db.s.CommentPosts[x.Comment.ID] = x.Post
			db.s.CommentsByPost[x.Post] = append(db.s.CommentsByPost[x.Post], x.Comment.ID)
		}
	}
	return nil
}

func (s *server) v2PersistLocked(typ string, value any) error { return s.db.appendEvent(typ, value) }

func (s *server) v2DispatchLocked(ctx context.Context, caller, method string, raw json.RawMessage) (any, error) {
	_ = ctx
	m := v2MapRaw(raw)
	arg := func(k string) string { return v2String(m, k) }
	getServer := func(id string) (*v2ServerRecord, error) {
		sr := s.db.s.Servers[id]
		if sr == nil {
			return nil, missing("Server not found")
		}
		return sr, nil
	}
	memberServer := func(id string) (*v2ServerRecord, error) {
		sr, e := getServer(id)
		if e != nil {
			return nil, e
		}
		if !v2IsMember(sr, caller) {
			return nil, denied("Server membership required")
		}
		return sr, nil
	}
	commit := func(typ string, value any, apply func()) error { return s.db.commit(typ, value, apply) }

	switch method {
	case "CreateServer":
		var p struct {
			Name              string   `json:"name"`
			Description       string   `json:"description"`
			Purpose           string   `json:"purpose"`
			Topics            []string `json:"topics"`
			Tags              []string `json:"tags"`
			JoinPolicy        string   `json:"join_policy"`
			Capabilities      []string `json:"capabilities"`
			Rules             []string `json:"rules"`
			Discoverable      bool     `json:"discoverable"`
			ConnectionMethods []string `json:"connection_methods"`
		}
		if e := v2Map(raw, &p); e != nil {
			return nil, e
		}
		if p.Name == "" {
			return nil, bad("Server name is required")
		}
		if p.JoinPolicy == "" {
			p.JoinPolicy = "public"
		}
		if p.JoinPolicy != "public" && p.JoinPolicy != "approval-required" && p.JoinPolicy != "invite-only" && p.JoinPolicy != "closed" {
			return nil, bad("invalid join policy")
		}
		id := s.db.nextID("server")
		sr := &v2ServerRecord{Server: v2Server{ID: id, Name: p.Name, Description: p.Description, Purpose: p.Purpose, Topics: p.Topics, Tags: p.Tags, OwnerID: caller, JoinPolicy: p.JoinPolicy, Capabilities: p.Capabilities, Rules: p.Rules, Discoverable: p.Discoverable, ConnectionMethods: p.ConnectionMethods, Version: "2"}, Members: map[string]*v2MemberRecord{}, RolePermissions: map[string]map[string]bool{}}
		sr.Members[caller] = &v2MemberRecord{IdentityID: caller, Role: "owner", JoinedAt: s.db.now().Format(time.RFC3339Nano)}
		if e := commit("v2_server", sr, func() { s.db.s.Servers[id] = sr; v2Audit(s, id, "server_created", caller, id, "Server created") }); e != nil {
			return nil, e
		}
		return sr.Server, nil
	case "GetServer":
		id := arg("server_id")
		sr, e := getServer(id)
		if e != nil {
			return nil, e
		}
		if !v2IsMember(sr, caller) && (!sr.Server.Discoverable || sr.Server.JoinPolicy != "public") {
			return nil, denied("Server metadata is private")
		}
		return v2ApplySelection(v2ServerView(sr), v2Options(m)), nil
	case "UpdateServer":
		id := arg("server_id")
		sr, e := getServer(id)
		if e != nil {
			return nil, e
		}
		if caller != sr.Server.OwnerID && !v2Can(sr, caller, "manage_settings") {
			return nil, denied("Server administration permission required")
		}
		var p struct {
			Name         *string  `json:"name"`
			Description  *string  `json:"description"`
			Purpose      *string  `json:"purpose"`
			Topics       []string `json:"topics"`
			Tags         []string `json:"tags"`
			JoinPolicy   *string  `json:"join_policy"`
			Rules        []string `json:"rules"`
			Discoverable *bool    `json:"discoverable"`
		}
		if e = v2Map(rawMapField(m, "patch"), &p); e != nil {
			return nil, e
		}
		if p.Name != nil {
			sr.Server.Name = *p.Name
		}
		if p.Description != nil {
			sr.Server.Description = *p.Description
		}
		if p.Purpose != nil {
			sr.Server.Purpose = *p.Purpose
		}
		if p.Topics != nil {
			sr.Server.Topics = p.Topics
		}
		if p.Tags != nil {
			sr.Server.Tags = p.Tags
		}
		if p.JoinPolicy != nil {
			sr.Server.JoinPolicy = *p.JoinPolicy
		}
		if p.Rules != nil {
			sr.Server.Rules = p.Rules
		}
		if p.Discoverable != nil {
			sr.Server.Discoverable = *p.Discoverable
		}
		v2Audit(s, id, "server_updated", caller, id, "Server settings updated")
		return sr.Server, nil
	case "ListServers", "DiscoverServers":
		var q struct {
			Query      string            `json:"query"`
			Topic      string            `json:"topic"`
			Tag        string            `json:"tag"`
			JoinPolicy string            `json:"join_policy"`
			OwnerID    string            `json:"owner_id"`
			Limit      int               `json:"limit"`
			Cursor     uint64            `json:"cursor"`
			Response   v2ResponseOptions `json:"response"`
		}
		_ = v2Map(raw, &q)
		out := []v2Server{}
		for _, sr := range s.db.s.Servers {
			if method == "DiscoverServers" && !sr.Server.Discoverable {
				continue
			}
			if method == "ListServers" && !v2IsMember(sr, caller) && sr.Server.OwnerID != caller {
				continue
			}
			if !v2Match(sr.Server.Name, q.Query) && !v2Match(sr.Server.Description, q.Query) {
				continue
			}
			if q.Topic != "" && !v2Contains(sr.Server.Topics, q.Topic) {
				continue
			}
			if q.Tag != "" && !v2Contains(sr.Server.Tags, q.Tag) {
				continue
			}
			if q.JoinPolicy != "" && sr.Server.JoinPolicy != q.JoinPolicy {
				continue
			}
			if q.OwnerID != "" && sr.Server.OwnerID != q.OwnerID {
				continue
			}
			out = append(out, v2ServerView(sr))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		start := int(q.Cursor)
		out, next, more := v2Page(out, q.Limit, start)
		_ = next
		_ = more
		shaped := make([]any, len(out))
		for i := range out {
			shaped[i] = v2ApplySelection(out[i], q.Response)
		}
		return shaped, nil
	case "JoinServer":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if v2IsMember(sr, caller) {
			return nil, nil
		}
		switch sr.Server.JoinPolicy {
		case "public":
		case "approval-required":
			for _, r := range s.db.s.V2Requests {
				if r.ServerID == sr.Server.ID && r.Requester == caller && r.Status == "approved" {
					goto allowed
				}
			}
			return nil, denied("approval required; request access")
		case "invite-only":
			return nil, denied("invitation required; accept invite")
		default:
			return nil, denied("Server is closed")
		}
	allowed:
		s.v2AddMemberLocked(sr, caller, "agent")
		if e := s.v2PersistLocked("v2_member_add", map[string]string{"server_id": sr.Server.ID, "participant": caller, "role": "agent"}); e != nil {
			return nil, e
		}
		v2Audit(s, sr.Server.ID, "member_joined", caller, caller, "Joined Server")
		return nil, nil
	case "RequestServerAccess":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if v2IsMember(sr, caller) {
			return nil, bad("already a Server member")
		}
		for _, r := range s.db.s.V2Requests {
			if r.ServerID == sr.Server.ID && r.Requester == caller && r.Status == "pending" || r.ServerID == sr.Server.ID && r.Requester == caller && r.Status == "approved" {
				return *r, nil
			}
		}
		id := s.db.nextID("request")
		r := &v2ServerRequest{ID: id, ServerID: sr.Server.ID, Requester: caller, Reason: arg("reason"), Status: "pending", CreatedAt: s.db.now().Format(time.RFC3339Nano)}
		if e = commit("v2_server_request", r, func() {
			s.db.s.V2Requests[id] = r
			s.v2NotifyLocked(sr.Server.ID, sr.Server.OwnerID, "membership_request", caller, id, "Server membership request")
		}); e != nil {
			return nil, e
		}
		return *r, nil
	case "ApproveServerRequest", "RejectServerRequest":
		r := s.db.s.V2Requests[arg("request_id")]
		if r == nil {
			return nil, missing("membership request not found")
		}
		sr, e := getServer(r.ServerID)
		if e != nil {
			return nil, e
		}
		if !v2Admin(sr, caller) {
			return nil, denied("Server administration permission required")
		}
		if r.Status != "pending" {
			return nil, nil
		}
		if method == "ApproveServerRequest" {
			r.Status = "approved"
		} else {
			r.Status = "rejected"
		}
		s.v2NotifyLocked(sr.Server.ID, r.Requester, strings.ToLower(method), caller, r.ID, "Server request updated")
		v2Audit(s, sr.Server.ID, "membership_request_updated", caller, r.Requester, "Server request updated")
		return nil, nil
	case "InviteToServer":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if !v2Admin(sr, caller) {
			return nil, denied("Server administration permission required")
		}
		invitee := arg("participant_id")
		if s.db.s.Identities[invitee] == nil {
			return nil, missing("participant not found")
		}
		for _, i := range s.db.s.V2Invites {
			if i.ServerID == sr.Server.ID && i.InviteeID == invitee && i.Status == "pending" {
				return *i, nil
			}
		}
		id := s.db.nextID("invite")
		i := &v2ServerInvite{ID: id, ServerID: sr.Server.ID, InviteeID: invitee, InvitedBy: caller, Status: "pending"}
		if e = commit("v2_server_invite", i, func() {
			s.db.s.V2Invites[id] = i
			s.v2NotifyLocked(sr.Server.ID, invitee, "invitation", caller, id, "Server invitation")
		}); e != nil {
			return nil, e
		}
		return *i, nil
	case "ListServerInvites":
		id := arg("server_id")
		out := []v2ServerInvite{}
		for _, i := range s.db.s.V2Invites {
			if i.ServerID == id && i.InviteeID == caller && i.Status == "pending" {
				out = append(out, *i)
			}
		}
		return out, nil
	case "AcceptServerInvite":
		i := s.db.s.V2Invites[arg("invite_id")]
		if i == nil || i.InviteeID != caller {
			return nil, denied("invitation not found")
		}
		sr, e := getServer(i.ServerID)
		if e != nil {
			return nil, e
		}
		i.Status = "accepted"
		s.v2AddMemberLocked(sr, caller, "agent")
		if e := s.v2PersistLocked("v2_member_add", map[string]string{"server_id": sr.Server.ID, "participant": caller, "role": "agent"}); e != nil {
			return nil, e
		}
		v2Audit(s, sr.Server.ID, "member_joined", caller, caller, "Joined Server by invitation")
		return nil, nil
	case "LeaveServer":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if caller == sr.Server.OwnerID {
			return nil, denied("owner cannot leave Server")
		}
		if !v2IsMember(sr, caller) {
			return nil, denied("Server membership required")
		}
		delete(sr.Members, caller)
		if e := s.v2PersistLocked("v2_member_remove", map[string]string{"server_id": sr.Server.ID, "participant": caller}); e != nil {
			return nil, e
		}
		delete(sr.RolePermissions, caller)
		v2Audit(s, sr.Server.ID, "member_left", caller, caller, "Left Server")
		return nil, nil
	case "RemoveServerMember":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		target := arg("participant_id")
		if !v2Admin(sr, caller) || target == sr.Server.OwnerID {
			return nil, denied("Server administration permission required")
		}
		delete(sr.Members, target)
		if e := s.v2PersistLocked("v2_member_remove", map[string]string{"server_id": sr.Server.ID, "participant": target}); e != nil {
			return nil, e
		}
		v2Audit(s, sr.Server.ID, "member_removed", caller, target, "Removed Server member")
		return nil, nil
	case "ListServerMembers", "FindServerMembers":
		var q struct {
			ServerID    string            `json:"server_id"`
			Name        string            `json:"name"`
			Capability  string            `json:"capability"`
			Harness     string            `json:"harness"`
			Topic       string            `json:"topic"`
			CurrentWork string            `json:"current_work"`
			Online      *bool             `json:"online"`
			Limit       int               `json:"limit"`
			Cursor      uint64            `json:"cursor"`
			Response    v2ResponseOptions `json:"response"`
		}
		if e := v2Map(raw, &q); e != nil {
			return nil, e
		}
		sr, e := memberServer(q.ServerID)
		if e != nil {
			return nil, e
		}
		out := []v2MemberView{}
		for id := range sr.Members {
			v, e := s.v2MemberViewLocked(sr, id)
			if e != nil {
				continue
			}
			if q.Name != "" && !v2Match(v.DisplayName, q.Name) && !v2Match(v.Handle, q.Name) {
				continue
			}
			if q.Capability != "" && !v2Contains(v.Capabilities, q.Capability) {
				continue
			}
			if q.Harness != "" && v.Harness != q.Harness {
				continue
			}
			if q.Topic != "" {
				matchedTopic := false
				for _, topic := range v.CollaborationTopics {
					if v2Match(topic, q.Topic) {
						matchedTopic = true
						break
					}
				}
				if !matchedTopic {
					continue
				}
			}
			if q.CurrentWork != "" && !v2Match(v.CurrentWork, q.CurrentWork) {
				continue
			}
			if q.Online != nil && v.Online != *q.Online {
				continue
			}
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
		out, _, _ = v2Page(out, q.Limit, int(q.Cursor))
		shaped := make([]any, len(out))
		for i := range out {
			shaped[i] = v2ApplySelection(out[i], q.Response)
		}
		return shaped, nil
	case "GetServerMember":
		sr, e := memberServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		v, e := s.v2MemberViewLocked(sr, arg("participant_id"))
		if e != nil {
			return nil, e
		}
		return v2ApplySelection(v, v2Options(m)), nil
	case "ListServerRequests":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if !v2Admin(sr, caller) {
			return nil, denied("Server administration permission required")
		}
		out := []v2ServerRequest{}
		for _, r := range s.db.s.V2Requests {
			if r.ServerID == sr.Server.ID {
				out = append(out, *r)
			}
		}
		return out, nil
	case "ListServerRoles":
		sr, e := memberServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		out := []map[string]any{}
		for id, m := range sr.Members {
			out = append(out, map[string]any{"server_id": sr.Server.ID, "participant": id, "role": m.Role, "permissions": m.Permissions})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["participant"].(string) < out[j]["participant"].(string) })
		return out, nil
	case "SetServerRole":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if !v2Can(sr, caller, "manage_roles") {
			return nil, denied("manage_roles permission required")
		}
		id, role := arg("participant_id"), arg("role")
		if s.db.s.Identities[id] == nil {
			return nil, missing("participant not found")
		}
		if role == "owner" && caller != sr.Server.OwnerID {
			return nil, denied("only owner can assign owner")
		}
		if sr.Members[id] == nil {
			s.v2AddMemberLocked(sr, id, "member")
		}
		sr.Members[id].Role = role
		if e := s.v2PersistLocked("v2_role", map[string]string{"server_id": sr.Server.ID, "participant": id, "role": role}); e != nil {
			return nil, e
		}
		v2Audit(s, sr.Server.ID, "role_changed", caller, id, "Server role changed")
		return nil, nil
	case "UpdateServerPermissions":
		sr, e := getServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if !v2Can(sr, caller, "manage_roles") {
			return nil, denied("manage_roles permission required")
		}
		var p struct {
			Role       string `json:"role"`
			Permission string `json:"permission"`
			Allowed    bool   `json:"allowed"`
		}
		if e = v2Map(raw, &p); e != nil {
			return nil, e
		}
		v2RolePermissions(sr, p.Role)[p.Permission] = p.Allowed
		v2Audit(s, sr.Server.ID, "permissions_changed", caller, p.Role, "Server permissions changed")
		return nil, nil
	case "GetServerAudit":
		sr, e := memberServer(arg("server_id"))
		if e != nil {
			return nil, e
		}
		if !v2Admin(sr, caller) {
			return nil, denied("Server administration permission required")
		}
		var o v2ResponseOptions
		_ = json.Unmarshal(m["response"], &o)
		out := []ActivityEvent{}
		for _, a := range s.db.s.V2Activities {
			if a.ServerID == sr.Server.ID {
				out = append(out, a.Event)
			}
		}
		if o.Limit > 0 && len(out) > o.Limit {
			out = out[len(out)-o.Limit:]
		}
		return out, nil
	case "CreateGroup":
		var p struct {
			ServerID    string `json:"server_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			JoinPolicy  string `json:"join_policy"`
			Private     bool   `json:"private"`
		}
		if e := v2Map(raw, &p); e != nil {
			return nil, e
		}
		sr, e := memberServer(p.ServerID)
		if e != nil {
			return nil, e
		}
		if !v2Can(sr, caller, "manage_groups") && caller != sr.Server.OwnerID {
			return nil, denied("group administration permission required")
		}
		if p.JoinPolicy == "" {
			p.JoinPolicy = "public"
		}
		id := s.db.nextID("group")
		g := &v2GroupRecord{ID: id, ServerID: p.ServerID, Name: p.Name, Description: p.Description, JoinPolicy: p.JoinPolicy, Private: p.Private, OwnerID: caller, Members: map[string]bool{caller: true}, Invites: map[string]string{}, Requests: map[string]string{}}
		if e = commit("v2_group", g, func() { s.db.s.V2Groups[id] = g; v2Audit(s, p.ServerID, "group_created", caller, id, "Group created") }); e != nil {
			return nil, e
		}
		return v2GroupView(g), nil
	case "SendGroupMessageV2Bridge":
		g := s.db.s.V2Groups[arg("group")]
		if g == nil {
			return nil, missing("group not found")
		}
		if !g.Members[caller] {
			// A public group on a public Server is a valid first-contact path
			// for the human UI: message send joins the scoped room atomically.
			sr := s.db.s.Servers[g.ServerID]
			if sr == nil || sr.Server.JoinPolicy != "public" || g.JoinPolicy != "public" {
				return nil, denied("group membership required")
			}
			g.Members[caller] = true
			if !v2IsMember(sr, caller) {
				s.v2AddMemberLocked(sr, caller, "guest")
				_ = s.v2PersistLocked("v2_member_add", map[string]string{"server_id": sr.Server.ID, "participant": caller, "role": "guest"})
			}
		}
		m := &Message{ID: s.db.nextID("msg"), SenderID: caller, GroupID: g.ID, Content: arg("content"), Sequence: s.db.s.Next, CreatedAt: s.db.now()}
		s.db.s.GroupMessages[g.ID] = append(s.db.s.GroupMessages[g.ID], m)
		e := ActivityEvent{Type: "group_message", ID: m.ID, ActorID: caller, TargetID: g.ID, Sequence: m.Sequence, CreatedAt: m.CreatedAt, Summary: "group message", Message: m}
		s.db.s.V2Activities = append(s.db.s.V2Activities, v2ActivityRecord{ServerID: g.ServerID, Event: e})
		s.db.recordActivity(e)
		return *m, nil
	case "GetGroupHistoryV2Bridge":
		g := s.db.s.V2Groups[arg("group")]
		if g == nil {
			return nil, missing("group not found")
		}
		if !g.Members[caller] {
			return nil, denied("group membership required")
		}
		out := []Message{}
		for _, m := range s.db.s.GroupMessages[g.ID] {
			out = append(out, *m)
		}
		return out, nil
	case "ReactV2Bridge":
		p := s.db.s.V2Posts[arg("target")]
		if p == nil {
			return nil, missing("reaction target not found")
		}
		if _, e := memberServer(p.ServerID); e != nil {
			return nil, e
		}
		if s.db.s.Reactions[p.Post.ID] == nil {
			s.db.s.Reactions[p.Post.ID] = map[string]int{}
		}
		s.db.s.Reactions[p.Post.ID][arg("reaction")]++
		v2Audit(s, p.ServerID, "reaction", caller, p.Post.ID, "Reaction")
		return nil, nil
	case "UpdateGroup":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		sr, e := memberServer(g.ServerID)
		if e != nil {
			return nil, e
		}
		if g.OwnerID != caller && !v2Can(sr, caller, "manage_groups") {
			return nil, denied("group administration permission required")
		}
		var p struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			JoinPolicy  *string `json:"join_policy"`
		}
		_ = v2Map(rawMapField(m, "patch"), &p)
		if p.Name != nil {
			g.Name = *p.Name
		}
		if p.Description != nil {
			g.Description = *p.Description
		}
		if p.JoinPolicy != nil {
			g.JoinPolicy = *p.JoinPolicy
		}
		v2Audit(s, g.ServerID, "group_updated", caller, g.ID, "Group updated")
		return v2GroupView(g), nil
	case "DeleteGroup":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		sr, e := memberServer(g.ServerID)
		if e != nil {
			return nil, e
		}
		if g.OwnerID != caller && !v2Can(sr, caller, "manage_groups") {
			return nil, denied("group administration permission required")
		}
		delete(s.db.s.V2Groups, g.ID)
		v2Audit(s, g.ServerID, "group_deleted", caller, g.ID, "Group deleted")
		return nil, nil
	case "DiscoverGroups", "ListGroups":
		var q struct {
			ServerID string            `json:"server_id"`
			Query    string            `json:"query"`
			Limit    int               `json:"limit"`
			Cursor   uint64            `json:"cursor"`
			Response v2ResponseOptions `json:"response"`
		}
		_ = v2Map(raw, &q)
		sr, e := memberServer(q.ServerID)
		if e != nil {
			return nil, e
		}
		out := []Group{}
		for _, g := range s.db.s.V2Groups {
			if g.ServerID != sr.Server.ID {
				continue
			}
			if method == "ListGroups" && !g.Members[caller] {
				continue
			}
			if method == "DiscoverGroups" && g.Private && !g.Members[caller] {
				continue
			}
			if !v2Match(g.Name, q.Query) {
				continue
			}
			out = append(out, v2GroupView(g))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out, _, _ = v2Page(out, q.Limit, int(q.Cursor))
		return out, nil
	case "JoinGroup":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		_, e := memberServer(g.ServerID)
		if e != nil {
			return nil, e
		}
		if g.Members[caller] {
			return nil, nil
		}
		switch g.JoinPolicy {
		case "public":
		case "approval-required":
			if g.Requests[caller] != "approved" {
				return nil, denied("approval required; request group access")
			}
		case "invite-only":
			if g.Invites[caller] == "" {
				return nil, denied("invitation required; accept invite")
			}
		default:
			return nil, denied("private group")
		}
		g.Members[caller] = true
		delete(g.Invites, caller)
		v2Audit(s, g.ServerID, "group_member_joined", caller, g.ID, "Joined group")
		return nil, nil
	case "RequestGroupAccess":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		if _, e := memberServer(g.ServerID); e != nil {
			return nil, e
		}
		if id := g.Requests[caller]; id != "" {
			return map[string]any{"id": id, "group_id": g.ID, "requester": caller, "status": "pending"}, nil
		}
		id := s.db.nextID("group-request")
		g.Requests[caller] = id
		s.v2NotifyLocked(g.ServerID, s.db.s.Servers[g.ServerID].Server.OwnerID, "group_request", caller, id, "Group access request")
		return map[string]any{"id": id, "group_id": g.ID, "requester": caller, "reason": arg("reason"), "status": "pending"}, nil
	case "ApproveGroupRequest", "RejectGroupRequest":
		id := arg("request_id")
		for _, g := range s.db.s.V2Groups {
			for user, rid := range g.Requests {
				if rid == id {
					sr, e := memberServer(g.ServerID)
					if e != nil {
						return nil, e
					}
					if !v2Admin(sr, caller) {
						return nil, denied("group administration permission required")
					}
					if method == "ApproveGroupRequest" {
						g.Requests[user] = "approved"
					} else {
						g.Requests[user] = "rejected"
					}
					return nil, nil
				}
			}
		}
		return nil, missing("group request not found")
	case "InviteToGroup":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		sr, e := memberServer(g.ServerID)
		if e != nil {
			return nil, e
		}
		if g.OwnerID != caller && !v2Can(sr, caller, "manage_groups") {
			return nil, denied("group administration permission required")
		}
		id := s.db.nextID("group-invite")
		g.Invites[arg("participant_id")] = id
		s.v2NotifyLocked(g.ServerID, arg("participant_id"), "invitation", caller, id, "Group invitation")
		return map[string]any{"id": id, "group_id": g.ID, "invitee_id": arg("participant_id"), "status": "pending"}, nil
	case "AcceptGroupInvite":
		id := arg("invite_id")
		for _, g := range s.db.s.V2Groups {
			for user, invite := range g.Invites {
				if invite == id && user == caller {
					g.Members[caller] = true
					delete(g.Invites, user)
					return nil, nil
				}
			}
		}
		return nil, denied("group invitation not found")
	case "LeaveGroup":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		if !g.Members[caller] {
			return nil, denied("group membership required")
		}
		if caller == g.OwnerID {
			return nil, denied("owner cannot leave group")
		}
		delete(g.Members, caller)
		return nil, nil
	case "RemoveGroupMember":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		sr, e := memberServer(g.ServerID)
		if e != nil {
			return nil, e
		}
		if g.OwnerID != caller && !v2Can(sr, caller, "manage_groups") {
			return nil, denied("group administration permission required")
		}
		delete(g.Members, arg("participant_id"))
		return nil, nil
	case "ListGroupMembers":
		g := s.db.s.V2Groups[arg("group_id")]
		if g == nil {
			return nil, missing("group not found")
		}
		if !g.Members[caller] {
			return nil, denied("group membership required")
		}
		sr := s.db.s.Servers[g.ServerID]
		out := []v2MemberView{}
		for id := range g.Members {
			v, e := s.v2MemberViewLocked(sr, id)
			if e == nil {
				out = append(out, v)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
		return out, nil
	case "CreatePost":
		var p struct {
			ServerID   string   `json:"server_id"`
			GroupID    string   `json:"group_id"`
			Title      string   `json:"title"`
			Content    string   `json:"content"`
			Visibility string   `json:"visibility"`
			Mentions   []string `json:"mentions"`
		}
		if e := v2Map(raw, &p); e != nil {
			return nil, e
		}
		_, e := memberServer(p.ServerID)
		if e != nil {
			return nil, e
		}
		if p.GroupID != "" {
			g := s.db.s.V2Groups[p.GroupID]
			if g == nil || !g.Members[caller] {
				return nil, denied("group membership required")
			}
		}
		if p.Visibility == "" {
			p.Visibility = "server-wide"
		}
		id := s.db.nextID("post")
		post := &v2PostRecord{Post: Post{ID: id, AuthorID: caller, Title: p.Title, Content: p.Content, CreatedAt: s.db.now()}, ServerID: p.ServerID, GroupID: p.GroupID, Visibility: p.Visibility, Mentions: p.Mentions}
		if e = commit("v2_post", post, func() {
			s.db.s.V2Posts[id] = post
			s.db.s.Posts[id] = &post.Post
			v2Audit(s, p.ServerID, "post", caller, id, p.Title)
		}); e != nil {
			return nil, e
		}
		for _, u := range p.Mentions {
			s.v2NotifyLocked(p.ServerID, u, "mention", caller, id, "You were mentioned")
		}
		return v2PostViewOf(post), nil
	case "EditPost":
		p := s.db.s.V2Posts[arg("post_id")]
		if p == nil {
			return nil, missing("post not found")
		}
		sr, e := memberServer(p.ServerID)
		if e != nil {
			return nil, e
		}
		if p.Post.AuthorID != caller && !v2Can(sr, caller, "moderate_posts") {
			return nil, denied("post edit permission required")
		}
		var x struct {
			Title      *string `json:"title"`
			Content    *string `json:"content"`
			Visibility *string `json:"visibility"`
		}
		_ = v2Map(rawMapField(m, "patch"), &x)
		if x.Title != nil {
			p.Post.Title = *x.Title
		}
		if x.Content != nil {
			p.Post.Content = *x.Content
		}
		if x.Visibility != nil {
			p.Visibility = *x.Visibility
		}
		return v2PostViewOf(p), nil
	case "DiscoverPosts", "SearchPosts":
		var q struct {
			ServerID   string            `json:"server_id"`
			GroupID    string            `json:"group_id"`
			Query      string            `json:"query"`
			Visibility string            `json:"visibility"`
			Limit      int               `json:"limit"`
			Cursor     uint64            `json:"cursor"`
			Response   v2ResponseOptions `json:"response"`
		}
		_ = v2Map(raw, &q)
		if _, e := memberServer(q.ServerID); e != nil {
			return nil, e
		}
		out := []v2PostView{}
		for _, p := range s.db.s.V2Posts {
			if p.ServerID != q.ServerID {
				continue
			}
			if p.GroupID != "" {
				g := s.db.s.V2Groups[p.GroupID]
				if g == nil || !g.Members[caller] {
					continue
				}
			}
			if q.GroupID != "" && p.GroupID != q.GroupID {
				continue
			}
			if q.Query != "" && !v2Match(p.Post.Title, q.Query) && !v2Match(p.Post.Content, q.Query) {
				continue
			}
			if q.Visibility != "" && p.Visibility != q.Visibility {
				continue
			}
			out = append(out, v2PostViewOf(p))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		out, _, _ = v2Page(out, q.Limit, int(q.Cursor))
		shaped := make([]any, len(out))
		for i := range out {
			shaped[i] = v2ApplySelection(out[i], q.Response)
		}
		return shaped, nil
	case "GetThread":
		p := s.db.s.V2Posts[arg("post_id")]
		if p == nil {
			p = s.db.s.V2Posts[arg("post")]
		}
		if p == nil {
			return nil, missing("post not found")
		}
		if _, e := memberServer(p.ServerID); e != nil {
			return nil, e
		}
		if p.GroupID != "" && !s.db.s.V2Groups[p.GroupID].Members[caller] {
			return nil, denied("group membership required")
		}
		return s.threadLocked(p.Post.ID), nil
	case "Comment":
		parent := arg("post_or_comment")
		postID := parent
		if c := s.db.s.Comments[parent]; c != nil {
			postID = s.db.s.CommentPosts[parent]
		}
		p := s.db.s.V2Posts[postID]
		if p == nil {
			return nil, missing("post or comment not found")
		}
		if _, e := memberServer(p.ServerID); e != nil {
			return nil, e
		}
		if p.GroupID != "" && !s.db.s.V2Groups[p.GroupID].Members[caller] {
			return nil, denied("group membership required")
		}
		content := arg("content")
		c := &CommentNode{ID: s.db.nextID("comment"), AuthorID: caller, Content: content, CreatedAt: s.db.now()}
		if parent != postID {
			c.ParentID = parent
		}
		s.db.s.Comments[c.ID] = c
		s.db.s.CommentPosts[c.ID] = postID
		s.db.s.CommentsByPost[postID] = append(s.db.s.CommentsByPost[postID], c.ID)
		if e := s.v2PersistLocked("v2_comment", map[string]any{"post": postID, "comment": c}); e != nil {
			return nil, e
		}
		v2Audit(s, p.ServerID, "comment", caller, postID, "Thread reply")
		for _, u := range v2Strings(m, "mentions") {
			s.v2NotifyLocked(p.ServerID, u, "mention", caller, postID, "You were mentioned")
		}
		for u := range s.db.s.Followers[postID] {
			s.v2NotifyLocked(p.ServerID, u, "reply", caller, postID, "Followed thread activity")
		}
		return *c, nil
	case "SharePost":
		p := s.db.s.V2Posts[arg("post_id")]
		if p == nil {
			return nil, missing("post not found")
		}
		if _, e := memberServer(p.ServerID); e != nil {
			return nil, e
		}
		return nil, nil
	case "ListNotifications":
		var q v2NotificationQuery
		_ = v2Map(raw, &q)
		out := []v2Notification{}
		for _, n := range s.db.s.V2Notifications[caller] {
			if n.Sequence <= q.AfterSequence || (q.UnreadOnly && n.Read) {
				continue
			}
			out = append(out, *n)
		}
		if q.Limit > 0 && len(out) > q.Limit {
			out = out[:q.Limit]
		}
		return out, nil
	case "MarkNotificationsRead":
		for _, id := range v2Strings(m, "notification_ids") {
			for _, n := range s.db.s.V2Notifications[caller] {
				if n.ID == id {
					n.Read = true
				}
			}
		}
		return nil, nil
	case "ApplyManifest":
		var x struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Server     string `json:"server"`
			Identity   struct {
				Name    string   `json:"name"`
				Profile *Profile `json:"profile"`
			} `json:"identity"`
			Membership struct {
				Join              string `json:"join"`
				RequestIfRequired bool   `json:"requestIfRequired"`
				AcceptInvitation  bool   `json:"acceptInvitation"`
			} `json:"membership"`
			Discover struct {
				Capabilities []string `json:"capabilities"`
				Harness      string   `json:"harness"`
				Limit        int      `json:"limit"`
			} `json:"discover"`
		}
		if e := v2Map(raw, &x); e != nil {
			return nil, e
		}
		if x.APIVersion != "harnesstalkie/v2" || x.Kind != "Session" || x.Server == "" || x.Identity.Name == "" {
			return nil, bad("invalid V2 Session manifest")
		}
		sr, e := getServer(x.Server)
		if e != nil {
			return nil, e
		}
		if x.Identity.Profile != nil {
			p := *x.Identity.Profile
			if p.DisplayName == "" {
				p.DisplayName = x.Identity.Name
			}
			s.db.s.Identities[caller].Profile = p
			s.db.s.Identities[caller].DisplayName = p.DisplayName
		}
		membership := "already-member"
		var req *v2ServerRequest
		if !v2IsMember(sr, caller) {
			if sr.Server.JoinPolicy == "public" || x.Membership.Join == "always" {
				s.v2AddMemberLocked(sr, caller, "agent")
				membership = "joined"
			} else if x.Membership.RequestIfRequired {
				for _, r := range s.db.s.V2Requests {
					if r.ServerID == sr.Server.ID && r.Requester == caller {
						req = r
					}
				}
				if req == nil {
					id := s.db.nextID("request")
					req = &v2ServerRequest{ID: id, ServerID: sr.Server.ID, Requester: caller, Status: "pending", CreatedAt: s.db.now().Format(time.RFC3339Nano)}
					s.db.s.V2Requests[id] = req
				}
				membership = "access-requested"
			} else {
				return nil, denied("membership required")
			}
		}
		parts := []Participant{}
		for id := range sr.Members {
			v, _ := s.v2MemberViewLocked(sr, id)
			if x.Discover.Capabilities == nil || v2Contains(v.Capabilities, x.Discover.Capabilities[0]) {
				parts = append(parts, v.Participant)
			}
		}
		groups := []Group{}
		for _, g := range s.db.s.V2Groups {
			if g.ServerID == sr.Server.ID && g.Members[caller] {
				groups = append(groups, v2GroupView(g))
			}
		}
		return v2ManifestResult{Identity: Identity{ID: caller, DisplayName: s.db.s.Identities[caller].DisplayName}, Server: v2ServerView(sr), Membership: membership, AccessRequest: req, Participants: parts, Groups: groups, Cursor: s.db.s.Next}, nil
	case "Batch":
		var b struct {
			Operations []struct {
				ID        string         `json:"id"`
				Operation string         `json:"operation"`
				Params    map[string]any `json:"params"`
			} `json:"operations"`
			Mode string `json:"mode"`
		}
		if e := v2Map(raw, &b); e != nil {
			return nil, e
		}
		results := []map[string]any{}
		for _, op := range b.Operations {
			r, e := s.v2DispatchLocked(ctx, caller, op.Operation, mustJSON(op.Params))
			x := map[string]any{"id": op.ID, "success": e == nil}
			if e != nil {
				x["error"] = map[string]any{"code": "operation_failed", "message": e.Error(), "recoverable": true, "action": "inspect error and retry"}
			} else if r != nil {
				if _, ok := r.(map[string]any); ok {
					x["result"] = r
				} else {
					x["result"] = map[string]any{"value": r}
				}
			}
			results = append(results, x)
		}
		return map[string]any{"results": results, "round_trips": 1}, nil
	case "Sync":
		var q struct {
			ServerID    string            `json:"server_id"`
			AfterCursor uint64            `json:"after_cursor"`
			Limit       int               `json:"limit"`
			Include     []string          `json:"include"`
			Response    v2ResponseOptions `json:"response"`
		}
		_ = v2Map(raw, &q)
		sr, e := memberServer(q.ServerID)
		if e != nil {
			return nil, e
		}
		events := []ActivityEvent{}
		messages := []Message{}
		comments := []CommentNode{}
		notifications := []v2Notification{}
		for _, a := range s.db.s.V2Activities {
			if a.ServerID != sr.Server.ID || a.Event.Sequence <= q.AfterCursor {
				continue
			}
			events = append(events, a.Event)
			if a.Event.Type == "group_message" && a.Event.Message != nil {
				messages = append(messages, *a.Event.Message)
			}
			if a.Event.Type == "comment" {
				if c := s.db.s.Comments[a.Event.TargetID]; c != nil {
					comments = append(comments, *c)
				}
			}
		}
		for _, n := range s.db.s.V2Notifications[caller] {
			if n.Sequence > q.AfterCursor {
				notifications = append(notifications, *n)
			}
		}
		cursor := s.db.s.Next
		if q.Limit > 0 && len(events) > q.Limit {
			events = events[:q.Limit]
		}
		return map[string]any{"cursor": cursor, "events": events, "messages": messages, "notifications": notifications, "more": false, "comments": comments}, nil
	default:
		return nil, missing("unknown V2 method")
	}
}

func rawMapField(m map[string]json.RawMessage, key string) json.RawMessage {
	if v := m[key]; len(v) > 0 {
		return v
	}
	return json.RawMessage("{}")
}

func isDiscoveryMethod(method string) bool {
	switch method {
	case "DiscoverProtocol", "GetSchema", "GetHelp", "ListPresets", "ApplyPreset", "ListTransports":
		return true
	}
	return false
}

func v2DiscoveryRequest(method string, raw json.RawMessage) (any, error) {
	ops := []string{"CreateServer", "GetServer", "UpdateServer", "ListServers", "DiscoverServers", "JoinServer", "RequestServerAccess", "ApproveServerRequest", "RejectServerRequest", "InviteToServer", "AcceptServerInvite", "LeaveServer", "RemoveServerMember", "ListServerMembers", "GetServerMember", "ListServerRequests", "ListServerInvites", "FindServerMembers", "ListServerRoles", "SetServerRole", "UpdateServerPermissions", "GetServerAudit", "CreateGroup", "UpdateGroup", "DeleteGroup", "DiscoverGroups", "ListGroups", "JoinGroup", "RequestGroupAccess", "ApproveGroupRequest", "RejectGroupRequest", "InviteToGroup", "AcceptGroupInvite", "LeaveGroup", "RemoveGroupMember", "ListGroupMembers", "CreatePost", "EditPost", "Comment", "GetThread", "DiscoverPosts", "SearchPosts", "SharePost", "ListNotifications", "MarkNotificationsRead", "ApplyManifest", "Batch", "Sync"}
	switch method {
	case "DiscoverProtocol":
		return map[string]any{"protocol": "harnesstalkie", "version": "2.0", "capabilities": []string{"servers", "membership", "permissions", "groups", "forums", "notifications", "manifests", "batch", "delta-sync", "response-shaping", "encrypted-jsonrpc"}, "operations": ops, "transports": []string{"jsonrpc"}, "schema_urls": []string{"/schema/harnesstalkie/v2"}, "documentation": []string{"/help"}, "auth_methods": []string{"bearer-session"}, "usage_hints": []string{"discover Server metadata before joining", "ApplyManifest bootstraps a session", "Sync returns authorized changes since a cursor"}}, nil
	case "GetSchema":
		return map[string]any{"name": "harnesstalkie/v2", "version": "2.0", "schema": map[string]any{"type": "object", "required": []string{"apiVersion", "kind", "server", "identity"}, "operations": ops, "join_policy": []string{"public", "approval-required", "invite-only", "closed"}, "visibility": []string{"server-wide", "group-only", "directly-shared", "private"}}}, nil
	case "GetHelp":
		return map[string]any{"commands": []string{"discover", "server", "members", "agents", "groups", "posts", "dm", "inbox", "requests", "invites", "roles", "permissions", "security", "status", "join"}, "examples": []string{"discover --json", "join SERVER_ID --json", "members --capability simulation --compact", "dm send PARTICIPANT_ID MESSAGE"}, "manifest": "ApplyManifest", "batch": "Batch"}, nil
	case "ListPresets":
		return []map[string]any{{"name": "minimal", "description": "identity and presence", "manifest": map[string]any{"apiVersion": "harnesstalkie/v2", "kind": "Session", "server": "server", "identity": map[string]any{"name": "agent"}}}, {"name": "collaborator", "description": "discover and synchronize collaboration", "manifest": map[string]any{"apiVersion": "harnesstalkie/v2", "kind": "Session", "server": "server", "identity": map[string]any{"name": "agent"}, "membership": map[string]any{"join": "if-allowed", "requestIfRequired": true}, "discover": map[string]any{"limit": 5}, "sync": map[string]any{"inbox": true, "mentions": true}}}}, nil
	case "ApplyPreset":
		var p struct {
			Preset string `json:"preset"`
		}
		_ = v2Map(raw, &p)
		if p.Preset != "minimal" && p.Preset != "collaborator" {
			return nil, missing("preset not found")
		}
		return map[string]any{"preset": p.Preset, "effective": map[string]any{"apiVersion": "harnesstalkie/v2", "kind": "Session", "server": "server", "identity": map[string]any{"name": "agent"}}}, nil
	case "ListTransports":
		return []map[string]any{{"name": "jsonrpc", "read": true, "write": true, "streaming": false, "address": "/rpc"}}, nil
	}
	return nil, missing("unknown discovery method")
}
