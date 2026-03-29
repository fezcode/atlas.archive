package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mholt/archiver/v3"
)

// ── Application State ───────────────────────────────────────────────────────

type AppStep int

const (
	StepSelectAction AppStep = iota
	StepSelectFiles
	StepInputName
	StepSelectFormat
	StepConfirm
	StepProcess
	StepDone
)

type AppAction string

const (
	ActionExtract AppAction = "Extract"
	ActionArchive AppAction = "Archive"
)

// ── Model ───────────────────────────────────────────────────────────────────

type model struct {
	step AppStep

	// Action Selection
	action       AppAction
	actionCursor int
	actions      []AppAction

	// File Browser
	cwd          string
	fileEntries  []os.DirEntry
	fileCursor   int
	selectedDirs map[string]bool
	extractFile  string

	// General options
	archiveName string
	nameInput   textinput.Model

	formatCursor int
	formats      []string

	// Processing
	processingMsg string
	err           error
	
	width  int
	height int
}

func initialModel() model {
	cwd, _ := os.Getwd()

	ti := textinput.New()
	ti.Placeholder = "my_archive"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	m := model{
		step:         StepSelectAction,
		actions:      []AppAction{ActionExtract, ActionArchive},
		actionCursor: 0,
		cwd:          cwd,
		selectedDirs: make(map[string]bool),
		nameInput:    ti,
		formats:      []string{".zip", ".tar.gz", ".tar", ".tar.bz2", ".tar.xz"},
		formatCursor: 0,
	}
	m.loadFileEntries()
	return m
}

func (m *model) loadFileEntries() {
	entries, err := os.ReadDir(m.cwd)
	if err != nil {
		return
	}

	// Sort: directories first, then alphabetical
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() && !entries[j].IsDir() {
			return true
		}
		if !entries[i].IsDir() && entries[j].IsDir() {
			return false
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	
	m.fileEntries = entries
	m.fileCursor = 0
}

// ── Update ──────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

type processFinishedMsg struct {
	err error
}

func startProcessing(m model) tea.Cmd {
	return func() tea.Msg {
		if m.action == ActionArchive {
			var filesToArchive []string
			for path := range m.selectedDirs {
				filesToArchive = append(filesToArchive, path)
			}
			outPath := filepath.Join(m.cwd, m.archiveName+m.formats[m.formatCursor])
			err := archiver.Archive(filesToArchive, outPath)
			return processFinishedMsg{err: err}
		} else {
			// Extract
			outDir := strings.TrimSuffix(filepath.Base(m.extractFile), filepath.Ext(m.extractFile))
			if outDir == filepath.Base(m.extractFile) {
				outDir = outDir + "_extracted" // fallback
			}
			outPath := filepath.Join(m.cwd, outDir)
			// create dir if not exists
			os.MkdirAll(outPath, os.ModePerm)
			err := archiver.Unarchive(m.extractFile, outPath)
			return processFinishedMsg{err: err}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.step == StepDone {
			return m, tea.Quit
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.step == StepSelectFiles {
				m.step = StepSelectAction
				return m, nil
			} else if m.step == StepInputName {
				m.step = StepSelectFiles
				return m, nil
			} else if m.step == StepSelectFormat {
				m.step = StepInputName
				return m, nil
			}
		}
	case processFinishedMsg:
		m.err = msg.err
		m.step = StepDone
		return m, nil
	}

	switch m.step {
	case StepSelectAction:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.actionCursor > 0 {
					m.actionCursor--
				}
			case "down", "j":
				if m.actionCursor < len(m.actions)-1 {
					m.actionCursor++
				}
			case "enter":
				m.action = m.actions[m.actionCursor]
				m.step = StepSelectFiles
			}
		}

	case StepSelectFiles:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.fileCursor > 0 {
					m.fileCursor--
				}
			case "down", "j":
				if m.fileCursor < len(m.fileEntries)-1 {
					m.fileCursor++
				}
			case "left", "h", "backspace":
				parent := filepath.Dir(m.cwd)
				if parent != m.cwd {
					m.cwd = parent
					m.loadFileEntries()
				}
			case "right", "l":
				if len(m.fileEntries) > 0 && m.fileCursor < len(m.fileEntries) && m.fileEntries[m.fileCursor].IsDir() {
					m.cwd = filepath.Join(m.cwd, m.fileEntries[m.fileCursor].Name())
					m.loadFileEntries()
				}
			case " ":
				if m.action == ActionArchive && len(m.fileEntries) > 0 {
					p := filepath.Join(m.cwd, m.fileEntries[m.fileCursor].Name())
					if m.selectedDirs[p] {
						delete(m.selectedDirs, p)
					} else {
						m.selectedDirs[p] = true
					}
				}
			case "enter":
				if m.action == ActionExtract {
					if len(m.fileEntries) > 0 && !m.fileEntries[m.fileCursor].IsDir() {
						m.extractFile = filepath.Join(m.cwd, m.fileEntries[m.fileCursor].Name())
						m.step = StepConfirm
					}
				} else {
					if len(m.selectedDirs) > 0 {
						m.step = StepInputName
					} else if len(m.fileEntries) > 0 {
						// Auto-select focused item if none selected
						p := filepath.Join(m.cwd, m.fileEntries[m.fileCursor].Name())
						m.selectedDirs[p] = true
						m.step = StepInputName
					}
				}
			}
		}

	case StepInputName:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				val := strings.TrimSpace(m.nameInput.Value())
				if val == "" {
					val = "archive"
				}
				m.archiveName = val
				m.step = StepSelectFormat
			}
		}
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd

	case StepSelectFormat:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.formatCursor > 0 {
					m.formatCursor--
				}
			case "down", "j":
				if m.formatCursor < len(m.formats)-1 {
					m.formatCursor++
				}
			case "enter":
				m.step = StepConfirm
			}
		}

	case StepConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				m.step = StepProcess
				m.processingMsg = "Working..."
				return m, startProcessing(m)
			case "n", "esc":
				if m.action == ActionExtract {
					m.step = StepSelectFiles
				} else {
					m.step = StepSelectFormat
				}
				return m, nil
			}
		}
	}

	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (m model) View() string {
	s := strings.Builder{}
	s.WriteString(banner() + "\n\n")

	// Step indicator
	steps := []string{"Action", "Files", "Name", "Format", "Confirm", "Finish"}
	if m.action == ActionExtract {
		steps = []string{"Action", "Files", "Confirm", "Finish"}
	}
	
	// Determine current logic step index
	currentIdx := 0
	switch m.step {
	case StepSelectAction: currentIdx = 0
	case StepSelectFiles: currentIdx = 1
	case StepInputName: currentIdx = 2
	case StepSelectFormat: currentIdx = 3
	case StepConfirm: 
		if m.action == ActionExtract {
			currentIdx = 2
		} else {
			currentIdx = 4
		}
	case StepProcess, StepDone:
		if m.action == ActionExtract {
			currentIdx = 3
		} else {
			currentIdx = 5
		}
	}

	var stepStrs []string
	for i, stepName := range steps {
		if i == currentIdx {
			stepStrs = append(stepStrs, activeStepStyle.Render("● "+stepName))
		} else if i < currentIdx {
			stepStrs = append(stepStrs, completedStepStyle.Render("✓ "+stepName))
		} else {
			stepStrs = append(stepStrs, inactiveStepStyle.Render("○ "+stepName))
		}
	}
	s.WriteString(stepIndicatorStyle.Render("  " + strings.Join(stepStrs, mutedStyle.Render("  ›  "))))
	s.WriteString("\n\n")

	switch m.step {
	case StepSelectAction:
		s.WriteString(titleStyle.Render("What would you like to do?"))
		s.WriteString("\n\n")
		for i, action := range m.actions {
			cursor := " "
			style := normalItemStyle
			if m.actionCursor == i {
				cursor = "❯"
				style = selectedItemStyle
			}
			s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, action)) + "\n")
		}
		s.WriteString(helpStyle.Render("\n↑/k: up • ↓/j: down • enter: select • ctrl+c: quit"))

	case StepSelectFiles:
		s.WriteString(titleStyle.Render("Select file(s) in " + m.cwd))
		s.WriteString("\n\n")
		
		// Rendering files
		// We should only show a window of files to prevent overflowing, but for simplicity let's just render 10 items.
		start := 0
		if m.fileCursor > 5 {
			start = m.fileCursor - 5
		}
		end := start + 10
		if end > len(m.fileEntries) {
			end = len(m.fileEntries)
		}

		if len(m.fileEntries) == 0 {
			s.WriteString(mutedStyle.Render("  (empty directory)\n"))
		} else {
			if start > 0 {
				s.WriteString(mutedStyle.Render("  ↑ ...\n"))
			}
			for i := start; i < end; i++ {
				entry := m.fileEntries[i]
				cursor := " "
				style := normalItemStyle
				
				if m.fileCursor == i {
					cursor = "❯"
					style = selectedItemStyle
				}

				nameStr := entry.Name()
				if entry.IsDir() {
					nameStr += "/"
				}

				checkbox := ""
				if m.action == ActionArchive {
					p := filepath.Join(m.cwd, entry.Name())
					if m.selectedDirs[p] {
						checkbox = checkedStyle.Render("☑") + " "
					} else {
						checkbox = checkboxStyle.Render("☐") + " "
					}
				}

				s.WriteString(style.Render(fmt.Sprintf("%s %s%s", cursor, checkbox, nameStr)) + "\n")
			}
			if end < len(m.fileEntries) {
				s.WriteString(mutedStyle.Render("  ↓ ...\n"))
			}
		}

		help := "\n↑/k: up • ↓/j: down • ←/h: parent dir • →/l: enter dir\n"
		if m.action == ActionArchive {
			help += "space: toggle selection • enter: confirm • ctrl+c: quit"
		} else {
			help += "enter: extract selected • ctrl+c: quit"
		}
		s.WriteString(helpStyle.Render(help))

	case StepInputName:
		s.WriteString(titleStyle.Render("Archive Name (without extension)"))
		s.WriteString("\n\n")
		s.WriteString("  " + m.nameInput.View() + "\n")
		s.WriteString(helpStyle.Render("\nenter: confirm • ctrl+c: quit"))

	case StepSelectFormat:
		s.WriteString(titleStyle.Render("Select Compression Format"))
		s.WriteString("\n\n")
		for i, format := range m.formats {
			cursor := " "
			style := normalItemStyle
			if i == m.formatCursor {
				cursor = "❯"
				style = selectedItemStyle
			}
			s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, format)) + "\n")
		}
		s.WriteString(helpStyle.Render("\n↑/k: up • ↓/j: down • enter: select • ctrl+c: quit"))

	case StepConfirm:
		s.WriteString(titleStyle.Render("Ready to " + string(m.action)))
		s.WriteString("\n\n")

		if m.action == ActionArchive {
			s.WriteString(fmt.Sprintf("  %s %s%s\n", keyStyle.Render("Archive Name:"), valueStyle.Render(m.archiveName), valueStyle.Render(m.formats[m.formatCursor])))
			s.WriteString(fmt.Sprintf("  %s %d\n", keyStyle.Render("Files to archive:"), len(m.selectedDirs)))
		} else {
			s.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("File to extract:"), valueStyle.Render(filepath.Base(m.extractFile))))
			outDir := strings.TrimSuffix(filepath.Base(m.extractFile), filepath.Ext(m.extractFile))
			if outDir == filepath.Base(m.extractFile) {
				outDir = outDir + "_extracted" // fallback
			}
			s.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Destination path:"), valueStyle.Render(filepath.Join(m.cwd, outDir))))
		}

		s.WriteString(confirmPanelStyle.Render("Proceed? (y/n)"))
		s.WriteString(helpStyle.Render("\ny: yes • n/esc: cancel • ctrl+c: quit"))

	case StepProcess:
		s.WriteString(titleStyle.Render("Processing..."))
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("  %s\n", m.processingMsg))

	case StepDone:
		if m.err != nil {
			s.WriteString(dangerStyle.Render("❌ Error: " + m.err.Error()))
		} else {
			s.WriteString(successStyle.Render("✅ operation completed successfully."))
		}
		s.WriteString("\n\nPress any key to exit.")
	}

	return appStyle.Render(s.String())
}

// Run starts the TUI application.
func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
