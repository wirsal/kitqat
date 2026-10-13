package usecase

import (
	"fmt"

	"github.com/wirsal/kitqat/internal/adapter/confluence"
)

// RenumberUseCase makes a Scenario Detail page's TC tables read TC01..TCnn in page
// order again, after QA deleted TC sections between Fill Scenario (UC0) and Download
// TC to Excel (UC1). It runs as the first step of UC1, so the exported Test IDs and
// the page agree before anything is uploaded to CBAgnito.
type RenumberUseCase struct {
	confluenceUpdater ConfluenceUpdater
}

func NewRenumberUseCase(cu ConfluenceUpdater) *RenumberUseCase {
	return &RenumberUseCase{confluenceUpdater: cu}
}

// RenumberResult summarises a renumber run.
type RenumberResult struct {
	Changes []confluence.TCRenumber // TC tables whose number changed
	// Blocked reports that numbering had gaps but the page already carries Jira
	// links, so it was left unchanged — those tickets' Test IDs are already fixed.
	Blocked bool
}

// Execute renumbers the page's TC tables and writes the page back when anything changed.
func (uc *RenumberUseCase) Execute(pageID string) (*RenumberResult, error) {
	title, version, body, err := uc.confluenceUpdater.GetPageRaw(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch confluence page: %w", err)
	}

	newBody, changes, linked := confluence.RenumberTCTables(body)
	if len(changes) == 0 {
		return &RenumberResult{}, nil
	}
	if linked {
		return &RenumberResult{Changes: changes, Blocked: true}, nil
	}

	if confluence.ValidateStorageBalance(body) == nil {
		if err := confluence.ValidateStorageBalance(newBody); err != nil {
			return nil, fmt.Errorf("renumbering would corrupt page markup (%w); page was left unchanged", err)
		}
	}
	if err := uc.confluenceUpdater.UpdatePageContent(pageID, title, version, newBody); err != nil {
		return nil, fmt.Errorf("update confluence page: %w", err)
	}
	return &RenumberResult{Changes: changes}, nil
}
