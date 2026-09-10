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
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio,omitempty"`
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
type Message struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id,omitempty"`
	GroupID     string    `json:"group_id,omitempty"`
	Content     string    `json:"content"`
	Sequence    uint64    `json:"sequence"`
	CreatedAt   time.Time `json:"created_at"`
	Read        bool      `json:"read,omitempty"`
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
	Profile  Profile         `json:"profile"`
	Name     string          `json:"name"`
	Token    string          `json:"token"`
	LastSeen time.Time       `json:"last_seen"`
	Contacts map[string]bool `json:"contacts"`
}
type groupRecord struct {
	Group
	Owner   string
	Invited map[string]bool
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
}

func newState() *state {
	return &state{
		Identities: map[string]*identityRecord{}, Groups: map[string]*groupRecord{},
		GroupMessages: map[string][]*Message{}, Posts: map[string]*Post{},
		Comments: map[string]*CommentNode{}, CommentPosts: map[string]string{},
		CommentsByPost: map[string][]string{}, Followers: map[string]map[string]bool{},
		Reactions: map[string]map[string]int{}, Read: map[string]map[string]bool{},
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
	case "dm":
		var m Message
		_ = json.Unmarshal(raw, &m)
		db.s.DMs = append(db.s.DMs, &m)
		db.s.Next = max(db.s.Next, m.Sequence)
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
		db.s.Groups[g.ID] = &g
	case "invite":
		if g := db.s.Groups[str("group")]; g != nil {
			g.Invited[str("user")] = true
		}
	case "join":
		if g := db.s.Groups[str("group")]; g != nil {
			user := str("user")
			if !contains(g.Members, user) {
				g.Members = append(g.Members, user)
			}
			delete(g.Invited, user)
		}
	case "leave":
		if g := db.s.Groups[str("group")]; g != nil {
			g.Members = remove(g.Members, str("user"))
		}
	case "group_message":
		var m Message
		_ = json.Unmarshal(raw, &m)
		db.s.GroupMessages[m.GroupID] = append(db.s.GroupMessages[m.GroupID], &m)
		db.s.Next = max(db.s.Next, m.Sequence)
	case "post":
		var p Post
		_ = json.Unmarshal(raw, &p)
		db.s.Posts[p.ID] = &p
	case "comment":
		var c CommentNode
		_ = json.Unmarshal(raw, &c)
		post := str("post")
		db.s.Comments[c.ID] = &c
		db.s.CommentPosts[c.ID] = post
		db.s.CommentsByPost[post] = append(db.s.CommentsByPost[post], c.ID)
	case "follow":
		post, user := str("post"), str("user")
		if db.s.Followers[post] == nil {
			db.s.Followers[post] = map[string]bool{}
		}
		db.s.Followers[post][user] = true
	case "unfollow":
		delete(db.s.Followers[str("post")], str("user"))
	case "react":
		target, reaction := str("target"), str("reaction")
		if db.s.Reactions[target] == nil {
			db.s.Reactions[target] = map[string]int{}
		}
		db.s.Reactions[target][reaction]++
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
		x.LastSeen = time.Now().UTC()
	}
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
	result, err := s.dispatch(context.Background(), caller, req.Method, req.Params)
	if err != nil {
		_, _ = w.Write(s.errorResult(req.ID, codeFor(err), err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
		p := Profile{DisplayName: arg("display_name"), Bio: arg("bio")}
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
		return Presence{IdentityID: id, Online: s.online(id) || id == caller, UpdatedAt: x.LastSeen}, nil
	case "ListOnline":
		out := []Presence{}
		for id, x := range s.db.s.Identities {
			if s.online(id) || id == caller {
				out = append(out, Presence{IdentityID: id, Online: true, UpdatedAt: x.LastSeen})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].IdentityID < out[j].IdentityID })
		return out, nil
	case "ConnectTo":
		to := arg("identity_id")
		if s.db.s.Identities[to] == nil {
			return nil, missing("identity not found")
		}
		if err := s.db.commit("connect", map[string]string{"from": caller, "to": to}, func() { s.db.s.Identities[caller].Contacts[to] = true; s.db.s.Identities[to].Contacts[caller] = true }); err != nil {
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
	case "SendDM":
		to, content := arg("to"), arg("content")
		if s.db.s.Identities[to] == nil {
			return nil, missing("recipient not found")
		}
		m := &Message{ID: s.db.nextID("msg"), SenderID: caller, RecipientID: to, Content: content, Sequence: s.db.s.Next, CreatedAt: s.db.now()}
		if err := s.db.commit("dm", m, func() { s.db.s.DMs = append(s.db.s.DMs, m) }); err != nil {
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
	case "ReceiveDMs":
		out := []Message{}
		for _, m := range s.db.s.DMs {
			if m.RecipientID == caller && !m.Read {
				out = append(out, *m)
			}
		}
		return out, nil
	case "MarkRead":
		ids := parseStrings(raw, "message_ids")
		if err := s.db.commit("read", map[string]any{"id": caller, "messages": ids}, func() {
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
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "CreateGroup":
		name := arg("name")
		if name == "" {
			return nil, bad("group name is required")
		}
		g := &groupRecord{Group: Group{ID: s.db.nextID("group"), Name: name, Members: []string{caller}}, Owner: caller, Invited: map[string]bool{}}
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
		if err := s.db.commit("invite", map[string]string{"group": gid, "user": user}, func() { g.Invited[user] = true }); err != nil {
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
		if err := s.db.commit("join", map[string]string{"group": gid, "user": caller}, func() {
			if !contains(g.Members, caller) {
				g.Members = append(g.Members, caller)
			}
			delete(g.Invited, caller)
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
		if err := s.db.commit("leave", map[string]string{"group": gid, "user": caller}, func() { g.Members = remove(g.Members, caller) }); err != nil {
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
		if err := s.db.commit("group_message", m, func() { s.db.s.GroupMessages[gid] = append(s.db.s.GroupMessages[gid], m) }); err != nil {
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
		if err := s.db.commit("post", p, func() { s.db.s.Posts[p.ID] = p }); err != nil {
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
		if err := s.db.commit("follow", map[string]string{"post": post, "user": caller}, func() {
			if s.db.s.Followers[post] == nil {
				s.db.s.Followers[post] = map[string]bool{}
			}
			s.db.s.Followers[post][caller] = true
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "UnfollowThread":
		post := arg("post")
		if s.db.s.Posts[post] == nil {
			return nil, missing("post not found")
		}
		if err := s.db.commit("unfollow", map[string]string{"post": post, "user": caller}, func() { delete(s.db.s.Followers[post], caller) }); err != nil {
			return nil, err
		}
		return nil, nil
	case "React":
		target, reaction := arg("target"), arg("reaction")
		if s.db.s.Posts[target] == nil && s.db.s.Comments[target] == nil {
			return nil, missing("reaction target not found")
		}
		if err := s.db.commit("react", map[string]string{"target": target, "reaction": reaction, "user": caller}, func() {
			if s.db.s.Reactions[target] == nil {
				s.db.s.Reactions[target] = map[string]int{}
			}
			s.db.s.Reactions[target][reaction]++
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
	case "ListGroups":
		out := []Group{}
		for _, g := range s.db.s.Groups {
			if contains(g.Members, caller) {
				out = append(out, g.Group)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	case "ListPosts":
		out := []Post{}
		for _, p := range s.db.s.Posts {
			out = append(out, *p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		return out, nil
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
	idle := flag.Duration("presence-idle", 750*time.Millisecond, "presence inactivity window")
	flag.Parse()
	db, err := newStore(*data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("HarnessTalkie listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, &server{db: db, idle: *idle}))
}
