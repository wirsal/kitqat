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
	wantTable := `<ac:layout-cell>` + normalizeTCTable(oneTCTable) + `</ac:layout-cell>`
	if n := strings.Count(got, wantTable); n != 2 {
		t.Errorf("expected both tables wrapped and normalized, got %d matches:\n%s", n, got)
	}
}

func TestWrapTCTablesInSections_SkipsAlreadyWrapped(t *testing.T) {
	// The already-wrapped table is also already normalized, so it's a true
	// no-op for that one; the plain table gets wrapped (and normalized) for
	// the first time.
	body := wrapInLayoutSection(normalizeTCTable(oneTCTable)) + oneTCTable

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 1 {
		t.Fatalf("wrapped = %d, want 1 (the already-wrapped table must be skipped)", wrapped)
	}
	if strings.Count(got, "<ac:layout>") != 2 {
		t.Errorf("want exactly 2 layout blocks (1 pre-existing + 1 new), got:\n%s", got)
	}
	if merged := MergeLayouts(got); strings.Count(merged, "<ac:layout>") != 1 || strings.Count(merged, "<ac:layout-section") != 2 {
		t.Errorf("MergeLayouts should join them into 1 layout with 2 sections, got:\n%s", merged)
	}
}

// A table sharing its layout cell with other content (the reference page puts
// a <br /> in front of each table) must not get a second <ac:layout> nested
// inside that cell.
func TestWrapTCTablesInSections_DoesNotNestInsideLayoutCell(t *testing.T) {
	body := `<ac:layout>` + layoutSection(`<br />`+oneTCTable) + `</ac:layout>`

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 0 {
		t.Errorf("wrapped = %d, want 0", wrapped)
	}
	if strings.Count(got, "<ac:layout>") != 1 {
		t.Errorf("a layout was nested inside the existing cell:\n%s", got)
	}
}

// Several sibling <ac:layout> blocks — what UC0 used to write — survive only
// until the next editor save, which drops every section. MergeLayouts joins
// them into the single layout Confluence supports, keeping section order.
func TestMergeLayouts_JoinsSiblingLayouts(t *testing.T) {
	body := wrapInLayoutSection(`<p>A</p>`) + wrapInLayoutSection(`<p>B</p>`) + wrapInLayoutSection(oneTCTable)

	got := MergeLayouts(body)
	want := `<ac:layout>` + layoutSection(`<p>A</p>`) + layoutSection(`<p>B</p>`) + layoutSection(oneTCTable) + `</ac:layout>`
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
	if MergeLayouts(got) != got {
		t.Error("MergeLayouts is not idempotent")
	}
}

// Content outside the layout gets a section per top-level block; blank blocks
// don't get empty sections of their own.
func TestMergeLayouts_WrapsLooseContent(t *testing.T) {
	body := `<p><strong>Banner</strong></p><p><strong>Test Execution :</strong></p>` +
		wrapInLayoutSection(oneTCTable) + `<p />` + oneTCTable

	got := MergeLayouts(body)
	want := `<ac:layout>` +
		layoutSection(`<p><strong>Banner</strong></p>`) +
		layoutSection(`<p><strong>Test Execution :</strong></p>`) +
		layoutSection(oneTCTable) +
		layoutSection(`<p />`+oneTCTable) +
		`</ac:layout>`
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
}

func TestMergeLayouts_NoOp(t *testing.T) {
	for _, body := range []string{
		oneTCTable, // no layout at all
		`<ac:layout>` + layoutSection(`<p>A</p>`) + layoutSection(oneTCTable) + `</ac:layout>`,
	} {
		if got := MergeLayouts(body); got != body {
			t.Errorf("body changed:\ngot:  %q\nwant: %q", got, body)
		}
	}
}

// On a page already laid out in one layout, creating the "Test Execution :"
// heading must not nest a layout inside the first TC table's cell; after
// MergeLayouts the page is still a single layout with the heading as its own
// section, right before the TC table's.
func TestEnsureTestExecutionHeading_InsideExistingLayout(t *testing.T) {
	body := `<ac:layout>` + layoutSection(`<p>Banner</p>`) + layoutSection(`<br />`+oneTCTable) + `</ac:layout>`

	got, changed := EnsureTestExecutionHeading(body)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	got = MergeLayouts(got)
	want := `<ac:layout>` + layoutSection(`<p>Banner</p>`) +
		layoutSection(`<p><strong>Test Execution :</strong></p>`) +
		layoutSection(`<br />`+oneTCTable) + `</ac:layout>`
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
	if err := ValidateStorageBalance(got); err != nil {
		t.Error(err)
	}
}

func TestWrapTCTablesInSections_NormalizesAlreadyWrappedTable(t *testing.T) {
	// A table that's already in its own section but was never normalized (e.g.
	// wrapped by an older KitQAt build) gets fixed up in place — no new
	// <ac:layout> block, just the style attribute and colgroup added.
	body := wrapInLayoutSection(oneTCTable)

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 0 {
		t.Errorf("wrapped = %d, want 0 (not newly wrapped, just normalized)", wrapped)
	}
	want := wrapInLayoutSection(normalizeTCTable(oneTCTable))
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
}

func TestWrapTCTablesInSections_NoOpWhenNothingToWrap(t *testing.T) {
	body := wrapInLayoutSection(normalizeTCTable(oneTCTable))

	got, wrapped := WrapTCTablesInSections(body)
	if wrapped != 0 || got != body {
		t.Errorf("wrapped = %d, got changed = %v, want 0 and unchanged", wrapped, got != body)
	}
}

func TestNormalizeTCTable(t *testing.T) {
	got := normalizeTCTable(oneTCTable)
	if !strings.HasPrefix(got, `<table style="width: 100.0%;"><colgroup>`) {
		t.Errorf("width style and/or colgroup were not added: %q", got[:min(80, len(got))])
	}

	// A table with its own style, or its own colgroup, is left untouched on
	// that front — each fixup is independent.
	styled := `<table style="width: 50.0%;"><tbody></tbody></table>`
	if got := normalizeTCTable(styled); !strings.HasPrefix(got, `<table style="width: 50.0%;"><colgroup>`) {
		t.Errorf("existing style was overwritten, or colgroup wasn't still added: got %q", got)
	}

	withColgroup := `<table><colgroup><col style="width: 30.0%;" /></colgroup><tbody></tbody></table>`
	if got := normalizeTCTable(withColgroup); !strings.HasPrefix(got, `<table style="width: 100.0%;"><colgroup><col style="width: 30.0%;" />`) {
		t.Errorf("existing colgroup was overwritten, or width wasn't still added: got %q", got)
	}
}
