package usecase

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/wirsal/kitqa/internal/adapter/confluence"
	"github.com/wirsal/kitqa/internal/adapter/jira"
)

// reJiraIssueKey matches a Jira issue key such as "BRSF-16163".
var reJiraIssueKey = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

// UpdateScenarioUseCase injects Jira ticket links into the "No. Test Case" cells of
// a Confluence Scenario Detail page. The source is a Jira Test Execution issue (e.g.
// BRSF-16163): its associated tests are matched to the page's TC tables by summary,
// and each test's key is injected into the matching cell.
type UpdateScenarioUseCase struct {
	confluenceUpdater ConfluenceUpdater
	jiraClient        *jira.Client
}

func NewUpdateScenarioUseCase(cu ConfluenceUpdater, jc *jira.Client) *UpdateScenarioUseCase {
	return &UpdateScenarioUseCase{confluenceUpdater: cu, jiraClient: jc}
}

// UpdateResult summarises the outcome of an update run.
type UpdateResult struct {
	PageTitle    string
	UpdatedCells int
	SkippedCells int      // cells that already had the correct Jira link
	UnmatchedTCs []string // TC tables on the page that matched no test
}

// Execute reads the tests associated with a Jira Test Execution issue, resolves the
// Confluence page, injects each test's link into the TC table whose scenario matches
// the test summary, and writes the page back.
func (uc *UpdateScenarioUseCase) Execute(pageURLOrID, source string) (*UpdateResult, error) {
	if uc.jiraClient == nil {
		return nil, fmt.Errorf("no Jira client configured")
	}
	issueKey := jiraIssueKey(source)
	if issueKey == "" {
		return nil, fmt.Errorf("enter a Jira issue key or URL (e.g. BRSF-16163) as the source")
	}

	// 1. Fetch the Test Execution's associated tests (key + summary) from Jira.
	tickets, err := uc.jiraTestsAsTickets(issueKey)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return nil, fmt.Errorf("Jira issue %s has no associated tests", issueKey)
	}

	// 2. Resolve and fetch the Confluence page.
	pageID, err := uc.confluenceUpdater.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve confluence page: %w", err)
	}
	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	// 3. Inject, matching each test to a TC table by scenario/summary text.
	serverName, serverID := confluence.ExtractJiraServerInfo(body)
	newBody, updated, skipped, unmatched := confluence.InjectJiraLinksByScenario(body, tickets, serverName, serverID)

	result := &UpdateResult{
		PageTitle:    title,
		UpdatedCells: updated,
		SkippedCells: skipped,
		UnmatchedTCs: unmatched,
	}

	// 4. Skip PUT if nothing changed; dump diagnostics when nothing matched.
	if newBody == body {
		if updated == 0 && skipped == 0 {
			dumpMatchDiagnostics(pageID, title, body, tickets)
		}
		return result, nil
	}

	// 5. Write the page back.
	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}
	return result, nil
}

// jiraTestsAsTickets reads the tests associated with a Test Execution issue and
// returns them as ScenarioTickets (Jira key + summary) in their listed order.
func (uc *UpdateScenarioUseCase) jiraTestsAsTickets(issueKey string) ([]confluence.ScenarioTicket, error) {
	issue, err := uc.jiraClient.GetIssue(issueKey)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira issue %s: %w", issueKey, err)
	}
	keys := extractTestKeys(issue.Fields)
	if len(keys) == 0 {
		return nil, fmt.Errorf("issue %s is not a Test Execution with associated tests", issueKey)
	}

	// Fetch the test summaries in one query.
	summaries, err := uc.fetchSummaries(keys)
	if err != nil {
		return nil, err
	}

	tickets := make([]confluence.ScenarioTicket, 0, len(keys))
	for _, k := range keys {
		tickets = append(tickets, confluence.ScenarioTicket{Summary: summaries[k], Key: k})
	}
	return tickets, nil
}

// fetchSummaries returns a key→summary map for the given issue keys.
func (uc *UpdateScenarioUseCase) fetchSummaries(keys []string) (map[string]string, error) {
	jql := "key in (" + strings.Join(keys, ",") + ")"
	issues, err := uc.jiraClient.SearchIssues(jql, len(keys))
	if err != nil {
		return nil, fmt.Errorf("fetch test summaries: %w", err)
	}
	m := make(map[string]string, len(issues))
	for _, is := range issues {
		if s, ok := is.Fields["summary"].(string); ok {
			m[is.Key] = s
		}
	}
	return m, nil
}

// jiraIssueKey extracts the issue key from a browse URL or bare key.
func jiraIssueKey(s string) string {
	return reJiraIssueKey.FindString(strings.TrimSpace(s))
}

// dumpMatchDiagnostics writes the Confluence page HTML and the Jira test summaries
// to the home directory when an Update run matches nothing, so a scenario-vs-summary
// mismatch can be inspected. Best-effort; errors ignored.
func dumpMatchDiagnostics(pageID, title, body string, tickets []confluence.ScenarioTicket) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	sep := string(os.PathSeparator)
	_ = os.WriteFile(home+sep+"kitqa_confluence_page.html",
		[]byte(fmt.Sprintf("<!-- page %s, title=%s -->\n%s", pageID, title, body)), 0644)

	var sb strings.Builder
	fmt.Fprintf(&sb, "Jira tests: %d\n\n", len(tickets))
	for i, t := range tickets {
		fmt.Fprintf(&sb, "%2d. key=%s\n    summary=%q\n", i+1, t.Key, t.Summary)
	}
	_ = os.WriteFile(home+sep+"kitqa_jira_tests.txt", []byte(sb.String()), 0644)
}

// reTCNum matches a "TCnn" token; shared with fill_scenario.go.
var reTCNum = regexp.MustCompile(`(?i)TC(\d+)`)

// atoi parses a leading run of digits, stopping at the first non-digit. Shared
// across the usecase package (also used by fill_scenario.go).
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
