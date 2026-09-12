package main

import (
	"context"
	"encoding/json"
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

func TestV2ServerMembersCanOpenAnEmptyScopedConversation(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "empty-dm-owner")
	member := testIdentity(t, s, "empty-dm-member")
	outsider := testIdentity(t, s, "empty-dm-outsider")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "DM workspace", "join_policy": "public"}).(v2Server)
	v2DispatchForTest(t, s, member.ID, "JoinServer", map[string]string{"server_id": serverValue.ID})
	history := v2DispatchForTest(t, s, owner.ID, "GetDMHistory", map[string]string{"server_id": serverValue.ID, "with": member.ID}).([]Message)
	if len(history) != 0 {
		t.Fatalf("empty DM history = %#v", history)
	}
	if _, err = s.dispatch(context.Background(), outsider.ID, "GetDMHistory", rpcParams(map[string]string{"server_id": serverValue.ID, "with": member.ID})); err == nil {
		t.Fatal("outsider opened a Server-scoped DM history")
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

func TestV2ApprovalImmediatelyAdmitsServerAndGroupMembers(t *testing.T) {
	db, err := newStore("")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, idle: time.Hour}
	owner := testIdentity(t, s, "approval-owner")
	applicant := testIdentity(t, s, "approval-applicant")
	serverValue := v2DispatchForTest(t, s, owner.ID, "CreateServer", map[string]any{"name": "Approval workspace", "join_policy": "approval-required"}).(v2Server)
	request := v2DispatchForTest(t, s, applicant.ID, "RequestServerAccess", map[string]string{"server_id": serverValue.ID}).(v2ServerRequest)
	v2DispatchForTest(t, s, owner.ID, "ApproveServerRequest", map[string]string{"request_id": request.ID})
	member := v2DispatchForTest(t, s, owner.ID, "GetServerMember", map[string]string{"server_id": serverValue.ID, "participant_id": applicant.ID}).(v2MemberView)
	if member.IdentityID != applicant.ID {
		t.Fatalf("approved Server member = %#v", member)
	}

	group := v2DispatchForTest(t, s, owner.ID, "CreateGroup", map[string]any{"server_id": serverValue.ID, "name": "Approval group", "join_policy": "approval-required"}).(Group)
	groupRequest := v2DispatchForTest(t, s, applicant.ID, "RequestGroupAccess", map[string]string{"group_id": group.ID}).(v2GroupRequestView)
	v2DispatchForTest(t, s, owner.ID, "ApproveGroupRequest", map[string]string{"request_id": groupRequest.ID})
	v2DispatchForTest(t, s, applicant.ID, "GetGroupHistory", map[string]string{"group": group.ID})
}

func TestV2PresetOverridesPreserveRequiredDefaults(t *testing.T) {
	result, err := v2DiscoveryRequest("ApplyPreset", rpcParams(map[string]any{
		"preset": "minimal",
		"overrides": map[string]any{
			"apiVersion": "",
			"kind":       "",
			"server":     "server-1",
			"identity":   map[string]any{"name": "preset-agent"},
			"response":   map[string]any{"mode": "compact"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	var got struct {
		Preset    string `json:"preset"`
		Effective struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Server     string `json:"server"`
			Identity   struct {
				Name string `json:"name"`
			} `json:"identity"`
			Response struct {
				Mode string `json:"mode"`
			} `json:"response"`
		} `json:"effective"`
	}
	if err = json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got.Preset != "minimal" || got.Effective.APIVersion != "harnesstalkie/v2" || got.Effective.Kind != "Session" || got.Effective.Server != "server-1" || got.Effective.Identity.Name != "preset-agent" || got.Effective.Response.Mode != "compact" {
		t.Fatalf("preset result = %#v", got)
	}
}
