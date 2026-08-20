package usecase

import "github.com/wirsal/kitqa/internal/domain"

type ConfluenceRepository interface {
	// ResolvePageID resolves a short URL (e.g. /x/DKFPZQ) or full URL to a numeric page ID.
	ResolvePageID(urlOrID string) (string, error)
	// FetchTestCases fetches test cases from the given Confluence page ID.
	FetchTestCases(pageID string) ([]domain.TestCase, error)
}

type ExcelExporter interface {
	Export(testCases []domain.TestCase, outputPath string) error
}

// ScenarioSource supplies scenario rows from a spreadsheet. Today the only
// implementation reads a local .xlsx/.csv file; a Google Sheets source can be added
// behind this interface without touching the use case.
type ScenarioSource interface {
	Load() ([]domain.ScenarioRow, error)
}

// ConfluenceUpdater provides write access to a Confluence page.
type ConfluenceUpdater interface {
	ResolvePageID(urlOrID string) (string, error)
	GetPageRaw(pageID string) (title string, version int, body string, err error)
	UpdatePageContent(pageID, title string, currentVersion int, newBody string) error
}
