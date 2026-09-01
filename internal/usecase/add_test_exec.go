package usecase

import (
	"fmt"
	"strings"

	"github.com/wirsal/kitqat/internal/adapter/confluence"
)

// AddTestExecUseCase inserts a Test Execution filter table (a Jira JQL table macro)
// at the top of a Confluence Scenario Detail page.
type AddTestExecUseCase struct {
	confluenceUpdater ConfluenceUpdater
}

func NewAddTestExecUseCase(cu ConfluenceUpdater) *AddTestExecUseCase {
	return &AddTestExecUseCase{confluenceUpdater: cu}
}

// AddTestExecResult summarises the outcome.
type AddTestExecResult struct {
	PageTitle string
	JQL       string
	Added     bool // false when the table was already present
}

// Execute resolves the page and inserts the Test Execution table. `source` may be a
// Test Execution issue key (e.g. BRSF-16163) — the JQL is built for you — or a full
// JQL filter (e.g. issue in testExecutionTests("BRSF-16163")).
func (uc *AddTestExecUseCase) Execute(pageURLOrID, source string) (*AddTestExecResult, error) {
	jql := buildTestExecJQL(source)
	if jql == "" {
		return nil, fmt.Errorf("enter a Test Execution issue (e.g. BRSF-16163) or a JQL filter")
	}

	pageID, err := uc.confluenceUpdater.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve confluence page: %w", err)
	}
	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	serverName, serverID := confluence.ExtractJiraServerInfo(body)
	newBody, added := confluence.InsertTestExecutionTable(body, serverName, serverID, jql)

	result := &AddTestExecResult{PageTitle: title, JQL: jql, Added: added}
	if !added {
		return result, nil // already present — nothing to write
	}

	// Refuse to write markup we have unbalanced (only when the original was well-formed).
	if confluence.ValidateStorageBalance(body) == nil {
		if err := confluence.ValidateStorageBalance(newBody); err != nil {
			return nil, fmt.Errorf("insertion would corrupt page markup: %w", err)
		}
	}

	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}
	return result, nil
}

// buildTestExecJQL turns a bare Test Execution issue key into a testExecutionTests
// JQL, or passes through a full JQL filter unchanged.
func buildTestExecJQL(source string) string {
	s := strings.TrimSpace(source)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "testexecutiontests") || strings.Contains(lower, " in ") || strings.ContainsAny(s, "()\"") {
		return s // looks like a full JQL filter
	}
	if key := reJiraIssueKey.FindString(s); key != "" {
		return `issue in testExecutionTests("` + key + `")`
	}
	return s
}
