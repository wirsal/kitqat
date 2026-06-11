package tui

import (
	"fmt"
	"strings"

	"github.com/wirsal/kitqa/internal/adapter/cbagnito"
	"github.com/wirsal/kitqa/internal/adapter/confluence"
	"github.com/wirsal/kitqa/internal/adapter/excel"
	"github.com/wirsal/kitqa/internal/adapter/jira"
	"github.com/wirsal/kitqa/internal/domain"
	"github.com/wirsal/kitqa/internal/usecase"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	stateMenu    viewState = iota
	stateForm              // form input
	stateRunning           // async operation in progress
	stateResult            // success/error
)

type activeUseCase int

const (
	ucDownload       activeUseCase = iota
	ucUpdateScenario               // update Scenario Detail page with Jira links
)

type menuItem struct {
	title       string
	description string
}

type downloadDoneMsg struct {
	err    error
	output string
	count  int
}

type updateDoneMsg struct {
	err    error
	result *usecase.UpdateResult
}

const (
	fieldUsername = 0
	fieldPassword = 1
	fieldPageURL  = 2
	fieldRepoPath = 3
	fieldProject  = 4 // UC1 only: project key (e.g. SPBRISURF)
	fieldOutput   = 5 // UC1: output file path
	fieldJobURL   = 3 // UC2: CBAgnito job URL (slot 3, after Page URL)

	fieldCountUC1 = 6 // UC1 has 6 input fields
	fieldCountUC2 = 4 // UC2 has 4 input fields (no Repo Path needed)
	fieldCount    = 6 // max — always allocate this many inputs
)

type Model struct {
	state     viewState
	activeUC  activeUseCase
	items     []menuItem
	cursor    int
	inputs    []textinput.Model
	formFocus int // 0..4 = text fields, 5 = button
	spinner   spinner.Model
	resultMsg string
	resultErr bool
	width     int
	height    int
}

func New() Model {
	items := []menuItem{
		{
			title: "1. Download TC to Excel",
			description: "Download test cases from a Confluence\n" +
				"Scenario Detail page and export to\n" +
				"XLSX format ready to upload to CBAgnito.\n\n" +
				"Fields:\n" +
				"  • Username & Password\n" +
				"  • Confluence page URL\n" +
				"  • Repo Path (default folder)\n" +
				"  • Project key (default: SPBRISURF)\n" +
				"  • Output file destination (.xlsx)",
		},
		{
			title: "2. Update Scenario Detail",
			description: "After uploading Excel to CBAgnito,\n" +
				"inject Jira ticket links into the\n" +
				"'No. Test Case' cells on Confluence.\n\n" +
				"Fields:\n" +
				"  • Username & Password\n" +
				"  • Confluence page URL\n" +
				"  • CBAgnito job URL",
		},
		{
			title:       "3. Exit",
			description: "Exit the application.",
		},
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		state:  stateMenu,
		items:  items,
		cursor: 0,
		spinner: s,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.state == stateRunning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case downloadDoneMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			m.resultMsg = "Downloaded " + itoa(msg.count) + " test cases.\nSaved to: " + msg.output
		}

	case updateDoneMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			r := msg.result
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Updated  : %d cell(s)\n", r.UpdatedCells))
			sb.WriteString(fmt.Sprintf("Skipped  : %d cell(s) (already linked)\n", r.SkippedCells))
			if len(r.UnmatchedTCs) > 0 {
				sb.WriteString("Unmatched: " + strings.Join(r.UnmatchedTCs, ", "))
			} else {
				sb.WriteString("All TCs matched.")
			}
			m.resultMsg = sb.String()
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateMenu:
		return m.updateMenu(msg)
	case stateForm:
		return m.updateForm(msg)
	case stateResult:
		return m.updateResult(msg)
	}
	return m, nil
}

// ── Menu ──────────────────────────────────────────────────────────────────────

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		switch m.cursor {
		case 0:
			m.activeUC = ucDownload
			m.state = stateForm
			m.inputs = buildInputs(ucDownload)
			m.formFocus = 0
			return m, m.inputs[0].Focus()
		case 1:
			m.activeUC = ucUpdateScenario
			m.state = stateForm
			m.inputs = buildInputs(ucUpdateScenario)
			m.formFocus = 0
			return m, m.inputs[0].Focus()
		case 2:
			return m, tea.Quit
		}
	}
	return m, nil
}

// ── Form ──────────────────────────────────────────────────────────────────────

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.state = stateMenu
		m.inputs = nil
		m.formFocus = 0
		return m, nil
	case "tab", "down":
		return m.formShift(+1)
	case "shift+tab", "up":
		return m.formShift(-1)
	case "enter":
		if m.formFocus == m.activeFieldButton() {
			return m.startAction()
		}
		return m.formShift(+1)
	default:
		if m.formFocus < m.activeFieldCount() {
			var cmd tea.Cmd
			m.inputs[m.formFocus], cmd = m.inputs[m.formFocus].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) activeFieldCount() int {
	if m.activeUC == ucDownload {
		return fieldCountUC1
	}
	return fieldCountUC2
}

func (m Model) activeFieldButton() int {
	return m.activeFieldCount()
}

func (m Model) formShift(delta int) (tea.Model, tea.Cmd) {
	btn := m.activeFieldButton()
	fc  := m.activeFieldCount()
	if m.formFocus < fc {
		m.inputs[m.formFocus].Blur()
	}
	m.formFocus = (m.formFocus + delta + btn + 1) % (btn + 1)
	if m.formFocus < fc {
		return m, m.inputs[m.formFocus].Focus()
	}
	return m, nil
}

func (m Model) startAction() (tea.Model, tea.Cmd) {
	username := m.inputs[fieldUsername].Value()
	password := m.inputs[fieldPassword].Value()
	pageURL  := m.inputs[fieldPageURL].Value()

	m.state = stateRunning

	if m.activeUC == ucDownload {
		repoPath := m.inputs[fieldRepoPath].Value()
		project  := m.inputs[fieldProject].Value()
		output   := strings.ReplaceAll(m.inputs[fieldOutput].Value(), `"`, "")
		if output == "" {
			output = "test_cases.xlsx"
		}
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return doDownload(username, password, pageURL, repoPath, project, output) },
		)
	}

	jobURL := m.inputs[fieldJobURL].Value()
	return m, tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return doUpdateScenario(username, password, pageURL, jobURL) },
	)
}

// ── Result ────────────────────────────────────────────────────────────────────

func (m Model) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "enter", "backspace":
		m.state = stateMenu
		m.resultMsg = ""
	}
	return m, nil
}

// ── Download logic ────────────────────────────────────────────────────────────

func doDownload(username, password, pageURL, repoPath, project, output string) tea.Msg {
	confluenceClient := confluence.NewClient("https://confluence.bri.co.id", username, password)
	repo             := confluence.NewRepository(confluenceClient)
	jiraClient       := jira.NewClient("https://jira.bri.co.id", username, password)
	repo.WithJiraClient(jiraClient)
	repo.SetDefaultRepoPath(repoPath)
	repo.SetDefaultProject(project)

	exp := excel.NewExporter()

	pageID, err := repo.ResolvePageID(pageURL)
	if err != nil {
		return downloadDoneMsg{err: err}
	}

	var testCases []domain.TestCase
	testCases, err = repo.FetchTestCases(pageID)
	if err != nil {
		return downloadDoneMsg{err: err}
	}
	for i := range testCases {
		testCases[i].No = i + 1
	}
	if err = exp.Export(testCases, output); err != nil {
		return downloadDoneMsg{err: err}
	}
	return downloadDoneMsg{count: len(testCases), output: output}
}

// ── Update Scenario logic ─────────────────────────────────────────────────────

func doUpdateScenario(username, password, pageURL, jobURL string) tea.Msg {
	confluenceClient := confluence.NewClient("https://confluence.bri.co.id", username, password)
	repo             := confluence.NewRepository(confluenceClient)
	cbClient         := cbagnito.NewClient(username, password)

	uc := usecase.NewUpdateScenarioUseCase(repo, cbClient)
	result, err := uc.Execute(pageURL, jobURL)
	return updateDoneMsg{err: err, result: result}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildInputs(uc activeUseCase) []textinput.Model {
	inputs := make([]textinput.Model, fieldCount)

	inputs[fieldUsername] = newInput("e.g. wirsal.dev", false)
	inputs[fieldPassword] = newInput("your password", true)
	inputs[fieldPageURL]  = newInput("https://confluence.bri.co.id/x/...", false)

	if uc == ucDownload {
		inputs[fieldRepoPath] = newInput("/Monitoring Subrogasi Unprocess", false)
		inputs[fieldProject]  = newInputWithValue("SPBRISURF", false)
		inputs[fieldOutput]   = newInputWithValue("test_cases.xlsx", false)
	} else {
		// UC2: slot 3 = Job URL (Repo Path not needed)
		inputs[fieldJobURL] = newInput("http://172.18.249.109:1014/cr-job/...", false)
	}

	return inputs
}

func newInput(placeholder string, isPassword bool) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.CharLimit = 500
	t.PromptStyle = lipgloss.NewStyle().Foreground(colorOrange)
	t.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	if isPassword {
		t.EchoMode = textinput.EchoPassword
		t.EchoCharacter = '•'
	}
	return t
}

func newInputWithValue(defaultValue string, isPassword bool) textinput.Model {
	t := newInput("", isPassword)
	t.SetValue(defaultValue)
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
