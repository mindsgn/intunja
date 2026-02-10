package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mindsgn-studio/intunja/engine"
)

// View types
type viewType int

const (
	viewMain viewType = iota
	viewDetails
	viewSettings
	viewSearch
)

// TorrentState represents the state of a torrent
type TorrentState struct {
	MetaInfo   *engine.MetaInfo
	Manager    *engine.DownloadManager
	Progress   float64
	Downloaded int64
	Uploaded   int64
	Peers      int
	Status     string // "downloading", "paused", "seeding", "stopped"
}

// Model is the main TUI model
type Model struct {
	// Current view
	currentView viewType

	// Torrents
	torrents    []*TorrentState
	selectedIdx int

	// Components
	mainTable   table.Model
	progressBar progress.Model

	// Window size
	width  int
	height int

	// Styles
	styles Styles
}

// Styles contains all lipgloss styles
type Styles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	StatusBar   lipgloss.Style
	ProgressBar lipgloss.Style
	Table       lipgloss.Style
	Selected    lipgloss.Style
	Help        lipgloss.Style
}

func defaultStyles() Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),
		ProgressBar: lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1),
		Table: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginTop(1),
	}
}

// NewModel creates a new TUI model
func NewModel() Model {
	// Create main table
	columns := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "Progress", Width: 12},
		{Title: "Down", Width: 12},
		{Title: "Up", Width: 12},
		{Title: "Peers", Width: 8},
		{Title: "Status", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(false)
	t.SetStyles(s)

	// Create progress bar
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return Model{
		currentView: viewMain,
		torrents:    make([]*TorrentState, 0),
		mainTable:   t,
		progressBar: prog,
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		m.updateTorrentStats()
		return m, tickCmd()

	case addTorrentMsg:
		return m.handleAddTorrent(msg)
	}

	// Update active component
	switch m.currentView {
	case viewMain:
		var cmd tea.Cmd
		m.mainTable, cmd = m.mainTable.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	switch m.currentView {
	case viewMain:
		return m.renderMainView()
	case viewDetails:
		return m.renderDetailsView()
	case viewSettings:
		return m.renderSettingsView()
	case viewSearch:
		return m.renderSearchView()
	}
	return ""
}

// renderMainView renders the main torrent list
func (m Model) renderMainView() string {
	title := m.styles.Title.Render("🌊 BitTorrent Client")
	subtitle := m.styles.Subtitle.Render(fmt.Sprintf("Active torrents: %d", len(m.torrents)))

	// Update table rows
	rows := make([]table.Row, len(m.torrents))
	for i, t := range m.torrents {
		rows[i] = table.Row{
			t.MetaInfo.Info.Name,
			fmt.Sprintf("%.1f%%", t.Progress*100),
			formatBytes(t.Downloaded),
			formatBytes(t.Uploaded),
			fmt.Sprintf("%d", t.Peers),
			t.Status,
		}
	}
	m.mainTable.SetRows(rows)

	tableView := m.styles.Table.Render(m.mainTable.View())

	help := m.styles.Help.Render(
		"[a] Add torrent  [d] Details  [p] Pause/Resume  [s] Settings  [q] Quit",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		"",
		tableView,
		help,
	)
}

// renderDetailsView shows detailed info for selected torrent
func (m Model) renderDetailsView() string {
	if m.selectedIdx >= len(m.torrents) {
		return "No torrent selected"
	}

	t := m.torrents[m.selectedIdx]

	title := m.styles.Title.Render(t.MetaInfo.Info.Name)

	info := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("Progress: %s", m.progressBar.ViewAs(t.Progress)),
		fmt.Sprintf("Downloaded: %s", formatBytes(t.Downloaded)),
		fmt.Sprintf("Uploaded: %s", formatBytes(t.Uploaded)),
		fmt.Sprintf("Peers: %d", t.Peers),
		fmt.Sprintf("Pieces: %d / %d", int(t.Progress*float64(t.MetaInfo.NumPieces())), t.MetaInfo.NumPieces()),
		fmt.Sprintf("Piece Size: %s", formatBytes(t.MetaInfo.Info.PieceLength)),
	)

	help := m.styles.Help.Render("[esc] Back  [p] Pause/Resume")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		"",
		help,
	)
}

// renderSettingsView shows configuration options
func (m Model) renderSettingsView() string {
	title := m.styles.Title.Render("⚙️  Settings")

	settings := lipgloss.JoinVertical(
		lipgloss.Left,
		"Download Directory: ~/Downloads",
		"Max Download Speed: Unlimited",
		"Max Upload Speed: Unlimited",
		"Max Peers: 50",
		"DHT: Enabled",
		"PEX: Enabled",
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

// renderSearchView shows torrent search interface
func (m Model) renderSearchView() string {
	title := m.styles.Title.Render("🔍 Search Torrents")

	help := m.styles.Help.Render("[esc] Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Search functionality coming soon...",
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
		// Add torrent (placeholder)
		return m, nil

	case "d":
		if m.currentView == viewMain {
			m.currentView = viewDetails
			m.selectedIdx = m.mainTable.Cursor()
		}
		return m, nil

	case "s":
		if m.currentView == viewMain {
			m.currentView = viewSettings
		}
		return m, nil

	case "esc":
		m.currentView = viewMain
		return m, nil

	case "p":
		// Toggle pause/resume
		if m.selectedIdx < len(m.torrents) {
			t := m.torrents[m.selectedIdx]
			if t.Status == "downloading" {
				t.Status = "paused"
			} else if t.Status == "paused" {
				t.Status = "downloading"
			}
		}
		return m, nil
	}

	return m, nil
}

// updateTorrentStats updates statistics for all torrents
func (m *Model) updateTorrentStats() {
	for _, t := range m.torrents {
		if t.Manager != nil {
			t.Progress = t.Manager.GetProgress()
			downloaded, uploaded, peers := t.Manager.GetStats()
			t.Downloaded = downloaded
			t.Uploaded = uploaded
			t.Peers = peers
		}
	}
}

// Messages
type tickMsg time.Time
type addTorrentMsg struct {
	metaInfo *engine.MetaInfo
	manager  *engine.DownloadManager
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) handleAddTorrent(msg addTorrentMsg) (tea.Model, tea.Cmd) {
	t := &TorrentState{
		MetaInfo: msg.metaInfo,
		Manager:  msg.manager,
		Status:   "downloading",
	}
	m.torrents = append(m.torrents, t)
	return m, nil
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
