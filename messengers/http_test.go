package messengers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackResponseHandling(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"success", 200, "ok", true}, {"rejected", 401, "no", false},
		{"oversized", 200, strings.Repeat("x", 65537), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			slack := &Slack{IncomingURL: server.URL}
			if got := slack.Post("sensitive output"); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if slack.Channel != "" {
				t.Fatal("mutated configuration")
			}
		})
	}
}

func TestNotificationDoesNotFollowRedirect(t *testing.T) {
	received := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	if (&Slack{IncomingURL: source.URL}).Post("secret") {
		t.Fatal("redirect accepted")
	}
	if received {
		t.Fatal("notification leaked to redirect target")
	}
}

func TestHipChat2Request(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/room/room/notification" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("invalid request")
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	if !(&HipChat2{BaseURL: server.URL + "/v2/", RoomID: "room", Token: "token"}).Post("hello") {
		t.Fatal("notification failed")
	}
}
