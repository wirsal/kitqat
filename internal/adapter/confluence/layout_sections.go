package confluence

import (
	"strings"

	"golang.org/x/net/html"
)

// ── Page-level layout sections ────────────────────────────────────────────────
//
// The reference page layout wraps every top-level block — the title banner, the
// "Test Execution :" heading, and each TC table — in its own Confluence layout
// section (a single-column ac:layout-section, the bordered box the editor's
// "Section" tool creates). UC0 (Fill Scenario & Function) reproduces that
// structure: the title banner is created once if missing, and every TC table
// that isn't already the sole content of a layout cell gets wrapped in one of
// its own. (The "Test Execution :" heading has its own equivalent,
// EnsureTestExecutionHeading, in testexec.go.)

// scenarioDetailHeadingText is the fixed title banner shown at the top of a
// Scenario Detail page, above the "Test Execution :" heading.
const scenarioDetailHeadingText = "TEST SCENARIO DETAIL & SCREEN CAPTURE UAT"

// scenarioDetailHeadingHTML is the title banner created when a page has none
// yet, wrapped in its own layout section like the reference page.
var scenarioDetailHeadingHTML = wrapInLayoutSection(`<p><strong>` + html.EscapeString(scenarioDetailHeadingText) + `</strong></p>`)

// EnsureScenarioDetailHeading makes sure the page has the "TEST SCENARIO DETAIL &
// SCREEN CAPTURE UAT" title banner, creating one at the very top of the page when
// missing. Returns the new HTML and whether it changed.
func EnsureScenarioDetailHeading(storageHTML string) (string, bool) {
	want := strings.ToLower(scenarioDetailHeadingText)
	for _, e := range topLevelElements(storageHTML, headingTags...) {
		if normalizeCellLabel(storageHTML[e.innerStart:e.innerEnd]) == want {
			return storageHTML, false
		}
	}
	return scenarioDetailHeadingHTML + storageHTML, true
}

// wrapInLayoutSection wraps inner in a single-column Confluence layout section —
// the bordered box the editor's "Section" tool creates.
func wrapInLayoutSection(inner string) string {
	return `<ac:layout><ac:layout-section ac:type="single"><ac:layout-cell>` +
		inner + `</ac:layout-cell></ac:layout-section></ac:layout>`
}

// WrapTCTablesInSections wraps every top-level vertical TC table that isn't
// already the sole content of a layout cell in its own single-column layout
// section, matching the reference page. A table already wrapped (from a
// previous run, or built into the page manually) is left untouched. Returns the
// new HTML and how many tables were newly wrapped.
func WrapTCTablesInSections(storageHTML string) (string, int) {
	var sb strings.Builder
	last := 0
	wrapped := 0
	for _, t := range topLevelElements(storageHTML, "table") {
		table := storageHTML[t.start:t.end]
		if !tableHasTCRow(table) || tableAlreadyInOwnCell(storageHTML, t.start, t.end) {
			continue
		}
		sb.WriteString(storageHTML[last:t.start])
		sb.WriteString(wrapInLayoutSection(table))
		last = t.end
		wrapped++
	}
	sb.WriteString(storageHTML[last:])
	if wrapped == 0 {
		return storageHTML, 0
	}
	return sb.String(), wrapped
}

// tableAlreadyInOwnCell reports whether the table spanning [start,end) in s is
// already the sole content of an <ac:layout-cell>...</ac:layout-cell> — i.e. it
// doesn't need wrapping again.
func tableAlreadyInOwnCell(s string, start, end int) bool {
	before := strings.TrimRight(s[:start], " \t\n\r")
	after := strings.TrimLeft(s[end:], " \t\n\r")
	return strings.HasSuffix(before, "<ac:layout-cell>") && strings.HasPrefix(after, "</ac:layout-cell>")
}
