package confluence

import (
	"fmt"
	"strings"
	"testing"
)

// tcTable renders a minimal TC table; value is the "No. Test Case" cell content.
func tcTable(value string) string {
	return `<table><tbody>` +
		`<tr><td><p><strong>No. Test Case</strong></p></td><td>` + value + `</td></tr>` +
		`<tr><td><p><strong>Scenario</strong></p></td><td><p>TC99 mentioned in text</p></td></tr>` +
		`</tbody></table>`
}

func TestRenumberTCTables_ClosesGapsFromDeletedSections(t *testing.T) {
	// TC01..TC50 with TC03 and TC05 deleted.
	var sb strings.Builder
	sb.WriteString(`<p>Intro</p>`)
	for i := 1; i <= 50; i++ {
		if i == 3 || i == 5 {
			continue
		}
		sb.WriteString(tcTable(fmt.Sprintf("<p>TC%02d -</p>", i)))
	}
	sb.WriteString(`<table><tbody><tr><td><p>Reviewer</p></td><td><p>TC77</p></td></tr></tbody></table>`)
	page := sb.String()

	got, changes, linked := RenumberTCTables(page)
	if linked {
		t.Fatal("linked = true, want false")
	}
	if len(changes) != 46 { // TC04 → TC03, TC06..TC50 → TC04..TC48
		t.Fatalf("changes = %d, want 46", len(changes))
	}
	if changes[0] != (TCRenumber{From: "04", To: "03"}) {
		t.Errorf("first change = %+v, want 04→03", changes[0])
	}

	var nums []string
	for _, tbl := range topLevelElements(got, "table") {
		if n := tableTCNumber(got[tbl.start:tbl.end]); n != "" {
			nums = append(nums, n)
		}
	}
	if len(nums) != 48 || nums[0] != "01" || nums[47] != "48" {
		t.Fatalf("numbers after renumber = %v", nums)
	}
	for i, n := range nums {
		if n != fmt.Sprintf("%02d", i+1) {
			t.Fatalf("table %d numbered %s", i+1, n)
		}
	}
	if strings.Count(got, "TC99 mentioned in text") != 48 || !strings.Contains(got, "<p>TC77</p>") {
		t.Error("text outside the No. Test Case cell was modified")
	}

	// Idempotent: a second run changes nothing.
	again, changes2, _ := RenumberTCTables(got)
	if again != got || len(changes2) != 0 {
		t.Errorf("second run changed %d tables", len(changes2))
	}
}

func TestRenumberTCTables_AlreadySequentialIsUntouched(t *testing.T) {
	page := tcTable("<p>TC01 -</p>") + tcTable("<p>TC02 -</p>")
	got, changes, _ := RenumberTCTables(page)
	if got != page || len(changes) != 0 {
		t.Errorf("sequential page modified: %d changes", len(changes))
	}
}

func TestRenumberTCTables_FollowsPageOrder(t *testing.T) {
	page := tcTable("<p>TC02 -</p>") + tcTable("<p>TC01 -</p>")
	got, _, _ := RenumberTCTables(page)
	tables := topLevelElements(got, "table")
	if tableTCNumber(got[tables[0].start:tables[0].end]) != "01" ||
		tableTCNumber(got[tables[1].start:tables[1].end]) != "02" {
		t.Errorf("tables not renumbered in page order:\n%s", got)
	}
}

func TestRenumberTCTables_ReportsJiraLinks(t *testing.T) {
	jira := buildJiraMacro("BRSF-100", "System JIRA", "")
	page := tcTable("<p>TC01 - </p>"+jira) + tcTable("<p>TC03 - </p>"+buildJiraMacro("BRSF-103", "System JIRA", ""))
	_, changes, linked := RenumberTCTables(page)
	if !linked || len(changes) != 1 {
		t.Errorf("linked = %v, changes = %d; want true, 1", linked, len(changes))
	}
}

func TestReplaceTextToken_SkipsTagsAndAttributes(t *testing.T) {
	in := `<span data-x="TC09"><strong>TC 04</strong> - </span>`
	want := `<span data-x="TC09"><strong>TC03</strong> - </span>`
	if got := replaceTextToken(in, "TC03"); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}
