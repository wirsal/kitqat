package confluence

import (
	"strings"

	"golang.org/x/net/html"
)

// testExecColumns are the columns shown in the Test Execution filter table,
// matching the reference page (Key, Summary, Status, TestRunStatus).
const testExecColumns = "key,summary,status,testrunstatus"

// BuildTestExecutionMacro builds a Jira Issues (JQL) table macro in storage format
// for a Test Execution's tests, e.g. jql = issue in testExecutionTests("BRSF-16163").
// server/serverID come from the page's existing Jira macros (ExtractJiraServerInfo).
func BuildTestExecutionMacro(serverName, serverID, jql string) string {
	if serverName == "" {
		serverName = "System JIRA"
	}
	var sb strings.Builder
	sb.WriteString(`<ac:structured-macro ac:name="jira" ac:schema-version="1">`)
	sb.WriteString(`<ac:parameter ac:name="server">` + html.EscapeString(serverName) + `</ac:parameter>`)
	if serverID != "" {
		sb.WriteString(`<ac:parameter ac:name="serverId">` + html.EscapeString(serverID) + `</ac:parameter>`)
	}
	sb.WriteString(`<ac:parameter ac:name="columns">` + testExecColumns + `</ac:parameter>`)
	sb.WriteString(`<ac:parameter ac:name="maximumIssues">5</ac:parameter>`)
	sb.WriteString(`<ac:parameter ac:name="jqlQuery">` + html.EscapeString(jql) + `</ac:parameter>`)
	sb.WriteString(`</ac:structured-macro>`)
	return sb.String()
}

// InsertTestExecutionTable inserts a Test Execution filter macro just before the
// first vertical TC table (i.e. under the "Test Execution :" heading). Idempotent:
// if a Jira macro already carries the same jqlQuery, the page is returned unchanged.
// Returns the new HTML and whether it changed.
func InsertTestExecutionTable(storageHTML, serverName, serverID, jql string) (string, bool) {
	// Already present? Match on the escaped jqlQuery value inside a jira macro.
	if strings.Contains(storageHTML, "<ac:parameter ac:name=\"jqlQuery\">"+html.EscapeString(jql)+"</ac:parameter>") {
		return storageHTML, false
	}

	macro := BuildTestExecutionMacro(serverName, serverID, jql)

	// Insert before the first TC table so it sits under the "Test Execution :" heading.
	for _, t := range topLevelElements(storageHTML, "table") {
		if tableHasTCRow(storageHTML[t.start:t.end]) {
			return storageHTML[:t.start] + macro + storageHTML[t.start:], true
		}
	}

	// No TC table on the page — append at the end.
	return storageHTML + macro, true
}
