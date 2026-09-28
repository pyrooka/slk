package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gammons/slk/internal/cache"
	slackclient "github.com/gammons/slk/internal/slack"
)

func TestUserProfileFetcherUsesRequestedWorkspace(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/users.info" || r.FormValue("user") != "U1" {
			t.Errorf("request = %s user=%q", r.URL.Path, r.FormValue("user"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U1","name":"avery","real_name":"Avery Chen","profile":{"display_name":"Avery","real_name":"Avery Chen","title":"Engineer","pronouns":"they/them","email":"avery@example.com","phone":"123","fields":{"X":{"label":"Team","value":"Core"}}}}}`))
	}))
	defer server.Close()
	router := newWorkspaceRouter()
	router.Add(&WorkspaceContext{TeamID: "T1", Client: newTestClient(t, server)})
	router.Add(&WorkspaceContext{TeamID: "T2", Client: &slackclient.Client{}})
	router.Set(router.ByID("T2"))
	profile, err := userProfileFetcher(router)("T1", "U1")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || profile.DisplayName != "Avery" || profile.RealName != "Avery Chen" || profile.Handle != "avery" || profile.Email != "avery@example.com" || profile.Phone != "123" || profile.Title != "Engineer" || profile.Pronouns != "they/them" {
		t.Fatalf("calls=%d profile=%+v", calls, profile)
	}
	if len(profile.Fields) != 1 || profile.Fields[0].Label != "Team" || profile.Fields[0].Value != "Core" {
		t.Fatalf("custom fields = %+v", profile.Fields)
	}
	if _, err := userProfileFetcher(router)("unknown", "U1"); err == nil {
		t.Fatal("unknown team succeeded")
	}
}

func TestConversationInfoFetcherUsesRequestedWorkspaceAndCachedNames(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/conversations.members" || r.FormValue("channel") != "G1" {
			t.Errorf("request = %s channel=%q", r.URL.Path, r.FormValue("channel"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"members":["U2","U1"],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()
	db := newTestDB(t)
	for _, user := range []cache.User{{ID: "U1", WorkspaceID: "T1", DisplayName: "Zoe"}, {ID: "U2", WorkspaceID: "T1", DisplayName: "Alice"}} {
		if err := db.UpsertUser(user); err != nil {
			t.Fatal(err)
		}
	}
	router := newWorkspaceRouter()
	router.Add(&WorkspaceContext{TeamID: "T1", Client: newTestClient(t, server)})
	router.Add(&WorkspaceContext{TeamID: "T2", Client: &slackclient.Client{}})
	router.Set(router.ByID("T2"))
	info, err := conversationInfoFetcher(router, db)("T1", "G1", "group_dm")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Members) != 2 || info.MemberCount != 2 || !info.HasMemberCount || info.Members[0].Name != "Alice" || info.Members[1].Name != "Zoe" {
		t.Fatalf("group info = %+v", info)
	}
	if _, err := conversationInfoFetcher(router, db)("missing", "G1", "group_dm"); err == nil {
		t.Fatal("unknown workspace succeeded")
	}
}

func TestConversationInfoFetcherLoadsChannelDetails(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/conversations.info" || r.FormValue("channel") != "C1" {
			t.Errorf("request = %s channel=%q", r.URL.Path, r.FormValue("channel"))
		}
		if r.FormValue("include_num_members") != "true" {
			t.Errorf("include_num_members = %q", r.FormValue("include_num_members"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C1","creator":"U9","num_members":12,"topic":{"value":"topic"},"purpose":{"value":"description"}}}`))
	}))
	defer server.Close()
	db := newTestDB(t)
	if err := db.UpsertUser(cache.User{ID: "U9", WorkspaceID: "T1", Name: "creator-handle", DisplayName: "Creator"}); err != nil {
		t.Fatal(err)
	}
	router := newWorkspaceRouter()
	router.Add(&WorkspaceContext{TeamID: "T1", Client: newTestClient(t, server)})
	info, err := conversationInfoFetcher(router, db)("T1", "C1", "channel")
	if err != nil {
		t.Fatal(err)
	}
	if info.Topic != "topic" || info.Description != "description" || info.Creator != "Creator" || info.MemberCount != 12 || !info.HasMemberCount {
		t.Fatalf("channel info = %+v", info)
	}
}
