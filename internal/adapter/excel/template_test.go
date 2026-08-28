package excel

import (
	"path/filepath"
	"strings"
	"testing"

	scenariosheet "github.com/wirsal/kitqa/internal/adapter/sheet"
)

// The template must round-trip through UC0's own reader: its header row has
// to be recognized (not "no header row found"), even though a blank template
// legitimately has no data rows yet.
func TestWriteScenarioTemplateIsReadableByFileSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario_template.xlsx")

	if err := WriteScenarioTemplate(path); err != nil {
		t.Fatalf("WriteScenarioTemplate: %v", err)
	}

	src := scenariosheet.NewFileSource(path)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() succeeded on a blank template; want a 'no data rows' error")
	}
	if !strings.Contains(err.Error(), "no data rows") {
		t.Errorf("Load() error = %q, want it to mention a recognized header with no data rows (not a header-detection failure)", err.Error())
	}
}

func TestWriteScenarioTemplateAppendsXLSXExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario_template")

	if err := WriteScenarioTemplate(path); err != nil {
		t.Fatalf("WriteScenarioTemplate: %v", err)
	}
	if _, err := scenariosheet.NewFileSource(path + ".xlsx").Load(); err == nil || !strings.Contains(err.Error(), "no data rows") {
		t.Errorf("expected the .xlsx file to exist and parse with a 'no data rows' error, got: %v", err)
	}
}
