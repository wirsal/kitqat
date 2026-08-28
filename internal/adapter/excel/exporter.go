package excel

import (
	"fmt"
	"strings"

	"github.com/wirsal/kitqa/internal/domain"
	"github.com/xuri/excelize/v2"
)

const sheet = "Create Test"

// Main header columns (row 1, cols A–I).
var mainHeaders = []string{
	"No",                  // A
	"Project",             // B
	"Issue Type",          // C
	"Test ID",             // D
	"Summary",             // E
	"Description",         // F
	"Assignee",            // G
	"Test Repository Path", // H
	"Expected Result",     // I
}

// manualStepsStart is the 1-based column index where Manual Test Steps begin (J = 10).
const manualStepsStart = 10
const manualStepsCount = 3 // number of step groups
// Each step group has 3 sub-columns: Action, Data, Expected Result.
// Total Manual Test Steps columns = 9 (cols J–R).
const manualStepsEnd = manualStepsStart + manualStepsCount*3 - 1 // R = 18

// Trailing header columns after Manual Test Steps (S, T, U).
var trailingHeaders = []string{
	"Test Plan",      // S = 19
	"Test Execution", // T = 20
	"Labels",         // U = 21
}

// testExecutionCol is the 1-based column index of "Test Execution" (T = 20):
// the second trailing header, i.e. manualStepsEnd + 1 (Test Plan) + 1.
const testExecutionCol = manualStepsEnd + 2

type Exporter struct{}

func NewExporter() *Exporter { return &Exporter{} }

func (e *Exporter) Export(testCases []domain.TestCase, outputPath string) error {
	if !strings.HasSuffix(strings.ToLower(outputPath), ".xlsx") {
		outputPath += ".xlsx"
	}

	f, err := e.build(testCases)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

// ExportBytes builds the same workbook as Export but returns it in-memory,
// for callers that need to stream it (e.g. an HTTP response) instead of
// writing to disk.
func (e *Exporter) ExportBytes(testCases []domain.TestCase) ([]byte, error) {
	f, err := e.build(testCases)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write to buffer: %w", err)
	}
	return buf.Bytes(), nil
}

func (e *Exporter) build(testCases []domain.TestCase) (*excelize.File, error) {
	f := excelize.NewFile()

	defaultSheet := f.GetSheetName(0)
	f.SetSheetName(defaultSheet, sheet)

	// Build styles
	blueHeaderStyle, err := buildBlueHeaderStyle(f)
	if err != nil {
		return nil, fmt.Errorf("create blue header style: %w", err)
	}
	yellowHeaderStyle, err := buildYellowHeaderStyle(f)
	if err != nil {
		return nil, fmt.Errorf("create yellow header style: %w", err)
	}
	yellowSubStyle, err := buildYellowSubStyle(f)
	if err != nil {
		return nil, fmt.Errorf("create yellow sub-header style: %w", err)
	}
	dataStyle, err := buildDataStyle(f)
	if err != nil {
		return nil, fmt.Errorf("create data style: %w", err)
	}

	// ── Row 1: main headers ────────────────────────────────────────────────────

	// A1–I1: blue headers
	for col, h := range mainHeaders {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, blueHeaderStyle)
	}

	// J1: "Manual Test Steps" merged across J1:R1 (yellow)
	startCell, _ := excelize.CoordinatesToCellName(manualStepsStart, 1)
	endCell, _ := excelize.CoordinatesToCellName(manualStepsEnd, 1)
	f.MergeCell(sheet, startCell, endCell)
	f.SetCellValue(sheet, startCell, "Manual Test Steps")
	f.SetCellStyle(sheet, startCell, endCell, yellowHeaderStyle)

	// S1–U1: blue trailing headers
	for i, h := range trailingHeaders {
		col := manualStepsEnd + 1 + i
		cell, _ := excelize.CoordinatesToCellName(col, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, blueHeaderStyle)
	}

	// ── Row 2: sub-headers ─────────────────────────────────────────────────────

	// A2–I2: apply blue header style (empty value, keeps border/color consistent)
	for col := 1; col <= len(mainHeaders); col++ {
		cell, _ := excelize.CoordinatesToCellName(col, 2)
		f.SetCellStyle(sheet, cell, cell, blueHeaderStyle)
	}

	// J2–R2: Action / Data / Expected Result × 3 steps (yellow sub-header)
	subCols := []string{"Action", "Data", "Expected Result"}
	for step := 0; step < manualStepsCount; step++ {
		for j, sub := range subCols {
			col := manualStepsStart + step*3 + j
			cell, _ := excelize.CoordinatesToCellName(col, 2)
			f.SetCellValue(sheet, cell, sub)
			f.SetCellStyle(sheet, cell, cell, yellowSubStyle)
		}
	}

	// S2–U2: blue header style (empty)
	for i := range trailingHeaders {
		col := manualStepsEnd + 1 + i
		cell, _ := excelize.CoordinatesToCellName(col, 2)
		f.SetCellStyle(sheet, cell, cell, blueHeaderStyle)
	}

	// ── Rows 3+: data ──────────────────────────────────────────────────────────

	for rowIdx, tc := range testCases {
		row := rowIdx + 3 // data starts at row 3
		values := []interface{}{
			tc.No,
			tc.Project,
			tc.IssueType,
			tc.TestID,
			tc.Summary,
			tc.Description,
			tc.Assignee,
			tc.TestRepositoryPath,
			tc.ExpectedResult,
		}
		for col, val := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, val)
			f.SetCellStyle(sheet, cell, cell, dataStyle)
		}
		// Apply data style to Manual Test Steps + trailing columns (left blank,
		// except Test Execution below).
		lastCol := manualStepsEnd + len(trailingHeaders)
		for col := len(mainHeaders) + 1; col <= lastCol; col++ {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			f.SetCellStyle(sheet, cell, cell, dataStyle)
		}
		// Fill the optional Test Execution column (T) when provided.
		if tc.TestExecution != "" {
			cell, _ := excelize.CoordinatesToCellName(testExecutionCol, row)
			f.SetCellValue(sheet, cell, tc.TestExecution)
		}
	}

	setColumnWidths(f)

	// Freeze first two header rows
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      2,
		TopLeftCell: "A3",
		ActivePane:  "bottomLeft",
	})

	return f, nil
}

// ── Styles ────────────────────────────────────────────────────────────────────

func buildBlueHeaderStyle(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F497D"}},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
			{Type: "top", Color: "FFFFFF", Style: 1},
			{Type: "bottom", Color: "FFFFFF", Style: 1},
		},
	})
}

func buildYellowHeaderStyle(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "000000", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F5E17A"}},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCAA00", Style: 1},
			{Type: "right", Color: "CCAA00", Style: 1},
			{Type: "top", Color: "CCAA00", Style: 1},
			{Type: "bottom", Color: "CCAA00", Style: 1},
		},
	})
}

func buildYellowSubStyle(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "000000", Size: 10},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF3C4"}},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCAA00", Style: 1},
			{Type: "right", Color: "CCAA00", Style: 1},
			{Type: "top", Color: "CCAA00", Style: 1},
			{Type: "bottom", Color: "CCAA00", Style: 1},
		},
	})
}

func buildDataStyle(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "top",
			WrapText: true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "D3D3D3", Style: 1},
			{Type: "right", Color: "D3D3D3", Style: 1},
			{Type: "top", Color: "D3D3D3", Style: 1},
			{Type: "bottom", Color: "D3D3D3", Style: 1},
		},
	})
}

func setColumnWidths(f *excelize.File) {
	widths := map[int]float64{
		1:  6,  // No
		2:  16, // Project
		3:  14, // Issue Type
		4:  22, // Test ID
		5:  45, // Summary
		6:  50, // Description
		7:  20, // Assignee
		8:  35, // Test Repository Path
		9:  50, // Expected Result
		// Manual Test Steps: 3 steps × 3 cols = cols 10–18
		10: 20, 11: 20, 12: 25, // Step 1
		13: 20, 14: 20, 15: 25, // Step 2
		16: 20, 17: 20, 18: 25, // Step 3
		19: 18, // Test Plan
		20: 18, // Test Execution
		21: 18, // Labels
	}
	for col, width := range widths {
		colName, _ := excelize.ColumnNumberToName(col)
		f.SetColWidth(sheet, colName, colName, width)
	}
	f.SetRowHeight(sheet, 1, 30)
	f.SetRowHeight(sheet, 2, 22)
}
