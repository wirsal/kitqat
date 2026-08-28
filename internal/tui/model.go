package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wirsal/kitqa/internal/adapter/confluence"
	"github.com/wirsal/kitqa/internal/adapter/excel"
	"github.com/wirsal/kitqa/internal/adapter/jira"
	"github.com/wirsal/kitqa/internal/adapter/sheet"
	"github.com/wirsal/kitqa/internal/domain"
	"github.com/wirsal/kitqa/internal/usecase"
)

type viewState int

const (
	stateLogin   viewState = iota // one-time username/password entry, cached for the session
	stateMenu
	stateForm              // form input
	stateRunning           // async operation in progress
	stateResult            // success/error
)

type activeUseCase int

const (
	ucDownload       activeUseCase = iota
	ucUpdateScenario               // update Scenario Detail page with Jira links
	ucSetDone                      // transition Jira tickets from CBAgnito job to Done
	ucFillScenario                 // fill Scenario/Function cells from a spreadsheet
	ucAddTestExec                  // insert a Test Execution filter table on the page
)

type menuItem struct {
	title       string
	description string
	uc          activeUseCase
	isExit      bool
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

type setDoneMsg struct {
	err    error
	result *usecase.SetDoneResult
}

type fillDoneMsg struct {
	err    error
	result *usecase.FillResult
}

type addTestExecMsg struct {
	err    error
	result *usecase.AddTestExecResult
}

// Username/password are entered once on the login screen and cached in the Model
// for the rest of the session (see stateLogin) — use-case forms no longer carry
// their own username/password fields.
const (
	fieldPageURL    = 0
	fieldRepoPath   = 1
	fieldProject    = 2 // UC1 only: project key (e.g. SPBRISURF)
	fieldOutput     = 3 // UC1: output file path
	fieldTestExec   = 4 // UC1 only: optional Test Execution key (slot 4)
	fieldJobURL     = 1 // UC2: Jira issue (slot 1, after Page URL)
	fieldJobURLUC3  = 0 // UC3: CBAgnito job URL (slot 0, no Page URL needed)
	fieldSourceFile = 1 // UC4: spreadsheet path (slot 1, after Page URL)
	fieldTestExecTE = 1 // UC5: Test Execution issue / JQL (slot 1, after Page URL)

	fieldCountUC1 = 5 // UC1 has 5 input fields (last one, Test Execution, is optional)
	fieldCountUC2 = 2 // UC2 has 2 input fields
	fieldCountUC3 = 1 // UC3 has 1 input field (job URL)
	fieldCountUC4 = 2 // UC4 has 2 input fields (page URL, sheet file)
	fieldCountUC5 = 2 // UC5 has 2 input fields (page URL, test exec)
	fieldCount    = 5 // max — always allocate this many inputs

	// Login screen (stateLogin) field indices — separate from the use-case forms above.
	loginFieldUsername = 0
	loginFieldPassword = 1
	loginFieldCount    = 2
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

	// username/password are entered once on the login screen and cached here for
	// the rest of the session — every use case reuses them without re-prompting.
	username string
	password string
}

func New() Model {
	items := []menuItem{
		{
			title: "0. Fill Scenario & Function",
			uc:    ucFillScenario,
			description: "Fill Scenario, Function, Input Data,\n" +
				"Steps and Expected Result on a Confluence\n" +
				"page from a spreadsheet. Creates TC tables\n" +
				"that don't exist yet; never overwrites\n" +
				"cells that already have content.\n\n" +
				"Runs before step 1.\n\n" +
				"Fields:\n" +
				"  • Confluence page URL\n" +
				"  • Sheet file (.xlsx / .csv)",
		},
		{
			title: "1. Download TC to Excel",
			uc:    ucDownload,
			description: "Download test cases from a Confluence\n" +
				"Scenario Detail page and export to\n" +
				"XLSX format ready to upload to CBAgnito.\n\n" +
				"Fields:\n" +
				"  • Confluence page URL\n" +
				"  • Repo Path (default folder)\n" +
				"  • Project key (default: SPBRISURF)\n" +
				"  • Output file destination (.xlsx)\n" +
				"  • Test Execution key (optional)",
		},
		{
			title: "2. Update Scenario Detail",
			uc:    ucUpdateScenario,
			description: "Inject Jira ticket links into the\n" +
				"'No. Test Case' cells on Confluence,\n" +
				"matched from a Jira issue's tests.\n\n" +
				"Fields:\n" +
				"  • Confluence page URL\n" +
				"  • Jira issue (e.g. BRSF-16163)",
		},
		{
			title: "3. Add Test Execution Table",
			uc:    ucAddTestExec,
			description: "Insert a Test Execution table (Jira\n" +
				"filter) at the top of a Confluence\n" +
				"Scenario Detail page.\n\n" +
				"Fields:\n" +
				"  • Confluence page URL\n" +
				"  • Test Execution issue (BRSF-16163)\n" +
				"    or a full JQL filter",
		},
		{
			title: "4. Mark Tickets as Done",
			uc:    ucSetDone,
			description: "For every test in a Jira Test\n" +
				"Execution: set the run status to PASS\n" +
				"and transition the issue to Done.\n\n" +
				"Fields:\n" +
				"  • Test Execution issue (BRSF-16163)",
		},
		{
			title:       "5. Exit",
			isExit:      true,
			description: "Exit the application.",
		},
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		state:     stateLogin,
		items:     items,
		cursor:    0,
		spinner:   s,
		inputs:    buildLoginInputs(),
		formFocus: 0,
	}
}

func (m Model) Init() tea.Cmd {
	if m.state == stateLogin && len(m.inputs) > 0 {
		return m.inputs[0].Focus()
	}
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

	case setDoneMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			r := msg.result
			var sb strings.Builder
			fmt.Fprintf(&sb, "Total    : %d test(s)\n", r.Total)
			fmt.Fprintf(&sb, "Passed   : %d run(s) set to PASS\n", r.Passed)
			fmt.Fprintf(&sb, "Done     : %d issue(s) transitioned\n", r.Done)
			if r.Skipped > 0 {
				fmt.Fprintf(&sb, "Skipped  : %d (no Done transition)\n", r.Skipped)
			}
			execState := "not transitioned"
			if r.ExecDone {
				execState = "transitioned to Done"
			}
			fmt.Fprintf(&sb, "Exec %s : %s\n", r.ExecKey, execState)
			if len(r.Failed) > 0 {
				sb.WriteString("Failed   : " + strings.Join(r.Failed, ", "))
			} else {
				sb.WriteString("All tests and the execution set to PASS/Done.")
			}
			m.resultMsg = sb.String()
		}

	case fillDoneMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			r := msg.result
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Page     : %s\n", r.PageTitle))
			sb.WriteString(fmt.Sprintf("Filled   : %d cell(s)\n", r.FilledCells))
			sb.WriteString(fmt.Sprintf("Skipped  : %d cell(s) (already filled)\n", r.SkippedCells))
			if len(r.CreatedTCs) > 0 {
				sb.WriteString("Created  : " + strings.Join(r.CreatedTCs, ", ") + "\n")
			}
			if len(r.UnmatchedTCs) > 0 {
				sb.WriteString("Unmatched: " + strings.Join(r.UnmatchedTCs, ", ") + "\n")
			}
			if !r.Changed {
				sb.WriteString("\nPage already up to date — nothing was written.")
			}
			m.resultMsg = strings.TrimRight(sb.String(), "\n")
		}

	case addTestExecMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			r := msg.result
			var sb strings.Builder
			fmt.Fprintf(&sb, "Page   : %s\n", r.PageTitle)
			fmt.Fprintf(&sb, "Filter : %s\n\n", r.JQL)
			if r.Added {
				sb.WriteString("Test Execution table inserted.")
			} else {
				sb.WriteString("Table already present — nothing was written.")
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
	case stateLogin:
		return m.updateLogin(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case stateForm:
		return m.updateForm(msg)
	case stateResult:
		return m.updateResult(msg)
	}
	return m, nil
}

// ── Login ─────────────────────────────────────────────────────────────────────
// Username/password are collected once here and cached on the Model for the rest
// of the session; every use case reads m.username/m.password instead of asking again.

func (m Model) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "down":
		return m.loginShift(+1)
	case "shift+tab", "up":
		return m.loginShift(-1)
	case "enter":
		if m.formFocus == loginFieldCount {
			if strings.TrimSpace(m.inputs[loginFieldUsername].Value()) == "" {
				m.formFocus = loginFieldUsername
				return m, m.inputs[loginFieldUsername].Focus()
			}
			m.username = m.inputs[loginFieldUsername].Value()
			m.password = m.inputs[loginFieldPassword].Value()
			m.state = stateMenu
			m.inputs = nil
			m.formFocus = 0
			return m, nil
		}
		return m.loginShift(+1)
	default:
		if m.formFocus < loginFieldCount {
			var cmd tea.Cmd
			m.inputs[m.formFocus], cmd = m.inputs[m.formFocus].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) loginShift(delta int) (tea.Model, tea.Cmd) {
	if m.formFocus < loginFieldCount {
		m.inputs[m.formFocus].Blur()
	}
	m.formFocus = (m.formFocus + delta + loginFieldCount + 1) % (loginFieldCount + 1)
	if m.formFocus < loginFieldCount {
		return m, m.inputs[m.formFocus].Focus()
	}
	return m, nil
}

func buildLoginInputs() []textinput.Model {
	inputs := make([]textinput.Model, loginFieldCount)
	inputs[loginFieldUsername] = newInput("e.g. wirsal.dev", false)
	inputs[loginFieldPassword] = newInput("your password", true)
	return inputs
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
	case "c":
		// Change cached credentials without exiting the app.
		m.state = stateLogin
		m.inputs = buildLoginInputs()
		m.inputs[loginFieldUsername].SetValue(m.username)
		m.formFocus = 0
		return m, m.inputs[0].Focus()
	case "enter":
		item := m.items[m.cursor]
		if item.isExit {
			return m, tea.Quit
		}
		m.activeUC = item.uc
		m.state = stateForm
		m.inputs = buildInputs(item.uc)
		m.formFocus = 0
		return m, m.inputs[0].Focus()
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
	switch m.activeUC {
	case ucDownload:
		return fieldCountUC1
	case ucUpdateScenario:
		return fieldCountUC2
	case ucSetDone:
		return fieldCountUC3
	case ucFillScenario:
		return fieldCountUC4
	case ucAddTestExec:
		return fieldCountUC5
	}
	return fieldCountUC1
}

func (m Model) activeFieldButton() int {
	return m.activeFieldCount()
}

func (m Model) formShift(delta int) (tea.Model, tea.Cmd) {
	btn := m.activeFieldButton()
	fc := m.activeFieldCount()
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
	username := m.username
	password := m.password
	pageURL := m.inputs[fieldPageURL].Value()

	m.state = stateRunning

	if m.activeUC == ucDownload {
		repoPath := m.inputs[fieldRepoPath].Value()
		project := m.inputs[fieldProject].Value()
		output := strings.ReplaceAll(m.inputs[fieldOutput].Value(), `"`, "")
		if output == "" {
			output = "test_cases.xlsx"
		}
		testExec := strings.TrimSpace(m.inputs[fieldTestExec].Value())
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return doDownload(username, password, pageURL, repoPath, project, output, testExec) },
		)
	}

	if m.activeUC == ucUpdateScenario {
		jobURL := m.inputs[fieldJobURL].Value()
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return doUpdateScenario(username, password, pageURL, jobURL) },
		)
	}

	if m.activeUC == ucFillScenario {
		sourceFile := strings.ReplaceAll(m.inputs[fieldSourceFile].Value(), `"`, "")
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return doFillScenario(username, password, pageURL, sourceFile) },
		)
	}

	if m.activeUC == ucAddTestExec {
		testExec := strings.TrimSpace(m.inputs[fieldTestExecTE].Value())
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return doAddTestExec(username, password, pageURL, testExec) },
		)
	}

	// UC3: Set Jira Tickets to Done
	jobURL := m.inputs[fieldJobURLUC3].Value()
	return m, tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return doSetDone(username, password, jobURL) },
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

func doDownload(username, password, pageURL, repoPath, project, output, testExec string) tea.Msg {
	confluenceClient := confluence.NewClient("https://confluence.bri.co.id", username, password)
	repo := confluence.NewRepository(confluenceClient)
	jiraClient := jira.NewClient("https://jira.bri.co.id", username, password)
	repo.WithJiraClient(jiraClient)
	repo.SetDefaultRepoPath(repoPath)
	repo.SetDefaultProject(project)
	repo.SetDefaultTestExecution(testExec)

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
	repo := confluence.NewRepository(confluenceClient)
	jiraClient := jira.NewClient("https://jira.bri.co.id", username, password)

	uc := usecase.NewUpdateScenarioUseCase(repo, jiraClient)
	result, err := uc.Execute(pageURL, jobURL)
	return updateDoneMsg{err: err, result: result}
}

// ── Set Done logic ────────────────────────────────────────────────────────────

func doSetDone(username, password, testExec string) tea.Msg {
	jiraClient := jira.NewClient("https://jira.bri.co.id", username, password)
	uc := usecase.NewSetDoneUseCase(jiraClient)
	result, err := uc.Execute(testExec)
	return setDoneMsg{err: err, result: result}
}

// ── Fill Scenario logic ───────────────────────────────────────────────────────

func doFillScenario(username, password, pageURL, sourceFile string) tea.Msg {
	confluenceClient := confluence.NewClient("https://confluence.bri.co.id", username, password)
	repo := confluence.NewRepository(confluenceClient)
	src := sheet.NewFileSource(sourceFile)

	uc := usecase.NewFillScenarioUseCase(repo, src)
	result, err := uc.Execute(pageURL)
	return fillDoneMsg{err: err, result: result}
}

// ── Add Test Execution logic ──────────────────────────────────────────────────

func doAddTestExec(username, password, pageURL, testExec string) tea.Msg {
	confluenceClient := confluence.NewClient("https://confluence.bri.co.id", username, password)
	repo := confluence.NewRepository(confluenceClient)

	uc := usecase.NewAddTestExecUseCase(repo)
	result, err := uc.Execute(pageURL, testExec)
	return addTestExecMsg{err: err, result: result}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildInputs(uc activeUseCase) []textinput.Model {
	inputs := make([]textinput.Model, fieldCount)

	inputs[fieldPageURL] = newInput("https://confluence.bri.co.id/x/...", false)

	if uc == ucDownload {
		inputs[fieldRepoPath] = newInput("/Monitoring Subrogasi Unprocess", false)
		inputs[fieldProject] = newInputWithValue("SPBRISURF", false)
		inputs[fieldOutput] = newInputWithValue("test_cases.xlsx", false)
		inputs[fieldTestExec] = newInput("optional, e.g. SPBRISURF-1234", false)
	} else if uc == ucUpdateScenario {
		// UC2: slot 1 = Jira issue (key or browse URL)
		inputs[fieldJobURL] = newInput("Jira issue e.g. BRSF-16163", false)
	} else if uc == ucFillScenario {
		// UC4: slot 1 = spreadsheet path
		inputs[fieldSourceFile] = newInput(`C:\Users\...\scenario.xlsx`, false)
	} else if uc == ucAddTestExec {
		// UC5: slot 1 = Test Execution issue or JQL filter
		inputs[fieldTestExecTE] = newInput("BRSF-16163 or a JQL filter", false)
	} else {
		// UC3: slot 0 = Jira Test Execution issue (no Page URL needed)
		inputs[fieldJobURLUC3] = newInput("Jira Test Execution e.g. BRSF-16163", false)
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
