package wizard

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sendKey(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()

	res, _ := m.Update(msg)

	mod, ok := res.(Model)
	if !ok {
		t.Fatal("expected Model from Update")
	}

	return mod
}

func TestModelInteractiveSteps(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "Errantfile.yaml")

	m := NewModel("")
	m.answers.OutputPath = outputPath

	if cmd := m.Init(); cmd == nil {
		t.Error("expected non-nil Init command")
	}

	m = advanceArchetypeAndProject(t, m)
	m = advanceImageAndSpecs(t, m)
	m = advanceNetworkAndReview(t, m)

	view := m.View()
	if !strings.Contains(view, "successfully generated") {
		t.Errorf("expected done view, got: %s", view)
	}

	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected tea.Quit command on done view exit")
	}

	if _, ok := res.(Model); !ok {
		t.Error("expected Model")
	}
}

func advanceArchetypeAndProject(t *testing.T, m Model) Model {
	t.Helper()

	// 1. Archetype selection
	view := m.View()
	if !strings.Contains(view, "Select Topology Archetype") {
		t.Errorf("expected archetype view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != stepProject {
		t.Fatalf("expected stepProject, got %d", m.step)
	}

	// 2. Project name input
	view = m.View()
	if !strings.Contains(view, "Enter Project Identifier") {
		t.Errorf("expected project view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepImage {
		t.Fatalf("expected stepImage, got %d", m.step)
	}

	return m
}

func advanceImageAndSpecs(t *testing.T, m Model) Model {
	t.Helper()

	// 3. Image selection
	view := m.View()
	if !strings.Contains(view, "Select Base Cloud OS Image") {
		t.Errorf("expected image view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != stepSpecs {
		t.Fatalf("expected stepSpecs, got %d", m.step)
	}

	// 4. Specs input
	view = m.View()
	if !strings.Contains(view, "Default Node Memory Sizing") {
		t.Errorf("expected specs view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepNetwork {
		t.Fatalf("expected stepNetwork, got %d", m.step)
	}

	return m
}

func advanceNetworkAndReview(t *testing.T, m Model) Model {
	t.Helper()

	// 5. Network input
	view := m.View()
	if !strings.Contains(view, "Private Subnet Network") {
		t.Errorf("expected network view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepReview {
		t.Fatalf("expected stepReview, got %d", m.step)
	}

	// 6. Review & Save
	view = m.View()
	if !strings.Contains(view, "Review Generated Errantfile.yaml") {
		t.Errorf("expected review view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepDone {
		t.Fatalf("expected stepDone, got %d", m.step)
	}

	return m
}

func TestModelCustomFlowAndCancel(t *testing.T) {
	m := NewModel("custom-cluster")

	// Select Custom (archetype index 3)
	for i := 0; i < 3; i++ {
		m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	// StepProject -> StepImage -> StepSpecs -> StepNetwork
	for i := 0; i < 4; i++ {
		m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}

	if m.step != stepCustomNodes {
		t.Fatalf("expected stepCustomNodes, got %d", m.step)
	}

	view := m.View()
	if !strings.Contains(view, "Custom Topology Node Counts") {
		t.Errorf("expected custom nodes view, got: %s", view)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepReview {
		t.Fatalf("expected stepReview, got %d", m.step)
	}

	// Quit with 'q'
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	mod, ok := newM.(Model)
	if !ok || cmd == nil || !mod.quitting {
		t.Error("expected quitting true and quit command")
	}

	view = mod.View()
	if !strings.Contains(view, "Wizard cancelled") {
		t.Errorf("expected cancelled view, got: %s", view)
	}
}

func TestModelEscAndCtrlC(t *testing.T) {
	m := NewModel("esc-test")

	// Advance to Step 2
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepProject {
		t.Fatalf("expected stepProject, got %d", m.step)
	}

	// Esc should go back to Step 1
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if m.step != stepArchetype {
		t.Fatalf("expected stepArchetype after esc, got %d", m.step)
	}

	// Ctrl+C should quit
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	mod, ok := newM.(Model)
	if !ok || cmd == nil || !mod.quitting {
		t.Error("expected quitting true on ctrl+c")
	}
}

func TestStylesAndDefinitions(t *testing.T) {
	styles := DefaultStyles()
	if styles.Header.GetPaddingLeft() == 0 {
		t.Error("unexpected uninitialized styles")
	}

	archetypes := DefaultArchetypes()
	if len(archetypes) == 0 {
		t.Error("archetypes list is empty")
	}

	images := DefaultOSImages()
	if len(images) == 0 {
		t.Error("images list is empty")
	}
}

func TestWizardEdgeCases(t *testing.T) {
	m := NewModel("edge-test")
	m.err = errors.New("simulated error")

	if !strings.Contains(m.View(), "simulated error") {
		t.Error("expected error message rendered in View")
	}

	// Boundary at top of archetypes
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	// WriteErrantfile error path
	ans := NewDefaultAnswers("bad-path")
	ans.OutputPath = "/dev/null/cannot/create/Errantfile.yaml"

	if _, err := WriteErrantfile(ans); err == nil {
		t.Error("expected error writing to invalid path")
	}
}
