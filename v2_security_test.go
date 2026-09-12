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
