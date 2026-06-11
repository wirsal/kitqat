package usecase

import (
	"fmt"
	"regexp"
	"strings"

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
	// 1. Fetch TC → Jira key mapping from CBAgnito.
	entries, err := uc.cbagnitoClient.GetJobTickets(jobURL)
	if err != nil {
		return nil, fmt.Errorf("fetch CBAgnito job tickets: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("CBAgnito job returned no ticket entries")
	}

	// Build mapping: TC number suffix ("01") → Jira key ("SPBRISURF-9736").
	tcNumToKey := buildTCMapping(entries)

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

	// 4. Inject Jira links.
	newBody, updated, skipped := confluence.InjectJiraLinks(body, tcNumToKey, serverName, serverID)

	// Collect TC numbers that had no mapping.
	unmatched := findUnmatchedTCs(body, tcNumToKey)

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

// buildTCMapping converts CBAgnito entries to a map[tcNumSuffix]jiraKey.
// e.g. TicketEntry{TCID:"BRSF02_38_01", TicketIssue:"SPBRISURF-9736"} → map["01"]="SPBRISURF-9736"
func buildTCMapping(entries []cbagnito.TicketEntry) map[string]string {
	m := make(map[string]string, len(entries))
	reSuffix := regexp.MustCompile(`_(\d+)$`)
	for _, e := range entries {
		// Also store by full TC ID for exact match.
		m[e.TCID] = e.TicketIssue
		// Store by numeric suffix (zero-padded).
		if sub := reSuffix.FindStringSubmatch(e.TCID); len(sub) > 1 {
			m[fmt.Sprintf("%02d", atoi(sub[1]))] = e.TicketIssue
		}
	}
	return m
}

// findUnmatchedTCs returns TC numbers present on the page but absent from tcNumToKey.
var reTCNum = regexp.MustCompile(`(?i)TC(\d+)`)

func findUnmatchedTCs(body string, tcNumToKey map[string]string) []string {
	var unmatched []string
	seen := make(map[string]bool)
	for _, m := range reTCNum.FindAllStringSubmatch(body, -1) {
		num := fmt.Sprintf("%02d", atoi(m[1]))
		if seen[num] {
			continue
		}
		seen[num] = true
		if _, ok := tcNumToKey[num]; !ok {
			unmatched = append(unmatched, "TC"+strings.TrimLeft(num, "0"))
		}
	}
	return unmatched
}

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
