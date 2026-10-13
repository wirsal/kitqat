package confluence

import (
	"fmt"
	"regexp"
	"strings"
)

// ── TC renumbering ────────────────────────────────────────────────────────────
//
// QA sometimes deletes TC sections from a page between Fill Scenario (UC0) and
// Download TC to Excel (UC1) — e.g. TC01..TC50 with TC03 and TC05 removed leaves
// TC01, 02, 04, 06 … 50. Test IDs are built from these numbers, so the page has to
// read TC01..TC48 again before it is exported and uploaded to CBAgnito.

// reTCToken matches the "TCnn" token inside a "No. Test Case" value cell.
var reTCToken = regexp.MustCompile(`(?i)TC\s*(\d+)`)

// TCRenumber records one TC table whose number was changed.
type TCRenumber struct {
	From string // two-digit number before, e.g. "04"
	To   string // two-digit number after, e.g. "03"
}

// RenumberTCTables renumbers the vertical TC tables TC01..TCnn in the order they
// appear on the page. Only the TC token in each "No. Test Case" cell is rewritten;
// everything else stays byte-identical. linked reports whether any TC cell already
// holds a Jira macro (the page has been through CBAgnito), so the caller can refuse
// to renumber tickets whose Test IDs are already fixed.
func RenumberTCTables(storageHTML string) (newHTML string, changes []TCRenumber, linked bool) {
	var sb strings.Builder
	last := 0
	n := 0
	for _, tbl := range topLevelElements(storageHTML, "table") {
		table := storageHTML[tbl.start:tbl.end]
		from := tableTCNumber(table)
		if from == "" {
			continue // not a vertical TC table
		}
		n++
		to := fmt.Sprintf("%02d", n)

		updated, hasJira := setTableTCNumber(table, to)
		if hasJira {
			linked = true
		}
		if from == to || updated == table {
			continue
		}
		changes = append(changes, TCRenumber{From: from, To: to})
		sb.WriteString(storageHTML[last:tbl.start])
		sb.WriteString(updated)
		last = tbl.end
	}
	sb.WriteString(storageHTML[last:])
	return sb.String(), changes, linked
}

// CountTCTables returns the number of vertical TC tables on the page.
func CountTCTables(storageHTML string) int {
	n := 0
	for _, tbl := range topLevelElements(storageHTML, "table") {
		if tableTCNumber(storageHTML[tbl.start:tbl.end]) != "" {
			n++
		}
	}
	return n
}

// setTableTCNumber rewrites the TC token in a table's "No. Test Case" cell to tcNum.
// hasJira reports whether that cell holds a Jira macro.
func setTableTCNumber(table, tcNum string) (updated string, hasJira bool) {
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}
		if !verticalTCLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])] {
			continue
		}
		vs, ve := cells[1].innerStart, cells[1].innerEnd
		inner := row[vs:ve]
		hasJira = strings.Contains(inner, `ac:name="jira"`)

		newInner := replaceTextToken(inner, "TC"+tcNum)
		if newInner == inner {
			return table, hasJira
		}
		return table[:r.start] + row[:vs] + newInner + row[ve:] + table[r.end:], hasJira
	}
	return table, false
}

// replaceTextToken replaces the first TC token found in text content — never inside
// a tag or its attributes — with repl.
func replaceTextToken(inner, repl string) string {
	pos := 0
	for pos < len(inner) {
		lt := strings.IndexByte(inner[pos:], '<')
		end := len(inner)
		if lt >= 0 {
			end = pos + lt
		}
		if loc := reTCToken.FindStringIndex(inner[pos:end]); loc != nil {
			return inner[:pos+loc[0]] + repl + inner[pos+loc[1]:]
		}
		if lt < 0 {
			break
		}
		gt := strings.IndexByte(inner[end:], '>')
		if gt < 0 {
			break
		}
		pos = end + gt + 1
	}
	return inner
}
