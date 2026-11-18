package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type step int

const (
	stepArchetype step = iota
	stepProject
	stepImage
	stepSpecs
	stepNetwork
	stepCustomNodes
	stepReview
	stepDone
)

// Model represents the interactive Bubble Tea state.
type Model struct {
	step        step
	styles      Styles
	archetypes  []Archetype
	images      []OSImage
	cursor      int
	imageCursor int
	textInput   textinput.Model
	answers     Answers
	yamlPreview string
	err         error
	quitting    bool
	writtenFile string
}

// NewModel initializes the wizard state with defaults.
func NewModel(initialProject string) Model {
	if initialProject == "" {
		initialProject = "errant-cluster"
	}

	ti := textinput.New()
	ti.Placeholder = initialProject
	ti.SetValue(initialProject)
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40

	return Model{
		step:        stepArchetype,
		styles:      DefaultStyles(),
		archetypes:  DefaultArchetypes(),
		images:      DefaultOSImages(),
		cursor:      0,
		imageCursor: 0,
		textInput:   ti,
		answers:     NewDefaultAnswers(initialProject),
	}
}

// Init implements tea.Model.
func (Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles tea messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.quitting = true

			return m, tea.Quit
		case "esc":
			if m.step > stepArchetype && m.step < stepDone {
				m.step--

				return m, nil
			}
		default:
		}
	}

	return m.dispatchStep(msg)
}

func (m Model) dispatchStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepArchetype:
		return m.updateArchetype(msg)
	case stepProject:
		return m.updateProject(msg)
	case stepImage:
		return m.updateImage(msg)
	case stepSpecs:
		return m.updateSpecs(msg)
	case stepNetwork:
		return m.updateNetwork(msg)
	case stepCustomNodes:
		return m.updateCustomNodes(msg)
	case stepReview:
		return m.updateReview(msg)
	case stepDone:
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	default:
	}

	return m, nil
}

func (m Model) updateArchetype(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.archetypes)-1 {
			m.cursor++
		}
	case "enter":
		m.answers.ArchetypeID = m.archetypes[m.cursor].ID
		m.step = stepProject
		m.textInput.SetValue(m.answers.ProjectName)
		m.textInput.Focus()
	default:
	}

	return m, nil
}

func (m Model) updateProject(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			m.answers.ProjectName = val
		}

		m.step = stepImage

		return m, nil
	}

	var cmd tea.Cmd

	m.textInput, cmd = m.textInput.Update(msg)

	return m, cmd
}

func (m Model) updateImage(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.imageCursor > 0 {
			m.imageCursor--
		}
	case "down", "j":
		if m.imageCursor < len(m.images)-1 {
			m.imageCursor++
		}
	case "enter":
		m.answers.ImageIndex = m.imageCursor
		m.step = stepSpecs
		m.textInput.SetValue(strconv.Itoa(m.answers.MemoryMB))
		m.textInput.Placeholder = "2048"
	default:
	}

	return m, nil
}

func (m Model) updateSpecs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if mem, err := strconv.Atoi(val); err == nil && mem > 0 {
			m.answers.MemoryMB = mem
		}

		m.step = stepNetwork
		m.textInput.SetValue(m.answers.Subnet)
		m.textInput.Placeholder = "192.168.56.0/24"

		return m, nil
	}

	var cmd tea.Cmd

	m.textInput, cmd = m.textInput.Update(msg)

	return m, cmd
}

func (m Model) updateNetwork(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			m.answers.Subnet = val
		}

		if m.answers.ArchetypeID == "custom" {
			m.step = stepCustomNodes
			m.textInput.SetValue(strconv.Itoa(m.answers.WorkerNodes))
			m.textInput.Placeholder = "2"

			return m, nil
		}

		return m.prepareReview()
	}

	var cmd tea.Cmd

	m.textInput, cmd = m.textInput.Update(msg)

	return m, cmd
}

func (m Model) updateCustomNodes(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if count, err := strconv.Atoi(val); err == nil && count >= 0 {
			m.answers.WorkerNodes = count
		}

		return m.prepareReview()
	}

	var cmd tea.Cmd

	m.textInput, cmd = m.textInput.Update(msg)

	return m, cmd
}

func (m Model) prepareReview() (tea.Model, tea.Cmd) {
	yamlBytes, err := RenderYAML(m.answers)
	if err != nil {
		m.err = err

		return m, nil
	}

	m.yamlPreview = string(yamlBytes)
	m.step = stepReview

	return m, nil
}

func (m Model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "enter", "y":
		written, err := WriteErrantfile(m.answers)
		if err != nil {
			m.err = err

			return m, nil
		}

		m.writtenFile = written
		m.step = stepDone
	case "q":
		m.quitting = true

		return m, tea.Quit
	default:
	}

	return m, nil
}

// View renders the terminal output.
func (m Model) View() string {
	if m.quitting {
		return "\n  Wizard cancelled. No files were written.\n\n"
	}

	var b strings.Builder

	b.WriteString(m.styles.Header.Render(" ✦ ERRANT TOPOLOGY WIZARD ✦ "))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtitle.Render("  Declarative multi-node VM topology generator for KVM / Libvirt"))
	b.WriteString("\n\n")

	if m.err != nil {
		fmt.Fprintf(&b, "  Error: %v\n\n", m.err)
	}

	switch m.step {
	case stepArchetype:
		b.WriteString(m.viewArchetype())
	case stepProject:
		b.WriteString(m.viewProject())
	case stepImage:
		b.WriteString(m.viewImage())
	case stepSpecs:
		b.WriteString(m.viewSpecs())
	case stepNetwork:
		b.WriteString(m.viewNetwork())
	case stepCustomNodes:
		b.WriteString(m.viewCustomNodes())
	case stepReview:
		b.WriteString(m.viewReview())
	case stepDone:
		b.WriteString(m.viewDone())
	default:
	}

	return b.String()
}

func (m Model) viewArchetype() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Step 1 of 5"))
	b.WriteString(m.styles.Title.Render("Select Topology Archetype"))
	b.WriteString("\n\n")

	for i, arch := range m.archetypes {
		cursor := "  "
		style := m.styles.Item

		if i == m.cursor {
			cursor = "▸ "
			style = m.styles.SelectedItem
		}

		fmt.Fprintf(&b, "%s%s\n", cursor, style.Render(arch.Title))
		fmt.Fprintf(&b, "    %s\n\n", m.styles.ItemDesc.Render(arch.Description))
	}

	b.WriteString(m.styles.Help.Render("  ↑/↓: navigate • enter: select • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewProject() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Step 2 of 5"))
	b.WriteString(m.styles.Title.Render("Enter Project Identifier"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.styles.InputPrompt.Render("Project Name: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("  enter: next • esc: back • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewImage() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Step 3 of 5"))
	b.WriteString(m.styles.Title.Render("Select Base Cloud OS Image"))
	b.WriteString("\n\n")

	for i, img := range m.images {
		cursor := "  "
		style := m.styles.Item

		if i == m.imageCursor {
			cursor = "▸ "
			style = m.styles.SelectedItem
		}

		fmt.Fprintf(&b, "%s%s\n", cursor, style.Render(img.Name))
		fmt.Fprintf(&b, "    %s\n\n", m.styles.ItemDesc.Render(img.URL))
	}

	b.WriteString(m.styles.Help.Render("  ↑/↓: navigate • enter: select • esc: back • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewSpecs() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Step 4 of 5"))
	b.WriteString(m.styles.Title.Render("Default Node Memory Sizing"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.styles.InputPrompt.Render("Memory (MB): "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("  enter: next • esc: back • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewNetwork() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Step 5 of 5"))
	b.WriteString(m.styles.Title.Render("Private Subnet Network"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.styles.InputPrompt.Render("CIDR Subnet: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("  enter: preview configuration • esc: back • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewCustomNodes() string {
	var b strings.Builder

	b.WriteString(m.styles.StepBadge.Render("Extra Step"))
	b.WriteString(m.styles.Title.Render("Custom Topology Node Counts"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.styles.InputPrompt.Render("Worker Node Count: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("  enter: preview configuration • esc: back • ctrl+c: quit\n"))

	return b.String()
}

func (m Model) viewReview() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render("Review Generated Errantfile.yaml"))
	b.WriteString("\n")
	b.WriteString(m.styles.PreviewBox.Render(m.yamlPreview))
	b.WriteString("\n")
	b.WriteString(m.styles.SuccessTitle.Render("  Target output: " + m.answers.OutputPath))
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("  enter / 'y': write file • 'q': cancel • esc: back\n"))

	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder

	b.WriteString(m.styles.SuccessTitle.Render("  ✔ Errantfile.yaml successfully generated!"))
	b.WriteString("\n\n")

	msg := fmt.Sprintf(`Saved to: %s

Next Steps:
  1. Validate configuration:
     $ errant validate

  2. Spin up cluster topology:
     $ errant up

  3. Connect to a node:
     $ errant ssh <node-name>
`, m.writtenFile)

	b.WriteString(m.styles.SuccessMessage.Render(msg))
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("  Press any key to exit.\n"))

	return b.String()
}
