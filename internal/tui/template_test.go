package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// "t" must still type into a UC0 text field normally — the hotkey only fires
// when focus is on the Fill button, never while editing Page URL/Sheet File.
func TestGenerateTemplateHotkeyDoesNotLeakIntoTextInput(t *testing.T) {
	m := Model{
		state:     stateForm,
		activeUC:  ucFillScenario,
		inputs:    buildInputs(ucFillScenario),
		formFocus: fieldPageURL,
	}
	m.inputs[fieldPageURL].Focus()

	next, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := next.(Model)

	if got.state != stateForm {
		t.Fatalf("state = %v, want stateForm (hotkey should not fire while editing a field)", got.state)
	}
	if got.inputs[fieldPageURL].Value() != "t" {
		t.Errorf("Page URL field = %q, want %q (typed character was swallowed)", got.inputs[fieldPageURL].Value(), "t")
	}
}

// On the Fill button, "t" generates a template instead of submitting the form.
func TestGenerateTemplateHotkeyFiresOnButton(t *testing.T) {
	m := Model{
		state:     stateForm,
		activeUC:  ucFillScenario,
		inputs:    buildInputs(ucFillScenario),
		formFocus: fieldCountUC4, // the button slot
		spinner:   New().spinner,
	}

	next, cmd := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := next.(Model)

	if got.state != stateRunning {
		t.Fatalf("state = %v, want stateRunning", got.state)
	}
	if !got.templateInProgress {
		t.Error("templateInProgress = false, want true")
	}
	if cmd == nil {
		t.Error("expected a command to be returned")
	}
}

// The hotkey is scoped to UC0 only — on another use case's button, "t" is a no-op.
func TestGenerateTemplateHotkeyScopedToUC0(t *testing.T) {
	m := Model{
		state:     stateForm,
		activeUC:  ucDownload,
		inputs:    buildInputs(ucDownload),
		formFocus: fieldCountUC1, // the button slot
	}

	next, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := next.(Model)

	if got.state != stateForm {
		t.Errorf("state = %v, want stateForm (hotkey must not fire outside UC0)", got.state)
	}
	if got.templateInProgress {
		t.Error("templateInProgress = true, want false")
	}
}
