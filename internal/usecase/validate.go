package usecase

import (
	"fmt"

	"github.com/wirsal/kitqat/internal/adapter/confluence"
)

// ValidateUseCase tidies a Scenario Detail page between Fill Scenario (UC0) and
// Download TC to Excel (UC1), in a single page write:
//   - TC tables are renumbered TC01..TCnn in page order, closing gaps left by
//     deleted TC sections;
//   - blank Scenario, Function, Input Data and Steps cells are set to "-".
type ValidateUseCase struct {
	confluenceUpdater ConfluenceUpdater
}

func NewValidateUseCase(cu ConfluenceUpdater) *ValidateUseCase {
	return &ValidateUseCase{confluenceUpdater: cu}
}

// ValidateResult summarises a validate run.
type ValidateResult struct {
	PageTitle string
	TCCount   int                     // vertical TC tables on the page
	Renumber  []confluence.TCRenumber // TC tables whose number changed (or would have, when RenumberBlocked)
	// RenumberBlocked reports that numbering had gaps but the page already carries
	// Jira links, so numbers were left as they are — those Test IDs are already fixed.
	RenumberBlocked bool
	DashFilled      int      // blank cells set to "-"
	DashFilledTCs   []string // TC numbers (after renumbering) whose cells were set to "-"
	Changed         bool     // whether the page was actually updated
}

// Execute validates and fixes the page, then writes it back when anything changed.
func (uc *ValidateUseCase) Execute(pageURLOrID string) (*ValidateResult, error) {
	pageID, err := uc.confluenceUpdater.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve confluence page: %w", err)
	}
	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	result := &ValidateResult{PageTitle: title}

	// 1. Renumber TC01..TCnn in page order, unless the page already has Jira links.
	newBody := body
	renumbered, changes, linked := confluence.RenumberTCTables(body)
	result.Renumber = changes
	if len(changes) > 0 {
		if linked {
			result.RenumberBlocked = true
		} else {
			newBody = renumbered
		}
	}
	result.TCCount = confluence.CountTCTables(newBody)
	if result.TCCount == 0 {
		return nil, fmt.Errorf("no TC tables found on page %q", title)
	}

	// 2. Set blank scenario fields to "-".
	newBody, result.DashFilled, result.DashFilledTCs = confluence.FillBlankFieldsWithDash(newBody)

	// 3. Skip the PUT when nothing changed, so no empty page version is created.
	if newBody == body {
		return result, nil
	}

	// 4. Refuse to write markup we have unbalanced (only when the original was well-formed).
	if confluence.ValidateStorageBalance(body) == nil {
		if err := confluence.ValidateStorageBalance(newBody); err != nil {
			return nil, fmt.Errorf("generated page markup is invalid (%w); page was left unchanged", err)
		}
	}
	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}
	result.Changed = true
	return result, nil
}
