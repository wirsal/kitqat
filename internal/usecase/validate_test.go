package usecase

import (
	"strings"
	"testing"
)

func validateTable(tc, scenario string) string {
	return `<table><tbody>` +
		`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>` + tc + ` -</p></td></tr>` +
		`<tr><td><p><strong>Scenario</strong></p></td><td><p>` + scenario + `</p></td></tr>` +
		`<tr><td><p><strong>Function</strong></p></td><td><p></p></td></tr>` +
		`</tbody></table>`
}

func TestValidateRenumbersAndFillsDashesInOneWrite(t *testing.T) {
	// TC03 was deleted; every table has a blank Function, TC04 a blank Scenario.
	page := &fakeUpdater{
		title:   "Scenario Detail BRSF02-39",
		version: 7,
		body: validateTable("TC01", "Satu") + validateTable("TC02", "Dua") +
			validateTable("TC04", ""),
	}

	res, err := NewValidateUseCase(page).Execute("https://confluence.bri.co.id/x/AAA")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if page.writes != 1 || !res.Changed {
		t.Fatalf("writes = %d, changed = %v; want 1, true", page.writes, res.Changed)
	}
	if res.TCCount != 3 || len(res.Renumber) != 1 || res.Renumber[0].From != "04" || res.Renumber[0].To != "03" {
		t.Errorf("renumber = %+v (count %d), want one 04→03 of 3", res.Renumber, res.TCCount)
	}
	if res.DashFilled != 4 {
		t.Errorf("DashFilled = %d, want 4", res.DashFilled)
	}
	if strings.Contains(page.body, "TC04") || !strings.Contains(page.body, "TC03 -") {
		t.Error("page not renumbered to TC03")
	}
	if strings.Contains(page.body, "<td><p></p></td>") {
		t.Error("blank cell left on page")
	}

	// Second run: nothing to do, no new page version.
	res2, err := NewValidateUseCase(page).Execute("x")
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if page.writes != 1 || res2.Changed {
		t.Errorf("second run wrote the page (writes = %d)", page.writes)
	}
}

func TestValidateErrorsWithoutTCTables(t *testing.T) {
	page := &fakeUpdater{title: "Empty", body: `<p>nothing</p>`}
	if _, err := NewValidateUseCase(page).Execute("x"); err == nil {
		t.Error("expected an error for a page without TC tables")
	}
}
