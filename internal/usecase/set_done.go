package usecase

import (
	"fmt"

	"github.com/wirsal/kitqa/internal/adapter/cbagnito"
	"github.com/wirsal/kitqa/internal/adapter/jira"
)

// SetDoneUseCase fetches ticket entries from a CBAgnito job and transitions
// each corresponding Jira issue to the "Done" state.
type SetDoneUseCase struct {
	jiraClient     *jira.Client
	cbagnitoClient *cbagnito.Client
}

func NewSetDoneUseCase(jc *jira.Client, cb *cbagnito.Client) *SetDoneUseCase {
	return &SetDoneUseCase{jiraClient: jc, cbagnitoClient: cb}
}

// SetDoneResult summarises the outcome.
type SetDoneResult struct {
	Total   int
	Updated int
	Skipped int      // already in done state or no transition found
	Failed  []string // issue keys that returned an error
}

// Execute fetches tickets from CBAgnito then transitions each Jira issue to Done.
func (uc *SetDoneUseCase) Execute(jobURL string) (*SetDoneResult, error) {
	entries, err := uc.cbagnitoClient.GetJobTickets(jobURL)
	if err != nil {
		return nil, fmt.Errorf("fetch CBAgnito tickets: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("CBAgnito job returned no ticket entries")
	}

	// Deduplicate Jira keys (CBAgnito may repeat entries).
	seen := make(map[string]bool)
	var keys []string
	for _, e := range entries {
		if e.TicketIssue != "" && !seen[e.TicketIssue] {
			seen[e.TicketIssue] = true
			keys = append(keys, e.TicketIssue)
		}
	}

	result := &SetDoneResult{Total: len(keys)}

	// Cache transition ID per project key (e.g. "SPBRISURF") — all tickets in
	// one project share the same workflow.
	transitionCache := make(map[string]string)

	for _, issueKey := range keys {
		project := projectOf(issueKey)

		transID, cached := transitionCache[project]
		if !cached {
			transitions, err := uc.jiraClient.GetTransitions(issueKey)
			if err != nil {
				result.Failed = append(result.Failed, issueKey)
				continue
			}
			transID = jira.FindDoneTransitionID(transitions)
			transitionCache[project] = transID
		}

		if transID == "" {
			result.Skipped++
			continue
		}

		if err := uc.jiraClient.TransitionIssue(issueKey, transID); err != nil {
			result.Failed = append(result.Failed, issueKey)
			continue
		}
		result.Updated++
	}

	return result, nil
}

// projectOf extracts the project key from a Jira issue key, e.g. "SPBRISURF-9932" → "SPBRISURF".
func projectOf(issueKey string) string {
	for i, c := range issueKey {
		if c == '-' {
			return issueKey[:i]
		}
	}
	return issueKey
}
