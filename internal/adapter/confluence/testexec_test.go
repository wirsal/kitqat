package confluence

import "testing"

const testExecTCTable = `<table><tbody><tr><th>No. Test Case</th><td>-</td></tr></tbody></table>`

// The macro must land right under the "Test Execution :" heading, ahead of
// whatever sits between the heading and the TC table (e.g. a screenshot
// placeholder image) — not merely "before the first TC table", which used to
// place it after such an image instead of directly under the heading text.
func TestInsertTestExecutionTable_UnderHeading(t *testing.T) {
	body := `<p><strong>Test Execution :</strong></p>` +
		`<p><ac:image><ri:attachment ri:filename="placeholder.png"/></ac:image></p>` +
		testExecTCTable

	got, changed := InsertTestExecutionTable(body, "System JIRA", "", `issue in testExecutionTests("BRSF-16163")`)
	if !changed {
		t.Fatal("changed = false, want true")
	}

	headingEnd := len(`<p><strong>Test Execution :</strong></p>`)
	if got[:headingEnd] != body[:headingEnd] {
		t.Fatalf("heading paragraph was altered:\ngot:  %q\nwant: %q", got[:headingEnd], body[:headingEnd])
	}
	afterHeading := got[headingEnd:]
	if want := `<ac:structured-macro ac:name="jira"`; len(afterHeading) < len(want) || afterHeading[:len(want)] != want {
		t.Errorf("macro was not inserted immediately after the heading; got tail: %q", afterHeading[:min(80, len(afterHeading))])
	}
}

// A page without a "Test Execution :" heading falls back to the old
// before-the-first-TC-table behavior, so older/differently laid out pages
// still get the macro somewhere sensible.
func TestInsertTestExecutionTable_FallsBackToBeforeTCTable(t *testing.T) {
	body := `<p>Some unrelated intro text.</p>` + testExecTCTable

	got, changed := InsertTestExecutionTable(body, "System JIRA", "", `issue in testExecutionTests("BRSF-16163")`)
	if !changed {
		t.Fatal("changed = false, want true")
	}

	tableStart := len(`<p>Some unrelated intro text.</p>`)
	if got[:tableStart] != body[:tableStart] {
		t.Fatalf("text before the TC table was altered:\ngot:  %q\nwant: %q", got[:tableStart], body[:tableStart])
	}
	afterIntro := got[tableStart:]
	if want := `<ac:structured-macro ac:name="jira"`; len(afterIntro) < len(want) || afterIntro[:len(want)] != want {
		t.Errorf("macro was not inserted immediately before the TC table; got: %q", afterIntro[:min(80, len(afterIntro))])
	}
}

// Idempotent: a page that already carries a jira macro with the same jqlQuery
// is returned unchanged, regardless of insertion strategy.
func TestInsertTestExecutionTable_Idempotent(t *testing.T) {
	jql := `issue in testExecutionTests("BRSF-16163")`
	body := `<p><strong>Test Execution :</strong></p>` + BuildTestExecutionMacro("System JIRA", "", jql) + testExecTCTable

	got, changed := InsertTestExecutionTable(body, "System JIRA", "", jql)
	if changed {
		t.Error("changed = true, want false (macro already present)")
	}
	if got != body {
		t.Error("body was modified despite being idempotent")
	}
}
