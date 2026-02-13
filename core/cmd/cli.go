package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mindsgn-studio/intunja/core/engine"
)

// View types
type viewType int

const (
	viewMain viewType = iota
	viewTorrentDetails
	viewSettings
	viewAddTorrent
)

// Model represents the CLI application state
type Model struct {
	// Engine
	engine *engine.Engine

	// UI State
	currentView viewType
	width       int
	height      int

	// Torrent list
	torrents    map[string]*engine.Torrent
	selectedIdx int
	torrentKeys []string // Ordered list of info hashes

	// Components
	mainTable   table.Model
	progressBar progress.Model
	textInput   textinput.Model

	// Input state
	inputMode   bool
	inputPrompt string

	// Error/success messages
	statusMsg   string
	statusStyle lipgloss.Style

	// Styles
	styles Styles
}

// Styles contains lipgloss styles
type Styles struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	StatusBar lipgloss.Style
	Help      lipgloss.Style
	Input     lipgloss.Style
	Error     lipgloss.Style
	Success   lipgloss.Style
}

func defaultStyles() Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D9FF")).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#00D9FF")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginTop(1),
		Input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00D9FF")).
			Padding(0, 1),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true),
	}
}

// NewModel creates a new CLI model
func NewModel(e *engine.Engine) Model {
	// Create table
	columns := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "Progress", Width: 10},
		{Title: "Size", Width: 12},
		{Title: "Down", Width: 12},
		{Title: "Status", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Style table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#00D9FF")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00D9FF")).
		Bold(false)
	t.SetStyles(s)

	// Create progress bar
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Enter text..."
	ti.CharLimit = 500
	ti.Width = 80

	return Model{
		engine:      e,
		currentView: viewMain,
		torrents:    make(map[string]*engine.Torrent),
		mainTable:   t,
		progressBar: prog,
		textInput:   ti,
		styles:      defaultStyles(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInputMode(msg)
		}
		return m.handleKeyPress(msg)

	case tickMsg:
		m.updateTorrentStats()
		return m, tickCmd()
	}

	// Update appropriate component based on mode
	if m.inputMode {
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	if m.currentView == viewMain {
		m.mainTable, cmd = m.mainTable.Update(msg)
		m.selectedIdx = m.mainTable.Cursor()
		return m, cmd
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.inputMode {
		return m.renderInputMode()
	}

	switch m.currentView {
	case viewMain:
		return m.renderMainView()
	case viewTorrentDetails:
		return m.renderDetailsView()
	case viewSettings:
		return m.renderSettingsView()
	default:
		return "Unknown view"
	}
}

// renderMainView renders the main torrent list
func (m Model) renderMainView() string {
	title := m.styles.Title.Render("🌊 Intunja - BitTorrent CLI Client")

	config := m.engine.Config()
	subtitle := m.styles.Subtitle.Render(fmt.Sprintf(
		"Active: %d torrents | Download Dir: %s | Port: %d",
		len(m.torrents),
		config.DownloadDirectory,
		config.IncomingPort,
	))

	// Update table rows - with nil checking
	rows := make([]table.Row, 0, len(m.torrentKeys))
	for _, key := range m.torrentKeys {
		t := m.torrents[key]
		if t == nil {
			// Skip nil torrents (can happen during deletion)
			continue
		}

		status := "Stopped"
		if t.Started {
			status = "Active"
		}
		if !t.Loaded {
			status = "Loading..."
		}

		rows = append(rows, table.Row{
			truncate(t.Name, 40),
			fmt.Sprintf("%.1f%%", t.Percent),
			formatBytes(t.Size),
			formatBytes(int64(t.DownloadRate)) + "/s",
			status,
		})
	}
	m.mainTable.SetRows(rows)

	tableView := m.mainTable.View()

	// Status message
	status := ""
	if m.statusMsg != "" {
		status = m.statusStyle.Render(m.statusMsg) + "\n"
	}

	help := m.styles.Help.Render(
		"[a] Add  [m] Magnet  [Enter] Details  [s] Start  [p] Pause  [d] Delete  [c] Config  [q] Quit",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		"",
		tableView,
		"",
		status,
		help,
	)
}

// renderDetailsView shows detailed info for selected torrent
func (m Model) renderDetailsView() string {
	if m.selectedIdx >= len(m.torrentKeys) {
		return "No torrent selected"
	}

	key := m.torrentKeys[m.selectedIdx]
	t := m.torrents[key]

	// Check if torrent still exists
	if t == nil {
		return m.styles.Error.Render("Torrent no longer exists\n\nPress [Esc] to go back")
	}

	title := m.styles.Title.Render("Torrent Details: " + t.Name)

	info := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("Info Hash: %s", t.InfoHash),
		fmt.Sprintf("Progress: %s %.1f%%", m.progressBar.ViewAs(float64(t.Percent)/100.0), t.Percent),
		fmt.Sprintf("Size: %s", formatBytes(t.Size)),
		fmt.Sprintf("Downloaded: %s", formatBytes(t.Downloaded)),
		fmt.Sprintf("Download Rate: %s/s", formatBytes(int64(t.DownloadRate))),
		fmt.Sprintf("Status: %s", map[bool]string{true: "Active", false: "Stopped"}[t.Started]),
		"",
		fmt.Sprintf("Files: %d", len(t.Files)),
	)

	// Show files if available
	if len(t.Files) > 0 && len(t.Files) <= 20 {
		info += "\n\nFiles:\n"
		for i, f := range t.Files {
			if i >= 10 {
				info += fmt.Sprintf("  ... and %d more files\n", len(t.Files)-10)
				break
			}
			if f != nil {
				info += fmt.Sprintf("  [%.0f%%] %s (%s)\n",
					f.Percent,
					filepath.Base(f.Path),
					formatBytes(f.Size))
			}
		}
	}

	help := m.styles.Help.Render("[esc] Back  [s] Start  [p] Pause  [d] Delete")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		"",
		help,
	)
}

// renderSettingsView shows configuration
func (m Model) renderSettingsView() string {
	title := m.styles.Title.Render("⚙️  Configuration")

	config := m.engine.Config()

	settings := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("Download Directory: %s", config.DownloadDirectory),
		fmt.Sprintf("Incoming Port: %d", config.IncomingPort),
		fmt.Sprintf("Upload Enabled: %t", config.EnableUpload),
		fmt.Sprintf("Seeding Enabled: %t", config.EnableSeeding),
		fmt.Sprintf("Auto Start: %t", config.AutoStart),
		fmt.Sprintf("Encryption: %s", map[bool]string{true: "Disabled", false: "Enabled"}[config.DisableEncryption]),
	)

	help := m.styles.Help.Render("[esc] Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		settings,
		"",
		help,
	)
}

// renderInputMode renders input prompt
func (m Model) renderInputMode() string {
	title := m.styles.Title.Render(m.inputPrompt)

	input := m.textInput.View()

	help := m.styles.Help.Render("[Enter] Submit  [Esc] Cancel  [Ctrl+U] Clear  [Ctrl+V] Paste")

	// Show status message if present
	status := ""
	if m.statusMsg != "" {
		status = "\n" + m.statusStyle.Render(m.statusMsg)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		input,
		status,
		"",
		help,
	)
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "a":
		// Add torrent file
		m.inputMode = true
		m.inputPrompt = "Enter .torrent file path:"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "/path/to/file.torrent"
		m.textInput.Focus()
		m.statusMsg = ""
		return m, textinput.Blink

	case "m":
		// Add magnet link
		m.inputMode = true
		m.inputPrompt = "Enter magnet URI:"
		m.textInput.SetValue("")
		m.textInput.Placeholder = "magnet:?xt=urn:btih:..."
		m.textInput.Focus()
		m.statusMsg = ""
		return m, textinput.Blink

	case "enter":
		if m.currentView == viewMain && len(m.torrentKeys) > 0 {
			m.currentView = viewTorrentDetails
		}
		return m, nil

	case "s":
		// Start torrent
		if m.selectedIdx < len(m.torrentKeys) {
			key := m.torrentKeys[m.selectedIdx]
			t := m.torrents[key]
			if t != nil {
				if err := m.engine.StartTorrent(key); err != nil {
					m.statusMsg = fmt.Sprintf("Error: %v", err)
					m.statusStyle = m.styles.Error
				} else {
					m.statusMsg = fmt.Sprintf("Started: %s", truncate(t.Name, 40))
					m.statusStyle = m.styles.Success
				}
			}
		}
		return m, nil

	case "p":
		// Pause torrent
		if m.selectedIdx < len(m.torrentKeys) {
			key := m.torrentKeys[m.selectedIdx]
			t := m.torrents[key]
			if t != nil {
				if err := m.engine.StopTorrent(key); err != nil {
					m.statusMsg = fmt.Sprintf("Error: %v", err)
					m.statusStyle = m.styles.Error
				} else {
					m.statusMsg = fmt.Sprintf("Paused: %s", truncate(t.Name, 40))
					m.statusStyle = m.styles.Success
				}
			}
		}
		return m, nil

	case "d":
		// Delete torrent
		if m.selectedIdx < len(m.torrentKeys) {
			key := m.torrentKeys[m.selectedIdx]
			t := m.torrents[key]
			if t != nil {
				torrentName := t.Name
				if err := m.engine.DeleteTorrent(key); err != nil {
					m.statusMsg = fmt.Sprintf("Error deleting torrent: %v", err)
					m.statusStyle = m.styles.Error
				} else {
					m.statusMsg = fmt.Sprintf("Deleted: %s", truncate(torrentName, 40))
					m.statusStyle = m.styles.Success

					// Adjust selection after deletion
					if m.selectedIdx >= len(m.torrentKeys)-1 && m.selectedIdx > 0 {
						m.selectedIdx--
					}
				}
			}
		}
		return m, nil

	case "c":
		m.currentView = viewSettings
		return m, nil

	case "esc":
		m.currentView = viewMain
		return m, nil
	}

	return m, nil
}

// handleInputMode processes input in input mode
func (m Model) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Process input
		value := strings.TrimSpace(m.textInput.Value())

		if value == "" {
			m.statusMsg = "Input cannot be empty"
			m.statusStyle = m.styles.Error
			return m, nil
		}

		m.inputMode = false
		m.textInput.Blur()

		if strings.Contains(m.inputPrompt, "magnet") {
			// Validate magnet link format
			if !strings.HasPrefix(value, "magnet:?") {
				m.statusMsg = "Invalid magnet link (must start with 'magnet:?')"
				m.statusStyle = m.styles.Error
				m.inputMode = true
				m.textInput.Focus()
				return m, textinput.Blink
			}

			// Add magnet link
			if err := m.engine.NewMagnet(value); err != nil {
				m.statusMsg = fmt.Sprintf("Error adding magnet: %v", err)
				m.statusStyle = m.styles.Error
				m.inputMode = true
				m.textInput.Focus()
				return m, textinput.Blink
			}

			m.statusMsg = "Magnet link added successfully!"
			m.statusStyle = m.styles.Success

		} else if strings.Contains(m.inputPrompt, "torrent") {
			// Validate file exists
			if _, err := os.Stat(value); os.IsNotExist(err) {
				m.statusMsg = fmt.Sprintf("File not found: %s", value)
				m.statusStyle = m.styles.Error
				m.inputMode = true
				m.textInput.Focus()
				return m, textinput.Blink
			}

			// Read and add torrent file
			// Note: This requires implementing torrent file reading
			m.statusMsg = "Torrent file support coming soon"
			m.statusStyle = m.styles.Error
		}

		return m, nil

	case tea.KeyEsc:
		m.inputMode = false
		m.textInput.Blur()
		m.statusMsg = ""
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	// Let textinput handle all other input (including paste)
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// updateTorrentStats updates torrent statistics
func (m *Model) updateTorrentStats() {
	m.torrents = m.engine.GetTorrents()

	// Update ordered keys - remove deleted torrents
	newKeys := make([]string, 0, len(m.torrents))
	for key := range m.torrents {
		newKeys = append(newKeys, key)
	}
	m.torrentKeys = newKeys

	// Adjust selection to stay in bounds
	if len(m.torrentKeys) == 0 {
		m.selectedIdx = 0
	} else if m.selectedIdx >= len(m.torrentKeys) {
		m.selectedIdx = len(m.torrentKeys) - 1
	}

	// Update table cursor position
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.torrentKeys) {
		m.mainTable.SetCursor(m.selectedIdx)
	}
}

// Messages
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Utility functions
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Run starts the CLI application
func Run(configPath string) error {
	// Initialize engine
	e := engine.New()

	// Configure engine
	config := engine.Config{
		AutoStart:         true,
		DisableEncryption: false,
		DownloadDirectory: "./downloads",
		EnableUpload:      true,
		EnableSeeding:     true,
		IncomingPort:      50007,
	}

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(config.DownloadDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	// Configure engine
	if err := e.Configure(config); err != nil {
		return fmt.Errorf("failed to configure engine: %w", err)
	}

	// Create and run TUI
	model := NewModel(e)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
