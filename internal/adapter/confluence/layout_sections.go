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
	return `<ac:layout>` + layoutSection(inner) + `</ac:layout>`
}

// layoutSection is a single-column layout section holding inner, without the
// enclosing <ac:layout>.
func layoutSection(inner string) string {
	return `<ac:layout-section ac:type="single"><ac:layout-cell>` + inner + `</ac:layout-cell></ac:layout-section>`
}

// WrapTCTablesInSections wraps every top-level vertical TC table that isn't
// already inside a layout section in its own single-column layout section,
// matching the reference page. A table already inside a layout cell (from a
// previous run, or built into the page manually — possibly sharing the cell
// with a leading <br /> or similar) is left in place except for the sizing
// fixups below; wrapping it again would nest an <ac:layout> inside a layout
// cell. Returns the new HTML and how many tables were newly wrapped.
//
// Every TC table this function touches — wrapped just now or already wrapped —
// is also normalized (normalizeTCTable) to a consistent overall width and
// label/value column split, so tables don't end up visibly inconsistent
// depending on how much text each one happens to hold.
//
// Each newly wrapped table gets its own <ac:layout>; run MergeLayouts before
// writing the page so they end up as sections of a single layout.
func WrapTCTablesInSections(storageHTML string) (string, int) {
	layouts := topLevelElements(storageHTML, "ac:layout")
	var sb strings.Builder
	last := 0
	wrapped := 0
	for _, t := range topLevelElements(storageHTML, "table") {
		table := storageHTML[t.start:t.end]
		if !tableHasTCRow(table) {
			continue
		}
		normalized := normalizeTCTable(table)

		if enclosing(layouts, t.start) != nil {
			if normalized == table {
				continue // already in a section and already sized — nothing to do
			}
			sb.WriteString(storageHTML[last:t.start])
			sb.WriteString(normalized)
			last = t.end
			continue
		}

		sb.WriteString(storageHTML[last:t.start])
		sb.WriteString(wrapInLayoutSection(normalized))
		last = t.end
		wrapped++
	}
	sb.WriteString(storageHTML[last:])
	if last == 0 {
		return storageHTML, 0
	}
	return sb.String(), wrapped
}

// enclosing returns the element in els whose span contains pos, or nil.
func enclosing(els []element, pos int) *element {
	for i := range els {
		if els[i].start <= pos && pos < els[i].end {
			return &els[i]
		}
	}
	return nil
}

// insertBlockAt inserts block — markup that carries its own <ac:layout> — at
// pos. When pos falls inside an existing <ac:layout>, a layout can't be nested
// there, so the block goes in front of the layout section holding pos instead,
// with that layout closed before the block and reopened after it. The result
// has sibling layouts, which MergeLayouts joins back into one.
func insertBlockAt(s string, pos int, block string) string {
	l := enclosing(topLevelElements(s, "ac:layout"), pos)
	if l == nil {
		return s[:pos] + block + s[pos:]
	}
	inner := s[l.innerStart:l.innerEnd]
	sec := enclosing(topLevelElements(inner, "ac:layout-section"), pos-l.innerStart)
	if sec == nil {
		return s[:pos] + block + s[pos:] // not inside a section; nothing better to do
	}
	at := l.innerStart + sec.start
	return s[:at] + `</ac:layout>` + block + `<ac:layout>` + s[at:]
}

// MergeLayouts rewrites a page that uses layout sections so it has exactly one
// <ac:layout> holding every section, with no content outside it.
//
// Confluence only supports a single <ac:layout> per page, containing all of the
// page's content. A body with several sibling <ac:layout> blocks (or content
// between them) is accepted by the REST API and even renders — but the next
// time the page goes through the editor, Confluence discards every section and
// flattens the page. That is what made UC0's bordered sections vanish after a
// later UC2 run or manual edit.
//
// Sections from every <ac:layout> are kept in order; loose top-level content
// between them is wrapped in single-column sections of its own. A page with no
// layout at all, or already in the one-layout form, is returned unchanged.
func MergeLayouts(storageHTML string) string {
	blocks := topLevelElements(storageHTML, "ac:layout", "ac:layout-section")
	if len(blocks) == 0 {
		return storageHTML
	}
	if len(blocks) == 1 && isLayoutElement(storageHTML, blocks[0]) &&
		strings.TrimSpace(storageHTML[:blocks[0].start]) == "" &&
		strings.TrimSpace(storageHTML[blocks[0].end:]) == "" {
		return storageHTML
	}

	var sb strings.Builder
	sb.WriteString(`<ac:layout>`)
	last := 0
	for _, b := range blocks {
		writeLooseSections(&sb, storageHTML[last:b.start])
		if isLayoutElement(storageHTML, b) {
			sb.WriteString(storageHTML[b.innerStart:b.innerEnd])
		} else {
			sb.WriteString(storageHTML[b.start:b.end]) // a stray bare section
		}
		last = b.end
	}
	writeLooseSections(&sb, storageHTML[last:])
	sb.WriteString(`</ac:layout>`)
	return sb.String()
}

// isLayoutElement reports whether e is an <ac:layout> (rather than an
// <ac:layout-section>).
func isLayoutElement(s string, e element) bool {
	tag := s[e.start:]
	return strings.HasPrefix(tag, "<ac:layout>") || strings.HasPrefix(tag, "<ac:layout ")
}

// looseBlockTags are the top-level elements MergeLayouts gives a section each
// when they sit outside any layout.
var looseBlockTags = append([]string{"table", "ul", "ol", "div", "ac:structured-macro"}, headingTags...)

// writeLooseSections wraps content found outside any layout in single-column
// sections — one per top-level block, so the title banner and the "Test
// Execution :" heading each keep a section of their own. Blank blocks (an empty
// <p />, a stray <br />) are carried into the next block's section instead of
// getting an empty one.
func writeLooseSections(sb *strings.Builder, loose string) {
	if strings.TrimSpace(loose) == "" {
		return
	}
	pending := ""
	last := 0
	for _, e := range topLevelElements(loose, looseBlockTags...) {
		pending += loose[last:e.end]
		last = e.end
		if !isBlankMarkup(loose[e.start:e.end]) {
			sb.WriteString(layoutSection(pending))
			pending = ""
		}
	}
	pending += loose[last:]
	if !isBlankMarkup(pending) {
		sb.WriteString(layoutSection(pending))
	}
}

// isBlankMarkup reports whether s carries no visible content.
func isBlankMarkup(s string) bool {
	if strings.Contains(s, "<ac:") || strings.Contains(s, "<ri:") || strings.Contains(s, "<table") || strings.Contains(s, "<img") {
		return false
	}
	return strings.TrimSpace(html.UnescapeString(reTagStrip.ReplaceAllString(s, ""))) == ""
}

// tcTableColgroup gives every TC table the same label/value column split, so
// "No. Test Case", "Function", "Scenario" etc. line up at the same width on
// every table instead of each one auto-sizing its label column to whatever its
// own text happens to need.
const tcTableColgroup = `<colgroup><col style="width: 20.0%;" /><col style="width: 80.0%;" /></colgroup>`

// normalizeTCTable makes a TC table consistent with every other one on the
// page: an explicit style="width: 100.0%;" (so it always fills its section
// instead of auto-sizing to its own text content) and a shared colgroup (so
// the label column doesn't vary in width table to table). Either fixup is
// skipped where the table already carries the corresponding thing of its own
// (a style attribute, or a colgroup) — set some other way and left alone.
func normalizeTCTable(table string) string {
	end := strings.IndexByte(table, '>')
	if end < 0 {
		return table
	}
	tag := table[:end]
	rest := table[end+1:]

	if !strings.Contains(tag, "style=") {
		tag += ` style="width: 100.0%;"`
	}
	if !strings.Contains(rest, "<colgroup>") {
		rest = tcTableColgroup + rest
	}
	return tag + ">" + rest
}
