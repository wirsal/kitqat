package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Every menu entry must launch the use case it advertises. The selection code reads
// menuItem.uc rather than switching on the cursor index, so reordering the menu can
// no longer point an entry at the wrong use case — this locks that in.
func TestMenuSelectionMatchesItem(t *testing.T) {
	m := New()

	for i, item := range m.items {
		m.cursor = i
		next, _ := m.updateMenu(tea.KeyMsg{Type: tea.KeyEnter})
		got := next.(Model)

		if item.isExit {
			if got.state == stateForm {
				t.Errorf("item %d (%q): Exit opened a form", i, item.title)
			}
			continue
		}
		if got.state != stateForm {
			t.Errorf("item %d (%q): state = %v, want stateForm", i, item.title, got.state)
		}
		if got.activeUC != item.uc {
			t.Errorf("item %d (%q): activeUC = %v, want %v", i, item.title, got.activeUC, item.uc)
		}
		if len(got.inputs) != fieldCount {
			t.Errorf("item %d (%q): got %d inputs, want %d", i, item.title, len(got.inputs), fieldCount)
		}
		// The form must be able to render and navigate every field of this use case.
		if got.activeFieldCount() > fieldCount {
			t.Errorf("item %d (%q): activeFieldCount %d exceeds allocation %d", i, item.title, got.activeFieldCount(), fieldCount)
		}
		for f := 0; f < got.activeFieldCount(); f++ {
			if strings.TrimSpace(got.formLabel(f)) == "" {
				t.Errorf("item %d (%q): field %d has no label", i, item.title, f)
			}
		}
	}
}

// Fill Scenario & Function runs before Download TC to Excel, so it leads the menu.
func TestMenuOrder(t *testing.T) {
	want := []struct {
		prefix string
		uc     activeUseCase
	}{
		{"0. Fill Scenario & Function", ucFillScenario},
		{"1. Download TC to Excel", ucDownload},
		{"2. Update Scenario Detail", ucUpdateScenario},
		{"3. Add Test Execution Table", ucAddTestExec},
		{"4. Mark Tickets as Done", ucSetDone},
	}

	m := New()
	if len(m.items) != len(want)+1 {
		t.Fatalf("got %d menu items, want %d", len(m.items), len(want)+1)
	}
	for i, w := range want {
		if m.items[i].title != w.prefix {
			t.Errorf("item %d title = %q, want %q", i, m.items[i].title, w.prefix)
		}
		if m.items[i].uc != w.uc {
			t.Errorf("item %d uc = %v, want %v", i, m.items[i].uc, w.uc)
		}
	}
	if last := m.items[len(m.items)-1]; !last.isExit {
		t.Errorf("last item %q is not the Exit entry", last.title)
	}
}
