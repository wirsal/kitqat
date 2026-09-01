package confluence

import (
	"fmt"
	"strings"

	"github.com/wirsal/kitqat/internal/domain"
	"golang.org/x/net/html"
)

// ── Scenario field injection ──────────────────────────────────────────────────
//
// InjectJiraLinks works at <tr> scope because the "No. Test Case" row carries its
// own identity. A row like `Scenario | (empty)` does not, so filling scenario fields
// has to work at <table> scope: find the TC number in the table, then fill the other
// rows of that same table.
//
// Element boundaries come from topLevelElements (htmlscan.go), never from a
// non-greedy regex — TC tables nest other tables inside expand macros, and a regex
// would cut them in half.

// scenarioFieldLabels maps a normalized row label to a canonical field name.
var scenarioFieldLabels = map[string]string{
	"scenario":              "scenario",
	"skenario":              "scenario",
	"function":              "function",
	"fungsi":                "function",
	"input data":            "input_data",
	"test data":             "input_data",
	"steps":                 "steps",
	"step":                  "steps",
	"langkah":               "steps",
	"expected result":       "expected_result",
	"expected":              "expected_result",
	"hasil yang diharapkan": "expected_result",
}

// placeholderValues are cell contents treated as "still empty" — a lone dash is how
// the template marks an unfilled field, so it may be overwritten.
var placeholderValues = map[string]bool{
	"":    true,
	"-":   true,
	"–":   true,
	"—":   true,
	".":   true,
	"n/a": true,
	"na":  true,
}

// InjectScenarioFields fills the Scenario, Function, Input Data, Steps and Expected
// Result rows of every vertical TC table that has a matching row in rows (keyed by
// two-digit TC number).
//
// Cells that already hold content are never overwritten; they are counted in skipped.
// matchedTCs reports which TC numbers from rows were found on the page, so the caller
// can create tables for the rest.
func InjectScenarioFields(storageHTML string, rows map[string]domain.ScenarioRow) (newHTML string, filled, skipped int, matchedTCs map[string]bool) {
	matchedTCs = make(map[string]bool, len(rows))

	var sb strings.Builder
	last := 0
	for _, tbl := range topLevelElements(storageHTML, "table") {
		table := storageHTML[tbl.start:tbl.end]

		tcNum := tableTCNumber(table)
		if tcNum == "" {
			continue // not a vertical TC table
		}
		row, ok := rows[tcNum]
		if !ok {
			continue // no spreadsheet row for this TC
		}
		matchedTCs[tcNum] = true

		updated, f, s := fillTableFields(table, row)
		filled += f
		skipped += s
		if updated == table {
			continue
		}
		sb.WriteString(storageHTML[last:tbl.start])
		sb.WriteString(updated)
		last = tbl.end
	}
	sb.WriteString(storageHTML[last:])

	return sb.String(), filled, skipped, matchedTCs
}

// rowCells returns the top-level cells of a <tr>, ignoring cells of nested tables.
func rowCells(row string) []element {
	return topLevelElements(row, "td", "th")
}

// tableTCNumber returns the two-digit TC number declared by a table's
// "No. Test Case" row, or "" when the table has none.
func tableTCNumber(table string) string {
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}
		if !verticalTCLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])] {
			continue
		}
		return extractTCNumber(reTagStrip.ReplaceAllString(row[cells[1].innerStart:cells[1].innerEnd], " "))
	}
	return ""
}

// fillTableFields writes scenario values into the empty value cells of one TC table.
func fillTableFields(table string, data domain.ScenarioRow) (updated string, filled, skipped int) {
	var sb strings.Builder
	last := 0
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]

		newRow, f, s := fillRow(row, data)
		filled += f
		skipped += s
		if newRow == row {
			continue
		}
		sb.WriteString(table[last:r.start])
		sb.WriteString(newRow)
		last = r.end
	}
	sb.WriteString(table[last:])
	return sb.String(), filled, skipped
}

// fillRow fills one label/value row when the spreadsheet has something to add.
func fillRow(row string, data domain.ScenarioRow) (updated string, filled, skipped int) {
	cells := rowCells(row)
	if len(cells) < 2 {
		return row, 0, 0
	}

	// Rows whose label cell is blank (the sample page has one) simply don't match.
	field, ok := scenarioFieldLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])]
	if !ok {
		return row, 0, 0
	}

	// A blank value, or a placeholder such as "-", means the spreadsheet has nothing
	// to say about this field — leave whatever the template holds.
	value := scenarioValue(data, field)
	if value == "" || isPlaceholderText(value) {
		return row, 0, 0
	}

	inner := row[cells[1].innerStart:cells[1].innerEnd]
	if !isEmptyCell(inner) {
		return row, 0, 1 // already filled — never overwrite
	}

	newInner := storageParagraphs(value)
	if newInner == inner {
		return row, 0, 0 // nothing would change; don't report it as filled
	}
	return row[:cells[1].innerStart] + newInner + row[cells[1].innerEnd:], 1, 0
}

func scenarioValue(d domain.ScenarioRow, field string) string {
	switch field {
	case "scenario":
		return d.Scenario
	case "function":
		return d.Function
	case "input_data":
		return d.InputData
	case "steps":
		return d.Steps
	case "expected_result":
		return d.ExpectedResult
	}
	return ""
}

// isEmptyCell reports whether a value cell holds no real content. Macros (images,
// expand blocks, Jira links) count as content even though they carry no text.
func isEmptyCell(inner string) bool {
	if strings.Contains(inner, "<ac:") || strings.Contains(inner, "<ri:") {
		return false
	}
	return isPlaceholderText(html.UnescapeString(reTagStrip.ReplaceAllString(inner, " ")))
}

// isPlaceholderText reports whether text carries no real information.
func isPlaceholderText(s string) bool {
	return placeholderValues[strings.ToLower(strings.Join(strings.Fields(s), " "))]
}

// storageParagraphs renders a plain-text value as Confluence storage-format
// paragraphs — one per line, escaped so spreadsheet content can never inject markup.
func storageParagraphs(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		sb.WriteString("<p>")
		sb.WriteString(html.EscapeString(line))
		sb.WriteString("</p>")
	}
	if sb.Len() == 0 {
		return "<p></p>"
	}
	return sb.String()
}

// ── New TC table creation ─────────────────────────────────────────────────────

// defaultTCTable is the fallback template used when the page has no TC table to clone.
const defaultTCTable = `<table><tbody>` +
	`<tr><td><p><strong>No. Test Case</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Link Defect</strong></p></td><td><p>-</p></td></tr>` +
	`<tr><td><p><strong>Function</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Input Data</strong></p></td><td><p>-</p></td></tr>` +
	`<tr><td><p><strong>Steps</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Expected Result</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Result</strong></p></td><td><p>PASS | <span style="text-decoration: line-through;">FAILED</span></p></td></tr>` +
	`<tr><td><p><strong>Screen Capture</strong></p></td><td><p></p></td></tr>` +
	`</tbody></table>`

// ExtractTCTableTemplate returns the last vertical TC table on the page, to be reused
// as the template for new test cases. Cloning a real table preserves the page's own
// column widths, styling and macro structure. Returns "" when the page has none.
func ExtractTCTableTemplate(storageHTML string) string {
	var last string
	for _, tbl := range topLevelElements(storageHTML, "table") {
		table := storageHTML[tbl.start:tbl.end]
		if tableTCNumber(table) != "" {
			last = table
		}
	}
	return last
}

// BuildTCTable renders a new TC table from a template (typically the page's last TC
// table, from ExtractTCTableTemplate) filled with the given scenario row. An empty or
// structurally broken template falls back to the built-in one.
func BuildTCTable(template, tcNum string, row domain.ScenarioRow) string {
	if strings.TrimSpace(template) == "" || ValidateStorageBalance(template) != nil {
		template = defaultTCTable
	}
	blank := blankTCTable(template, tcNum)
	filled, _, _ := fillTableFields(blank, row)
	return filled
}

// blankTCTable clears a cloned table so it can be refilled: the TC row gets the new
// number, scenario rows are emptied, and any carried-over screenshots or macro bodies
// are stripped. Static template text (Link Defect "-", "PASS | FAILED") is preserved.
func blankTCTable(table, tcNum string) string {
	var sb strings.Builder
	last := 0
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}

		label := normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])
		inner := row[cells[1].innerStart:cells[1].innerEnd]

		var newInner string
		switch {
		case verticalTCLabels[label]:
			newInner = fmt.Sprintf("<p>TC%s -</p>", tcNum)
		case scenarioFieldLabels[label] != "":
			newInner = "<p></p>"
		default:
			newInner = sanitizeTemplateCell(inner)
		}
		if newInner == inner {
			continue
		}

		sb.WriteString(table[last:r.start])
		sb.WriteString(row[:cells[1].innerStart])
		sb.WriteString(newInner)
		sb.WriteString(row[cells[1].innerEnd:])
		last = r.end
	}
	sb.WriteString(table[last:])
	return sb.String()
}

// sanitizeTemplateCell removes content that must not be carried over from the cloned
// test case — attachment images and the contents of expand blocks — while keeping the
// surrounding macro structure intact and balanced.
func sanitizeTemplateCell(inner string) string {
	inner = removeElements(inner, "ac:image")
	return replaceInnerElements(inner, "ac:rich-text-body", "<p></p>")
}

// removeElements deletes every top-level element with the given name.
func removeElements(s, name string) string {
	var sb strings.Builder
	last := 0
	for _, e := range topLevelElements(s, name) {
		sb.WriteString(s[last:e.start])
		last = e.end
	}
	sb.WriteString(s[last:])
	return sb.String()
}

// replaceInnerElements swaps the content of every top-level element with the given
// name, leaving the element's own tags (and anything nested deeper) balanced.
func replaceInnerElements(s, name, newInner string) string {
	var sb strings.Builder
	last := 0
	for _, e := range topLevelElements(s, name) {
		if e.innerStart == e.innerEnd {
			continue // self-closing
		}
		sb.WriteString(s[last:e.innerStart])
		sb.WriteString(newInner)
		last = e.innerEnd
	}
	sb.WriteString(s[last:])
	return sb.String()
}
