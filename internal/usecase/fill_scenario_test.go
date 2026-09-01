package usecase

import (
	"strings"
	"testing"

	"github.com/wirsal/kitqat/internal/domain"
)

// fakeUpdater is an in-memory ConfluenceUpdater.
type fakeUpdater struct {
	title   string
	version int
	body    string
	writes  int
}

func (f *fakeUpdater) ResolvePageID(string) (string, error) { return "123", nil }
func (f *fakeUpdater) GetPageRaw(string) (string, int, string, error) {
	return f.title, f.version, f.body, nil
}
func (f *fakeUpdater) UpdatePageContent(_, _ string, _ int, newBody string) error {
	f.body = newBody
	f.writes++
	return nil
}

type fakeSource []domain.ScenarioRow

func (f fakeSource) Load() ([]domain.ScenarioRow, error) { return f, nil }

const pageWithTC01 = `<table><tbody>` +
	`<tr><td><p><strong>No. Test Case</strong></p></td><td><p>TC01 -</p></td></tr>` +
	`<tr><td><p><strong>Function</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Scenario</strong></p></td><td><p></p></td></tr>` +
	`<tr><td><p><strong>Expected Result</strong></p></td><td><p></p></td></tr>` +
	`</tbody></table>`

func TestFillScenarioFillsAndCreates(t *testing.T) {
	page := &fakeUpdater{title: "Scenario Detail BRSF02-39", version: 4, body: pageWithTC01}
	src := fakeSource{
		{TCNumber: "01", Scenario: "Skenario satu", Function: "Fungsi satu"},
		{TCNumber: "02", Scenario: "Skenario dua", ExpectedResult: "Hasil dua"},
	}

	res, err := NewFillScenarioUseCase(page, src).Execute("https://confluence.bri.co.id/x/AAA")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.PageTitle != "Scenario Detail BRSF02-39" {
		t.Errorf("PageTitle = %q", res.PageTitle)
	}
	if res.FilledCells != 2 { // TC01 Scenario + Function
		t.Errorf("FilledCells = %d, want 2", res.FilledCells)
	}
	if len(res.CreatedTCs) != 1 || res.CreatedTCs[0] != "TC02" {
		t.Errorf("CreatedTCs = %v, want [TC02]", res.CreatedTCs)
	}
	if !res.Changed || page.writes != 1 {
		t.Errorf("Changed = %v, writes = %d, want true and 1", res.Changed, page.writes)
	}
	for _, want := range []string{"<p>Skenario satu</p>", "<p>Fungsi satu</p>", "<p>TC02 -</p>", "<p>Hasil dua</p>"} {
		if !strings.Contains(page.body, want) {
			t.Errorf("page missing %q:\n%s", want, page.body)
		}
	}
}

func TestFillScenarioSecondRunWritesNothing(t *testing.T) {
	page := &fakeUpdater{title: "T", version: 1, body: pageWithTC01}
	src := fakeSource{{TCNumber: "01", Scenario: "Skenario satu"}}
	uc := NewFillScenarioUseCase(page, src)

	if _, err := uc.Execute("x"); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	res, err := uc.Execute("x")
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if res.Changed || page.writes != 1 {
		t.Errorf("second run wrote to the page: Changed = %v, writes = %d", res.Changed, page.writes)
	}
	if res.FilledCells != 0 || len(res.CreatedTCs) != 0 {
		t.Errorf("second run reported work: %+v", res)
	}
	if strings.Count(page.body, "<p>TC01 -</p>") != 1 {
		t.Error("TC01 table was duplicated")
	}
}

func TestFillScenarioReportsUnmatchedPageTCs(t *testing.T) {
	page := &fakeUpdater{title: "T", version: 1, body: pageWithTC01}
	src := fakeSource{{TCNumber: "07", Scenario: "Hanya di sheet"}}

	res, err := NewFillScenarioUseCase(page, src).Execute("x")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.UnmatchedTCs) != 1 || res.UnmatchedTCs[0] != "TC1" {
		t.Errorf("UnmatchedTCs = %v, want [TC1]", res.UnmatchedTCs)
	}
	if len(res.CreatedTCs) != 1 || res.CreatedTCs[0] != "TC07" {
		t.Errorf("CreatedTCs = %v, want [TC07]", res.CreatedTCs)
	}
}
