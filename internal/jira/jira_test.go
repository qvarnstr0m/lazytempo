package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lazytempo/internal/httpx"
)

func testDoer(srv *httptest.Server) *httpx.Doer {
	d := httpx.NewDoer(srv.Client())
	d.MaxRetries = 1
	return d
}

func TestMyself(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		// Verify Basic auth header is present and correct.
		auth := r.Header.Get("Authorization")
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("me@example.com:token123"))
		if auth != want {
			t.Errorf("auth = %q, want %q", auth, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"accountId": "abc-123"})
	}))
	defer srv.Close()

	c := New(srv.URL, "me@example.com", "token123", testDoer(srv))
	id, err := c.Myself(context.Background())
	if err != nil {
		t.Fatalf("Myself: %v", err)
	}
	if id != "abc-123" {
		t.Errorf("accountId = %q, want abc-123", id)
	}
}

func TestSearchIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["jql"]; !ok {
			t.Error("missing jql in request body")
		}
		resp := `{
			"issues": [
				{"id":"10991","key":"PROJ-123","fields":{"summary":"Fix bug","status":{"name":"In Progress"},"project":{"id":"100","key":"PROJ"}}},
				{"id":"10992","key":"PROJ-124","fields":{"summary":"Add feature","status":{"name":"To Do"},"project":{"id":"100","key":"PROJ"}}}
			],
			"isLast": true
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	c := New(srv.URL, "me@example.com", "token123", testDoer(srv))
	issues, err := c.SearchIssues(context.Background(), "assignee = currentUser()", 50)
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}
	if issues[0].ID != "10991" || issues[0].Key != "PROJ-123" || issues[0].ProjectID != "100" {
		t.Errorf("unexpected first issue: %+v", issues[0])
	}
	if issues[0].Summary != "Fix bug" || issues[0].Status != "In Progress" {
		t.Errorf("unexpected fields: %+v", issues[0])
	}
}

func TestSearchIssuesPaging(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls++
		if body["nextPageToken"] == nil || body["nextPageToken"] == "" {
			_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"A-1","fields":{"project":{"id":"100"}}}],"nextPageToken":"tok","isLast":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"issues":[{"id":"2","key":"A-2","fields":{"project":{"id":"100"}}}],"isLast":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "me@example.com", "token123", testDoer(srv))
	issues, err := c.SearchIssues(context.Background(), "x", 50)
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if len(issues) != 2 || calls != 2 {
		t.Errorf("paging failed: got %d issues in %d calls", len(issues), calls)
	}
}

func TestSearchIssuesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["Bad JQL"]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "me@example.com", "token123", testDoer(srv))
	_, err := c.SearchIssues(context.Background(), "bad", 50)
	if err == nil || !strings.Contains(err.Error(), "Bad JQL") {
		t.Errorf("expected error containing server body, got %v", err)
	}
}
