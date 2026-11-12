// Package tempo is a client for the Tempo REST API v4. Tempo works with numeric
// issueId/projectId rather than Jira keys.
//
// The behaviour is defined by the Client interface so the UI can be tested with
// mocks. The ACCOUNT work-attribute key is resolved dynamically (commonly
// "_Account_" but configurable per installation) and used to read and write the
// mandatory account on every worklog.
package tempo

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"lazytempo/internal/domain"
	"lazytempo/internal/httpx"
)

// DefaultBaseURL is the Tempo Cloud API v4 base URL.
const DefaultBaseURL = "https://api.tempo.io/4"

// DefaultAccountAttributeKey is Tempo's usual ACCOUNT attribute key. The real
// key is resolved from GET /work-attributes at startup; this is only a fallback.
const DefaultAccountAttributeKey = "_Account_"

// WorklogInput is the data required to create or update a worklog.
type WorklogInput struct {
	IssueID          string
	AuthorAccountID  string
	TimeSpentSeconds int
	StartDate        string
	StartTime        string
	Description      string
	AccountKey       string
	// RemainingEstimateSeconds is sent as-is; Tempo requires it when the
	// project uses estimate tracking. Pass the issue's current remaining
	// estimate to preserve it.
	RemainingEstimateSeconds int
}

// Client is the behaviour lazytempo depends on from Tempo.
type Client interface {
	// SetAccountAttributeKey sets the ACCOUNT work-attribute key used when
	// reading and writing worklogs (resolved from WorkAttributes at startup).
	SetAccountAttributeKey(key string)
	WorkAttributes(ctx context.Context) ([]domain.WorkAttribute, error)
	Accounts(ctx context.Context) ([]domain.Account, error)
	AccountLinks(ctx context.Context, projectID string) ([]domain.AccountLink, error)
	Worklogs(ctx context.Context, accountID, from, to string) ([]domain.Worklog, error)
	CreateWorklog(ctx context.Context, in WorklogInput) (domain.Worklog, error)
	UpdateWorklog(ctx context.Context, id int, in WorklogInput) (domain.Worklog, error)
	DeleteWorklog(ctx context.Context, id int) error
}

// HTTPClient is the concrete Client backed by the Tempo REST API.
type HTTPClient struct {
	baseURL string
	token   string
	http    *httpx.Doer

	mu          sync.RWMutex
	accountAttr string
}

// New builds a Tempo client. baseURL defaults to DefaultBaseURL when empty.
func New(baseURL, token string, doer *httpx.Doer) *HTTPClient {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &HTTPClient{
		baseURL:     baseURL,
		token:       token,
		http:        doer,
		accountAttr: DefaultAccountAttributeKey,
	}
}

// SetAccountAttributeKey overrides the ACCOUNT attribute key used when reading
// and writing worklogs. Call it after resolving the key from WorkAttributes.
func (c *HTTPClient) SetAccountAttributeKey(key string) {
	if key == "" {
		return
	}
	c.mu.Lock()
	c.accountAttr = key
	c.mu.Unlock()
}

func (c *HTTPClient) accountKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accountAttr
}

func (c *HTTPClient) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	req, err := httpx.NewJSONRequest(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return req, nil
}

// --- Work attributes ---

type workAttrResponse struct {
	Results []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"results"`
}

// WorkAttributes implements Client.
func (c *HTTPClient) WorkAttributes(ctx context.Context) ([]domain.WorkAttribute, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/work-attributes", nil)
	if err != nil {
		return nil, err
	}
	var out workAttrResponse
	if err := c.http.DoJSON(req, &out); err != nil {
		return nil, fmt.Errorf("tempo work-attributes: %w", err)
	}
	attrs := make([]domain.WorkAttribute, 0, len(out.Results))
	for _, a := range out.Results {
		attrs = append(attrs, domain.WorkAttribute{Key: a.Key, Name: a.Name, Type: a.Type})
	}
	return attrs, nil
}

// --- Accounts ---

type accountResponse struct {
	Results []struct {
		Key    string `json:"key"`
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"results"`
	Metadata struct {
		Next string `json:"next"`
	} `json:"metadata"`
}

// Accounts implements Client, paging through all accounts.
func (c *HTTPClient) Accounts(ctx context.Context) ([]domain.Account, error) {
	path := "/accounts?limit=200"
	var accounts []domain.Account
	for path != "" {
		req, err := c.newRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var out accountResponse
		if err := c.http.DoJSON(req, &out); err != nil {
			return nil, fmt.Errorf("tempo accounts: %w", err)
		}
		for _, a := range out.Results {
			accounts = append(accounts, domain.Account{Key: a.Key, ID: a.ID, Name: a.Name, Status: a.Status})
		}
		path = relativeNext(c.baseURL, out.Metadata.Next)
	}
	return accounts, nil
}

// --- Account links ---

type accountLinkResponse struct {
	Results []struct {
		Account struct {
			ID   int    `json:"id"`
			Self string `json:"self"`
		} `json:"account"`
		Scope struct {
			ID int `json:"id"`
		} `json:"scope"`
		Default bool `json:"default"`
	} `json:"results"`
}

// AccountLinks implements Client, returning the account links for a project.
//
// Tempo identifies the linked account by numeric id. When the id is not inlined
// it is parsed from the account's `self` URL (e.g. .../accounts/164).
func (c *HTTPClient) AccountLinks(ctx context.Context, projectID string) ([]domain.AccountLink, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/account-links/project/"+projectID, nil)
	if err != nil {
		return nil, err
	}
	var out accountLinkResponse
	if err := c.http.DoJSON(req, &out); err != nil {
		return nil, fmt.Errorf("tempo account-links: %w", err)
	}
	links := make([]domain.AccountLink, 0, len(out.Results))
	for _, l := range out.Results {
		id := l.Account.ID
		if id == 0 {
			id = idFromSelf(l.Account.Self)
		}
		if id == 0 {
			continue
		}
		links = append(links, domain.AccountLink{
			AccountID: id,
			ProjectID: projectID,
			Default:   l.Default,
		})
	}
	return links, nil
}

// idFromSelf extracts the trailing numeric id from a Tempo `self` URL such as
// "https://api.tempo.io/4/accounts/164".
func idFromSelf(self string) int {
	if self == "" {
		return 0
	}
	last := self[strings.LastIndex(self, "/")+1:]
	n, err := strconv.Atoi(last)
	if err != nil {
		return 0
	}
	return n
}

// --- Worklogs ---

type worklogJSON struct {
	TempoWorklogID   int    `json:"tempoWorklogId"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	StartDate        string `json:"startDate"`
	StartTime        string `json:"startTime"`
	Description      string `json:"description"`
	Issue            struct {
		ID int `json:"id"`
	} `json:"issue"`
	Author struct {
		AccountID string `json:"accountId"`
	} `json:"author"`
	Attributes struct {
		Values []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"values"`
	} `json:"attributes"`
}

func (c *HTTPClient) toDomainWorklog(w worklogJSON) domain.Worklog {
	accountKey := ""
	wantKey := c.accountKey()
	for _, a := range w.Attributes.Values {
		if a.Key == wantKey {
			accountKey = a.Value
			break
		}
	}
	return domain.Worklog{
		ID:               w.TempoWorklogID,
		IssueID:          strconv.Itoa(w.Issue.ID),
		AuthorAccountID:  w.Author.AccountID,
		TimeSpentSeconds: w.TimeSpentSeconds,
		StartDate:        w.StartDate,
		StartTime:        w.StartTime,
		Description:      w.Description,
		AccountKey:       accountKey,
	}
}

type worklogListResponse struct {
	Results  []worklogJSON `json:"results"`
	Metadata struct {
		Next string `json:"next"`
	} `json:"metadata"`
}

// Worklogs implements Client, fetching the user's worklogs in a date range.
func (c *HTTPClient) Worklogs(ctx context.Context, accountID, from, to string) ([]domain.Worklog, error) {
	path := fmt.Sprintf("/worklogs/user/%s?from=%s&to=%s&limit=200", accountID, from, to)
	var worklogs []domain.Worklog
	for path != "" {
		req, err := c.newRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var out worklogListResponse
		if err := c.http.DoJSON(req, &out); err != nil {
			return nil, fmt.Errorf("tempo worklogs: %w", err)
		}
		for _, w := range out.Results {
			worklogs = append(worklogs, c.toDomainWorklog(w))
		}
		path = relativeNext(c.baseURL, out.Metadata.Next)
	}
	return worklogs, nil
}

// worklogPayload is the request body for create/update.
type worklogPayload struct {
	IssueID                  int               `json:"issueId"`
	AuthorAccountID          string            `json:"authorAccountId"`
	TimeSpentSeconds         int               `json:"timeSpentSeconds"`
	StartDate                string            `json:"startDate"`
	StartTime                string            `json:"startTime,omitempty"`
	Description              string            `json:"description"`
	RemainingEstimateSeconds int               `json:"remainingEstimateSeconds"`
	Attributes               []attributeKeyVal `json:"attributes"`
}

type attributeKeyVal struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (c *HTTPClient) buildPayload(in WorklogInput) (worklogPayload, error) {
	issueID, err := strconv.Atoi(in.IssueID)
	if err != nil {
		return worklogPayload{}, fmt.Errorf("invalid issueId %q: %w", in.IssueID, err)
	}
	startTime := in.StartTime
	if startTime == "" {
		startTime = "09:00:00"
	}
	// The account attribute is sent only when a key is provided; an empty
	// account is simply omitted, so logging without an account is possible when
	// policy allows it (the require-account rule is enforced upstream).
	attrs := []attributeKeyVal{}
	if in.AccountKey != "" {
		attrs = append(attrs, attributeKeyVal{Key: c.accountKey(), Value: in.AccountKey})
	}
	return worklogPayload{
		IssueID:                  issueID,
		AuthorAccountID:          in.AuthorAccountID,
		TimeSpentSeconds:         in.TimeSpentSeconds,
		StartDate:                in.StartDate,
		StartTime:                startTime,
		Description:              in.Description,
		RemainingEstimateSeconds: in.RemainingEstimateSeconds,
		Attributes:               attrs,
	}, nil
}

// CreateWorklog implements Client.
func (c *HTTPClient) CreateWorklog(ctx context.Context, in WorklogInput) (domain.Worklog, error) {
	payload, err := c.buildPayload(in)
	if err != nil {
		return domain.Worklog{}, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/worklogs", payload)
	if err != nil {
		return domain.Worklog{}, err
	}
	var out worklogJSON
	if err := c.http.DoJSON(req, &out); err != nil {
		return domain.Worklog{}, fmt.Errorf("tempo create worklog: %w", err)
	}
	return c.toDomainWorklog(out), nil
}

// UpdateWorklog implements Client.
func (c *HTTPClient) UpdateWorklog(ctx context.Context, id int, in WorklogInput) (domain.Worklog, error) {
	payload, err := c.buildPayload(in)
	if err != nil {
		return domain.Worklog{}, err
	}
	req, err := c.newRequest(ctx, http.MethodPut, fmt.Sprintf("/worklogs/%d", id), payload)
	if err != nil {
		return domain.Worklog{}, err
	}
	var out worklogJSON
	if err := c.http.DoJSON(req, &out); err != nil {
		return domain.Worklog{}, fmt.Errorf("tempo update worklog: %w", err)
	}
	return c.toDomainWorklog(out), nil
}

// DeleteWorklog implements Client.
func (c *HTTPClient) DeleteWorklog(ctx context.Context, id int) error {
	req, err := c.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/worklogs/%d", id), nil)
	if err != nil {
		return err
	}
	if err := c.http.DoJSON(req, nil); err != nil {
		return fmt.Errorf("tempo delete worklog: %w", err)
	}
	return nil
}
