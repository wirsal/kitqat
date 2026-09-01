package cbagnito

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const baseURL = "http://172.18.249.109:1014"

// dumpToHome writes diagnostic content to <home>/<name> and returns the path.
// Used to capture the fetched CBAgnito job HTML when parsing finds no tickets,
// regardless of which directory the binary was launched from.
func dumpToHome(name, content string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	path := home + string(os.PathSeparator) + name
	if os.WriteFile(path, []byte(content), 0644) != nil {
		return ""
	}
	return path
}

// Client calls the CBAgnito portal API.
// Authentication uses Flask session cookies obtained via CSRF-aware form login.
type Client struct {
	httpClient *http.Client
	username   string
	password   string
}

func NewClient(username, password string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second, Jar: jar},
		username:   username,
		password:   password,
	}
}

// reCSRF extracts the csrf_token value from a Flask-WTF login form.
var reCSRF = regexp.MustCompile(`name="csrf_token"[^>]*value="([^"]+)"`)

// login performs a two-step Flask login:
//  1. GET /login to obtain the CSRF token and initial session cookie.
//  2. POST /login with form-encoded credentials + CSRF token.
//
// We do NOT follow the redirect after POST (the chain goes through /index which
// only accepts POST, causing 405 for standard GET-on-redirect behaviour).
// Instead, we check that the server issued a 302 — that alone confirms success.
func (c *Client) login() error {
	loginURL := baseURL + "/login"

	// Step 1: GET /login — fetch CSRF token and initial session cookie.
	getReq, err := http.NewRequest(http.MethodGet, loginURL, nil)
	if err != nil {
		return fmt.Errorf("build login GET: %w", err)
	}
	getResp, err := c.httpClient.Do(getReq)
	if err != nil {
		return fmt.Errorf("login GET: %w", err)
	}
	defer getResp.Body.Close()
	htmlBody, _ := io.ReadAll(getResp.Body)

	csrfMatch := reCSRF.FindSubmatch(htmlBody)
	if len(csrfMatch) < 2 {
		return fmt.Errorf("cannot find csrf_token on CBAgnito login page")
	}
	csrfToken := string(csrfMatch[1])

	// Step 2: POST /login — do NOT follow the redirect chain.
	noRedirectClient := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     c.httpClient.Jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{
		"csrf_token": {csrfToken},
		"username":   {c.username},
		"password":   {c.password},
		"login":      {""},
	}

	postReq, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login POST: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	postResp, err := noRedirectClient.Do(postReq)
	if err != nil {
		return fmt.Errorf("login POST: %w", err)
	}
	defer postResp.Body.Close()
	io.ReadAll(postResp.Body) // drain

	// Flask returns 302 on success and 200 (back to login page) on failure.
	if postResp.StatusCode != http.StatusFound {
		return fmt.Errorf("CBAgnito login failed (HTTP %d) — check username and password", postResp.StatusCode)
	}

	// A failed login can ALSO redirect (302) — back to /login — so a 302 alone
	// is not proof of success. Verify the redirect target does not point back to
	// the login page; otherwise the credentials were rejected and any later page
	// fetch would come back as 404 (CBAgnito hides jobs from unauthenticated sessions).
	if loc := postResp.Header.Get("Location"); strings.Contains(strings.ToLower(loc), "login") {
		return fmt.Errorf("CBAgnito login failed — credentials rejected (redirected to %s). "+
			"Note: CBAgnito uses your NIP as the username (e.g. 90185155), which may differ from your Confluence/Jira username", loc)
	}
	return nil
}

// GetJobTickets logs in then fetches ticket entries for the given job URL.
func (c *Client) GetJobTickets(jobURL string) ([]TicketEntry, error) {
	if err := c.login(); err != nil {
		return nil, err
	}
	return c.fetchJobTickets(jobURL)
}

// TicketEntry represents one ticket row from a CBAgnito job result.
type TicketEntry struct {
	TicketIssue string // e.g. "SPBRISURF-9736"
	TCID        string // e.g. "BRSF02_38_01" (may be empty for some jobs)
	Summary     string // scenario summary, used to match the ticket to a page TC table
}

func (c *Client) fetchJobTickets(jobURL string) ([]TicketEntry, error) {
	// Strip whitespace and surrounding quotes (copy-paste artefacts). A trailing
	// space becomes %20 once encoded and makes the server return 404.
	base := strings.TrimSpace(jobURL)
	base = strings.Trim(base, `"'`)
	base = strings.TrimSpace(base)
	base = strings.TrimRight(base, "/")
	parts := strings.Split(base, "/")
	jobID := parts[len(parts)-1]

	// Try JSON API endpoints first, then fall back to HTML parsing.
	apiCandidates := []string{
		base + "/tickets",
		base + "/list",
		hostOf(base) + "/api/cr-job/" + jobID,
		hostOf(base) + "/api/cr-job/" + jobID + "/tickets",
		hostOf(base) + "/api/job/" + jobID + "/tickets",
	}

	for _, u := range apiCandidates {
		if u == "" {
			continue
		}
		entries, err := c.tryJSONEndpoint(u)
		if err == nil && len(entries) > 0 {
			return entries, nil
		}
	}

	// Fallback: fetch the job page as HTML and parse the ticket table.
	return c.parseHTMLPage(base)
}

func (c *Client) tryJSONEndpoint(u string) ([]TicketEntry, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "<") {
		return nil, fmt.Errorf("HTML response")
	}
	return parseJSONResponse(body)
}

// parseHTMLPage fetches the job page as HTML and extracts ticket rows from the table.
func (c *Client) parseHTMLPage(pageURL string) ([]TicketEntry, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch job page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("CBAgnito returned 403 — login may have failed or session expired")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("CBAgnito returned 404 for %s — check the job URL for a wrong job ID or a "+
			"stray space/quote from copy-paste (a trailing space becomes %%20 and 404s)", pageURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CBAgnito returned HTTP %d for %s", resp.StatusCode, pageURL)
	}

	body, _ := io.ReadAll(resp.Body)
	entries := parseHTMLTable(string(body))
	if len(entries) == 0 {
		dump := fmt.Sprintf("<!-- GET %s -> HTTP %d, %d bytes -->\n%s",
			pageURL, resp.StatusCode, len(body), string(body))
		if path := dumpToHome("kitqat_cbagnito_page.html", dump); path != "" {
			return nil, fmt.Errorf("no ticket entries found at CBAgnito job URL: %s\n"+
				"(page HTML dumped to %s — send it for diagnosis)", pageURL, path)
		}
		return nil, fmt.Errorf("no ticket entries found at CBAgnito job URL: %s", pageURL)
	}
	return entries, nil
}

var reJiraKey = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)
var reTCID = regexp.MustCompile(`[A-Z][A-Z0-9]+_[A-Z0-9]+_\d+`)

// parseHTMLTable uses golang.org/x/net/html to walk the DOM and extract
// ticket rows from any table on the page.
func parseHTMLTable(body string) []TicketEntry {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var entries []TicketEntry
	var walkTable func(*html.Node)
	walkTable = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if e := extractEntryFromRow(n); e != nil {
				entries = append(entries, *e)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTable(c)
		}
	}
	walkTable(doc)
	return entries
}

func extractEntryFromRow(tr *html.Node) *TicketEntry {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, strings.TrimSpace(nodeText(c)))
		}
	}

	var ticketIssue, tcID string
	for _, v := range cells {
		if ticketIssue == "" && reJiraKey.MatchString(v) {
			ticketIssue = reJiraKey.FindString(v)
		}
		if tcID == "" && reTCID.MatchString(v) {
			tcID = reTCID.FindString(v)
		}
	}
	// A row is a ticket as soon as it carries a Jira key. The TC ID may be blank
	// (some CBAgnito jobs don't populate it) — tickets are matched to page TC
	// tables by their Summary text instead.
	if ticketIssue == "" {
		return nil
	}
	// Column order on the CBAgnito job table is: Ticket Issue, TC Id, Summary, …
	var summary string
	if len(cells) > 2 {
		summary = cells[2]
	}
	return &TicketEntry{TicketIssue: ticketIssue, TCID: tcID, Summary: summary}
}

// nodeText recursively collects all text nodes under n.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// ── JSON parsing ──────────────────────────────────────────────────────────────

func parseJSONResponse(body []byte) ([]TicketEntry, error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err == nil {
		return parseJSONRows(rows)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("unrecognised CBAgnito response format")
	}
	for _, v := range obj {
		var arr []json.RawMessage
		if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
			return parseJSONRows(arr)
		}
	}
	return nil, nil
}

type rawRow struct {
	TicketIssue string `json:"ticketIssue"`
	Ticket      string `json:"ticket"`
	IssueKey    string `json:"issueKey"`
	Issue       string `json:"issue"`
	Key         string `json:"key"`
	JiraKey     string `json:"jiraKey"`
	TCID        string `json:"tcId"`
	TCIDalt     string `json:"tc_id"`
	TestID      string `json:"testId"`
	TCID2       string `json:"tcID"`
}

func (r rawRow) ticketIssue() string {
	for _, v := range []string{r.TicketIssue, r.Ticket, r.IssueKey, r.Issue, r.Key, r.JiraKey} {
		if v != "" {
			return v
		}
	}
	return ""
}

func (r rawRow) tcID() string {
	for _, v := range []string{r.TCID, r.TCIDalt, r.TestID, r.TCID2} {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseJSONRows(rows []json.RawMessage) ([]TicketEntry, error) {
	var entries []TicketEntry
	for _, raw := range rows {
		var r rawRow
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		issue := r.ticketIssue()
		tcID := r.tcID()
		if issue == "" || tcID == "" {
			continue
		}
		entries = append(entries, TicketEntry{TicketIssue: issue, TCID: tcID})
	}
	return entries, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func hostOf(rawURL string) string {
	count := 0
	for i, c := range rawURL {
		if c == '/' {
			count++
			if count == 3 {
				return rawURL[:i]
			}
		}
	}
	return rawURL
}
