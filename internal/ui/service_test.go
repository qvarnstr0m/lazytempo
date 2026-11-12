package ui

import (
	"context"
	"errors"
	"os"
	"testing"

	"lazytempo/internal/cache"
	"lazytempo/internal/config"
	"lazytempo/internal/domain"
	"lazytempo/internal/tempo"
)

// TestMain isolates the on-disk cache in a temp dir so tests never read or write
// the user's real cache file.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lazytempo-test")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_CACHE_HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// --- mocks ---

type mockJira struct {
	accountID string
	issues    []domain.Issue
	err       error
}

func (m *mockJira) Myself(context.Context) (string, error) { return m.accountID, m.err }
func (m *mockJira) SearchIssues(context.Context, string, int) ([]domain.Issue, error) {
	return m.issues, m.err
}

type mockTempo struct {
	attrs      []domain.WorkAttribute
	accounts   []domain.Account
	links      []domain.AccountLink
	worklogs   []domain.Worklog
	created    *tempo.WorklogInput
	deleted    int
	setAttrKey string
}

func (m *mockTempo) SetAccountAttributeKey(k string) { m.setAttrKey = k }
func (m *mockTempo) WorkAttributes(context.Context) ([]domain.WorkAttribute, error) {
	return m.attrs, nil
}
func (m *mockTempo) Accounts(context.Context) ([]domain.Account, error) { return m.accounts, nil }
func (m *mockTempo) AccountLinks(context.Context, string) ([]domain.AccountLink, error) {
	return m.links, nil
}
func (m *mockTempo) Worklogs(context.Context, string, string, string) ([]domain.Worklog, error) {
	return m.worklogs, nil
}
func (m *mockTempo) CreateWorklog(_ context.Context, in tempo.WorklogInput) (domain.Worklog, error) {
	m.created = &in
	return domain.Worklog{ID: 1, AccountKey: in.AccountKey}, nil
}
func (m *mockTempo) UpdateWorklog(_ context.Context, id int, in tempo.WorklogInput) (domain.Worklog, error) {
	m.created = &in
	return domain.Worklog{ID: id, AccountKey: in.AccountKey}, nil
}
func (m *mockTempo) DeleteWorklog(_ context.Context, id int) error {
	m.deleted = id
	return nil
}

func newTestService(j *mockJira, tp *mockTempo) (*Service, *cache.Cache) {
	c, _ := cache.Open()
	return NewService(config.Default(), j, tp, c), c
}

func TestLoadReferenceResolvesAttrKey(t *testing.T) {
	j := &mockJira{accountID: "acc-1"}
	tp := &mockTempo{
		attrs:    []domain.WorkAttribute{{Key: "_Account_", Type: "ACCOUNT"}},
		accounts: []domain.Account{{Key: "CORE", Name: "Core", Status: "OPEN"}},
	}
	svc, _ := newTestService(j, tp)

	msg := svc.LoadReference()().(referenceLoadedMsg)
	if msg.err != nil {
		t.Fatalf("unexpected err: %v", msg.err)
	}
	if msg.accountID != "acc-1" || msg.attrKey != "_Account_" {
		t.Errorf("unexpected reference: %+v", msg)
	}
	if tp.setAttrKey != "_Account_" {
		t.Errorf("account attribute key not propagated to client: %q", tp.setAttrKey)
	}
}

func TestSaveWorklogEnforcesAccount(t *testing.T) {
	j := &mockJira{}
	tp := &mockTempo{}
	svc, _ := newTestService(j, tp)

	// Missing account: the command must fail before any create call is made.
	msg := svc.SaveWorklog(0, tempo.WorklogInput{
		IssueID:          "10991",
		TimeSpentSeconds: 3600,
		StartDate:        "2026-11-12",
	})().(worklogSavedMsg)
	if !errors.Is(msg.err, domain.ErrNoAccount) {
		t.Errorf("expected ErrNoAccount, got %v", msg.err)
	}
	if tp.created != nil {
		t.Error("worklog without account must never reach the API")
	}

	// With an account: the create goes through and last-used is cached.
	svc.SetAccountID("acc-1")
	msg = svc.SaveWorklog(0, tempo.WorklogInput{
		IssueID:          "10991",
		TimeSpentSeconds: 3600,
		StartDate:        "2026-11-12",
		AccountKey:       "CORE",
	})().(worklogSavedMsg)
	if msg.err != nil {
		t.Fatalf("unexpected err: %v", msg.err)
	}
	if tp.created == nil || tp.created.AccountKey != "CORE" {
		t.Errorf("account not sent to API: %+v", tp.created)
	}
}

func TestLoadIssuesCaches(t *testing.T) {
	j := &mockJira{issues: []domain.Issue{{ID: "1", Key: "A-1", ProjectID: "100"}}}
	tp := &mockTempo{}
	svc, c := newTestService(j, tp)

	msg := svc.LoadIssues()().(issuesLoadedMsg)
	if msg.err != nil || len(msg.issues) != 1 {
		t.Fatalf("unexpected issues result: %+v", msg)
	}
	if len(c.Issues()) != 1 {
		t.Error("issues not written to cache")
	}
}
