package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	focusSearch = iota
	focusSidebar
	focusMain
	focusSong
)

var (
	// Colors
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	// Global Borders
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle)

	// Focused Border (Brighter)
	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(highlight)
)

// --- MODEL ---

type model struct {
	textInput    textinput.Model
	songs        []Song
	playlists    []Playlist
	playerStatus PlayerStatus

	// Navigation State
	focus      int
	cursorMain int
	cursorSide int
	mainOffset int

	// Window Dimensions
	width  int
	height int

	// App State
	err            error
	loading        bool
	playlistAmount int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Search songs..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return model{
		textInput:  ti,
		songs:      []Song{},
		focus:      focusSearch,
		cursorMain: 0,
		cursorSide: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		getPlaylists(),
		syncPlayerCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	// Handle Window Resize
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Key Presses
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		// Tab Cycles Focus: Search -> Sidebar -> Main -> Song -> Search
		case "tab":
			m.focus = (m.focus + 1) % 4
			if m.focus == focusSearch {
				m.textInput.Focus()
			} else {
				m.textInput.Blur()
			}

		case "enter":
			if m.focus == focusSearch {
				// Trigger Search
				query := m.textInput.Value()
				if query != "" {
					m.loading = true
					m.focus = focusMain
					m.textInput.Blur()
					return m, searchSongsCmd(query)
				}
			} else if m.focus == focusMain {
				// Play Song
				if len(m.songs) > 0 {
					selected := m.songs[m.cursorMain]
					go playSong(selected.ID)
				}
			} else if m.focus == focusSidebar {
				// Open playlist
				m.loading = true
				m.focus = focusMain
				return m, getPlaylistSongs((m.playlists[m.cursorSide]).ID)
			}
		case "up", "k": // Navigation up
			if m.focus == focusMain && m.cursorMain > 0 {
				m.cursorMain--
				if m.cursorMain < m.mainOffset {
					m.mainOffset = m.cursorMain
				}
			} else if m.focus == focusSidebar && m.cursorSide > 0 {
				m.cursorSide--
			}
		case "down", "j": // Navigation down
			if m.focus == focusMain && m.cursorMain < len(m.songs)-1 {
				m.cursorMain++

				// Height - Search(3) - Footer(6) - Margins(4) - TableHeader(2)
				visibleRows := m.height - 17

				if m.cursorMain >= m.mainOffset+visibleRows {
					m.mainOffset++
				}
			} else if m.focus == focusSidebar && m.cursorSide < len(m.playlists)-1 {
				m.cursorSide++
			}
		case "p": // Play/pause
			if m.focus != focusSearch {
				togglePause()
			}
		}

	case songsResultMsg:
		m.loading = false
		m.songs = msg.songs
		m.cursorMain = 0
		m.mainOffset = 0
		m.focus = focusMain
		m.textInput.Blur()

	case playlistResultMsg:
		m.playlists = msg.playlists

	case errMsg:
		m.loading = false
		m.err = msg.err

	case statusMsg:
		m.playerStatus = PlayerStatus(msg)
		return m, syncPlayerCmd()
	}

	// Update inputs
	if m.focus == focusSearch {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	// SIZING
	searchHeight := int(float64(m.height) * 0.035)
	mainHeight := int(float64(m.height) * 0.75)
	footerHeight := int(float64(m.height) * 0.104)

	sidebarWidth := int(float64(m.width) * 0.25)
	mainWidth := m.width - sidebarWidth - 4

	// SEARCH BAR
	searchBorder := borderStyle
	if m.focus == focusSearch {
		searchBorder = activeBorderStyle
	}

	searchView := searchBorder.Width(m.width - 2).Height(searchHeight).Render("Search: " + m.textInput.View())

	// SIZE BAR
	sideBorder := borderStyle
	if m.focus == focusSidebar {
		sideBorder = activeBorderStyle
	}

	sidebarContent := lipgloss.NewStyle().Bold(true).Render("  PLAYLISTS") + "\n\n"

	for i, item := range m.playlists {

		if i >= mainHeight-3 {
			break
		}

		cursor := "  "
		if m.cursorSide == i && m.focus == focusSidebar {
			cursor = "> "
		}

		style := lipgloss.NewStyle()
		if m.cursorSide == i && m.focus == focusSidebar {
			style = style.Foreground(highlight).Bold(true)
		}

		line := cursor + item.Name
		sidebarContent += style.Render(line) + "\n"
	}

	leftPane := sideBorder.
		Width(sidebarWidth).
		Height(mainHeight).
		Render(sidebarContent)

	// MAIN VIEW
	mainBorder := borderStyle
	if m.focus == focusMain {
		mainBorder = activeBorderStyle
	}

	mainContent := ""
	if m.loading {
		mainContent = "\n  Searching your library..."
	} else if len(m.songs) == 0 {
		mainContent = "\n  Use the search bar to find music."
	} else {
		availableWidth := mainWidth - 4
		colTitle := int(float64(availableWidth) * 0.40)
		colArtist := int(float64(availableWidth) * 0.15)
		colAlbum := int(float64(availableWidth) * 0.25)
		// Time takes whatever is left

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(subtle)
		header := fmt.Sprintf("  %-*s %-*s %-*s %s",
			colTitle, "TITLE",
			colArtist, "ARTIST",
			colAlbum, "ALBUM",
			"TIME")

		mainContent += headerStyle.Render(header) + "\n"
		mainContent += lipgloss.NewStyle().Foreground(subtle).Render("  "+strings.Repeat("-", mainWidth-4)) + "\n"

		headerHeight := 4
		visibleRows := mainHeight - headerHeight
		if visibleRows < 1 {
			visibleRows = 1
		}

		start := m.mainOffset

		end := start + visibleRows
		if end >= len(m.songs) {
			end = len(m.songs)
		}

		for i := start; i <= end; i++ {
			if i >= len(m.songs) {
				break
			}

			song := m.songs[i]

			cursor := "  "
			style := lipgloss.NewStyle()

			if m.cursorMain == i {
				cursor = "> "
				if m.focus == focusMain {
					style = style.Foreground(highlight).Bold(true)
				} else {
					style = style.Foreground(subtle)
				}
			}

			trunc := func(s string, w int) string {
				if w <= 1 {
					return ""
				}
				if len(s) > w {
					return s[:w-1] + "…"
				}
				return s
			}

			row := fmt.Sprintf("%-*s %-*s %-*s %s",
				colTitle, trunc(song.Title, colTitle),
				colArtist, trunc(song.Artist, colArtist),
				colAlbum, trunc(song.Album, colAlbum),
				formatDuration(song.Duration),
			)

			mainContent += fmt.Sprintf("%s%s\n", cursor, style.Render(row))
		}
	}

	rightPane := mainBorder.
		Width(mainWidth).
		Height(mainHeight).
		Render(mainContent)

	// Join sidebar and main view
	centerView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	songBorder := borderStyle
	if m.focus == focusSong {
		songBorder = activeBorderStyle
	}

	// FOOTER
	title := ""
	artist := ""

	if m.playerStatus.Title == "" {
		title = "Not Playing"
	} else {
		title = m.playerStatus.Title
		artist = m.playerStatus.Artist + " - " + m.playerStatus.Album
	}

	barWidth := m.width - 20
	if barWidth < 10 {
		barWidth = 10
	}

	percent := 0.0
	if m.playerStatus.Duration > 0 {
		percent = m.playerStatus.Current / m.playerStatus.Duration
	}
	filledChars := int(percent * float64(barWidth))
	if filledChars > barWidth {
		filledChars = barWidth
	}

	barStr := ""
	if filledChars > 0 {
		barStr = strings.Repeat("=", filledChars-1) + ">"
	}
	emptyChars := barWidth - filledChars
	if emptyChars > 0 {
		barStr += strings.Repeat("-", emptyChars)
	}

	currStr := formatDuration(int(m.playerStatus.Current))
	durStr := formatDuration(int(m.playerStatus.Duration))

	rowTitle := lipgloss.NewStyle().Bold(true).Foreground(highlight).Render("  " + title)
	rowArtist := lipgloss.NewStyle().Foreground(subtle).Render("  " + artist)
	rowProgress := fmt.Sprintf("  %s %s %s",
		currStr,
		lipgloss.NewStyle().Foreground(special).Render("["+barStr+"]"),
		durStr,
	)

	footerContent := fmt.Sprintf("%s\n%s\n\n%s", rowTitle, rowArtist, rowProgress)

	footerView := songBorder.
		Width(m.width - 2).
		Height(footerHeight).
		Render(footerContent)

	// COMBINE ALL VERTICALLY
	return lipgloss.JoinVertical(lipgloss.Left,
		searchView,
		centerView,
		footerView,
	)
}

type songsResultMsg struct {
	songs []Song
}

type playlistResultMsg struct {
	playlists []Playlist
}

type errMsg struct {
	err error
}

type statusMsg PlayerStatus

func searchSongsCmd(query string) tea.Cmd {
	return func() tea.Msg {
		songs, err := subsonicSearchSong(query, 0)
		if err != nil {
			return errMsg{err}
		}
		return songsResultMsg{songs}
	}
}

func getPlaylists() tea.Cmd {
	return func() tea.Msg {
		playlists, err := subsonicGetPlaylists()
		if err != nil {
			return errMsg{err}
		}
		return playlistResultMsg{playlists}
	}
}

func getPlaylistSongs(id string) tea.Cmd {
	return func() tea.Msg {
		songs, err := subsonicGetPlaylistSongs(id)
		if err != nil {
			return errMsg{err}
		}
		return songsResultMsg{songs}
	}
}

func syncPlayerCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return statusMsg(getPlayerStatus())
	})
}
