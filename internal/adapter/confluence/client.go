package confluence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// Do not follow redirects automatically — we need to capture them for page ID resolution.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type pageResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
}

// ResolvePageID converts a short URL (/x/CODE), full Confluence URL, or plain page ID
// into a numeric Confluence page ID.
func (c *Client) ResolvePageID(input string) (string, error) {
	// Already a numeric ID
	if isNumericID(input) {
		return input, nil
	}

	// Build full URL if given a path
	fullURL := input
	if !strings.HasPrefix(input, "http") {
		fullURL = c.baseURL + "/" + strings.TrimLeft(input, "/")
	}

	// Follow all redirects, re-attaching Basic Auth on each hop.
	// Go's default http.Client strips the Authorization header on cross-host redirects;
	// Confluence short URLs (/x/CODE) redirect to viewpage.action?pageId=NNN on the
	// same host, but we set auth explicitly to be safe.
	resolveClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			req.SetBasicAuth(c.username, c.password)
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := resolveClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve short url: %w", err)
	}
	resp.Body.Close()

	// After all redirects, resp.Request.URL is the final destination.
	finalURL := resp.Request.URL.String()

	// Extract pageId from ?pageId=NNN
	if id := extractPageIDFromURL(finalURL); id != "" {
		return id, nil
	}

	// Try /pages/NNN path pattern
	re := regexp.MustCompile(`/pages/(\d+)`)
	if m := re.FindStringSubmatch(finalURL); len(m) > 1 {
		return m[1], nil
	}

	return "", fmt.Errorf("unable to resolve page ID from: %s\n(final URL: %s)", input, finalURL)
}

// GetPage fetches the page storage body by numeric page ID.
func (c *Client) GetPage(pageID string) (*pageResponse, error) {
	endpoint := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,version", c.baseURL, pageID)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	// Use a client that follows redirects for the actual API call
	apiClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed — check CONFLUENCE_USERNAME and CONFLUENCE_PASSWORD")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence API returned %d: %s", resp.StatusCode, string(body))
	}

	var page pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &page, nil
}

// UpdatePage replaces the storage body of a Confluence page (optimistic lock via version number).
func (c *Client) UpdatePage(pageID, title string, newVersion int, newContent string) error {
	endpoint := fmt.Sprintf("%s/rest/api/content/%s", c.baseURL, pageID)

	payload := map[string]interface{}{
		"id":   pageID,
		"type": "page",
		"title": title,
		"version": map[string]interface{}{
			"number": newVersion,
		},
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          newContent,
				"representation": "storage",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build update request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	apiClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("http put page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed — check username and password")
	}
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("page version conflict — page was modified concurrently, please retry")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("confluence API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func isNumericID(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func extractPageIDFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("pageId")
}
