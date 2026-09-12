package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func rpcParams(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestSecureWirePreservesUint64(t *testing.T) {
	secured, err := secureWireJSON(map[string]any{"after_cursor": uint64(math.MaxUint64)}, "session-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		AfterCursor uint64 `json:"after_cursor"`
	}
	if err = json.Unmarshal(secured, &got); err != nil {
		t.Fatalf("decode secured payload: %v (%s)", err, secured)
	}
	if got.AfterCursor != math.MaxUint64 {
		t.Fatalf("after_cursor = %d, want %d", got.AfterCursor, uint64(math.MaxUint64))
	}
}

func TestHumanApplicationAndDeepRoutesAreServed(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	for _, route := range []string{"/", "/inbox", "/servers/server-1/members", "/posts/post-1"} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", route, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{"<title>HarnessTalkie</title>", `<div id="root"></div>`, `/assets/index-`} {
			if !strings.Contains(body, want) {
				t.Fatalf("GET %s missing %q", route, want)
			}
		}
	}
}

func testIdentity(t *testing.T, s *server, name string) Identity {
	t.Helper()
	v, err := s.dispatch(nil, "", "CreateOrLoadIdentity", rpcParams(map[string]string{"identity": name}))
	if err != nil {
		t.Fatal(err)
	}
	return v.(Identity)
}

func TestExistingIdentityRequiresBearerSession(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	identity := testIdentity(t, s, "protected-identity")
	if _, err = s.dispatch(context.Background(), "", "CreateOrLoadIdentity", rpcParams(map[string]string{"identity": "protected-identity"})); err == nil {
		t.Fatal("existing identity token was issued without authentication")
	}
	attacker := testIdentity(t, s, "different-identity")
	if _, err = s.dispatch(context.Background(), attacker.ID, "CreateOrLoadIdentity", rpcParams(map[string]string{"identity": identity.ID})); err == nil {
		t.Fatal("a different identity's token was accepted for the protected identity")
	}
	resumed, err := s.dispatch(context.Background(), identity.ID, "CreateOrLoadIdentity", rpcParams(map[string]string{"identity": identity.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if resumed.(Identity).SessionToken != identity.SessionToken {
		t.Fatal("authenticated identity did not resume its session")
	}
}

func TestDiscoveryCursorAndIdempotency(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: 45 * 1000000000}
	a, b := testIdentity(t, s, "discovery-a"), testIdentity(t, s, "discovery-b")
	created, err := s.dispatch(context.Background(), a.ID, "CreateServer", rpcParams(map[string]any{"name": "discovery", "join_policy": "public"}))
	if err != nil {
		t.Fatal(err)
	}
	serverID := created.(v2Server).ID
	if _, err = s.dispatch(context.Background(), b.ID, "JoinServer", rpcParams(map[string]string{"server_id": serverID})); err != nil {
		t.Fatal(err)
	}
	profile := Profile{DisplayName: "Discovery A", Repository: "repo-a", Harness: "harness-a", Capabilities: []string{"retrieval"}, CurrentWork: "integration"}
	if _, err = s.dispatch(context.Background(), a.ID, "PublishProfile", rpcParams(profile)); err != nil {
		t.Fatal(err)
	}
	peers, err := s.dispatch(context.Background(), b.ID, "FindPeers", rpcParams(ParticipantQuery{Capability: "retrieval"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(peers.([]Participant)) != 1 || peers.([]Participant)[0].Repository != "repo-a" {
		t.Fatalf("peers = %#v", peers)
	}
	boot, err := s.dispatch(context.Background(), b.ID, "Bootstrap", rpcParams(map[string]bool{"include_profiles": true}))
	if err != nil {
		t.Fatal(err)
	}
	cursor := boot.(Bootstrap).Cursor
	first, err := s.dispatch(context.Background(), a.ID, "SendDM", rpcParams(SendDMRequest{ServerID: serverID, To: b.ID, Content: "one", ClientMessageID: "one"}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.dispatch(context.Background(), a.ID, "SendDM", rpcParams(SendDMRequest{ServerID: serverID, To: b.ID, Content: "one", ClientMessageID: "one"}))
	if err != nil {
		t.Fatal(err)
	}
	if first.(Message).ID != second.(Message).ID {
		t.Fatal("idempotency key created duplicate")
	}
	batch, err := s.waitForEvents(context.Background(), b.ID, rpcParams(EventQuery{AfterSequence: cursor, Limit: 10, Ack: true}))
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.Events[0].Message == nil {
		t.Fatalf("event batch = %#v", batch)
	}
	page, err := s.dispatch(context.Background(), b.ID, "ReceiveDMsPage", rpcParams(MessageQuery{ServerID: serverID, Limit: 10}))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.(MessagePage).Messages) != 0 {
		t.Fatalf("acked page = %#v", page)
	}
}

func TestDurabilityAuthorizationAndThreads(t *testing.T) {
	path := t.TempDir() + "/events"
	db, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: 750000000}
	a, b, c := testIdentity(t, s, "test-a"), testIdentity(t, s, "test-b"), testIdentity(t, s, "test-c")
	created, err := s.dispatch(context.Background(), a.ID, "CreateServer", rpcParams(map[string]any{"name": "durable-dm", "join_policy": "public"}))
	if err != nil {
		t.Fatal(err)
	}
	serverID := created.(v2Server).ID
	if _, err = s.dispatch(context.Background(), b.ID, "JoinServer", rpcParams(map[string]string{"server_id": serverID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, a.ID, "ConnectTo", rpcParams(map[string]string{"identity_id": b.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, a.ID, "SendDM", rpcParams(map[string]string{"server_id": serverID, "to": b.ID, "content": "private"})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(nil, c.ID, "GetDMHistory", rpcParams(map[string]string{"server_id": serverID, "with": b.ID})); err == nil {
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
	h, err := sr.dispatch(nil, b.ID, "GetDMHistory", rpcParams(map[string]string{"server_id": serverID, "with": a.ID}))
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

func TestV2ServerIsolationPoliciesAndReplay(t *testing.T) {
	path := t.TempDir() + "/v2-events"
	db, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	a, b := testIdentity(t, s, "v2-owner"), testIdentity(t, s, "v2-member")
	created, err := s.dispatch(context.Background(), a.ID, "CreateServer", rpcParams(map[string]any{"name": "research", "join_policy": "public", "discoverable": true, "tags": []string{"research"}}))
	if err != nil {
		t.Fatal(err)
	}
	serverView := created.(v2Server)
	if _, err = s.dispatch(context.Background(), b.ID, "JoinServer", rpcParams(map[string]string{"server_id": serverView.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(context.Background(), b.ID, "ListServerMembers", rpcParams(map[string]string{"server_id": serverView.ID})); err != nil {
		t.Fatal(err)
	}
	closed, err := s.dispatch(context.Background(), a.ID, "CreateServer", rpcParams(map[string]any{"name": "private", "join_policy": "closed", "discoverable": true}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.dispatch(context.Background(), b.ID, "ListServerMembers", rpcParams(map[string]string{"server_id": closed.(v2Server).ID})); err == nil {
		t.Fatal("cross-server member enumeration succeeded")
	}
	if _, err = newStore(path); err != nil {
		t.Fatal(err)
	}
	replayed, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed.s.Servers) != 2 {
		t.Fatalf("replayed servers = %d", len(replayed.s.Servers))
	}
	if _, ok := replayed.s.Servers[serverView.ID].Members[b.ID]; !ok {
		t.Fatal("replayed membership missing")
	}
}
