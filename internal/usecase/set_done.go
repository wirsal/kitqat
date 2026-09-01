package usecase

import (
	"fmt"

	"github.com/wirsal/kitqat/internal/adapter/jira"
)

// SetDoneUseCase reads the tests of a Jira Test Execution and, for each one, sets
// its Xray test run status to PASS and transitions the test issue to Done.
type SetDoneUseCase struct {
	jiraClient *jira.Client
}

func NewSetDoneUseCase(jc *jira.Client) *SetDoneUseCase {
	return &SetDoneUseCase{jiraClient: jc}
}

// SetDoneResult summarises the outcome.
type SetDoneResult struct {
	Total    int      // number of tests in the execution
	Passed   int      // test runs set to PASS
	Done     int      // test issues transitioned to Done
	Skipped  int      // issues with no Done transition available
	Failed   []string // issue keys that returned an error in either step
	ExecKey  string   // the Test Execution issue key
	ExecDone bool     // whether the Test Execution issue itself was transitioned
}

// Execute reads the Test Execution's tests and, per test, sets the run status to
// PASS and transitions the issue to Done. `source` is a Test Execution issue key or
// browse URL (e.g. BRSF-16163).
func (uc *SetDoneUseCase) Execute(source string) (*SetDoneResult, error) {
	key := jiraIssueKey(source)
	if key == "" {
		return nil, fmt.Errorf("enter a Jira Test Execution issue (e.g. BRSF-16163)")
	}

	issue, err := uc.jiraClient.GetIssue(key)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira issue %s: %w", key, err)
	}
	tests := extractTests(issue.Fields)
	if len(tests) == 0 {
		return nil, fmt.Errorf("issue %s has no associated tests", key)
	}

	result := &SetDoneResult{Total: len(tests), ExecKey: key}

	// Transition ID is shared per project workflow — cache it.
	transitionCache := make(map[string]string)

	for _, t := range tests {
		failed := false

		// 1. Set the Xray test run status to PASS.
		if t.RunID > 0 {
			if err := uc.jiraClient.SetTestRunStatus(t.RunID, "PASS"); err != nil {
				result.Failed = append(result.Failed, t.Key)
				failed = true
			} else {
				result.Passed++
			}
		}

		// 2. Transition the test issue to Done.
		project := projectOf(t.Key)
		transID, cached := transitionCache[project]
		if !cached {
			transitions, err := uc.jiraClient.GetTransitions(t.Key)
			if err != nil {
				if !failed {
					result.Failed = append(result.Failed, t.Key)
				}
				continue
			}
			transID = jira.FindDoneTransitionID(transitions)
			transitionCache[project] = transID
		}
		if transID == "" {
			result.Skipped++
			continue
		}
		if err := uc.jiraClient.TransitionIssue(t.Key, transID); err != nil {
			if !failed {
				result.Failed = append(result.Failed, t.Key)
			}
			continue
		}
		result.Done++
	}

	// Finally, transition the Test Execution issue itself to Done (e.g. CLOSED).
	if transitions, err := uc.jiraClient.GetTransitions(key); err == nil {
		if id := jira.FindDoneTransitionID(transitions); id != "" {
			if err := uc.jiraClient.TransitionIssue(key, id); err == nil {
				result.ExecDone = true
			} else {
				result.Failed = append(result.Failed, key)
			}
		}
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
