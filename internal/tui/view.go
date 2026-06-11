package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	appTitle = "KitQA"
	divider  = "────────────────────────────────────────────────────"
)

func (m Model) View() string {
	sidebar := m.renderSidebar()
	content := m.renderContent()

	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", content)

	var helpText string
	switch m.state {
	case stateForm:
		helpText = "tab/↑↓: navigate fields | enter: confirm | esc: back | q: quit"
	case stateResult:
		helpText = "enter/esc: back to menu | q: quit"
	default:
		helpText = "j/k: navigate | enter: select | esc: back | q: quit"
	}

	help := helpStyle.Render(helpText)
	return lipgloss.JoinVertical(lipgloss.Left, layout, help)
}

// ── Sidebar ───────────────────────────────────────────────────────────────────

func (m Model) renderSidebar() string {
	var sb strings.Builder

	sb.WriteString(appTitleStyle.Render(appTitle))
	sb.WriteString("\n\n")
	sb.WriteString(sectionLabelStyle.Render("> How to Use"))
	sb.WriteString("\n")

	for i, item := range m.items {
		if i == m.cursor && m.state == stateMenu {
			sb.WriteString(menuSelectedStyle.Render("> " + item.title))
		} else {
			sb.WriteString(menuItemStyle.Render(item.title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n\n")
	sb.WriteString(helpStyle.Render("j/k: navigate | enter: select\nesc: back | q: quit | mouse: click"))

	return sidebarStyle.Render(sb.String())
}

// ── Content ───────────────────────────────────────────────────────────────────

func (m Model) renderContent() string {
	switch m.state {
	case stateMenu:
		return m.renderMenuContent()
	case stateForm:
		return m.renderForm()
	case stateRunning:
		return m.renderRunning()
	case stateResult:
		return m.renderResult()
	}
	return contentStyle.Render("")
}

func (m Model) renderMenuContent() string {
	if len(m.items) == 0 {
		return contentStyle.Render("")
	}
	item := m.items[m.cursor]

	var sb strings.Builder
	sb.WriteString(contentTitleStyle.Render(item.title))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(divider[:48]))
	sb.WriteString("\n\n")
	sb.WriteString(descStyle.Render(item.description))

	return contentStyle.Render(sb.String())
}

// ── Form ──────────────────────────────────────────────────────────────────────

var formLabelsDownload = [fieldCountUC1]string{
	"Username  :",
	"Password  :",
	"Page URL  :",
	"Repo Path :",
	"Project   :",
	"Output    :",
}

var formLabelsUpdate = [fieldCountUC2]string{
	"Username  :",
	"Password  :",
	"Page URL  :",
	"Job URL   :",
}

func (m Model) formLabel(i int) string {
	if m.activeUC == ucUpdateScenario {
		if i < fieldCountUC2 {
			return formLabelsUpdate[i]
		}
		return ""
	}
	return formLabelsDownload[i]
}

func (m Model) activeTitle() string {
	if m.activeUC == ucUpdateScenario {
		return "Update Scenario Detail"
	}
	return "Download TC to Excel"
}

func (m Model) renderForm() string {
	if m.inputs == nil {
		return contentStyle.Render("")
	}

	var sb strings.Builder
	sb.WriteString(contentTitleStyle.Render(m.activeTitle()))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(divider[:48]))
	sb.WriteString("\n\n")

	for i := 0; i < m.activeFieldCount(); i++ {
		var lbl string
		if m.formFocus == i {
			lbl = inputLabelFocusStyle.Render(m.formLabel(i))
		} else {
			lbl = labelStyle.Render(m.formLabel(i))
		}
		sb.WriteString(lbl + " " + m.inputs[i].View())
		sb.WriteString("\n\n")
	}

	sb.WriteString("\n")
	if m.activeUC == ucUpdateScenario {
		if m.formFocus == m.activeFieldButton() {
			sb.WriteString(btnFocusStyle.Render("  Update  "))
		} else {
			sb.WriteString(btnBlurStyle.Render(" Update "))
		}
	} else {
		if m.formFocus == m.activeFieldButton() {
			sb.WriteString(btnFocusStyle.Render("  Download  "))
		} else {
			sb.WriteString(btnBlurStyle.Render(" Download "))
		}
	}

	return contentStyle.Render(sb.String())
}

// ── Running ───────────────────────────────────────────────────────────────────

func (m Model) renderRunning() string {
	var runText string
	if m.activeUC == ucUpdateScenario {
		runText = "Updating Scenario Detail page on Confluence...\nPlease wait."
	} else {
		runText = "Fetching test cases from Confluence...\nPlease wait."
	}

	var sb strings.Builder
	sb.WriteString(contentTitleStyle.Render(m.activeTitle()))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(divider[:48]))
	sb.WriteString("\n\n\n")
	sb.WriteString(m.spinner.View())
	sb.WriteString("  ")
	sb.WriteString(descStyle.Render(runText))
	return contentStyle.Render(sb.String())
}

// ── Result ────────────────────────────────────────────────────────────────────

func (m Model) renderResult() string {
	var sb strings.Builder
	sb.WriteString(contentTitleStyle.Render(m.activeTitle()))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(divider[:48]))
	sb.WriteString("\n\n\n")

	if m.resultErr {
		sb.WriteString(errorStyle.Render("✗  Error"))
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(m.resultMsg))
	} else {
		sb.WriteString(successStyle.Render("✓  Success!"))
		sb.WriteString("\n\n")
		sb.WriteString(successStyle.Render(m.resultMsg))
	}

	sb.WriteString("\n\n\n")
	sb.WriteString(helpStyle.Render("Press enter or esc to return to menu."))
	return contentStyle.Render(sb.String())
}
