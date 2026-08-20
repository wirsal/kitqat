package sheet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestLoadCSVHeaderOnFirstRow(t *testing.T) {
	path := writeTemp(t, "s.csv", `No. Test Case,Scenario,Function,Input Data,Steps,Expected Result
TC01 -,Buka halaman,Mapping Program,NIK 123,Klik menu,Halaman tampil
TC02 -,Simpan data,Mapping Program,,Klik simpan,Data tersimpan
,,,,,
`)

	rows, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (blank row should be dropped)", len(rows))
	}
	if rows[0].TCNumber != "01" || rows[0].Scenario != "Buka halaman" || rows[0].ExpectedResult != "Halaman tampil" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].TCNumber != "02" || rows[1].InputData != "" {
		t.Errorf("row 1 = %+v", rows[1])
	}
}

func TestLoadCSVHeaderBelowTitleRows(t *testing.T) {
	// Mirrors the exporter layout, where the real header is not on row 1.
	path := writeTemp(t, "s.csv", `Scenario Detail BRSF02-39,,
,,
Test ID,Summary,Description
BRSF02_39_03,Skenario tiga,Fungsi tiga
`)

	rows, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].TCNumber != "03" {
		t.Errorf("TCNumber = %q, want %q", rows[0].TCNumber, "03")
	}
	if rows[0].Scenario != "Skenario tiga" || rows[0].Function != "Fungsi tiga" {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestLoadCSVFallsBackToRowPosition(t *testing.T) {
	path := writeTemp(t, "s.csv", `Scenario,Expected Result
Pertama,Hasil satu
Kedua,Hasil dua
`)

	rows, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rows[0].TCNumber != "01" || rows[1].TCNumber != "02" {
		t.Errorf("TC numbers = %q, %q; want 01, 02", rows[0].TCNumber, rows[1].TCNumber)
	}
}

func TestLoadXLSXPrefersCreateTestSheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.xlsx")

	f := excelize.NewFile()
	defer f.Close()
	// Sheet1 comes first but must lose to the exporter's own sheet name.
	mustSet(t, f, "Sheet1", "A1", "Scenario")
	mustSet(t, f, "Sheet1", "B1", "Expected Result")
	mustSet(t, f, "Sheet1", "A2", "Dari sheet salah")

	if _, err := f.NewSheet(preferredSheetName); err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	// Header on row 2, mirroring an exported file with a banner row above it.
	mustSet(t, f, preferredSheetName, "A2", "No. Test Case")
	mustSet(t, f, preferredSheetName, "B2", "Scenario")
	mustSet(t, f, preferredSheetName, "C2", "Steps")
	mustSet(t, f, preferredSheetName, "A3", "TC04 -")
	mustSet(t, f, preferredSheetName, "B3", "Skenario empat")
	mustSet(t, f, preferredSheetName, "C3", "Langkah empat")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	rows, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].TCNumber != "04" || rows[0].Scenario != "Skenario empat" || rows[0].Steps != "Langkah empat" {
		t.Errorf("row = %+v", rows[0])
	}
}

func mustSet(t *testing.T, f *excelize.File, sheet, cell, value string) {
	t.Helper()
	if err := f.SetCellValue(sheet, cell, value); err != nil {
		t.Fatalf("SetCellValue(%s!%s): %v", sheet, cell, err)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("no header", func(t *testing.T) {
		path := writeTemp(t, "s.csv", "a,b,c\n1,2,3\n")
		if _, err := NewFileSource(path).Load(); err == nil {
			t.Error("expected an error when no header row is recognised")
		}
	})

	t.Run("header but no data", func(t *testing.T) {
		path := writeTemp(t, "s.csv", "Scenario,Expected Result\n")
		if _, err := NewFileSource(path).Load(); err == nil {
			t.Error("expected an error when there are no data rows")
		}
	})

	t.Run("unsupported extension", func(t *testing.T) {
		path := writeTemp(t, "s.txt", "Scenario\nx\n")
		if _, err := NewFileSource(path).Load(); err == nil {
			t.Error("expected an error for an unsupported extension")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := NewFileSource("/nope/missing.csv").Load(); err == nil {
			t.Error("expected an error for a missing file")
		}
	})
}

func TestNewFileSourceStripsQuotes(t *testing.T) {
	got := NewFileSource(`  "C:\Users\qa\scenario.xlsx"  `).Path()
	if got != `C:\Users\qa\scenario.xlsx` {
		t.Errorf("Path() = %q", got)
	}
}

func TestExtractNumber(t *testing.T) {
	cases := map[string]string{
		"TC01 -":       "01",
		"tc 7":         "07",
		"BRSF02_38_01": "01",
		"3":            "03",
		"":             "",
		"n/a":          "",
		"0":            "",
	}
	for in, want := range cases {
		if got := extractNumber(in); got != want {
			t.Errorf("extractNumber(%q) = %q, want %q", in, got, want)
		}
	}
}
