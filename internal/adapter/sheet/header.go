package sheet

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wirsal/kitqa/internal/domain"
)

// scenarioAliases maps a normalized header cell to a canonical field name.
// Follows the same shape as columnAliases in the confluence parser.
var scenarioAliases = map[string]string{
	// TC number
	"no. test case": "tc",
	"no test case":  "tc",
	"no. tc":        "tc",
	"test case":     "tc",
	"test id":       "tc",
	"testid":        "tc",
	"tc":            "tc",
	"no":            "tc",
	"#":             "tc",

	// Scenario
	"scenario": "scenario",
	"skenario": "scenario",
	"summary":  "scenario",

	// Function
	"function":    "function",
	"fungsi":      "function",
	"description": "function",

	// Input data
	"input data": "input_data",
	"test data":  "input_data",
	"data":       "input_data",

	// Steps
	"steps":             "steps",
	"step":              "steps",
	"langkah":           "steps",
	"manual test steps": "steps",
	"test steps":        "steps",

	// Expected result
	"expected result":       "expected_result",
	"expected":              "expected_result",
	"hasil yang diharapkan": "expected_result",
}

// headerScanLimit is how many leading rows are searched for the header row.
// The exporter writes its own header on rows 1-2 with data from row 3, so a
// small window is enough while still tolerating title/banner rows above it.
const headerScanLimit = 10

// columnMap maps a canonical field name to its column index.
type columnMap map[string]int

// detectHeader scans the leading rows for the first one that looks like a header,
// returning its index and the column mapping. A row qualifies when it matches at
// least two distinct canonical fields — a single "no" column is too weak a signal.
func detectHeader(rows [][]string) (headerIdx int, cm columnMap, err error) {
	limit := headerScanLimit
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		candidate := mapColumns(rows[i])
		if len(candidate) >= 2 {
			return i, candidate, nil
		}
	}
	return 0, nil, fmt.Errorf("no header row found in the first %d rows: expected columns such as %q and %q", limit, "No. Test Case", "Scenario")
}

func mapColumns(row []string) columnMap {
	cm := make(columnMap)
	for i, cell := range row {
		key := normalizeHeader(cell)
		canonical, ok := scenarioAliases[key]
		if !ok {
			continue
		}
		// First column wins, so a later "Expected Result" sub-header under a merged
		// "Manual Test Steps" group doesn't displace the primary one.
		if _, taken := cm[canonical]; !taken {
			cm[canonical] = i
		}
	}
	return cm
}

// normalizeHeader lowercases a header cell and collapses all whitespace
// (including the NBSP that spreadsheets often carry).
func normalizeHeader(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func (cm columnMap) at(row []string, key string) string {
	idx, ok := cm[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// buildRows converts data rows into ScenarioRows using the detected column map.
// Rows with no usable content are dropped. When the TC column is missing or blank,
// the TC number falls back to the position among the kept rows.
func buildRows(dataRows [][]string, cm columnMap) []domain.ScenarioRow {
	var out []domain.ScenarioRow
	for _, row := range dataRows {
		sr := domain.ScenarioRow{
			Scenario:       cm.at(row, "scenario"),
			Function:       cm.at(row, "function"),
			InputData:      cm.at(row, "input_data"),
			Steps:          cm.at(row, "steps"),
			ExpectedResult: cm.at(row, "expected_result"),
		}
		if sr.IsEmpty() {
			continue
		}
		sr.TCNumber = normalizeTCNumber(cm.at(row, "tc"), len(out)+1)
		out = append(out, sr)
	}
	return out
}

// normalizeTCNumber turns a cell like "TC01 -", "BRSF02_38_01" or "3" into "01".
// Falls back to the row position when the cell holds no number.
func normalizeTCNumber(cell string, position int) string {
	if n := extractNumber(cell); n != "" {
		return n
	}
	return fmt.Sprintf("%02d", position)
}

var (
	// reTCPrefixed matches an explicit TC marker, e.g. "TC01 -" or "tc 7".
	reTCPrefixed = regexp.MustCompile(`(?i)TC\s*(\d+)`)
	// reTrailingNumber matches the last number in the cell, covering both a bare
	// "3" and a full test ID like "BRSF02_38_01".
	reTrailingNumber = regexp.MustCompile(`(\d+)\s*$`)
)

// extractNumber pulls a two-digit TC number out of a spreadsheet cell.
// Returns "" when the cell holds no digits.
func extractNumber(cell string) string {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return ""
	}
	m := reTCPrefixed.FindStringSubmatch(cell)
	if m == nil {
		m = reTrailingNumber.FindStringSubmatch(cell)
	}
	if m == nil {
		return ""
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n == 0 {
		return ""
	}
	return fmt.Sprintf("%02d", n)
}
