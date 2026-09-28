package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
