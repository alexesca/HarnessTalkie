package main

import (
	"context"
	"testing"
	"time"
)

func v2DispatchForTest(t *testing.T, s *server, caller, method string, params any) any {
	t.Helper()
	result, err := s.dispatch(context.Background(), caller, method, rpcParams(params))
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return result
}

func TestV2ServerScopedDMDeniesOutsiderAndNotifiesMember(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "v2-owner")
	member := testIdentity(t, s, "v2-member")
	outsider := testIdentity(t, s, "v2-outsider")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "Secure workspace", "join_policy": "public", "discoverable": true}).(v2Server)
	v2DispatchForTest(t, s, member.ID, "JoinServer", map[string]string{"server_id": serverValue.ID})
	if _, err = s.dispatch(context.Background(), outsider.ID, "SendDM", rpcParams(SendDMRequest{ServerID: serverValue.ID, To: member.ID, Content: "forbidden"})); err == nil {
		t.Fatal("outsider sent a Server-scoped DM")
	}
	v2DispatchForTest(t, s, owner.ID, "SendDM", SendDMRequest{ServerID: serverValue.ID, To: member.ID, Content: "authorized"})
	notifications := v2DispatchForTest(t, s, member.ID, "ListNotifications", map[string]any{"unread_only": true}).([]v2Notification)
	if len(notifications) != 1 || notifications[0].Type != "dm" || notifications[0].ServerID != serverValue.ID {
		t.Fatalf("notifications = %#v", notifications)
	}
}

func TestV2FutureCursorAndStateSurviveRestart(t *testing.T) {
	path := t.TempDir() + "/events"
	db, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "restart-owner")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "Durable workspace", "join_policy": "public", "discoverable": true}).(v2Server)
	if _, err = s.dispatch(context.Background(), owner.ID, "Sync", rpcParams(map[string]any{"server_id": serverValue.ID, "after_cursor": ^uint64(0)})); err == nil {
		t.Fatal("future cursor was accepted")
	}
	reloaded, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &server{db: reloaded, idle: time.Hour}
	got := v2DispatchForTest(t, restarted, owner.ID, "GetServer", map[string]string{"server_id": serverValue.ID}).(v2Server)
	if got.Name != "Durable workspace" || got.OwnerID != owner.ID || got.MemberCount != 1 {
		t.Fatalf("reloaded Server = %#v", got)
	}
}

func TestV2RemovingServerMemberRevokesGroupAccess(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "revoke-owner")
	member := testIdentity(t, s, "revoke-member")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "Revocation workspace", "join_policy": "public"}).(v2Server)
	v2DispatchForTest(t, s, member.ID, "JoinServer", map[string]string{"server_id": serverValue.ID})
	group := v2DispatchForTest(t, s, owner.ID, "CreateGroup", map[string]any{"server_id": serverValue.ID, "name": "Operations", "join_policy": "public"}).(Group)
	v2DispatchForTest(t, s, member.ID, "JoinGroup", map[string]string{"group_id": group.ID})
	v2DispatchForTest(t, s, owner.ID, "RemoveServerMember", map[string]string{"server_id": serverValue.ID, "participant_id": member.ID})
	if _, err = s.dispatch(context.Background(), member.ID, "GetGroupHistory", rpcParams(map[string]string{"group": group.ID})); err == nil {
		t.Fatal("removed Server member retained group history access")
	}
	if s.db.s.V2Groups[group.ID].Members[member.ID] {
		t.Fatal("removed Server member remained in group membership state")
	}
}

func TestV2PostVisibilityIsEnforcedAcrossReadSurfaces(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "posts-owner")
	shared := testIdentity(t, s, "posts-shared")
	other := testIdentity(t, s, "posts-other")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "Forum workspace", "join_policy": "public"}).(v2Server)
	for _, identity := range []Identity{shared, other} {
		v2DispatchForTest(t, s, identity.ID, "JoinServer", map[string]string{"server_id": serverValue.ID})
	}
	privatePost := v2DispatchForTest(t, s, owner.ID, "CreatePost", map[string]any{"server_id": serverValue.ID, "title": "Private", "content": "secret", "visibility": "private", "mentions": []string{shared.ID}}).(v2PostView)
	if _, err = s.dispatch(context.Background(), shared.ID, "GetThread", rpcParams(map[string]string{"post_id": privatePost.ID})); err == nil {
		t.Fatal("mentioned participant read a private post")
	}
	posts := v2DispatchForTest(t, s, shared.ID, "SearchPosts", map[string]any{"server_id": serverValue.ID, "query": "secret"}).([]any)
	if len(posts) != 0 {
		t.Fatalf("private post leaked through search: %#v", posts)
	}
	if notifications := v2DispatchForTest(t, s, shared.ID, "ListNotifications", map[string]any{"unread_only": true}).([]v2Notification); len(notifications) != 0 {
		t.Fatalf("private mention leaked through notifications: %#v", notifications)
	}
	sharedPost := v2DispatchForTest(t, s, owner.ID, "CreatePost", map[string]any{"server_id": serverValue.ID, "title": "Shared", "content": "targeted", "visibility": "directly-shared", "shared_with": []string{shared.ID}}).(v2PostView)
	v2DispatchForTest(t, s, shared.ID, "GetThread", map[string]string{"post_id": sharedPost.ID})
	if _, err = s.dispatch(context.Background(), other.ID, "GetThread", rpcParams(map[string]string{"post_id": sharedPost.ID})); err == nil {
		t.Fatal("unshared participant read a directly-shared post")
	}
}
