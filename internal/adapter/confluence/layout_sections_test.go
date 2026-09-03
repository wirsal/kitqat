package confluence

import (
	"strings"
	"testing"
)

const oneTCTable = `<table><tbody>` +
	`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC01 -</p></td></tr>` +
	`</tbody></table>`

func TestEnsureScenarioDetailHeading_CreatesWhenMissing(t *testing.T) {
	got, changed := EnsureScenarioDetailHeading(oneTCTable)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if !strings.HasPrefix(got, "<ac:layout>") {
		t.Errorf("heading was not prepended at the top of the page: %q", got[:min(60, len(got))])
	}
	if !strings.Contains(got, "TEST SCENARIO DETAIL &amp; SCREEN CAPTURE UAT") {
		t.Error("banner text missing or not HTML-escaped")
	}
	if !strings.HasSuffix(got, oneTCTable) {
		t.Error("existing page content was altered")
	}
}

func TestEnsureScenarioDetailHeading_NoOpWhenPresent(t *testing.T) {
	body := `<p><strong>Test Scenario Detail &amp; Screen Capture UAT</strong></p>` + oneTCTable

	got, changed := EnsureScenarioDetailHeading(body)
	if changed {
		t.Error("changed = true, want false (banner already present, case-insensitive)")
	}
	if got != body {
		t.Error("body was modified despite banner already being present")
	}
}

func TestWrapTCTablesInSections_WrapsUnwrappedTables(t *testing.T) {
	body := oneTCTable + oneTCTable // two plain TC tables

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 2 {
		t.Fatalf("wrapped = %d, want 2", wrapped)
	}
	if n := strings.Count(got, "<ac:layout-cell>"+oneTCTable+"</ac:layout-cell>"); n != 2 {
		t.Errorf("expected both tables wrapped in their own layout-cell, got %d matches:\n%s", n, got)
	}
}

func TestWrapTCTablesInSections_SkipsAlreadyWrapped(t *testing.T) {
	body := wrapInLayoutSection(oneTCTable) + oneTCTable // one already wrapped, one plain

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 1 {
		t.Fatalf("wrapped = %d, want 1 (the already-wrapped table must be skipped)", wrapped)
	}
	if strings.Count(got, "<ac:layout>") != 2 {
		t.Errorf("want exactly 2 layout blocks (1 pre-existing + 1 new), got:\n%s", got)
	}
}

func TestWrapTCTablesInSections_NoOpWhenNothingToWrap(t *testing.T) {
	body := wrapInLayoutSection(oneTCTable)

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 0 || got != body {
		t.Errorf("wrapped = %d, got changed = %v, want 0 and unchanged", wrapped, got != body)
	}
}
