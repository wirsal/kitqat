package usecase

import (
	"fmt"

	"github.com/wirsal/kitqa/internal/domain"
)

type DownloadUseCase struct {
	confluenceRepo ConfluenceRepository
	excelExporter  ExcelExporter
}

func NewDownloadUseCase(confluenceRepo ConfluenceRepository, excelExporter ExcelExporter) *DownloadUseCase {
	return &DownloadUseCase{
		confluenceRepo: confluenceRepo,
		excelExporter:  excelExporter,
	}
}

func (uc *DownloadUseCase) Execute(pageURLOrID, outputPath string) error {
	pageID, err := uc.confluenceRepo.ResolvePageID(pageURLOrID)
	if err != nil {
		return fmt.Errorf("resolve page id: %w", err)
	}

	testCases, err := uc.confluenceRepo.FetchTestCases(pageID)
	if err != nil {
		return fmt.Errorf("fetch test cases: %w", err)
	}

	if len(testCases) == 0 {
		return fmt.Errorf("no test cases found on page %s", pageID)
	}

	// Re-number sequentially
	for i := range testCases {
		testCases[i].No = i + 1
	}

	if err := uc.excelExporter.Export(testCases, outputPath); err != nil {
		return fmt.Errorf("export excel: %w", err)
	}

	return nil
}

// FetchOnly returns test cases without exporting, useful for preview/debug.
func (uc *DownloadUseCase) FetchOnly(pageURLOrID string) ([]domain.TestCase, error) {
	pageID, err := uc.confluenceRepo.ResolvePageID(pageURLOrID)
	if err != nil {
		return nil, fmt.Errorf("resolve page id: %w", err)
	}
	return uc.confluenceRepo.FetchTestCases(pageID)
}
