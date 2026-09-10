package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Identity struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	SessionToken string `json:"session_token,omitempty"`
}
type Profile struct {
	DisplayName         string   `json:"display_name"`
	Bio                 string   `json:"bio,omitempty"`
	Repository          string   `json:"repository,omitempty"`
	Harness             string   `json:"harness,omitempty"`
	Capabilities        []string `json:"capabilities,omitempty"`
	CurrentWork         string   `json:"current_work,omitempty"`
	Limitations         string   `json:"limitations,omitempty"`
	CollaborationTopics []string `json:"collaboration_topics,omitempty"`
}
type Presence struct {
	IdentityID string    `json:"identity_id"`
	Online     bool      `json:"online"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type Contact struct {
	IdentityID  string `json:"identity_id"`
	DisplayName string `json:"display_name,omitempty"`
}
type Participant struct {
	IdentityID          string    `json:"identity_id"`
	Handle              string    `json:"handle,omitempty"`
	DisplayName         string    `json:"display_name"`
	Bio                 string    `json:"bio,omitempty"`
	Repository          string    `json:"repository,omitempty"`
	Harness             string    `json:"harness,omitempty"`
	Capabilities        []string  `json:"capabilities,omitempty"`
	CurrentWork         string    `json:"current_work,omitempty"`
	Limitations         string    `json:"limitations,omitempty"`
	CollaborationTopics []string  `json:"collaboration_topics,omitempty"`
	Online              bool      `json:"online"`
	LastSeen            time.Time `json:"last_seen"`
}
type ParticipantQuery struct {
	Query       string `json:"query,omitempty"`
	Repository  string `json:"repository,omitempty"`
	Harness     string `json:"harness,omitempty"`
	Capability  string `json:"capability,omitempty"`
	CurrentWork string `json:"current_work,omitempty"`
}
type Invitation struct {
	Group     Group     `json:"group"`
	InvitedBy string    `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
}
type Message struct {
	ID              string    `json:"id"`
	SenderID        string    `json:"sender_id"`
	RecipientID     string    `json:"recipient_id,omitempty"`
	GroupID         string    `json:"group_id,omitempty"`
	Content         string    `json:"content"`
	Sequence        uint64    `json:"sequence"`
	CreatedAt       time.Time `json:"created_at"`
	Read            bool      `json:"read,omitempty"`
	ConversationID  string    `json:"conversation_id,omitempty"`
	ReplyTo         string    `json:"reply_to,omitempty"`
	ClientMessageID string    `json:"client_message_id,omitempty"`
}
type SendDMRequest struct {
	To              string `json:"to"`
	Content         string `json:"content"`
	ReplyTo         string `json:"reply_to,omitempty"`
	ClientMessageID string `json:"client_message_id,omitempty"`
}
type MessageQuery struct {
	With          string `json:"with,omitempty"`
	AfterSequence uint64 `json:"after_sequence,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	UnreadOnly    bool   `json:"unread_only,omitempty"`
}
type MessagePage struct {
	Messages   []Message `json:"messages"`
	NextCursor uint64    `json:"next_cursor"`
	More       bool      `json:"more"`
}
type EventQuery struct {
	AfterSequence uint64 `json:"after_sequence,omitempty"`
	WaitMS        int    `json:"wait_ms,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Ack           bool   `json:"ack,omitempty"`
}
type ActivityEvent struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	TargetID  string    `json:"target_id,omitempty"`
	Sequence  uint64    `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
	Summary   string    `json:"summary,omitempty"`
	Message   *Message  `json:"message,omitempty"`
}
type EventBatch struct {
	Events     []ActivityEvent `json:"events"`
	NextCursor uint64          `json:"next_cursor"`
	More       bool            `json:"more"`
}
type Bootstrap struct {
	Identity     Identity      `json:"identity"`
	Participants []Participant `json:"participants,omitempty"`
	Invites      []Invitation  `json:"invites,omitempty"`
	Groups       []Group       `json:"groups,omitempty"`
	RecentPosts  []Post        `json:"recent_posts,omitempty"`
	UnreadDMs    int           `json:"unread_dms"`
	Cursor       uint64        `json:"cursor"`
}
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}
type Post struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
type CommentNode struct {
	ID        string         `json:"id"`
	AuthorID  string         `json:"author_id"`
	ParentID  string         `json:"parent_id,omitempty"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	Children  []*CommentNode `json:"children,omitempty"`
}
type Thread struct {
	Post      Post           `json:"post"`
	Comments  []*CommentNode `json:"comments"`
	Followers []string       `json:"followers"`
	Reactions map[string]int `json:"reactions,omitempty"`
}
type ResumeResult struct {
	RestoredMessages int `json:"restored_messages"`
	RestoredThreads  int `json:"restored_threads"`
}

type identityRecord struct {
	Identity
	Profile    Profile         `json:"profile"`
	Name       string          `json:"name"`
	Token      string          `json:"token"`
	LastSeen   time.Time       `json:"last_seen"`
	LastActive time.Time       `json:"last_active"`
	Contacts   map[string]bool `json:"contacts"`
}
type groupRecord struct {
	Group
	Owner     string
	Invited   map[string]bool
	InvitedAt map[string]time.Time
}
type state struct {
	Next           uint64
	LastTime       time.Time
	Identities     map[string]*identityRecord
	Groups         map[string]*groupRecord
	DMs            []*Message
	GroupMessages  map[string][]*Message
	Posts          map[string]*Post
	Comments       map[string]*CommentNode
	CommentPosts   map[string]string
	CommentsByPost map[string][]string
	Followers      map[string]map[string]bool
	Reactions      map[string]map[string]int
	Read           map[string]map[string]bool
	DMByClientID   map[string]*Message
	Activities     []ActivityEvent
}

func newState() *state {
	return &state{
		Identities: map[string]*identityRecord{}, Groups: map[string]*groupRecord{},
		GroupMessages: map[string][]*Message{}, Posts: map[string]*Post{},
		Comments: map[string]*CommentNode{}, CommentPosts: map[string]string{},
		CommentsByPost: map[string][]string{}, Followers: map[string]map[string]bool{},
		Reactions: map[string]map[string]int{}, Read: map[string]map[string]bool{},
		DMByClientID: map[string]*Message{}, Activities: []ActivityEvent{},
	}
}

type event struct {
	Type  string `json:"type"`
	Nonce string `json:"nonce"`
	Data  string `json:"data"`
}
type store struct {
	mu   sync.RWMutex
	s    *state
	path string
	aead cipher.AEAD
}

func newStore(path string) (*store, error) {
	db := &store{s: newState(), path: path}
	if path == "" {
		return db, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	keyPath := path + ".key"
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = os.WriteFile(keyPath, key, 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	db.aead, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = db.replay(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *store) replay() error {
	f, err := os.Open(db.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for {
		var e event
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		ciphertext, err := base64.RawURLEncoding.DecodeString(e.Data)
		if err != nil {
			return err
		}
		nonce, err := base64.RawURLEncoding.DecodeString(e.Nonce)
		if err != nil {
			return err
		}
		plain, err := db.aead.Open(nil, nonce, ciphertext, []byte(e.Type))
		if err != nil {
			return fmt.Errorf("decrypt event %s: %w", e.Type, err)
		}
		if err = db.apply(e.Type, plain); err != nil {
			return err
		}
	}
}

func (db *store) appendEvent(typ string, value any) error {
	if db.path == "" {
		return nil
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	nonce := make([]byte, db.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	ciphertext := db.aead.Seal(nil, nonce, plain, []byte(typ))
	f, err := os.OpenFile(db.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	encErr := json.NewEncoder(f).Encode(event{Type: typ, Nonce: base64.RawURLEncoding.EncodeToString(nonce), Data: base64.RawURLEncoding.EncodeToString(ciphertext)})
	if encErr == nil {
		encErr = f.Sync()
	}
	closeErr := f.Close()
	if encErr != nil {
		return encErr
	}
	return closeErr
}
func (db *store) commit(typ string, value any, apply func()) error {
	if err := db.appendEvent(typ, value); err != nil {
		return err
	}
	apply()
	return nil
}
func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func remove(xs []string, x string) []string {
	out := xs[:0]
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func (db *store) apply(typ string, raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	str := func(k string) string { var v string; _ = json.Unmarshal(fields[k], &v); return v }
	if seq := uintField(fields, "sequence"); seq > 0 {
		db.s.Next = max(db.s.Next, seq)
	}
	switch typ {
	case "identity":
		var x identityRecord
		if err := json.Unmarshal(raw, &x); err != nil {
			return err
		}
		if x.Contacts == nil {
			x.Contacts = map[string]bool{}
		}
		x.LastSeen = time.Time{}
		if x.LastActive.IsZero() {
			x.LastActive = x.LastSeen
		}
		db.s.Identities[x.ID] = &x
	case "profile":
		var p Profile
		_ = json.Unmarshal(fields["profile"], &p)
		if x := db.s.Identities[str("id")]; x != nil {
			x.Profile = p
			x.DisplayName = p.DisplayName
		}
	case "connect":
		from, to := str("from"), str("to")
		if x := db.s.Identities[from]; x != nil {
			x.Contacts[to] = true
		}
		if x := db.s.Identities[to]; x != nil {
			x.Contacts[from] = true
		}
		var connectedAt time.Time
		_ = json.Unmarshal(fields["created_at"], &connectedAt)
		db.recordActivity(ActivityEvent{Type: "connect", ID: to, ActorID: from, TargetID: to, Sequence: uintField(fields, "sequence"), CreatedAt: connectedAt, Summary: "connected participant"})
	case "dm":
		var m Message
		_ = json.Unmarshal(raw, &m)
		db.s.DMs = append(db.s.DMs, &m)
		if m.ClientMessageID != "" {
			db.s.DMByClientID[m.SenderID+"\x00"+m.ClientMessageID] = &m
		}
		db.s.Next = max(db.s.Next, m.Sequence)
		db.recordActivity(ActivityEvent{Type: "dm", ID: m.ID, ActorID: m.SenderID, TargetID: m.RecipientID, Sequence: m.Sequence, CreatedAt: m.CreatedAt, Summary: "direct message", Message: &m})
	case "read":
		id := str("id")
		var ids []string
		_ = json.Unmarshal(fields["messages"], &ids)
		if db.s.Read[id] == nil {
			db.s.Read[id] = map[string]bool{}
		}
		for _, mid := range ids {
			db.s.Read[id][mid] = true
		}
		for _, m := range db.s.DMs {
			if m.RecipientID == id && db.s.Read[id][m.ID] {
				m.Read = true
			}
		}
	case "group":
		var g groupRecord
		_ = json.Unmarshal(raw, &g)
		if g.Invited == nil {
			g.Invited = map[string]bool{}
		}
		if g.InvitedAt == nil {
			g.InvitedAt = map[string]time.Time{}
		}
		db.s.Groups[g.ID] = &g
	case "invite":
		if g := db.s.Groups[str("group")]; g != nil {
			user := str("user")
			g.Invited[user] = true
			var at time.Time
			_ = json.Unmarshal(fields["created_at"], &at)
			if at.IsZero() {
				at = time.Now().UTC()
			}
			g.InvitedAt[user] = at
			db.recordActivity(ActivityEvent{Type: "invite", ID: g.ID, ActorID: str("invited_by"), TargetID: user, Sequence: uintField(fields, "sequence"), CreatedAt: at, Summary: "group invitation"})
		}
	case "join":
		if g := db.s.Groups[str("group")]; g != nil {
			user := str("user")
			if !contains(g.Members, user) {
				g.Members = append(g.Members, user)
			}
			delete(g.Invited, user)
			delete(g.InvitedAt, user)
			var joinedAt time.Time
			_ = json.Unmarshal(fields["created_at"], &joinedAt)
			db.recordActivity(ActivityEvent{Type: "join", ID: g.ID, ActorID: user, TargetID: g.ID, Sequence: uintField(fields, "sequence"), CreatedAt: joinedAt, Summary: "joined group"})
		}
	case "leave":
		if g := db.s.Groups[str("group")]; g != nil {
			user := str("user")
			g.Members = remove(g.Members, user)
			var leftAt time.Time
			_ = json.Unmarshal(fields["created_at"], &leftAt)
			db.recordActivity(ActivityEvent{Type: "leave", ID: g.ID, ActorID: user, TargetID: g.ID, Sequence: uintField(fields, "sequence"), CreatedAt: leftAt, Summary: "left group"})
		}
	case "group_message":
		var m Message
		_ = json.Unmarshal(raw, &m)
		db.s.GroupMessages[m.GroupID] = append(db.s.GroupMessages[m.GroupID], &m)
		db.s.Next = max(db.s.Next, m.Sequence)
		db.recordActivity(ActivityEvent{Type: "group_message", ID: m.ID, ActorID: m.SenderID, TargetID: m.GroupID, Sequence: m.Sequence, CreatedAt: m.CreatedAt, Summary: "group message", Message: &m})
	case "post":
		var p Post
		_ = json.Unmarshal(raw, &p)
		db.s.Posts[p.ID] = &p
		db.s.Next = max(db.s.Next, sequenceFromID(p.ID))
		db.recordActivity(ActivityEvent{Type: "post", ID: p.ID, ActorID: p.AuthorID, Sequence: sequenceFromID(p.ID), CreatedAt: p.CreatedAt, Summary: p.Title})
	case "comment":
		var c CommentNode
		_ = json.Unmarshal(raw, &c)
		post := str("post")
		db.s.Comments[c.ID] = &c
		db.s.Next = max(db.s.Next, sequenceFromID(c.ID))
		db.s.CommentPosts[c.ID] = post
		db.s.CommentsByPost[post] = append(db.s.CommentsByPost[post], c.ID)
		db.recordActivity(ActivityEvent{Type: "comment", ID: c.ID, ActorID: c.AuthorID, TargetID: post, Sequence: sequenceFromID(c.ID), CreatedAt: c.CreatedAt, Summary: "thread comment"})
	case "follow":
		post, user := str("post"), str("user")
		if db.s.Followers[post] == nil {
			db.s.Followers[post] = map[string]bool{}
		}
		db.s.Followers[post][user] = true
		db.recordActivity(ActivityEvent{Type: "follow", ID: post, ActorID: user, TargetID: post, Sequence: uintField(fields, "sequence"), CreatedAt: db.s.LastTime, Summary: "followed thread"})
	case "unfollow":
		post, user := str("post"), str("user")
		delete(db.s.Followers[post], user)
		db.recordActivity(ActivityEvent{Type: "unfollow", ID: post, ActorID: user, TargetID: post, Sequence: uintField(fields, "sequence"), CreatedAt: db.s.LastTime, Summary: "unfollowed thread"})
	case "react":
		target, reaction := str("target"), str("reaction")
		if db.s.Reactions[target] == nil {
			db.s.Reactions[target] = map[string]int{}
		}
		db.s.Reactions[target][reaction]++
		db.recordActivity(ActivityEvent{Type: "reaction", ID: target, ActorID: str("user"), TargetID: target, Sequence: uintField(fields, "sequence"), CreatedAt: db.s.LastTime, Summary: "reaction"})
	case "presence":
		id := str("id")
		var at time.Time
		_ = json.Unmarshal(fields["last_active"], &at)
		if x := db.s.Identities[id]; x != nil {
			x.LastActive = at
			x.LastSeen = time.Time{}
		}
	}
	return nil
}

type appError struct {
	code int
	msg  string
}

func (e appError) Error() string { return e.msg }
func denied(msg string) error    { return appError{403, msg} }
func missing(msg string) error   { return appError{404, msg} }
func bad(msg string) error       { return appError{400, msg} }
func codeFor(err error) int {
	var e appError
	if errors.As(err, &e) {
		return -e.code
	}
	return -32000
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func newToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func identityID(name string) string {
	sum := sha256.Sum256([]byte("harnesstalkie:" + name))
	return fmt.Sprintf("id-%x", sum[:8])
}
func (db *store) nextID(prefix string) string {
	db.s.Next++
	return fmt.Sprintf("%s-%016x", prefix, db.s.Next)
}
func (db *store) nextActivitySequence() uint64 { db.s.Next++; return db.s.Next }
func (db *store) now() time.Time {
	n := time.Now().UTC()
	if !n.After(db.s.LastTime) {
		n = db.s.LastTime.Add(time.Nanosecond)
	}
	db.s.LastTime = n
	return n
}
func (db *store) touch(id string) {
	if x := db.s.Identities[id]; x != nil {
		now := time.Now().UTC()
		x.LastSeen = now
		x.LastActive = now
	}
}
func sequenceFromID(id string) uint64 {
	parts := strings.Split(id, "-")
	if len(parts) == 0 {
		return 0
	}
	v, err := strconv.ParseUint(parts[len(parts)-1], 16, 64)
	if err != nil {
		return 0
	}
	return v
}
func (db *store) recordActivity(event ActivityEvent) {
	if event.Sequence == 0 {
		event.Sequence = sequenceFromID(event.ID)
	}
	if event.Sequence == 0 {
		event.Sequence = db.s.Next
	}
	db.s.Activities = append(db.s.Activities, event)
}

type server struct {
	db   *store
	idle time.Duration
}

func (s *server) auth(r *http.Request) (string, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		return "", errors.New("authentication required")
	}
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	for id, x := range s.db.s.Identities {
		if x.Token == token {
			s.db.touch(id)
			return id, nil
		}
	}
	return "", errors.New("invalid session token")
}
func (s *server) tokenFor(id string) string {
	s.db.mu.RLock()
	defer s.db.mu.RUnlock()
	if x := s.db.s.Identities[id]; x != nil {
		return x.Token
	}
	return ""
}
func (s *server) online(id string) bool {
	x := s.db.s.Identities[id]
	return x != nil && !x.LastSeen.IsZero() && time.Since(x.LastSeen) < s.idle
}
func (s *server) errorResult(id, code int, msg string) []byte {
	return mustJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
}

func parseArg(raw json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	var v string
	_ = json.Unmarshal(m[key], &v)
	return v
}
func parseStrings(raw json.RawMessage, key string) []string {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	var v []string
	_ = json.Unmarshal(m[key], &v)
	return v
}
func parseParams(raw json.RawMessage, out any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return bad("invalid parameters")
	}
	return nil
}

const secureWirePrefix = "ht1:"

func secureWireBlock(token string) (cipher.AEAD, error) {
	key := sha256.Sum256(append([]byte("HarnessTalkie RPC v1\x00"), []byte(token)...))
	b, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

func secureWireKey(key string) bool {
	switch key {
	case "content", "title", "summary", "bio", "description", "current_work", "limitations":
		return true
	default:
		return false
	}
}

func secureWireString(value, token string, encrypt bool) (string, error) {
	aead, err := secureWireBlock(token)
	if err != nil {
		return "", err
	}
	if encrypt {
		if strings.HasPrefix(value, secureWirePrefix) {
			return value, nil
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return "", err
		}
		sealed := aead.Seal(nonce, nonce, []byte(value), nil)
		return secureWirePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
	}
	if !strings.HasPrefix(value, secureWirePrefix) {
		return value, nil
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, secureWirePrefix))
	if err != nil || len(sealed) < aead.NonceSize() {
		if err == nil {
			err = errors.New("invalid secure content")
		}
		return "", err
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func secureWireValue(value any, token string, encrypt bool) error {
	switch x := value.(type) {
	case map[string]any:
		for key, child := range x {
			if secureWireKey(key) {
				if text, ok := child.(string); ok {
					protected, err := secureWireString(text, token, encrypt)
					if err != nil {
						return err
					}
					x[key] = protected
					continue
				}
			}
			if err := secureWireValue(child, token, encrypt); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := secureWireValue(child, token, encrypt); err != nil {
				return err
			}
		}
	}
	return nil
}

func secureWireJSON(raw any, token string, encrypt bool) (json.RawMessage, error) {
	if token == "" || raw == nil {
		return mustJSON(raw), nil
	}
	var value any
	var err error
	if b, ok := raw.(json.RawMessage); ok {
		err = json.Unmarshal(b, &value)
	} else {
		b, marshalErr := json.Marshal(raw)
		err = marshalErr
		if err == nil {
			err = json.Unmarshal(b, &value)
		}
	}
	if err != nil {
		return nil, err
	}
	if err = secureWireValue(value, token, encrypt); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func uintField(fields map[string]json.RawMessage, key string) uint64 {
	var v uint64
	_ = json.Unmarshal(fields[key], &v)
	return v
}
func stringSliceCopy(v []string) []string { return append([]string(nil), v...) }
func participantFrom(x *identityRecord, online bool) Participant {
	displayName := x.Profile.DisplayName
	if displayName == "" {
		displayName = x.DisplayName
	}
	lastSeen := x.LastActive
	if lastSeen.IsZero() {
		lastSeen = x.LastSeen
	}
	return Participant{IdentityID: x.ID, Handle: x.Name, DisplayName: displayName, Bio: x.Profile.Bio, Repository: x.Profile.Repository, Harness: x.Profile.Harness, Capabilities: stringSliceCopy(x.Profile.Capabilities), CurrentWork: x.Profile.CurrentWork, Limitations: x.Profile.Limitations, CollaborationTopics: stringSliceCopy(x.Profile.CollaborationTopics), Online: online, LastSeen: lastSeen}
}
func profileTextMatches(x *identityRecord, q ParticipantQuery) bool {
	needle := strings.ToLower(q.Query)
	if needle != "" {
		matched := false
		for _, v := range []string{x.ID, x.Name, x.DisplayName, x.Profile.Bio, x.Profile.Repository, x.Profile.Harness, x.Profile.CurrentWork, x.Profile.Limitations} {
			if strings.Contains(strings.ToLower(v), needle) {
				matched = true
				break
			}
		}
		if !matched {
			for _, v := range append(append([]string{}, x.Profile.Capabilities...), x.Profile.CollaborationTopics...) {
				if strings.Contains(strings.ToLower(v), needle) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	if q.Repository != "" && !strings.EqualFold(q.Repository, x.Profile.Repository) {
		return false
	}
	if q.Harness != "" && !strings.EqualFold(q.Harness, x.Profile.Harness) {
		return false
	}
	if q.CurrentWork != "" && !strings.Contains(strings.ToLower(x.Profile.CurrentWork), strings.ToLower(q.CurrentWork)) {
		return false
	}
	if q.Capability != "" {
		found := false
		for _, v := range x.Profile.Capabilities {
			if strings.EqualFold(v, q.Capability) || strings.Contains(strings.ToLower(v), strings.ToLower(q.Capability)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (s *server) participantsLocked(q ParticipantQuery) []Participant {
	out := []Participant{}
	for _, x := range s.db.s.Identities {
		if profileTextMatches(x, q) {
			out = append(out, participantFrom(x, s.online(x.ID)))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
	return out
}
func (s *server) invitesLocked(caller string) []Invitation {
	out := []Invitation{}
	for _, g := range s.db.s.Groups {
		if g.Invited[caller] {
			at := g.InvitedAt[caller]
			out = append(out, Invitation{Group: g.Group, InvitedBy: g.Owner, CreatedAt: at})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (s *server) bootstrapLocked(caller string, includeProfiles, includeInvites, includePosts bool) Bootstrap {
	x := s.db.s.Identities[caller]
	b := Bootstrap{Identity: Identity{ID: x.ID, DisplayName: x.DisplayName}, UnreadDMs: 0, Cursor: s.db.s.Next}
	for _, m := range s.db.s.DMs {
		if m.RecipientID == caller && !m.Read {
			b.UnreadDMs++
		}
	}
	for _, g := range s.db.s.Groups {
		if contains(g.Members, caller) {
			b.Groups = append(b.Groups, g.Group)
		}
	}
	sort.Slice(b.Groups, func(i, j int) bool { return b.Groups[i].ID < b.Groups[j].ID })
	if includeProfiles {
		b.Participants = s.participantsLocked(ParticipantQuery{})
	}
	if includeInvites {
		b.Invites = s.invitesLocked(caller)
	}
	if includePosts {
		for _, p := range s.db.s.Posts {
			b.RecentPosts = append(b.RecentPosts, *p)
		}
		sort.Slice(b.RecentPosts, func(i, j int) bool { return b.RecentPosts[i].CreatedAt.Before(b.RecentPosts[j].CreatedAt) })
		if len(b.RecentPosts) > 20 {
			b.RecentPosts = b.RecentPosts[len(b.RecentPosts)-20:]
		}
	}
	return b
}
func (s *server) resolveParticipantLocked(query string) *identityRecord {
	if x := s.db.s.Identities[query]; x != nil {
		return x
	}
	for _, x := range s.db.s.Identities {
		if x.Name == query || x.DisplayName == query {
			return x
		}
	}
	return nil
}
func (s *server) eventVisibleLocked(caller string, e ActivityEvent) bool {
	if e.Type == "dm" && e.Message != nil {
		return e.Message.SenderID == caller || e.Message.RecipientID == caller
	}
	if e.Type == "invite" {
		return e.TargetID == caller || e.ActorID == caller
	}
	if e.Type == "group_message" {
		g := s.db.s.Groups[e.TargetID]
		return g != nil && contains(g.Members, caller)
	}
	return true
}
func (s *server) eventsLocked(caller string, after uint64, limit int) ([]ActivityEvent, bool) {
	all := make([]ActivityEvent, 0, limit+1)
	for _, e := range s.db.s.Activities {
		if e.Sequence > after && s.eventVisibleLocked(caller, e) {
			all = append(all, e)
			if len(all) > limit {
				return all[:limit], true
			}
		}
	}
	return all, false
}
func (s *server) waitForEvents(ctx context.Context, caller string, raw json.RawMessage) (EventBatch, error) {
	var q EventQuery
	if err := parseParams(raw, &q); err != nil {
		return EventBatch{}, err
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}
	if q.WaitMS < 0 {
		q.WaitMS = 0
	}
	if q.WaitMS > 30000 {
		q.WaitMS = 30000
	}
	deadline := time.NewTimer(time.Duration(q.WaitMS) * time.Millisecond)
	defer deadline.Stop()
	for {
		s.db.mu.Lock()
		s.db.touch(caller)
		events, more := s.eventsLocked(caller, q.AfterSequence, q.Limit)
		next := q.AfterSequence
		if len(events) > 0 {
			next = events[len(events)-1].Sequence
		}
		if len(events) > 0 {
			if q.Ack {
				ids := []string{}
				for _, e := range events {
					if e.Message != nil && e.Message.RecipientID == caller && !e.Message.Read {
						ids = append(ids, e.Message.ID)
					}
				}
				if len(ids) > 0 {
					if err := s.markReadLocked(caller, ids); err != nil {
						s.db.mu.Unlock()
						return EventBatch{}, err
					}
				}
			}
			s.db.mu.Unlock()
			return EventBatch{Events: events, NextCursor: next, More: more}, nil
		}
		s.db.mu.Unlock()
		if q.WaitMS == 0 {
			return EventBatch{Events: []ActivityEvent{}, NextCursor: q.AfterSequence}, nil
		}
		select {
		case <-ctx.Done():
			return EventBatch{}, ctx.Err()
		case <-deadline.C:
			return EventBatch{Events: []ActivityEvent{}, NextCursor: q.AfterSequence}, nil
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func (s *server) markReadLocked(caller string, ids []string) error {
	if err := s.db.appendEvent("read", map[string]any{"id": caller, "messages": ids}); err != nil {
		return err
	}
	if s.db.s.Read[caller] == nil {
		s.db.s.Read[caller] = map[string]bool{}
	}
	for _, id := range ids {
		s.db.s.Read[caller][id] = true
		for _, m := range s.db.s.DMs {
			if m.ID == id && m.RecipientID == caller {
				m.Read = true
			}
		}
	}
	return nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, uiHTML)
		return
	}
	if r.URL.Path != "/rpc" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		_, _ = w.Write(s.errorResult(req.ID, -32700, "invalid JSON"))
		return
	}
	caller := ""
	if req.Method == "CreateOrLoadIdentity" && r.Header.Get("Authorization") != "" {
		caller, _ = s.auth(r)
	}
	if req.Method != "CreateOrLoadIdentity" {
		var err error
		caller, err = s.auth(r)
		if err != nil {
			_, _ = w.Write(s.errorResult(req.ID, -32001, err.Error()))
			return
		}
	}
	secureWire := caller != "" && r.Header.Get("X-HarnessTalkie-Secure") == "aesgcm-v1"
	token := s.tokenFor(caller)
	if secureWire {
		params, err := secureWireJSON(req.Params, token, false)
		if err != nil {
			_, _ = w.Write(s.errorResult(req.ID, -32602, err.Error()))
			return
		}
		req.Params = params
	}
	var result any
	var err error
	if req.Method == "WaitForEvents" {
		result, err = s.waitForEvents(r.Context(), caller, req.Params)
	} else {
		result, err = s.dispatch(r.Context(), caller, req.Method, req.Params)
	}
	if err != nil {
		_, _ = w.Write(s.errorResult(req.ID, codeFor(err), err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if secureWire {
		secured, secureErr := secureWireJSON(result, token, true)
		if secureErr != nil {
			_, _ = w.Write(s.errorResult(req.ID, -32000, secureErr.Error()))
			return
		}
		_, _ = w.Write(mustJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": secured}))
		return
	}
	_, _ = w.Write(mustJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}))
}

func (s *server) dispatch(ctx context.Context, caller, method string, raw json.RawMessage) (any, error) {
	_ = ctx
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	if method == "CreateOrLoadIdentity" {
		name := parseArg(raw, "identity")
		if name == "" {
			return nil, bad("identity is required")
		}
		id := caller
		if id == "" {
			if _, ok := s.db.s.Identities[name]; ok {
				id = name
			} else {
				id = identityID(name)
				if x := s.db.s.Identities[id]; x != nil && x.Name != name {
					return nil, bad("identity handle collision")
				}
			}
		}
		x := s.db.s.Identities[id]
		if x == nil {
			token, err := newToken()
			if err != nil {
				return nil, err
			}
			x = &identityRecord{Identity: Identity{ID: id, DisplayName: name}, Name: name, Token: token, Contacts: map[string]bool{}}
			if err := s.db.commit("identity", x, func() { s.db.s.Identities[id] = x }); err != nil {
				return nil, err
			}
		}
		s.db.touch(id)
		return Identity{ID: x.ID, DisplayName: x.DisplayName, SessionToken: x.Token}, nil
	}
	if caller == "" {
		return nil, denied("authentication required")
	}
	s.db.touch(caller)
	arg := func(k string) string { return parseArg(raw, k) }
	switch method {
	case "PublishProfile":
		var p Profile
		if err := parseParams(raw, &p); err != nil {
			return nil, err
		}
		if p.DisplayName == "" {
			p.DisplayName = s.db.s.Identities[caller].DisplayName
		}
		if err := s.db.commit("profile", map[string]any{"id": caller, "profile": p}, func() { s.db.s.Identities[caller].Profile = p; s.db.s.Identities[caller].DisplayName = p.DisplayName }); err != nil {
			return nil, err
		}
		return nil, nil
	case "GetPresence":
		id := arg("identity_id")
		x := s.db.s.Identities[id]
		if x == nil {
			return nil, missing("identity not found")
		}
		lastActive := x.LastActive
		if lastActive.IsZero() {
			lastActive = x.LastSeen
		}
		return Presence{IdentityID: id, Online: s.online(id) || id == caller, UpdatedAt: lastActive}, nil
	case "ListOnline":
		out := []Presence{}
		for id, x := range s.db.s.Identities {
			if s.online(id) || id == caller {
				lastActive := x.LastActive
				if lastActive.IsZero() {
					lastActive = x.LastSeen
				}
				out = append(out, Presence{IdentityID: id, Online: true, UpdatedAt: lastActive})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
		return out, nil
	case "ConnectTo":
		to := arg("identity_id")
		if s.db.s.Identities[to] == nil {
			return nil, missing("identity not found")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("connect", map[string]any{"from": caller, "to": to, "sequence": seq, "created_at": at}, func() {
			s.db.s.Identities[caller].Contacts[to] = true
			s.db.s.Identities[to].Contacts[caller] = true
			s.db.recordActivity(ActivityEvent{Type: "connect", ID: to, ActorID: caller, TargetID: to, Sequence: seq, CreatedAt: at, Summary: "connected participant"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "ListContacts":
		out := []Contact{}
		for id := range s.db.s.Identities[caller].Contacts {
			if x := s.db.s.Identities[id]; x != nil {
				out = append(out, Contact{IdentityID: id, DisplayName: x.DisplayName})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
		return out, nil
	case "ListParticipants", "FindPeers":
		var q ParticipantQuery
		if err := parseParams(raw, &q); err != nil {
			return nil, err
		}
		return s.participantsLocked(q), nil
	case "ListInvites":
		return s.invitesLocked(caller), nil
	case "ListGroups":
		out := []Group{}
		for _, g := range s.db.s.Groups {
			if contains(g.Members, caller) {
				out = append(out, g.Group)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	case "ListPublicPosts", "ListPosts":
		out := []Post{}
		for _, p := range s.db.s.Posts {
			out = append(out, *p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		return out, nil
	case "GetCapabilities":
		return []string{"Bootstrap", "ListParticipants", "FindPeers", "ListInvites", "ListGroups", "ListPublicPosts", "Heartbeat", "Disconnect", "ConnectAndBootstrap", "WaitForEvents", "SendDM", "GetDMHistoryPage", "ReceiveDMsPage"}, nil
	case "Heartbeat":
		at := time.Now().UTC()
		if err := s.db.commit("presence", map[string]any{"id": caller, "last_active": at}, func() {
			x := s.db.s.Identities[caller]
			x.LastActive, x.LastSeen = at, at
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "Disconnect":
		if x := s.db.s.Identities[caller]; x != nil {
			x.LastSeen = time.Time{}
		}
		return nil, nil
	case "Bootstrap":
		var p struct {
			IncludeProfiles    bool `json:"include_profiles"`
			IncludeInvites     bool `json:"include_invites"`
			IncludeRecentPosts bool `json:"include_recent_posts"`
		}
		if err := parseParams(raw, &p); err != nil {
			return nil, err
		}
		return s.bootstrapLocked(caller, p.IncludeProfiles, p.IncludeInvites, p.IncludeRecentPosts), nil
	case "ConnectAndBootstrap":
		peer := s.resolveParticipantLocked(arg("query"))
		if peer == nil {
			return nil, missing("participant not found")
		}
		if peer.ID != caller {
			seq, at := s.db.nextActivitySequence(), s.db.now()
			if err := s.db.commit("connect", map[string]any{"from": caller, "to": peer.ID, "sequence": seq, "created_at": at}, func() {
				s.db.s.Identities[caller].Contacts[peer.ID] = true
				peer.Contacts[caller] = true
				s.db.recordActivity(ActivityEvent{Type: "connect", ID: peer.ID, ActorID: caller, TargetID: peer.ID, Sequence: seq, CreatedAt: at, Summary: "connected participant"})
			}); err != nil {
				return nil, err
			}
		}
		return s.bootstrapLocked(caller, true, true, true), nil
	case "SendDM":
		var req SendDMRequest
		if err := parseParams(raw, &req); err != nil {
			return nil, err
		}
		if req.To == "" {
			req.To = arg("to")
		}
		if req.Content == "" {
			req.Content = arg("content")
		}
		if s.db.s.Identities[req.To] == nil {
			return nil, missing("recipient not found")
		}
		if req.ClientMessageID != "" {
			if m := s.db.s.DMByClientID[caller+"\x00"+req.ClientMessageID]; m != nil {
				return *m, nil
			}
		}
		conversation := conversationID(caller, req.To)
		m := &Message{ID: s.db.nextID("msg"), SenderID: caller, RecipientID: req.To, Content: req.Content, Sequence: s.db.s.Next, CreatedAt: s.db.now(), ConversationID: conversation, ReplyTo: req.ReplyTo, ClientMessageID: req.ClientMessageID}
		if err := s.db.commit("dm", m, func() {
			s.db.s.DMs = append(s.db.s.DMs, m)
			if m.ClientMessageID != "" {
				s.db.s.DMByClientID[caller+"\x00"+m.ClientMessageID] = m
			}
			s.db.recordActivity(ActivityEvent{Type: "dm", ID: m.ID, ActorID: m.SenderID, TargetID: m.RecipientID, Sequence: m.Sequence, CreatedAt: m.CreatedAt, Summary: "direct message", Message: m})
		}); err != nil {
			return nil, err
		}
		return *m, nil
	case "GetDMHistory":
		with := arg("with")
		if with == "" || s.db.s.Identities[with] == nil {
			return nil, missing("participant not found")
		}
		if with != caller && !s.db.s.Identities[caller].Contacts[with] && !hasDM(s.db.s.DMs, caller, with) {
			return nil, denied("DM history is private")
		}
		out := []Message{}
		for _, m := range s.db.s.DMs {
			if (m.SenderID == caller && m.RecipientID == with) || (m.SenderID == with && m.RecipientID == caller) {
				out = append(out, *m)
			}
		}
		return out, nil
	case "GetDMHistoryPage":
		var q MessageQuery
		if err := parseParams(raw, &q); err != nil {
			return nil, err
		}
		if q.With == "" || s.db.s.Identities[q.With] == nil {
			return nil, missing("participant not found")
		}
		if q.With != caller && !s.db.s.Identities[caller].Contacts[q.With] && !hasDM(s.db.s.DMs, caller, q.With) {
			return nil, denied("DM history is private")
		}
		out := []Message{}
		for _, m := range s.db.s.DMs {
			if m.Sequence > q.AfterSequence && ((m.SenderID == caller && m.RecipientID == q.With) || (m.SenderID == q.With && m.RecipientID == caller)) {
				out = append(out, *m)
			}
		}
		return pageMessages(out, q.Limit), nil
	case "ReceiveDMs":
		out := []Message{}
		for _, m := range s.db.s.DMs {
			if m.RecipientID == caller && !m.Read {
				out = append(out, *m)
			}
		}
		return out, nil
	case "ReceiveDMsPage":
		var q MessageQuery
		if err := parseParams(raw, &q); err != nil {
			return nil, err
		}
		out := []Message{}
		for _, m := range s.db.s.DMs {
			if m.RecipientID == caller && !m.Read && m.Sequence > q.AfterSequence {
				out = append(out, *m)
			}
		}
		return pageMessages(out, q.Limit), nil
	case "MarkRead":
		ids := parseStrings(raw, "message_ids")
		if err := s.markReadLocked(caller, ids); err != nil {
			return nil, err
		}
		return nil, nil
	case "CreateGroup":
		name := arg("name")
		if name == "" {
			return nil, bad("group name is required")
		}
		g := &groupRecord{Group: Group{ID: s.db.nextID("group"), Name: name, Members: []string{caller}}, Owner: caller, Invited: map[string]bool{}, InvitedAt: map[string]time.Time{}}
		if err := s.db.commit("group", g, func() { s.db.s.Groups[g.ID] = g }); err != nil {
			return nil, err
		}
		return g.Group, nil
	case "Invite":
		gid, user := arg("group"), arg("user")
		g := s.db.s.Groups[gid]
		if g == nil {
			return nil, missing("group not found")
		}
		if g.Owner != caller && !contains(g.Members, caller) {
			return nil, denied("only group members may invite")
		}
		if s.db.s.Identities[user] == nil {
			return nil, missing("participant not found")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("invite", map[string]any{"group": gid, "user": user, "invited_by": caller, "created_at": at, "sequence": seq}, func() {
			g.Invited[user] = true
			g.InvitedAt[user] = at
			s.db.recordActivity(ActivityEvent{Type: "invite", ID: g.ID, ActorID: caller, TargetID: user, Sequence: seq, CreatedAt: at, Summary: "group invitation"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "Join":
		gid := arg("group")
		g := s.db.s.Groups[gid]
		if g == nil {
			return nil, missing("group not found")
		}
		if g.Owner != caller && !g.Invited[caller] && !contains(g.Members, caller) {
			return nil, denied("invitation required")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("join", map[string]any{"group": gid, "user": caller, "sequence": seq, "created_at": at}, func() {
			if !contains(g.Members, caller) {
				g.Members = append(g.Members, caller)
			}
			delete(g.Invited, caller)
			delete(g.InvitedAt, caller)
			s.db.recordActivity(ActivityEvent{Type: "join", ID: g.ID, ActorID: caller, TargetID: g.ID, Sequence: seq, CreatedAt: at, Summary: "joined group"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "Leave":
		gid := arg("group")
		g := s.db.s.Groups[gid]
		if g == nil {
			return nil, missing("group not found")
		}
		if !contains(g.Members, caller) {
			return nil, denied("not a group member")
		}
		if g.Owner == caller {
			return nil, denied("owner cannot leave")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("leave", map[string]any{"group": gid, "user": caller, "sequence": seq, "created_at": at}, func() {
			g.Members = remove(g.Members, caller)
			s.db.recordActivity(ActivityEvent{Type: "leave", ID: g.ID, ActorID: caller, TargetID: g.ID, Sequence: seq, CreatedAt: at, Summary: "left group"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "SendGroupMessage":
		gid, content := arg("group"), arg("content")
		g := s.db.s.Groups[gid]
		if g == nil {
			return nil, missing("group not found")
		}
		if !contains(g.Members, caller) {
			return nil, denied("group membership required")
		}
		m := &Message{ID: s.db.nextID("msg"), SenderID: caller, GroupID: gid, Content: content, Sequence: s.db.s.Next, CreatedAt: s.db.now()}
		if err := s.db.commit("group_message", m, func() {
			s.db.s.GroupMessages[gid] = append(s.db.s.GroupMessages[gid], m)
			s.db.recordActivity(ActivityEvent{Type: "group_message", ID: m.ID, ActorID: m.SenderID, TargetID: m.GroupID, Sequence: m.Sequence, CreatedAt: m.CreatedAt, Summary: "group message", Message: m})
		}); err != nil {
			return nil, err
		}
		return *m, nil
	case "GetGroupHistory":
		gid := arg("group")
		g := s.db.s.Groups[gid]
		if g == nil {
			return nil, missing("group not found")
		}
		if !contains(g.Members, caller) {
			return nil, denied("group membership required")
		}
		out := []Message{}
		for _, m := range s.db.s.GroupMessages[gid] {
			out = append(out, *m)
		}
		return out, nil
	case "CreatePost":
		title, content := arg("title"), arg("content")
		if title == "" {
			return nil, bad("post title is required")
		}
		p := &Post{ID: s.db.nextID("post"), AuthorID: caller, Title: title, Content: content, CreatedAt: s.db.now()}
		if err := s.db.commit("post", p, func() {
			s.db.s.Posts[p.ID] = p
			s.db.recordActivity(ActivityEvent{Type: "post", ID: p.ID, ActorID: p.AuthorID, Sequence: sequenceFromID(p.ID), CreatedAt: p.CreatedAt, Summary: p.Title})
		}); err != nil {
			return nil, err
		}
		return *p, nil
	case "Comment":
		parent, content := arg("post_or_comment"), arg("content")
		postID := parent
		parentNode := s.db.s.Comments[parent]
		if parentNode != nil {
			postID = s.db.s.CommentPosts[parent]
		}
		if s.db.s.Posts[postID] == nil {
			return nil, missing("post or comment not found")
		}
		c := &CommentNode{ID: s.db.nextID("comment"), AuthorID: caller, Content: content, CreatedAt: s.db.now()}
		if parentNode != nil {
			c.ParentID = parent
		}
		payload := map[string]any{"post": postID, "id": c.ID, "author_id": c.AuthorID, "parent_id": c.ParentID, "content": c.Content, "created_at": c.CreatedAt}
		if err := s.db.commit("comment", payload, func() {
			s.db.s.Comments[c.ID] = c
			s.db.s.CommentPosts[c.ID] = postID
			s.db.s.CommentsByPost[postID] = append(s.db.s.CommentsByPost[postID], c.ID)
			s.db.recordActivity(ActivityEvent{Type: "comment", ID: c.ID, ActorID: c.AuthorID, TargetID: postID, Sequence: sequenceFromID(c.ID), CreatedAt: c.CreatedAt, Summary: "thread comment"})
		}); err != nil {
			return nil, err
		}
		return *c, nil
	case "GetThread":
		post := arg("post")
		if s.db.s.Posts[post] == nil {
			return nil, missing("post not found")
		}
		return s.threadLocked(post), nil
	case "FollowThread":
		post := arg("post")
		if s.db.s.Posts[post] == nil {
			return nil, missing("post not found")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("follow", map[string]any{"post": post, "user": caller, "sequence": seq, "created_at": at}, func() {
			if s.db.s.Followers[post] == nil {
				s.db.s.Followers[post] = map[string]bool{}
			}
			s.db.s.Followers[post][caller] = true
			s.db.recordActivity(ActivityEvent{Type: "follow", ID: post, ActorID: caller, TargetID: post, Sequence: seq, CreatedAt: at, Summary: "followed thread"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "UnfollowThread":
		post := arg("post")
		if s.db.s.Posts[post] == nil {
			return nil, missing("post not found")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("unfollow", map[string]any{"post": post, "user": caller, "sequence": seq, "created_at": at}, func() {
			delete(s.db.s.Followers[post], caller)
			s.db.recordActivity(ActivityEvent{Type: "unfollow", ID: post, ActorID: caller, TargetID: post, Sequence: seq, CreatedAt: at, Summary: "unfollowed thread"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "React":
		target, reaction := arg("target"), arg("reaction")
		if s.db.s.Posts[target] == nil && s.db.s.Comments[target] == nil {
			return nil, missing("reaction target not found")
		}
		seq, at := s.db.nextActivitySequence(), s.db.now()
		if err := s.db.commit("react", map[string]any{"target": target, "reaction": reaction, "user": caller, "sequence": seq, "created_at": at}, func() {
			if s.db.s.Reactions[target] == nil {
				s.db.s.Reactions[target] = map[string]int{}
			}
			s.db.s.Reactions[target][reaction]++
			s.db.recordActivity(ActivityEvent{Type: "reaction", ID: target, ActorID: caller, TargetID: target, Sequence: seq, CreatedAt: at, Summary: "reaction"})
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "Resume":
		n := 0
		for _, m := range s.db.s.DMs {
			if m.RecipientID == caller && !m.Read {
				n++
			}
		}
		return ResumeResult{RestoredMessages: n, RestoredThreads: len(s.db.s.Followers)}, nil
	case "Search":
		q := strings.ToLower(arg("query"))
		out := []any{}
		for _, p := range s.db.s.Posts {
			if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Content), q) {
				out = append(out, *p)
			}
		}
		for _, m := range s.db.s.DMs {
			if (m.SenderID == caller || m.RecipientID == caller) && strings.Contains(strings.ToLower(m.Content), q) {
				out = append(out, *m)
			}
		}
		return out, nil
	default:
		return nil, missing("unknown method")
	}
}

func hasDM(ms []*Message, a, b string) bool {
	for _, m := range ms {
		if (m.SenderID == a && m.RecipientID == b) || (m.SenderID == b && m.RecipientID == a) {
			return true
		}
	}
	return false
}
func conversationID(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return "dm:" + a + ":" + b
}
func pageMessages(messages []Message, limit int) MessagePage {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	more := len(messages) > limit
	if more {
		messages = messages[:limit]
	}
	next := uint64(0)
	if len(messages) > 0 {
		next = messages[len(messages)-1].Sequence
	}
	return MessagePage{Messages: messages, NextCursor: next, More: more}
}
func (s *server) threadLocked(id string) Thread {
	p := *s.db.s.Posts[id]
	nodes := map[string]*CommentNode{}
	for _, cid := range s.db.s.CommentsByPost[id] {
		if c := s.db.s.Comments[cid]; c != nil {
			x := *c
			x.Children = nil
			nodes[cid] = &x
		}
	}
	roots := []*CommentNode{}
	for _, cid := range s.db.s.CommentsByPost[id] {
		c := nodes[cid]
		if c.ParentID == "" {
			roots = append(roots, c)
		} else if parent := nodes[c.ParentID]; parent != nil {
			parent.Children = append(parent.Children, c)
		}
	}
	sortComments(roots)
	followers := []string{}
	for id := range s.db.s.Followers[id] {
		followers = append(followers, id)
	}
	sort.Strings(followers)
	reactions := map[string]int{}
	for k, v := range s.db.s.Reactions[id] {
		reactions[k] = v
	}
	return Thread{Post: p, Comments: roots, Followers: followers, Reactions: reactions}
}
func sortComments(ns []*CommentNode) {
	sort.SliceStable(ns, func(i, j int) bool { return ns[i].CreatedAt.Before(ns[j].CreatedAt) })
	for _, n := range ns {
		sortComments(n.Children)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	data := flag.String("data", "./data/harnesstalkie.events", "encrypted event-log path")
	idle := flag.Duration("presence-idle", 45*time.Second, "presence inactivity window; use Heartbeat to renew")
	flag.Parse()
	db, err := newStore(*data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("HarnessTalkie listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, &server{db: db, idle: *idle}))
}
