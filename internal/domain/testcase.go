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
}
