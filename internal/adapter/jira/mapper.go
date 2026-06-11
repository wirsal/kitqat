package jira

import (
	"fmt"
	"strings"

	"github.com/brisurf/kitqa/internal/domain"
)

// knownRepoPathNames are candidate field name substrings for "Test Repository Path".
var knownRepoPathNames = []string{
	"test repository path",
	"test repository",
	"repository path",
	"folder",
}

// knownExpectedResultNames are candidate field name substrings for "Expected Result".
var knownExpectedResultNames = []string{
	"expected result",
	"expected outcome",
	"expected",
}

// FieldIndex maps lower-cased field names to their Jira custom field IDs.
type FieldIndex map[string]string // lower(name) → fieldID

// BuildFieldIndex creates a FieldIndex from the Jira field list.
func BuildFieldIndex(fields []FieldMeta) FieldIndex {
	idx := make(FieldIndex, len(fields))
	for _, f := range fields {
		idx[strings.ToLower(f.Name)] = f.ID
	}
	return idx
}

// FindField returns the field ID whose name contains any of the given substrings.
func (fi FieldIndex) FindField(substrings ...string) string {
	for name, id := range fi {
		for _, sub := range substrings {
			if strings.Contains(name, sub) {
				return id
			}
		}
	}
	return ""
}

// MapIssue converts a Jira Issue to a domain.TestCase.
// repoPathFieldID and expectedResultFieldID may be empty strings if not found.
func MapIssue(issue Issue, repoPathFieldID, expectedResultFieldID string) domain.TestCase {
	f := issue.Fields

	tc := domain.TestCase{
		TestID:  issue.Key,
		Project: stringFromNestedMap(f, "project", "key"),
		Summary: stringField(f, "summary"),
	}

	if d, ok := f["description"].(string); ok {
		tc.Description = d
	}

	// Assignee — prefer username/accountId over displayName to match the Excel sample
	if a, ok := f["assignee"].(map[string]interface{}); ok {
		if name := stringField(a, "name"); name != "" {
			tc.Assignee = name
		} else if name := stringField(a, "accountId"); name != "" {
			tc.Assignee = name
		} else {
			tc.Assignee = stringField(a, "displayName")
		}
	}

	// Test Repository Path
	if repoPathFieldID != "" {
		tc.TestRepositoryPath = stringField(f, repoPathFieldID)
	}
	// Fallback: look for any string field whose value starts with "/" (path-like)
	if tc.TestRepositoryPath == "" {
		tc.TestRepositoryPath = findPathLikeField(f)
	}

	// Expected Result
	if expectedResultFieldID != "" {
		tc.ExpectedResult = stringField(f, expectedResultFieldID)
	}
	// Fallback: extract from test steps array (Xray format)
	if tc.ExpectedResult == "" {
		tc.ExpectedResult = extractExpectedFromSteps(f)
	}

	return tc
}

// ── helpers ───────────────────────────────────────────────────────────────────

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func stringFromNestedMap(m map[string]interface{}, outerKey, innerKey string) string {
	if outer, ok := m[outerKey].(map[string]interface{}); ok {
		return stringField(outer, innerKey)
	}
	return ""
}

// findPathLikeField finds any string custom field value that looks like a folder path (/...).
func findPathLikeField(fields map[string]interface{}) string {
	for key, val := range fields {
		if !strings.HasPrefix(key, "customfield_") {
			continue
		}
		if s, ok := val.(string); ok && strings.HasPrefix(s, "/") {
			return s
		}
	}
	return ""
}

// extractExpectedFromSteps tries to build the expected result from Xray test steps.
// Xray stores steps as: customfield_XXXX = {"steps": [{"fields": {"Action":"...","Data":"...","Expected Result":"..."}}]}
func extractExpectedFromSteps(fields map[string]interface{}) string {
	for _, val := range fields {
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		steps, ok := m["steps"].([]interface{})
		if !ok {
			continue
		}
		var parts []string
		for i, step := range steps {
			sm, ok := step.(map[string]interface{})
			if !ok {
				continue
			}
			sf, ok := sm["fields"].(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range sf {
				if strings.Contains(strings.ToLower(k), "expected") {
					if s, ok := v.(string); ok && s != "" {
						parts = append(parts, fmt.Sprintf("Step %d: %s", i+1, s))
					}
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return ""
}
