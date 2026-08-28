package confluence

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ── Jira Macro extraction ──────────────────────────────────────────────────────

// JiraMacro holds parameters from an <ac:structured-macro ac:name="jira"> element.
type JiraMacro struct {
	JQL        string
	Keys       []string // explicit issue keys (when no JQL)
	MaxResults int
	ServerURL  string // linked Jira base URL hint, if present
	Server     string // display name, e.g. "System JIRA"
	ServerID   string // UUID used in storage format, e.g. "abc-123-..."
}

var (
	reJiraMacro = regexp.MustCompile(`(?s)ac:name="jira".*?</ac:structured-macro>`)
	reParam     = regexp.MustCompile(`ac:name="([^"]+)"[^>]*>([^<]*)`)
)

// ParseJiraMacros extracts all Jira macro blocks from Confluence storage format.
func ParseJiraMacros(content string) []JiraMacro {
	blocks := reJiraMacro.FindAllString(content, -1)
	var macros []JiraMacro
	for _, block := range blocks {
		macro := parseSingleJiraMacro(block)
		if macro.JQL != "" || len(macro.Keys) > 0 {
			macros = append(macros, macro)
		}
	}
	return macros
}

func parseSingleJiraMacro(block string) JiraMacro {
	var m JiraMacro
	params := reParam.FindAllStringSubmatch(block, -1)
	for _, p := range params {
		if len(p) < 3 {
			continue
		}
		name := strings.TrimSpace(p[1])
		val := strings.TrimSpace(p[2])
		switch name {
		case "jqlQuery":
			m.JQL = unescapeHTML(val)
		case "key":
			if val != "" {
				m.Keys = append(m.Keys, val)
			}
		case "maximumIssues":
			if n := atoi(val); n > 0 {
				m.MaxResults = n
			}
		case "serverUrl":
			m.ServerURL = val
		case "server":
			m.Server = val
		case "serverId":
			m.ServerID = val
		}
	}
	return m
}

// ── HTML table parsing ─────────────────────────────────────────────────────────

// tableRow holds raw cell text for one HTML table row.
type tableRow []string

// parseTablesFromHTML extracts all tables from HTML/storage-format content.
func parseTablesFromHTML(content string) [][]tableRow {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	var tables [][]tableRow
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables = append(tables, extractTable(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

func extractTable(tableNode *html.Node) []tableRow {
	var rows []tableRow
	var walkTable func(*html.Node)
	walkTable = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, extractRow(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTable(c)
		}
	}
	walkTable(tableNode)
	return rows
}

func extractRow(rowNode *html.Node) tableRow {
	var cells []string
	for c := rowNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, extractText(c))
		}
	}
	return cells
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(strings.Join(strings.Fields(sb.String()), " "))
}

// ── Column detection ───────────────────────────────────────────────────────────

var columnAliases = map[string]string{
	"no":                   "no",
	"#":                    "no",
	"project":              "project",
	"test id":              "test_id",
	"testid":               "test_id",
	"id":                   "test_id",
	"summary":              "summary",
	"description":          "description",
	"assignee":             "assignee",
	"assigned to":          "assignee",
	"test repository path": "repository_path",
	"repository path":      "repository_path",
	"test repository":      "repository_path",
	"expected result":      "expected_result",
	"expected":             "expected_result",
}

type columnMap map[string]int

func detectColumns(header tableRow) columnMap {
	cm := make(columnMap)
	for i, cell := range header {
		key := strings.ToLower(strings.TrimSpace(cell))
		if canonical, ok := columnAliases[key]; ok {
			cm[canonical] = i
		}
	}
	_, hasSummary := cm["summary"]
	_, hasTestID := cm["test_id"]
	if !hasSummary && !hasTestID {
		return nil
	}
	return cm
}

type RawTestCase struct {
	No                 string
	Project            string
	TestID             string
	Summary            string
	Description        string
	Assignee           string
	TestRepositoryPath string
	ExpectedResult     string
}

func cellAt(row tableRow, cm columnMap, key string) string {
	idx, ok := cm[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return row[idx]
}

// ParseTestCasesFromHTML extracts test-case rows from the first matching HTML table.
func ParseTestCasesFromHTML(content string) []RawTestCase {
	tables := parseTablesFromHTML(content)
	for _, table := range tables {
		if len(table) < 2 {
			continue
		}
		cm := detectColumns(table[0])
		if cm == nil {
			continue
		}
		var results []RawTestCase
		for _, row := range table[1:] {
			if isSubHeader(row) {
				continue
			}
			tc := RawTestCase{
				No:                 cellAt(row, cm, "no"),
				Project:            cellAt(row, cm, "project"),
				TestID:             cellAt(row, cm, "test_id"),
				Summary:            cellAt(row, cm, "summary"),
				Description:        cellAt(row, cm, "description"),
				Assignee:           cellAt(row, cm, "assignee"),
				TestRepositoryPath: cellAt(row, cm, "repository_path"),
				ExpectedResult:     cellAt(row, cm, "expected_result"),
			}
			if tc.Summary == "" && tc.TestID == "" {
				continue
			}
			results = append(results, tc)
		}
		if len(results) > 0 {
			return results
		}
	}
	return nil
}

func isSubHeader(row tableRow) bool {
	if len(row) == 0 {
		return false
	}
	subHeaders := map[string]bool{"action": true, "data": true, "step": true, "steps": true}
	count := 0
	for _, cell := range row {
		if subHeaders[strings.ToLower(strings.TrimSpace(cell))] {
			count++
		}
	}
	return count >= 2
}

// ── Vertical table parser ─────────────────────────────────────────────────────
// Used for "Scenario Detail & Screen Capture" pages where each test case is a
// two-column table with labels in the first column and values in the second.

// verticalLabels that identify a test-case block.
var verticalTCLabels = map[string]bool{
	"no. test case": true,
	"no test case":  true,
	"no. tc":        true,
}

// ParseVerticalTestCases extracts test cases from two-column vertical tables.
// pageTitle is used to derive the test-ID prefix (e.g. "BRSF02-39" → "BRSF02_39").
func ParseVerticalTestCases(content, pageTitle string) []RawTestCase {
	codePrefix := extractCodeFromTitle(pageTitle)
	tables := parseTablesFromHTML(content)

	var results []RawTestCase
	for _, table := range tables {
		kv := buildKV(table)
		if kv == nil {
			continue
		}
		raw := mapKVToRaw(kv, codePrefix)
		if raw == nil {
			continue
		}
		results = append(results, *raw)
	}
	return results
}

// buildKV turns a two-column table into a label→value map.
// Returns nil if the table doesn't look like a key-value test-case block.
func buildKV(table []tableRow) map[string]string {
	kv := make(map[string]string)
	for _, row := range table {
		if len(row) < 2 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(row[0]))
		value := strings.TrimSpace(row[1])
		kv[label] = value
	}
	// Must contain at least one test-case label.
	for label := range kv {
		if verticalTCLabels[label] {
			return kv
		}
	}
	return nil
}

// mapKVToRaw converts a KV map from a vertical table to RawTestCase.
func mapKVToRaw(kv map[string]string, codePrefix string) *RawTestCase {
	noTC := kv["no. test case"]
	if noTC == "" {
		noTC = kv["no test case"]
	}
	if noTC == "" && kv["scenario"] == "" {
		return nil
	}

	tcNumber := extractTCNumber(noTC)
	testID := ""
	if codePrefix != "" && tcNumber != "" {
		testID = codePrefix + "_" + tcNumber
	} else if tcNumber != "" {
		testID = tcNumber
	}

	expectedResult := kv["expected result"]
	if expectedResult == "" {
		expectedResult = kv["expected"]
	}
	if expectedResult == "" {
		expectedResult = kv["hasil yang diharapkan"]
	}

	return &RawTestCase{
		No:             tcNumber,
		TestID:         testID,
		Summary:        kv["scenario"],
		Description:    kv["function"],
		ExpectedResult: expectedResult,
	}
}

// extractCodeFromTitle finds a pattern like "BRSF02-39" in the page title
// and returns it with "-" replaced by "_" (e.g. "BRSF02_39").
func extractCodeFromTitle(title string) string {
	re := regexp.MustCompile(`\b([A-Z]+\d{2}-\d+)\b`)
	m := re.FindString(title)
	if m == "" {
		return ""
	}
	return strings.ReplaceAll(m, "-", "_")
}

// extractTCNumber parses "TC01 -" or "TC01" → "01" (zero-padded to 2 digits).
func extractTCNumber(s string) string {
	re := regexp.MustCompile(`TC(\d+)`)
	m := re.FindStringSubmatch(strings.ToUpper(s))
	if len(m) < 2 {
		return ""
	}
	n := atoi(m[1])
	return fmt.Sprintf("%02d", n)
}

// ── Jira link injection ───────────────────────────────────────────────────────

// ExtractJiraServerInfo scans the storage HTML for the first Jira macro that carries
// both a server name and a serverId, and returns them for reuse when injecting new macros.
// Falls back to ("System JIRA", "") when none is found.
func ExtractJiraServerInfo(storageHTML string) (serverName, serverID string) {
	for _, m := range ParseJiraMacros(storageHTML) {
		if m.Server != "" {
			return m.Server, m.ServerID
		}
	}
	return "System JIRA", ""
}

// reTagStrip removes HTML/storage tags, leaving text content.
var reTagStrip = regexp.MustCompile(`(?is)<[^>]+>`)

// reExistingJiraKey extracts the key value from an already-injected Jira macro.
var reExistingJiraKey = regexp.MustCompile(`(?i)<ac:parameter\s+ac:name="key">([^<]+)</ac:parameter>`)

// reFullJiraMacro matches a complete ac:structured-macro jira block for replacement.
var reFullJiraMacro = regexp.MustCompile(`(?is)<ac:structured-macro\s+ac:name="jira"[^>]*>.*?</ac:structured-macro>`)

// normalizeCellLabel strips tags and decodes entities (including &nbsp;), then
// collapses whitespace so a label like "<p><strong>No.&nbsp;Test&nbsp;Case</strong></p>"
// normalizes to "no. test case" — matching how the DOM parser reads labels.
func normalizeCellLabel(inner string) string {
	s := reTagStrip.ReplaceAllString(inner, " ")
	s = html.UnescapeString(s)
	// strings.Fields splits on all Unicode whitespace, including NBSP (U+00A0).
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

// ScenarioTicket pairs a CBAgnito ticket's Jira key with its scenario summary.
// The summary — not a positional index — is what matches the ticket to a page's
// TC table, so a job whose tickets don't belong to the page injects nothing.
type ScenarioTicket struct {
	Summary string
	Key     string
}

// scenarioMatchMinLen is the shortest normalized summary considered a real match,
// guarding against a short/generic string pairing with the wrong table.
const scenarioMatchMinLen = 10

// reTCPrefix matches a leading "TC01 -" style prefix, stripped before comparing
// a scenario cell to a ticket summary.
var reTCPrefix = regexp.MustCompile(`(?i)^\s*tc\s*\d+\s*[-–—:.\s]*`)

// reNonAlnum collapses runs of non-alphanumeric characters to a single space.
var reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeMatchText lowercases, strips tags/entities and any leading "TCnn -"
// prefix, then reduces the rest to space-separated alphanumeric tokens so a page
// Scenario cell and a CBAgnito Summary can be compared robustly.
func normalizeMatchText(s string) string {
	s = html.UnescapeString(reTagStrip.ReplaceAllString(s, " "))
	s = strings.ToLower(s)
	s = reTCPrefix.ReplaceAllString(s, " ")
	s = reNonAlnum.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// scenarioTextMatches reports whether a normalized ticket summary corresponds to a
// table's normalized scenario text (either contains the other, both long enough).
func scenarioTextMatches(tableText, ticketNorm string) bool {
	if len(ticketNorm) < scenarioMatchMinLen || len(tableText) < scenarioMatchMinLen {
		return false
	}
	return strings.Contains(tableText, ticketNorm) || strings.Contains(ticketNorm, tableText)
}

// InjectJiraLinksByScenario injects a Jira structured-macro link into the
// "No. Test Case" cell of each vertical TC table, choosing the ticket whose Summary
// matches that table's Scenario/Function text. Tables with no matching ticket are
// left untouched — this is the safety that stops a mismatched job from writing.
//
// Each ticket is used at most once. When the "No. Test Case" cell is blank, it is
// seeded with the table's own TC label (e.g. "TC01 - ") before the link. Idempotent:
// a cell already linked to the same key is skipped.
func InjectJiraLinksByScenario(storageHTML string, tickets []ScenarioTicket, serverName, serverID string) (newHTML string, updated, skipped int, unmatched []string) {
	if serverName == "" {
		serverName = "System JIRA"
	}

	type normTicket struct {
		norm string
		key  string
		used bool
	}
	norm := make([]*normTicket, 0, len(tickets))
	for _, t := range tickets {
		n := normalizeMatchText(t.Summary)
		if t.Key == "" || len(n) < scenarioMatchMinLen {
			continue
		}
		norm = append(norm, &normTicket{norm: n, key: t.Key})
	}

	var sb strings.Builder
	last := 0
	for _, tbl := range topLevelElements(storageHTML, "table") {
		table := storageHTML[tbl.start:tbl.end]

		if !tableHasTCRow(table) {
			continue // not a vertical TC table
		}
		// TC label number for seeding a blank cell — from the No. Test Case cell,
		// else the Scenario cell ("TC01 - ..."). May be "" if neither carries one.
		tcNum := tableTCNumber(table)
		if tcNum == "" {
			tcNum = extractTCNumber(reTagStrip.ReplaceAllString(tableFieldText(table, "scenario"), " "))
		}

		matchText := normalizeMatchText(
			tableFieldText(table, "scenario") + " " + tableFieldText(table, "function"))

		// Pick the unused ticket with the longest matching summary.
		var best *normTicket
		for _, t := range norm {
			if t.used {
				continue
			}
			if scenarioTextMatches(matchText, t.norm) && (best == nil || len(t.norm) > len(best.norm)) {
				best = t
			}
		}
		if best == nil {
			unmatched = append(unmatched, "TC"+strings.TrimLeft(tcNum, "0"))
			continue
		}
		best.used = true

		newTable, u, s := injectKeyIntoTCTable(table, tcNum, best.key, serverName, serverID)
		updated += u
		skipped += s
		if newTable == table {
			continue
		}
		sb.WriteString(storageHTML[last:tbl.start])
		sb.WriteString(newTable)
		last = tbl.end
	}
	sb.WriteString(storageHTML[last:])
	return sb.String(), updated, skipped, unmatched
}

// tableHasTCRow reports whether the table contains a "No. Test Case" label row,
// identifying it as a vertical TC table regardless of whether that cell holds a number.
func tableHasTCRow(table string) bool {
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) >= 2 && verticalTCLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])] {
			return true
		}
	}
	return false
}

// tableFieldText returns the tag-stripped value-cell inner HTML of the first row in
// the table whose label maps to the given canonical field (see scenarioFieldLabels).
func tableFieldText(table, field string) string {
	for _, r := range topLevelElements(table, "tr") {
		row := table[r.start:r.end]
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}
		if scenarioFieldLabels[normalizeCellLabel(row[cells[0].innerStart:cells[0].innerEnd])] == field {
			return row[cells[1].innerStart:cells[1].innerEnd]
		}
	}
	return ""
}

// injectKeyIntoTCTable injects (or replaces) the Jira link in a single TC table's
// "No. Test Case" value cell. Returns the table unchanged with skipped=1 when the
// cell is already linked to the same key.
func injectKeyIntoTCTable(table, tcNum, jiraKey, serverName, serverID string) (newTable string, updated, skipped int) {
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

		// Idempotency: an existing Jira macro in the cell.
		if ek := reExistingJiraKey.FindStringSubmatch(inner); len(ek) > 1 {
			if strings.TrimSpace(ek[1]) == jiraKey {
				return table, 0, 1 // already linked to the same key
			}
			inner = reFullJiraMacro.ReplaceAllString(inner, "")
		}

		// Seed the TC label when the cell is otherwise empty (just a <br/>/placeholder).
		base := strings.TrimRight(inner, " \t\n\r")
		if !strings.Contains(inner, "<ac:") && strings.TrimSpace(reTagStrip.ReplaceAllString(inner, "")) == "" {
			if tcNum != "" {
				base = "<p>TC" + tcNum + " - </p>"
			} else {
				base = ""
			}
		}
		newRow := row[:vs] + base + buildJiraMacro(jiraKey, serverName, serverID) + row[ve:]
		return table[:r.start] + newRow + table[r.end:], 1, 0
	}
	return table, 0, 0
}

func buildJiraMacro(key, serverName, serverID string) string {
	var sb strings.Builder
	sb.WriteString(`<ac:structured-macro ac:name="jira" ac:schema-version="1">`)
	sb.WriteString(`<ac:parameter ac:name="server">`)
	sb.WriteString(serverName)
	sb.WriteString(`</ac:parameter>`)
	if serverID != "" {
		sb.WriteString(`<ac:parameter ac:name="serverId">`)
		sb.WriteString(serverID)
		sb.WriteString(`</ac:parameter>`)
	}
	sb.WriteString(`<ac:parameter ac:name="key">`)
	sb.WriteString(key)
	sb.WriteString(`</ac:parameter>`)
	sb.WriteString(`</ac:structured-macro>`)
	return sb.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func unescapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
