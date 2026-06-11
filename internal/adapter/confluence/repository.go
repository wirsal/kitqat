package confluence

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brisurf/kitqa/internal/adapter/jira"
	"github.com/brisurf/kitqa/internal/domain"
)

// Repository implements usecase.ConfluenceRepository.
type Repository struct {
	client          *Client
	jiraClient      *jira.Client // optional; used when Jira macros are found
	defaultRepoPath string       // fallback if Jira custom field not found
	defaultProject  string       // fallback project key for vertical table pages
}

func NewRepository(client *Client) *Repository {
	return &Repository{client: client}
}

// WithJiraClient attaches a Jira client for fetching issues referenced by Jira macros.
func (r *Repository) WithJiraClient(jc *jira.Client) {
	r.jiraClient = jc
}

// SetDefaultRepoPath sets a fallback Test Repository Path used when the value
// cannot be discovered from Jira custom fields.
func (r *Repository) SetDefaultRepoPath(path string) {
	r.defaultRepoPath = path
}

// SetDefaultProject sets the project key used for vertical table pages.
// Defaults to "SPBRISURF" if not set.
func (r *Repository) SetDefaultProject(project string) {
	r.defaultProject = project
}

func (r *Repository) ResolvePageID(urlOrID string) (string, error) {
	return r.client.ResolvePageID(urlOrID)
}

// GetPageRaw returns the page title, current version number, and storage body.
func (r *Repository) GetPageRaw(pageID string) (title string, version int, body string, err error) {
	page, err := r.client.GetPage(pageID)
	if err != nil {
		return "", 0, "", err
	}
	return page.Title, page.Version.Number, page.Body.Storage.Value, nil
}

// UpdatePageContent replaces the storage body of a page using an incremented version number.
func (r *Repository) UpdatePageContent(pageID, title string, currentVersion int, newBody string) error {
	return r.client.UpdatePage(pageID, title, currentVersion+1, newBody)
}

func (r *Repository) FetchTestCases(pageID string) ([]domain.TestCase, error) {
	page, err := r.client.GetPage(pageID)
	if err != nil {
		return nil, err
	}

	// Try 1: vertical two-column tables (Scenario Detail / Screen Capture pages).
	if rawCases := ParseVerticalTestCases(page.Body.Storage.Value, page.Title); len(rawCases) > 0 {
		return r.verticalToTestCases(rawCases), nil
	}

	// Try 2: horizontal table with header row.
	if rawCases := ParseTestCasesFromHTML(page.Body.Storage.Value); len(rawCases) > 0 {
		return rawToTestCases(rawCases), nil
	}

	// Try 3: Jira macros → fetch from Jira API.
	if r.jiraClient != nil {
		macros := ParseJiraMacros(page.Body.Storage.Value)
		if len(macros) > 0 {
			return r.fetchFromJira(macros)
		}
	}

	return nil, fmt.Errorf("no test cases found in page %q (title: %s)", pageID, page.Title)
}

// fetchFromJira resolves test cases from Jira macros using the Jira API.
func (r *Repository) fetchFromJira(macros []JiraMacro) ([]domain.TestCase, error) {
	// Discover custom field IDs for test-specific fields.
	fields, err := r.jiraClient.GetFields()
	if err != nil {
		// Non-fatal — we'll proceed without custom fields.
		fields = nil
	}
	fieldIdx := jira.BuildFieldIndex(fields)
	repoPathFieldID := fieldIdx.FindField(
		"test repository path", "test repository", "repository path", "folder",
	)
	expectedFieldID := fieldIdx.FindField(
		"expected result", "expected outcome",
	)

	var allIssues []jira.Issue
	for _, macro := range macros {
		jql := macro.JQL
		if jql == "" && len(macro.Keys) > 0 {
			jql = "key in (" + strings.Join(macro.Keys, ",") + ")"
		}
		if jql == "" {
			continue
		}
		max := macro.MaxResults
		if max <= 0 {
			max = 200
		}
		issues, err := r.jiraClient.SearchIssues(jql, max)
		if err != nil {
			return nil, fmt.Errorf("jira search: %w", err)
		}
		allIssues = append(allIssues, issues...)
	}

	if len(allIssues) == 0 {
		return nil, fmt.Errorf("jira returned 0 issues for the Jira macros on this page")
	}

	testCases := make([]domain.TestCase, len(allIssues))
	for i, issue := range allIssues {
		tc := jira.MapIssue(issue, repoPathFieldID, expectedFieldID)
		tc.No = i + 1
		if tc.TestRepositoryPath == "" {
			tc.TestRepositoryPath = r.defaultRepoPath
		}
		testCases[i] = tc
	}
	return testCases, nil
}

// verticalToTestCases converts vertical-table raw cases, applying repo/issue-type defaults.
func (r *Repository) verticalToTestCases(rawCases []RawTestCase) []domain.TestCase {
	testCases := make([]domain.TestCase, len(rawCases))
	for i, raw := range rawCases {
		no := atoi(raw.No)
		if no == 0 {
			no = i + 1
		}
		project := r.defaultProject
		if project == "" {
			project = "SPBRISURF"
		}
		testCases[i] = domain.TestCase{
			No:                 no,
			Project:            project,
			IssueType:          "Test",
			TestID:             raw.TestID,
			Summary:            raw.Summary,
			Description:        raw.Description,
			TestRepositoryPath: r.defaultRepoPath,
			ExpectedResult:     raw.ExpectedResult,
		}
	}
	return testCases
}

func rawToTestCases(rawCases []RawTestCase) []domain.TestCase {
	testCases := make([]domain.TestCase, len(rawCases))
	for i, raw := range rawCases {
		no, _ := strconv.Atoi(raw.No)
		if no == 0 {
			no = i + 1
		}
		testCases[i] = domain.TestCase{
			No:                 no,
			Project:            raw.Project,
			TestID:             raw.TestID,
			Summary:            raw.Summary,
			Description:        raw.Description,
			Assignee:           raw.Assignee,
			TestRepositoryPath: raw.TestRepositoryPath,
			ExpectedResult:     raw.ExpectedResult,
		}
	}
	return testCases
}
