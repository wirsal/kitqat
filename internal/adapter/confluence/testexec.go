package confluence

import (
	"regexp"
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

// headingTags are the storage-format elements that can carry the page's
// "Test Execution :" section heading — usually a bold paragraph, occasionally
// an actual heading element.
var headingTags = []string{"p", "h1", "h2", "h3", "h4", "h5", "h6"}

// testExecHeadingText matches the "Test Execution" heading text after
// normalizeCellLabel has stripped tags and collapsed whitespace, tolerating an
// optional trailing colon so "Test Execution :", "Test Execution:" and
// "TEST EXECUTION" all match.
var testExecHeadingText = regexp.MustCompile(`^test execution\s*:?$`)

// testExecHeading returns the page's "Test Execution :" heading/paragraph
// element, or nil if the page has none.
func testExecHeading(storageHTML string) *element {
	for _, e := range topLevelElements(storageHTML, headingTags...) {
		text := normalizeCellLabel(storageHTML[e.innerStart:e.innerEnd])
		if testExecHeadingText.MatchString(text) {
			el := e
			return &el
		}
	}
	return nil
}

// testExecHeadingHTML is the "Test Execution :" section heading created when a
// page has none yet, matching the bold-paragraph style of the reference page.
const testExecHeadingHTML = `<p><strong>Test Execution :</strong></p>`

// InsertTestExecutionTable inserts a Test Execution filter macro right after the
// page's "Test Execution :" heading — the reference page layout has a screenshot
// placeholder and the TC table below that heading, and the macro belongs directly
// under the heading text, ahead of both. When the page has no such heading yet,
// one is created (testExecHeadingHTML) directly above the macro, placed just
// before the first vertical TC table so the new section still leads straight
// into the test cases, or appended at the end of the page when it has no TC
// table either. Idempotent: if a Jira macro already carries the same jqlQuery,
// the page is returned unchanged. Returns the new HTML and whether it changed.
func InsertTestExecutionTable(storageHTML, serverName, serverID, jql string) (string, bool) {
	// Already present? Match on the escaped jqlQuery value inside a jira macro.
	if strings.Contains(storageHTML, "<ac:parameter ac:name=\"jqlQuery\">"+html.EscapeString(jql)+"</ac:parameter>") {
		return storageHTML, false
	}

	macro := BuildTestExecutionMacro(serverName, serverID, jql)

	if h := testExecHeading(storageHTML); h != nil {
		return storageHTML[:h.end] + macro + storageHTML[h.end:], true
	}

	// No heading found — create one right above the macro. Place the new
	// section just before the first TC table, so it still leads straight
	// into the test cases below it.
	block := testExecHeadingHTML + macro
	for _, t := range topLevelElements(storageHTML, "table") {
		if tableHasTCRow(storageHTML[t.start:t.end]) {
			return storageHTML[:t.start] + block + storageHTML[t.start:], true
		}
	}

	// No TC table on the page either — append the whole new section at the end.
	return storageHTML + block, true
}
