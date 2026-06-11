package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Issue is a minimal Jira issue with raw fields map so we can handle any custom fields.
type Issue struct {
	Key    string                 `json:"key"`
	Fields map[string]interface{} `json:"fields"`
}

type searchResult struct {
	Total      int     `json:"total"`
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Issues     []Issue `json:"issues"`
}

// FieldMeta describes one Jira field (from /rest/api/2/field).
type FieldMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchIssues executes a JQL query and returns up to maxResults issues.
// Pass maxResults <= 0 to use the default (50).
func (c *Client) SearchIssues(jql string, maxResults int) ([]Issue, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	var all []Issue
	startAt := 0

	for {
		batch, total, err := c.searchPage(jql, startAt, maxResults)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		startAt += len(batch)
		if startAt >= total || len(batch) == 0 {
			break
		}
	}
	return all, nil
}

func (c *Client) searchPage(jql string, startAt, maxResults int) ([]Issue, int, error) {
	endpoint := c.baseURL + "/rest/api/2/search"

	params := url.Values{}
	params.Set("jql", jql)
	params.Set("fields", "*all")
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("startAt", fmt.Sprintf("%d", startAt))

	req, err := http.NewRequest(http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("jira search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, 0, fmt.Errorf("jira authentication failed — check credentials")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("jira returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var result searchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decode jira response: %w", err)
	}
	return result.Issues, result.Total, nil
}

// GetFields returns all field definitions from the Jira instance.
func (c *Client) GetFields() ([]FieldMeta, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/rest/api/2/field", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get fields: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get fields returned %d", resp.StatusCode)
	}

	var fields []FieldMeta
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return nil, fmt.Errorf("decode fields: %w", err)
	}
	return fields, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
