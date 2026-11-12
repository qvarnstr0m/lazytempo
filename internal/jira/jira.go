// Package jira is a small client for the Jira Cloud REST API v3, exposing only
// what lazytempo needs: resolving the current user and searching issues with
// JQL (which also resolves issue key -> numeric issueId/projectId).
//
// The behaviour is defined by the Client interface so the UI can be exercised
// with mocks in tests.
package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"lazytempo/internal/domain"
	"lazytempo/internal/httpx"
)

// Client is the behaviour lazytempo depends on from Jira.
type Client interface {
	// Myself returns the authenticated user's Jira accountId.
	Myself(ctx context.Context) (string, error)
	// SearchIssues runs a JQL query and returns matching issues.
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]domain.Issue, error)
}

// HTTPClient is the concrete Client backed by the Jira Cloud REST API.
type HTTPClient struct {
	baseURL string
	email   string
	token   string
	http    *httpx.Doer
}

// New builds a Jira client using Basic auth (email + API token).
func New(baseURL, email, token string, doer *httpx.Doer) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		email:   email,
		token:   token,
		http:    doer,
	}
}

func (c *HTTPClient) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	req, err := httpx.NewJSONRequest(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	return req, nil
}

type myselfResponse struct {
	AccountID string `json:"accountId"`
}

// Myself implements Client.
func (c *HTTPClient) Myself(ctx context.Context) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/rest/api/3/myself", nil)
	if err != nil {
		return "", err
	}
	var out myselfResponse
	if err := c.http.DoJSON(req, &out); err != nil {
		return "", fmt.Errorf("jira myself: %w", err)
	}
	if out.AccountID == "" {
		return "", fmt.Errorf("jira myself: empty accountId in response")
	}
	return out.AccountID, nil
}

// searchRequest is the body for the enhanced JQL search endpoint
// (POST /rest/api/3/search/jql).
type searchRequest struct {
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults"`
	Fields        []string `json:"fields"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

type searchResponse struct {
	Issues        []issueJSON `json:"issues"`
	NextPageToken string      `json:"nextPageToken"`
	IsLast        bool        `json:"isLast"`
}

type issueJSON struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		Project struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"project"`
	} `json:"fields"`
}

// SearchIssues implements Client. It pages through results until maxResults is
// reached or the server reports the last page.
func (c *HTTPClient) SearchIssues(ctx context.Context, jql string, maxResults int) ([]domain.Issue, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	fields := []string{"summary", "status", "project"}
	var issues []domain.Issue
	token := ""
	for {
		pageSize := maxResults - len(issues)
		if pageSize > 100 {
			pageSize = 100
		}
		req, err := c.newRequest(ctx, http.MethodPost, "/rest/api/3/search/jql", searchRequest{
			JQL:           jql,
			MaxResults:    pageSize,
			Fields:        fields,
			NextPageToken: token,
		})
		if err != nil {
			return nil, err
		}
		var out searchResponse
		if err := c.http.DoJSON(req, &out); err != nil {
			return nil, fmt.Errorf("jira search: %w", err)
		}
		for _, i := range out.Issues {
			issues = append(issues, domain.Issue{
				ID:         i.ID,
				Key:        i.Key,
				Summary:    i.Fields.Summary,
				ProjectID:  i.Fields.Project.ID,
				ProjectKey: i.Fields.Project.Key,
				Status:     i.Fields.Status.Name,
			})
		}
		if out.IsLast || out.NextPageToken == "" || len(issues) >= maxResults {
			break
		}
		token = out.NextPageToken
	}
	return issues, nil
}

// EscapeJQL is a tiny helper for building key-based lookups safely.
func EscapeJQL(key string) string {
	return url.QueryEscape(key)
}
