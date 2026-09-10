package main

import (
	"encoding/json"
	"testing"
)

func rpcParams(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func testIdentity(t *testing.T, s *server, name string) Identity {
	t.Helper()
	v, err := s.dispatch(nil, "", "CreateOrLoadIdentity", rpcParams(map[string]string{"identity": name}))
	if err != nil {
		t.Fatal(err)
	}
	return v.(Identity)
}

func TestDurabilityAuthorizationAndThreads(t *testing.T) {
	path := t.TempDir() + "/events"
	db, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: 750000000}
	a, b, c := testIdentity(t, s, "test-a"), testIdentity(t, s, "test-b"), testIdentity(t, s, "test-c")
	if _, err = s.dispatch(nil, a.ID, "ConnectTo", rpcParams(map[string]string{"identity_id": b.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, a.ID, "SendDM", rpcParams(map[string]string{"to": b.ID, "content": "private"})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, c.ID, "GetDMHistory", rpcParams(map[string]string{"with": b.ID})); err == nil {
		t.Fatal("unauthorized DM read succeeded")
	}
	gAny, err := s.dispatch(nil, a.ID, "CreateGroup", rpcParams(map[string]string{"name": "test-group"}))
	if err != nil {
		t.Fatal(err)
	}
	g := gAny.(Group)
	if _, err = s.dispatch(nil, a.ID, "Invite", rpcParams(map[string]string{"group": g.ID, "user": b.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, b.ID, "Join", rpcParams(map[string]string{"group": g.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, b.ID, "SendGroupMessage", rpcParams(map[string]string{"group": g.ID, "content": "ordered"})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, c.ID, "GetGroupHistory", rpcParams(map[string]string{"group": g.ID})); err == nil {
		t.Fatal("unauthorized group read succeeded")
	}
	pAny, err := s.dispatch(nil, a.ID, "CreatePost", rpcParams(map[string]string{"title": "thread", "content": "root"}))
	if err != nil {
		t.Fatal(err)
	}
	p := pAny.(Post)
	rootAny, err := s.dispatch(nil, b.ID, "Comment", rpcParams(map[string]string{"post_or_comment": p.ID, "content": "root-comment"}))
	if err != nil {
		t.Fatal(err)
	}
	root := rootAny.(CommentNode)
	nestedAny, err := s.dispatch(nil, c.ID, "Comment", rpcParams(map[string]string{"post_or_comment": root.ID, "content": "nested"}))
	if err != nil {
		t.Fatal(err)
	}
	nested := nestedAny.(CommentNode)
	if nested.ParentID != root.ID {
		t.Fatalf("parent id = %q, want %q", nested.ParentID, root.ID)
	}
	if _, err = s.dispatch(nil, b.ID, "GetThread", rpcParams(map[string]string{"post": p.ID})); err != nil {
		t.Fatal(err)
	}

	reloaded, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	sr := &server{db: reloaded, idle: 750000000}
	h, err := sr.dispatch(nil, b.ID, "GetDMHistory", rpcParams(map[string]string{"with": a.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.([]Message)) != 1 || h.([]Message)[0].Content != "private" {
		t.Fatalf("reloaded DM history = %#v", h)
	}
	thread, err := sr.dispatch(nil, c.ID, "GetThread", rpcParams(map[string]string{"post": p.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.(Thread).Comments) != 1 || len(thread.(Thread).Comments[0].Children) != 1 {
		t.Fatalf("reloaded thread = %#v", thread)
	}
}
