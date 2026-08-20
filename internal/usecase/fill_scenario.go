package usecase

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wirsal/kitqa/internal/adapter/confluence"
	"github.com/wirsal/kitqa/internal/domain"
)

// FillScenarioUseCase writes Scenario, Function, Input Data, Steps and Expected Result
// values from a spreadsheet into the vertical test-case tables of a Confluence
// Scenario Detail page, creating tables for test cases the page doesn't have yet.
type FillScenarioUseCase struct {
	confluenceUpdater ConfluenceUpdater
	source            ScenarioSource
}

func NewFillScenarioUseCase(cu ConfluenceUpdater, src ScenarioSource) *FillScenarioUseCase {
	return &FillScenarioUseCase{confluenceUpdater: cu, source: src}
}

// FillResult summarises the outcome of a fill run.
type FillResult struct {
	PageTitle    string
	FilledCells  int
	SkippedCells int      // cells that already had content and were left untouched
	CreatedTCs   []string // TC numbers for which a new table was appended
	UnmatchedTCs []string // TC tables on the page with no row in the spreadsheet
	Changed      bool     // whether the page was actually updated
}

// Execute loads the spreadsheet, fills the page's TC tables, appends tables for any
// missing test cases, and writes the result back to Confluence.
func (uc *FillScenarioUseCase) Execute(pageURLOrID string) (*FillResult, error) {
	// 1. Load scenario rows from the spreadsheet.
	rows, err := uc.source.Load()
	if err != nil {
		return nil, fmt.Errorf("read spreadsheet: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("spreadsheet has no data rows")
	}
	byTC := make(map[string]domain.ScenarioRow, len(rows))
	for _, r := range rows {
		byTC[r.TCNumber] = r
	}

	// 2. Resolve and fetch the Confluence page.
	pageID, err := uc.confluenceUpdater.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve confluence page: %w", err)
	}
	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	// 3. Fill the TC tables that already exist.
	newBody, filled, skipped, matched := confluence.InjectScenarioFields(body, byTC)

	// 4. Append a table for every spreadsheet row without one, in TC order.
	template := confluence.ExtractTCTableTemplate(body)
	var created []string
	for _, tcNum := range missingTCs(byTC, matched) {
		newBody += confluence.BuildTCTable(template, tcNum, byTC[tcNum])
		created = append(created, "TC"+tcNum)
	}

	result := &FillResult{
		PageTitle:    title,
		FilledCells:  filled,
		SkippedCells: skipped,
		CreatedTCs:   created,
		UnmatchedTCs: unmatchedPageTCs(body, byTC),
	}

	// 5. Skip the PUT when nothing changed, so no empty page version is created.
	if newBody == body {
		return result, nil
	}

	// 6. Refuse to write markup we've broken. Confluence answers a malformed body with
	// a 400 anyway; failing here keeps the page untouched and names the real problem.
	// Only report it when the original page was well-formed to begin with.
	if err := confluence.ValidateStorageBalance(newBody); err != nil {
		if confluence.ValidateStorageBalance(body) == nil {
			return nil, fmt.Errorf("generated page markup is invalid (%w); page was left unchanged", err)
		}
	}

	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}
	result.Changed = true
	return result, nil
}

// missingTCs returns the sorted TC numbers present in the spreadsheet but not on the page.
func missingTCs(byTC map[string]domain.ScenarioRow, matched map[string]bool) []string {
	var missing []string
	for tcNum := range byTC {
		if !matched[tcNum] {
			missing = append(missing, tcNum)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return atoi(missing[i]) < atoi(missing[j]) })
	return missing
}

// unmatchedPageTCs returns TC numbers that have a table on the page but no
// spreadsheet row. Mirrors findUnmatchedTCs in updater.go.
func unmatchedPageTCs(body string, byTC map[string]domain.ScenarioRow) []string {
	var unmatched []string
	seen := make(map[string]bool)
	for _, m := range reTCNum.FindAllStringSubmatch(body, -1) {
		num := fmt.Sprintf("%02d", atoi(m[1]))
		if seen[num] {
			continue
		}
		seen[num] = true
		if _, ok := byTC[num]; !ok {
			unmatched = append(unmatched, "TC"+strings.TrimLeft(num, "0"))
		}
	}
	return unmatched
}
