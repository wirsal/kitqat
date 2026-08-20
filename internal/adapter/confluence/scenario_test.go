package confluence

import (
	"strings"
	"testing"

	"github.com/wirsal/kitqa/internal/domain"
)

// samplePage mirrors the real Scenario Detail layout: TC01 has a filled Function,
// TC02 has a row with an empty label cell (the "Result" row in the source page),
// plus an unrelated table that must not be touched.
const samplePage = `<p>Intro</p>` +
	`<table><tbody>` +
	`<tr><td><p><strong>No.&nbsp;Test Case</strong></p></td><td><p>TC01 -</p></td></tr>` +
	`<tr><td><p><strong>Link Defect</strong></p></td><td><p>-</p></td></tr>` +
	`<tr><td><p><strong>Function</strong></p></td><td><p>Mapping Program Subrogation Thirdparty</p></td></tr>` +
	`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Input Data</strong></p></td><td><p>-</p></td></tr>` +
	`<tr><td><p><strong>Steps</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Expected Result</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Result</strong></p></td><td><p>PASS | FAILED</p></td></tr>` +
	`<tr><td><p><strong>Screen Capture</strong></p></td><td><ac:structured-macro ac:name="expand"><ac:rich-text-body><ac:image><ri:attachment ri:filename="tc01.png"/></ac:image></ac:rich-text-body></ac:structured-macro></td></tr>` +
	`</tbody></table>` +
	`<table><tbody>` +
	`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC02 -</p></td></tr>` +
	`<tr><td><p><strong>Function</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p></p></td><td><p>PASS | FAILED</p></td></tr>` +
	`</tbody></table>` +
	`<table><tbody><tr><td><p>Reviewer</p></td><td><p>Someone</p></td></tr></tbody></table>`

func sampleRows() map[string]domain.ScenarioRow {
	return map[string]domain.ScenarioRow{
		"01": {
			TCNumber:       "01",
			Scenario:       "User membuka halaman mapping",
			Function:       "Function dari sheet",
			InputData:      "NIK: 123",
			Steps:          "Buka menu\nKlik simpan",
			ExpectedResult: "Data tersimpan",
		},
		"02": {
			TCNumber: "02",
			Scenario: "Skenario kedua",
			Function: "Mapping Program",
		},
	}
}

func TestInjectScenarioFieldsFillsEmptyCells(t *testing.T) {
	out, filled, skipped, matched := InjectScenarioFields(samplePage, sampleRows())

	// TC01: Scenario, Steps, Expected Result, Input Data ("-" is a placeholder) = 4.
	// TC02: Function, Scenario = 2.
	if filled != 6 {
		t.Errorf("filled = %d, want 6", filled)
	}
	// TC01 Function already had real content.
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if !matched["01"] || !matched["02"] {
		t.Errorf("matched = %v, want both 01 and 02", matched)
	}

	for _, want := range []string{
		"<p>User membuka halaman mapping</p>",
		"<p>NIK: 123</p>",
		"<p>Buka menu</p><p>Klik simpan</p>", // multi-line becomes one <p> per line
		"<p>Data tersimpan</p>",
		"<p>Skenario kedua</p>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}

	// The already-filled Function cell of TC01 must survive untouched.
	if !strings.Contains(out, "<p>Mapping Program Subrogation Thirdparty</p>") {
		t.Error("existing Function value was overwritten")
	}
	if strings.Contains(out, "<p>Function dari sheet</p>") {
		t.Error("sheet value overwrote a non-empty cell")
	}

	// The row with a blank label and the unrelated table stay as they were.
	if !strings.Contains(out, `<tr><td><p></p></td><td><p>PASS | FAILED</p></td></tr>`) {
		t.Error("row with empty label cell was modified")
	}
	if !strings.Contains(out, `<tr><td><p>Reviewer</p></td><td><p>Someone</p></td></tr>`) {
		t.Error("unrelated table was modified")
	}
}

func TestInjectScenarioFieldsIsIdempotent(t *testing.T) {
	first, _, _, _ := InjectScenarioFields(samplePage, sampleRows())
	second, filled, _, _ := InjectScenarioFields(first, sampleRows())

	if filled != 0 {
		t.Errorf("second run filled = %d, want 0", filled)
	}
	if second != first {
		t.Error("second run changed the page")
	}
}

// A "-" in the spreadsheet means "nothing here". Writing it into a cell that already
// shows "-" would report a fill on every single run, so it must be left alone.
func TestInjectScenarioFieldsIgnoresPlaceholderValues(t *testing.T) {
	rows := map[string]domain.ScenarioRow{
		"01": {TCNumber: "01", InputData: "-", Steps: "  n/a  "},
	}
	out, filled, skipped, _ := InjectScenarioFields(samplePage, rows)

	if filled != 0 || skipped != 0 {
		t.Errorf("filled = %d, skipped = %d; want 0 and 0", filled, skipped)
	}
	if out != samplePage {
		t.Error("placeholder values from the spreadsheet changed the page")
	}
}

func TestInjectScenarioFieldsEscapesSheetContent(t *testing.T) {
	rows := map[string]domain.ScenarioRow{
		"02": {TCNumber: "02", Scenario: `<script>alert("x")</script> a & b`},
	}
	out, _, _, _ := InjectScenarioFields(samplePage, rows)

	if strings.Contains(out, "<script>") {
		t.Error("raw markup from the spreadsheet leaked into the page")
	}
	if !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "a &amp; b") {
		t.Errorf("value not escaped as expected:\n%s", out)
	}
}

func TestInjectScenarioFieldsSkipsUnknownTC(t *testing.T) {
	rows := map[string]domain.ScenarioRow{"09": {TCNumber: "09", Scenario: "x"}}
	out, filled, _, matched := InjectScenarioFields(samplePage, rows)

	if filled != 0 || len(matched) != 0 {
		t.Errorf("filled = %d, matched = %v, want 0 and empty", filled, matched)
	}
	if out != samplePage {
		t.Error("page changed even though no TC matched")
	}
}

func TestExtractTCTableTemplateReturnsLastTCTable(t *testing.T) {
	tmpl := ExtractTCTableTemplate(samplePage)
	if tableTCNumber(tmpl) != "02" {
		t.Errorf("template TC number = %q, want %q", tableTCNumber(tmpl), "02")
	}
	if strings.Contains(tmpl, "Reviewer") {
		t.Error("template picked up the non-TC table")
	}
	if ExtractTCTableTemplate("<p>no tables here</p>") != "" {
		t.Error("expected empty template for a page with no TC table")
	}
}

func TestBuildTCTableClonesAndClears(t *testing.T) {
	tmpl := ExtractTCTableTemplate(samplePage[:strings.Index(samplePage, `<table><tbody><tr><td><p><strong>No. Test Case`)])
	row := domain.ScenarioRow{TCNumber: "05", Scenario: "Skenario baru", Steps: "Langkah 1"}
	out := BuildTCTable(tmpl, "05", row)

	if tableTCNumber(out) != "05" {
		t.Errorf("new table TC number = %q, want %q", tableTCNumber(out), "05")
	}
	if !strings.Contains(out, "<p>Skenario baru</p>") || !strings.Contains(out, "<p>Langkah 1</p>") {
		t.Errorf("new table missing scenario values:\n%s", out)
	}
	// TC01's own content must not be cloned into the new table.
	if strings.Contains(out, "Mapping Program Subrogation Thirdparty") {
		t.Error("cloned the template's Function value")
	}
	if strings.Contains(out, "tc01.png") {
		t.Error("cloned the template's screenshot attachment")
	}
	// Static template text is kept.
	if !strings.Contains(out, "PASS | FAILED") {
		t.Error("dropped the Result row template text")
	}
}

func TestBuildTCTableFallsBackToDefaultTemplate(t *testing.T) {
	row := domain.ScenarioRow{TCNumber: "01", Scenario: "S", Function: "F", ExpectedResult: "E"}
	out := BuildTCTable("", "01", row)

	if tableTCNumber(out) != "01" {
		t.Errorf("TC number = %q, want %q", tableTCNumber(out), "01")
	}
	for _, want := range []string{"<p>S</p>", "<p>F</p>", "<p>E</p>"} {
		if !strings.Contains(out, want) {
			t.Errorf("default template output missing %q:\n%s", want, out)
		}
	}
}

// nestedPage reproduces the markup that made Confluence reject a real page with
// "Unexpected EOF; was expecting a close tag for element <ac:structured-macro>":
// the Screen Capture cell holds an expand macro wrapping another table, so any
// non-greedy `<table>.*?</table>` match cuts the TC table in half.
const nestedPage = `<table><tbody>` +
	`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC01 -</p></td></tr>` +
	`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Steps</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Screen Capture</strong></p></td><td>` +
	`<ac:structured-macro ac:name="expand"><ac:parameter ac:name="title">Click here to expand...</ac:parameter>` +
	`<ac:rich-text-body><table><tbody><tr><td><p>before</p></td><td>` +
	`<ac:image><ri:attachment ri:filename="tc01.png"/></ac:image></td></tr></tbody></table>` +
	`</ac:rich-text-body></ac:structured-macro></td></tr>` +
	`</tbody></table>`

func TestNestedTablesStayBalanced(t *testing.T) {
	if err := ValidateStorageBalance(nestedPage); err != nil {
		t.Fatalf("fixture itself is unbalanced: %v", err)
	}

	rows := map[string]domain.ScenarioRow{
		"01": {TCNumber: "01", Scenario: "Skenario satu", Steps: "Langkah satu"},
	}
	out, filled, _, matched := InjectScenarioFields(nestedPage, rows)

	if err := ValidateStorageBalance(out); err != nil {
		t.Fatalf("injection produced unbalanced markup: %v\n%s", err, out)
	}
	if filled != 2 || !matched["01"] {
		t.Errorf("filled = %d, matched = %v; want 2 and 01", filled, matched)
	}
	// The nested table and its cell must survive untouched — the inner <td><p>before</p>
	// must not be mistaken for a label/value pair of the outer table.
	if !strings.Contains(out, "<p>before</p>") {
		t.Error("nested table content was lost")
	}
	if strings.Count(out, "</ac:structured-macro>") != 1 {
		t.Error("macro structure was altered")
	}
}

func TestExtractTCTableTemplateKeepsNestedTable(t *testing.T) {
	tmpl := ExtractTCTableTemplate(nestedPage)

	if err := ValidateStorageBalance(tmpl); err != nil {
		t.Fatalf("template is unbalanced: %v\n%s", err, tmpl)
	}
	if tmpl != nestedPage {
		t.Errorf("template should be the whole TC table, got:\n%s", tmpl)
	}
}

func TestBuildTCTableFromNestedTemplateIsBalanced(t *testing.T) {
	tmpl := ExtractTCTableTemplate(nestedPage)
	out := BuildTCTable(tmpl, "06", domain.ScenarioRow{TCNumber: "06", Scenario: "Skenario enam"})

	if err := ValidateStorageBalance(out); err != nil {
		t.Fatalf("generated table is unbalanced: %v\n%s", err, out)
	}
	if tableTCNumber(out) != "06" {
		t.Errorf("TC number = %q, want 06", tableTCNumber(out))
	}
	if !strings.Contains(out, "<p>Skenario enam</p>") {
		t.Errorf("scenario not written:\n%s", out)
	}
	// The cloned expand macro keeps its shell but loses the previous TC's screenshot
	// and nested content.
	if strings.Contains(out, "tc01.png") || strings.Contains(out, "<p>before</p>") {
		t.Errorf("cloned the template's screen capture content:\n%s", out)
	}
	if !strings.Contains(out, `ac:name="expand"`) {
		t.Error("expand macro shell was dropped")
	}
}

func TestValidateStorageBalance(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"balanced", nestedPage, false},
		{"truncated macro", `<table><tbody><tr><td><ac:structured-macro><ac:rich-text-body></tbody></table>`, true},
		{"stray close", `</td>`, true},
		{"mismatched", `<table><tr></table></tr>`, true},
		{"cdata with angle brackets", `<ac:parameter><![CDATA[a < b > c]]></ac:parameter>`, false},
		{"comment", `<table><!-- </table> --><tbody></tbody></table>`, false},
		{"self-closing image", `<td><ac:image ri:filename="a.png"/></td>`, false},
	}
	for _, c := range cases {
		err := ValidateStorageBalance(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr = %v", c.name, err, c.wantErr)
		}
	}
}

func TestIsEmptyCell(t *testing.T) {
	cases := []struct {
		inner string
		want  bool
	}{
		{"<p></p>", true},
		{"<p>-</p>", true},
		{"<p>&nbsp;</p>", true},
		{"<p>N/A</p>", true},
		{"<p>real content</p>", false},
		{`<ac:structured-macro ac:name="jira"/>`, false},
	}
	for _, c := range cases {
		if got := isEmptyCell(c.inner); got != c.want {
			t.Errorf("isEmptyCell(%q) = %v, want %v", c.inner, got, c.want)
		}
	}
}
