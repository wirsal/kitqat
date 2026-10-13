package confluence

import (
	"strings"
	"testing"
)

func TestFillBlankFieldsWithDash(t *testing.T) {
	page := `<table><tbody>` +
		`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC01 -</p></td></tr>` +
		`<tr><td><p><strong>Function</strong></p></td><td><p>Sudah terisi</p></td></tr>` +
		`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
		`<tr><td><p><strong>Input Data</strong></p></td><td><p>&nbsp;</p></td></tr>` +
		`<tr><td><p><strong>Steps</strong></p></td><td><br/></td></tr>` +
		`<tr><td><p><strong>Expected Result</strong></p></td><td><p></p></td></tr>` +
		`<tr><td><p><strong>Screen Capture</strong></p></td><td><p></p></td></tr>` +
		`</tbody></table>` +
		`<table><tbody>` +
		`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC02 -</p></td></tr>` +
		`<tr><td><p><strong>Function</strong></p></td><td><p>-</p></td></tr>` +
		`<tr><td><p><strong>Scenario</strong></p></td><td><ac:structured-macro ac:name="info"/></td></tr>` +
		`</tbody></table>` +
		`<table><tbody><tr><td><p>Scenario</p></td><td><p></p></td></tr></tbody></table>`

	got, filled, tcs := FillBlankFieldsWithDash(page)
	if filled != 3 {
		t.Errorf("filled = %d, want 3 (Scenario, Input Data, Steps of TC01)", filled)
	}
	if len(tcs) != 1 || tcs[0] != "01" {
		t.Errorf("tcs = %v, want [01]", tcs)
	}
	for _, want := range []string{
		`<strong>Function</strong></p></td><td><p>Sudah terisi</p>`,
		`<strong>Scenario</strong></p></td><td><p>-</p>`,
		`<strong>Input Data</strong></p></td><td><p>-</p>`,
		`<strong>Steps</strong></p></td><td><p>-</p>`,
		`<strong>Expected Result</strong></p></td><td><p></p>`, // not a dash field
		`<strong>Screen Capture</strong></p></td><td><p></p>`,
		`<ac:structured-macro ac:name="info"/>`,                                           // macro counts as content
		`<table><tbody><tr><td><p>Scenario</p></td><td><p></p></td></tr></tbody></table>`, // not a TC table
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s", want)
		}
	}
	if err := ValidateStorageBalance(got); err != nil {
		t.Errorf("unbalanced: %v", err)
	}

	// Idempotent.
	again, filled2, _ := FillBlankFieldsWithDash(got)
	if again != got || filled2 != 0 {
		t.Errorf("second run filled %d cells", filled2)
	}
}
