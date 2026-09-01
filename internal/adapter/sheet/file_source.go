package sheet

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wirsal/kitqat/internal/domain"
	"github.com/xuri/excelize/v2"
)

// preferredSheetName is the worksheet the excel exporter writes, so a file produced
// by "Download TC to Excel" can be fed straight back in.
const preferredSheetName = "Create Test"

// FileSource loads scenario rows from a local spreadsheet file (.xlsx/.xlsm/.csv).
type FileSource struct {
	path string
}

// NewFileSource creates a source for the given path. Surrounding double quotes are
// stripped — a copy-as-path artefact on Windows.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: strings.TrimSpace(strings.ReplaceAll(path, `"`, ""))}
}

// Path returns the cleaned file path.
func (s *FileSource) Path() string { return s.path }

// Load reads the spreadsheet and returns its scenario rows.
func (s *FileSource) Load() ([]domain.ScenarioRow, error) {
	if s.path == "" {
		return nil, fmt.Errorf("spreadsheet path is empty")
	}
	if _, err := os.Stat(s.path); err != nil {
		return nil, fmt.Errorf("open spreadsheet %q: %w", s.path, err)
	}

	var (
		rows [][]string
		err  error
	)
	switch ext := strings.ToLower(filepath.Ext(s.path)); ext {
	case ".csv":
		rows, err = readCSV(s.path)
	case ".xlsx", ".xlsm":
		rows, err = readXLSX(s.path)
	default:
		return nil, fmt.Errorf("unsupported spreadsheet format %q: use .xlsx, .xlsm or .csv", ext)
	}
	if err != nil {
		return nil, err
	}

	headerIdx, cm, err := detectHeader(rows)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(s.path), err)
	}

	scenarios := buildRows(rows[headerIdx+1:], cm)
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("%s: header found but no data rows below it", filepath.Base(s.path))
	}
	return scenarios, nil
}

func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // rows may be ragged
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	return rows, nil
}

func readXLSX(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx has no worksheets")
	}
	sheet := sheets[0]
	for _, name := range sheets {
		if name == preferredSheetName {
			sheet = name
			break
		}
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read worksheet %q: %w", sheet, err)
	}
	return rows, nil
}
