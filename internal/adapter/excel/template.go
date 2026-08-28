package excel

import (
	"fmt"
	"strings"

	scenariosheet "github.com/wirsal/kitqa/internal/adapter/sheet"
	"github.com/xuri/excelize/v2"
)

const templateSheetName = "Create Test"

// templateBlankRows is how many pre-styled (but empty) data rows to leave
// under the header so the sheet doesn't look like a bare header line when
// opened in Excel — purely cosmetic, matching buildDataStyle's borders.
const templateBlankRows = 20

// WriteScenarioTemplate creates a blank spreadsheet at outputPath ready for
// UC0 (Fill Scenario & Function): a single header row built from
// sheet.ScenarioTemplateHeaders — the same labels the UC0 parser recognizes —
// followed by templateBlankRows bordered/wrap-text data rows so the columns
// are visibly delineated once opened.
func WriteScenarioTemplate(outputPath string) error {
	if !strings.HasSuffix(strings.ToLower(outputPath), ".xlsx") {
		outputPath += ".xlsx"
	}

	f := excelize.NewFile()
	defer f.Close()

	defaultSheet := f.GetSheetName(0)
	f.SetSheetName(defaultSheet, templateSheetName)

	headerStyle, err := buildBlueHeaderStyle(f)
	if err != nil {
		return fmt.Errorf("create header style: %w", err)
	}
	dataStyle, err := buildDataStyle(f)
	if err != nil {
		return fmt.Errorf("create data style: %w", err)
	}

	headers := scenariosheet.ScenarioTemplateHeaders
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(templateSheetName, cell, h)
		f.SetCellStyle(templateSheetName, cell, cell, headerStyle)
	}

	for row := 2; row <= templateBlankRows+1; row++ {
		for col := range headers {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellStyle(templateSheetName, cell, cell, dataStyle)
		}
	}

	widths := map[int]float64{1: 14, 2: 40, 3: 28, 4: 35, 5: 45, 6: 45}
	for col, w := range widths {
		colName, _ := excelize.ColumnNumberToName(col)
		f.SetColWidth(templateSheetName, colName, colName, w)
	}
	f.SetRowHeight(templateSheetName, 1, 24)

	if err := f.SetPanes(templateSheetName, &excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return fmt.Errorf("freeze header row: %w", err)
	}

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}
