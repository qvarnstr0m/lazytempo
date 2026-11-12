package tempo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"lazytempo/internal/httpx"
)

func testDoer(srv *httptest.Server) *httpx.Doer {
	d := httpx.NewDoer(srv.Client())
	d.MaxRetries = 1
	return d
}

func TestWorkAttributesAndKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth = %q, want Bearer tok", got)
		}
		_, _ = w.Write([]byte(`{"results":[
			{"key":"_Comment_","name":"Comment","type":"TEXT"},
			{"key":"_Account_","name":"Account","type":"ACCOUNT"}
		]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", testDoer(srv))
	attrs, err := c.WorkAttributes(context.Background())
	if err != nil {
		t.Fatalf("WorkAttributes: %v", err)
	}
	if len(attrs) != 2 {
		t.Fatalf("got %d attrs, want 2", len(attrs))
	}
}

func TestWorklogsReadsAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[
			{"tempoWorklogId":5,"timeSpentSeconds":3600,"startDate":"2026-11-12","startTime":"09:00:00","description":"Work","issue":{"id":10991},"author":{"accountId":"abc"},"attributes":{"values":[{"key":"_Account_","value":"CORE"}]}},
			{"tempoWorklogId":6,"timeSpentSeconds":1800,"startDate":"2026-11-12","issue":{"id":10992},"author":{"accountId":"abc"},"attributes":{"values":[]}}
		]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", testDoer(srv))
	logs, err := c.Worklogs(context.Background(), "abc", "2026-11-09", "2026-11-15")
	if err != nil {
		t.Fatalf("Worklogs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
	if logs[0].IssueID != "10991" || logs[0].AccountKey != "CORE" || !logs[0].HasAccount() {
		t.Errorf("unexpected first worklog: %+v", logs[0])
	}
	// Second worklog has no account attribute — must be flagged as missing.
	if logs[1].HasAccount() {
		t.Errorf("expected second worklog to have no account: %+v", logs[1])
	}
}

func TestCreateWorklogSendsAccount(t *testing.T) {
	var received worklogPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/worklogs" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		// Echo back a created worklog.
		_, _ = w.Write([]byte(`{"tempoWorklogId":42,"timeSpentSeconds":3600,"startDate":"2026-11-12","issue":{"id":10991},"attributes":{"values":[{"key":"_Account_","value":"CORE"}]}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", testDoer(srv))
	wl, err := c.CreateWorklog(context.Background(), WorklogInput{
		IssueID:                  "10991",
		AuthorAccountID:          "abc",
		TimeSpentSeconds:         3600,
		StartDate:                "2026-11-12",
		StartTime:                "09:00:00",
		Description:              "Code review",
		AccountKey:               "CORE",
		RemainingEstimateSeconds: 7200,
	})
	if err != nil {
		t.Fatalf("CreateWorklog: %v", err)
	}
	if wl.ID != 42 || wl.AccountKey != "CORE" {
		t.Errorf("unexpected created worklog: %+v", wl)
	}
	// The account must be present in the outgoing payload.
	if received.IssueID != 10991 {
		t.Errorf("issueId = %d, want 10991", received.IssueID)
	}
	if len(received.Attributes) != 1 || received.Attributes[0].Key != "_Account_" || received.Attributes[0].Value != "CORE" {
		t.Errorf("account attribute not sent correctly: %+v", received.Attributes)
	}
	// Tempo requires remainingEstimateSeconds when estimate tracking is on.
	if received.RemainingEstimateSeconds != 7200 {
		t.Errorf("remainingEstimateSeconds = %d, want 7200", received.RemainingEstimateSeconds)
	}
}

func TestCreateWorklogRejectsMissingAccount(t *testing.T) {
	// No server call should be made; the client rejects before sending.
	c := New("http://unused", "tok", httpx.NewDoer(nil))
	_, err := c.CreateWorklog(context.Background(), WorklogInput{
		IssueID:          "10991",
		TimeSpentSeconds: 3600,
		StartDate:        "2026-11-12",
	})
	if err == nil {
		t.Fatal("expected error for missing account, got nil")
	}
}

func TestAccountLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/account-links/project/100" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		// Real Tempo shape: the account is referenced by id (inline and/or via a
		// `self` URL), never by key.
		_, _ = w.Write([]byte(`{"results":[
			{"account":{"self":"https://api.tempo.io/4/accounts/164"},"scope":{"id":100},"default":true},
			{"account":{"id":48,"self":"https://api.tempo.io/4/accounts/48"},"scope":{"id":100},"default":false}
		]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", testDoer(srv))
	links, err := c.AccountLinks(context.Background(), "100")
	if err != nil {
		t.Fatalf("AccountLinks: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
	// First link's id comes from the self URL; second from the inline id.
	if links[0].AccountID != 164 || !links[0].Default {
		t.Errorf("unexpected first link: %+v", links[0])
	}
	if links[1].AccountID != 48 {
		t.Errorf("unexpected second link: %+v", links[1])
	}
}

func TestDeleteWorklog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/worklogs/42" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok", testDoer(srv))
	if err := c.DeleteWorklog(context.Background(), 42); err != nil {
		t.Fatalf("DeleteWorklog: %v", err)
	}
}
