package domain

type TestCase struct {
	No                 int
	Project            string
	IssueType          string
	TestID             string
	Summary            string
	Description        string
	Assignee           string
	TestRepositoryPath string
	ExpectedResult     string
	TestExecution      string // optional Jira Test Execution key, written to the Test Execution column
}

// ScenarioRow is one scenario definition read from a spreadsheet (local file today,
// Google Sheets later). It feeds the "Fill Scenario & Function" use case, which writes
// these values into the vertical test-case tables of a Confluence Scenario Detail page.
type ScenarioRow struct {
	TCNumber       string // two-digit TC number, e.g. "01"
	Scenario       string
	Function       string
	InputData      string
	Steps          string
	ExpectedResult string
}

// IsEmpty reports whether the row carries no usable content.
func (r ScenarioRow) IsEmpty() bool {
	return r.Scenario == "" && r.Function == "" && r.InputData == "" &&
		r.Steps == "" && r.ExpectedResult == ""
}
