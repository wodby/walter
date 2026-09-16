package services

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUpdateReplacesSymlink(t *testing.T) {
	dir := t.TempDir()
	victim, dest := filepath.Join(dir, "victim"), filepath.Join(dir, "status")
	if err := os.WriteFile(victim, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, dest); err != nil {
		t.Fatal(err)
	}
	if !SaveLastUpdate(dest, Update{Status: "done"}) {
		t.Fatal("save failed")
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "untouched" {
		t.Fatal("symlink target modified")
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("status permissions not private")
	}
}

func TestGitHubEmptyAndFailedCommits(t *testing.T) {
	for _, status := range []int{200, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/org/repo/commits" {
				w.WriteHeader(status)
			}
			w.Write([]byte("[]"))
		}))
		base, _ := url.Parse(server.URL + "/")
		client := &GitHubClient{From: "org", Repo: "repo", BaseUrl: base, Token: "test"}
		commits, err := client.GetCommits(Update{})
		server.Close()
		if status == 200 && (err != nil || commits.Len() != 0) {
			t.Fatalf("empty repository: %v", err)
		}
		if status == 500 && err == nil {
			t.Fatal("API error ignored")
		}
	}
}
