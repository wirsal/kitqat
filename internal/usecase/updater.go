package usecase

import (
	"fmt"
	"regexp"

	"github.com/wirsal/kitqa/internal/adapter/cbagnito"
	"github.com/wirsal/kitqa/internal/adapter/confluence"
)

// UpdateScenarioUseCase injects Jira ticket links into the "No. Test Case" cells
// of a Confluence Scenario Detail page, using a CBAgnito job result as the source
// of the TC ID → Jira key mapping.
type UpdateScenarioUseCase struct {
	confluenceUpdater ConfluenceUpdater
	cbagnitoClient    *cbagnito.Client
}

func NewUpdateScenarioUseCase(cu ConfluenceUpdater, cb *cbagnito.Client) *UpdateScenarioUseCase {
	return &UpdateScenarioUseCase{confluenceUpdater: cu, cbagnitoClient: cb}
}

// UpdateResult summarises the outcome of an update run.
type UpdateResult struct {
	PageTitle    string
	UpdatedCells int
	SkippedCells int      // cells that already had the correct Jira link
	UnmatchedTCs []string // TC numbers for which no CBAgnito mapping was found
}

// Execute fetches the CBAgnito job tickets, resolves the Confluence page, injects
// Jira links, and writes the updated content back to Confluence.
func (uc *UpdateScenarioUseCase) Execute(pageURLOrID, jobURL string) (*UpdateResult, error) {
	// 1. Fetch tickets (Jira key + scenario Summary) from CBAgnito.
	entries, err := uc.cbagnitoClient.GetJobTickets(jobURL)
	if err != nil {
		return nil, fmt.Errorf("fetch CBAgnito job tickets: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("CBAgnito job returned no ticket entries")
	}

	// Tickets are matched to page TC tables by their Summary text, so a job whose
	// tickets belong to a different page injects nothing.
	tickets := make([]confluence.ScenarioTicket, 0, len(entries))
	for _, e := range entries {
		tickets = append(tickets, confluence.ScenarioTicket{Summary: e.Summary, Key: e.TicketIssue})
	}

	// 2. Resolve and fetch Confluence page.
	pageID, err := uc.confluenceUpdater.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve confluence page: %w", err)
	}

	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	// 3. Extract Jira server metadata from the page (for macro rendering).
	serverName, serverID := confluence.ExtractJiraServerInfo(body)

	// 4. Inject Jira links, matching each ticket to a TC table by scenario text.
	newBody, updated, skipped, unmatched := confluence.InjectJiraLinksByScenario(body, tickets, serverName, serverID)

	result := &UpdateResult{
		PageTitle:    title,
		UpdatedCells: updated,
		SkippedCells: skipped,
		UnmatchedTCs: unmatched,
	}

	// 5. Skip PUT if nothing changed.
	if newBody == body {
		return result, nil
	}

	// 6. Update Confluence page.
	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}

	return result, nil
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
