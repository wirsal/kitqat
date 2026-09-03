package confluence

import (
	"strings"
	"testing"
)

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

// A page without a "Test Execution :" heading gets one created, directly
// above the macro, positioned just before the first TC table.
func TestInsertTestExecutionTable_CreatesHeadingWhenMissing(t *testing.T) {
	body := `<p>Some unrelated intro text.</p>` + testExecTCTable

	got, changed := InsertTestExecutionTable(body, "System JIRA", "", `issue in testExecutionTests("BRSF-16163")`)
	if !changed {
		t.Fatal("changed = false, want true")
	}

	introEnd := len(`<p>Some unrelated intro text.</p>`)
	if got[:introEnd] != body[:introEnd] {
		t.Fatalf("text before the new section was altered:\ngot:  %q\nwant: %q", got[:introEnd], body[:introEnd])
	}

	wantHeading := `<p>Some unrelated intro text.</p>` + testExecHeadingHTML
	if len(got) < len(wantHeading) || got[:len(wantHeading)] != wantHeading {
		t.Fatalf("heading was not created right before the macro:\ngot:  %q\nwant prefix: %q", got[:min(len(got), len(wantHeading)+40)], wantHeading)
	}
	afterHeading := got[len(wantHeading):]
	if want := `<ac:structured-macro ac:name="jira"`; len(afterHeading) < len(want) || afterHeading[:len(want)] != want {
		t.Errorf("macro was not inserted immediately after the new heading; got: %q", afterHeading[:min(80, len(afterHeading))])
	}
	if !strings.HasSuffix(got, testExecTCTable) {
		t.Errorf("TC table should immediately follow the new section; got tail: %q", got[max(0, len(got)-80):])
	}
}

// When the page has neither a heading nor a TC table, the whole new section
// (heading + macro) is appended at the end.
func TestInsertTestExecutionTable_CreatesHeadingAtEndWhenNoTCTable(t *testing.T) {
	body := `<p>A page with no test cases yet.</p>`

	got, changed := InsertTestExecutionTable(body, "System JIRA", "", `issue in testExecutionTests("BRSF-16163")`)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	want := body + testExecHeadingHTML
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("heading was not appended after the existing body:\ngot:  %q\nwant prefix: %q", got[:min(len(got), len(want)+40)], want)
	}
}

// EnsureTestExecutionHeading (used by UC0, which has no Jira macro to attach)
// creates the same bordered-section heading, placed just before the first TC
// table, when the page has none yet.
func TestEnsureTestExecutionHeading_CreatesWhenMissing(t *testing.T) {
	body := `<p>Some unrelated intro text.</p>` + testExecTCTable

	got, changed := EnsureTestExecutionHeading(body)
	if !changed {
		t.Fatal("changed = false, want true")
	}

	want := `<p>Some unrelated intro text.</p>` + testExecHeadingHTML + testExecTCTable
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "<ac:layout-section") {
		t.Error("created heading is not wrapped in a layout section")
	}
}

// A page that already has a "Test Execution :" heading (in any style) is left
// untouched — EnsureTestExecutionHeading never duplicates it.
func TestEnsureTestExecutionHeading_NoOpWhenPresent(t *testing.T) {
	body := `<p><strong>Test Execution :</strong></p>` + testExecTCTable

	got, changed := EnsureTestExecutionHeading(body)
	if changed {
		t.Error("changed = true, want false (heading already present)")
	}
	if got != body {
		t.Error("body was modified despite heading already being present")
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
