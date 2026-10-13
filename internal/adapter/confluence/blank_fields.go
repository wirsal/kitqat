package confluence

import (
	"html"
	"strings"
)

// dashFieldNames are the scenario fields that must not be left blank before a page
// is exported: an empty cell becomes "-". Expected Result is deliberately excluded.
var dashFieldNames = map[string]bool{
	"scenario":   true,
	"function":   true,
	"input_data": true,
	"steps":      true,
}

// FillBlankFieldsWithDash writes "-" into every blank Scenario, Function, Input Data
// and Steps value cell of the page's vertical TC tables. A cell counts as blank only
// when it has no text at all (a "-" placeholder is already filled) and no macro or
// attachment. tcs lists the TC numbers (two-digit) of the tables that were touched.
func FillBlankFieldsWithDash(storageHTML string) (newHTML string, filled int, tcs []string) {
	var sb strings.Builder
	last := 0
	for _, tbl := range topLevelElements(storageHTML, "table") {
		table := storageHTML[tbl.start:tbl.end]
		tcNum := tableTCNumber(table)
		if tcNum == "" {
			continue // not a vertical TC table
		}

		updated, f := dashBlankRows(table)
		if f == 0 {
			continue
		}
		filled += f
		tcs = append(tcs, tcNum)
		sb.WriteString(storageHTML[last:tbl.start])
		sb.WriteString(updated)
		last = tbl.end
	}
	sb.WriteString(storageHTML[last:])
	return sb.String(), filled, tcs
}

// dashBlankRows fills the blank dash-field rows of one TC table.
func dashBlankRows(table string) (updated string, filled int) {
	var sb strings.Builder
	last := 0
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}
		field := scenarioFieldLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])]
		if !dashFieldNames[field] {
			continue
		}
		if !isBlankCell(row[cells[1].innerStart:cells[1].innerEnd]) {
			continue
		}
		sb.WriteString(table[last:r.start])
		sb.WriteString(row[:cells[1].innerStart])
		sb.WriteString("<p>-</p>")
		sb.WriteString(row[cells[1].innerEnd:])
		last = r.end
		filled++
	}
	sb.WriteString(table[last:])
	return sb.String(), filled
}

// isBlankCell reports whether a value cell has no text and no macro/attachment.
// Unlike isEmptyCell it treats placeholders such as "-" as content, so filling is
// idempotent.
func isBlankCell(inner string) bool {
	if strings.Contains(inner, "<ac:") || strings.Contains(inner, "<ri:") {
		return false
	}
	return len(strings.Fields(html.UnescapeString(reTagStrip.ReplaceAllString(inner, " ")))) == 0
}
