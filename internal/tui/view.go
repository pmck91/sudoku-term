package tui

import (
	"fmt"
	"strings"
	"time"

	"sudoku-term/internal/puzzle"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type palette struct{ text, muted, line, accent, peer, match, selected, danger, hint string }

func (m *Model) colors() palette {
	if !m.dark {
		return palette{"#243344", "#586978", "#738595", "#006D70", "#EDF3F4", "#D5E8E9", "#B8DDDF", "#AA2038", "#F7E9B7"}
	}
	return palette{"#DFE7EF", "#8899AD", "#4C6073", "#70D6CE", "#192631", "#25454D", "#195A63", "#FF8E9A", "#51452D"}
}
func (m *Model) style(color string) lipgloss.Style {
	s := lipgloss.NewStyle()
	if !m.plain {
		s = s.Foreground(lipgloss.Color(color))
	}
	return s
}
func (m *Model) text(s string) string   { return m.style(m.colors().text).Render(s) }
func (m *Model) muted(s string) string  { return m.style(m.colors().muted).Render(s) }
func (m *Model) accent(s string) string { return m.style(m.colors().accent).Bold(true).Render(s) }
func duration(t time.Duration) string {
	n := int(t.Seconds())
	if n < 0 {
		n = 0
	}
	if n >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", n/3600, n/60%60, n%60)
	}
	return fmt.Sprintf("%02d:%02d", n/60, n%60)
}

func (m *Model) View() tea.View {
	var content string
	switch {
	case m.width < 80 || m.height < 24:
		content = m.accent("SUDOKU") + "\n\n" + fmt.Sprintf("Resize to at least 80 × 24\nCurrent size: %d × %d", m.width, m.height) + "\n\n" + m.muted("Play is paused · q quit")
	case m.errorSave != nil:
		content = m.card("Couldn't save your game", m.text("Your latest changes are still in memory.")+"\n\n"+m.wrap(m.errorSave.Error(), 54)+"\n\n"+m.accent("r  Retry saving")+"\n\n"+m.text("q  Quit without saving pending changes"))
	case m.quitting:
		content = m.accent("Saving your game…")
	case m.screen == playing || m.screen == hint:
		content = m.playView()
	case m.screen == help:
		content = m.helpView()
	case m.screen == generating:
		frames := []string{"◐", "◓", "◑", "◒"}
		heading := fmt.Sprintf("%s  Finding a %s puzzle", frames[(m.frame/2)%len(frames)], m.selectedLevel)
		body := "Checking one solution and a logical solving path.\n\nTake a breath. A fresh challenge is on its way.\n\n" + m.muted("Esc  Cancel · Your previous game is kept")
		if m.pendingNew {
			heading = "Preparing your board"
			body = "Saving the new puzzle…"
		}
		content = m.card(heading, body)
	default:
		content = m.menuView()
	}
	if m.width > 0 && m.height > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "Sudoku"
	v.ReportFocus = true
	return v
}
func (m *Model) wrap(s string, width int) string { return lipgloss.Wrap(s, width, " ") }
func (m *Model) card(title, body string) string {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).Width(64)
	if !m.plain {
		style = style.BorderForeground(lipgloss.Color(m.colors().line))
	}
	return style.Render(m.accent(title) + "\n\n" + m.text(body))
}
func (m *Model) menuView() string {
	title, sub := "S U D O K U", "Dont let your boss see."
	switch m.screen {
	case paused:
		title = "Take your time"
		sub = "Your game is paused."
	case difficulty:
		title = "Choose your challenge"
		sub = "Every puzzle has one solution and a logical path."
	case confirm:
		title = "Start a fresh game?"
		sub = "This replaces your current game once the new board is ready."
	case victory:
		title = "Beautifully solved."
		sub = "Every square in its place."
	}
	body := m.muted(sub) + "\n"
	if m.game != nil && (m.screen == home || m.screen == paused || m.screen == victory) {
		filled, total := m.game.Progress()
		summary := fmt.Sprintf("%s  ·  %d / %d filled", m.game.Difficulty, filled, total)
		if m.game.TimerVisible {
			summary += "  ·  " + duration(m.game.Elapsed)
		}
		body += "\n" + m.text(summary) + "\n"
	}
	if m.loadErr != nil && (m.screen == home || m.screen == confirm) {
		body += "\n" + m.wrap("Your saved game couldn't be opened. It will be kept until you choose to replace it.", 56) + "\n" + m.muted(m.wrap(m.loadErr.Error(), 56)) + "\n"
	}
	for i, label := range m.options() {
		prefix := "   "
		if i == m.menu {
			prefix = " › "
		}
		line := prefix + label
		if m.screen == difficulty {
			descriptions := []string{"  ·  Singles and gentle starts", "  ·  Pairs and locked candidates", "  ·  Triples and X-Wings", "  ·  Wings, fish, and coloring"}
			line += descriptions[i]
		}
		if i == m.menu {
			line = m.accent(line)
		} else {
			line = m.text(line)
		}
		body += "\n" + line
	}
	body += "\n\n" + m.muted("↑↓ / jk  Choose    Enter  Select    Esc  Back")
	if m.message != "" && time.Now().Before(m.messageUntil) {
		body += "\n" + m.wrap(m.message, 56)
	}
	return m.card(title, body)
}
func (m *Model) helpView() string {
	return m.card("Make yourself at home", strings.Join([]string{
		"Fill each row, column, and 3×3 box with 1–9.",
		"Starting clues stay fixed. Red answers need another look.",
		"",
		"Arrows / hjkl       Move around the board",
		"1–9                 Enter an answer or toggle a note",
		"n                   Switch Answer / Notes mode",
		"Backspace / Del / 0  Clear the selected cell",
		"u / Ctrl+R          Undo / redo a whole action",
		"i                   Explain a hint; Enter applies it",
		"t                   Show / hide the timer",
		"Esc                 Pause / return",
		"q / Ctrl+C          Save and quit",
		"",
		"                                                      ",
		"Your progress and undo history are saved as you play.",
		"",
		"Enter / Esc  Return",
	}, "\n"))
}

func (m *Model) playView() string {
	expanded := m.expandedBoard()
	boardWidth := 9*(cellWidth(expanded)+1) + 1
	// Leave at least 20 columns for the sidebar and one column at each edge.
	contentWidth := max(78, boardWidth+3+20)
	panelWidth := contentWidth - boardWidth - 3
	g := m.game
	right := g.Difficulty.String()
	if g.TimerVisible {
		right += "  ·  " + duration(g.Elapsed)
	}
	status := "Saved"
	if m.saving || len(m.queue) > 0 {
		status = "Saving…"
	}
	right += "  ·  " + status
	left := "S U D O K U"
	header := m.accent(left) + strings.Repeat(" ", contentWidth-lipgloss.Width(left)-lipgloss.Width(right)) + m.muted(right)
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.boardView(expanded), "   ", m.panelView(panelWidth))
	footer := m.muted("arrows/hjkl move  1–9 enter  n notes  u undo  Ctrl+R redo  i hint  ? help")
	second := m.muted("Backspace clear   ·   Esc pause   ·   t timer   ·   q quit")
	if m.screen == hint {
		footer = m.accent("Enter  Apply this step    Esc  Keep thinking")
		second = m.muted("Hints pause the timer. Every applied step can be undone.")
	}
	if m.message != "" && time.Now().Before(m.messageUntil) {
		second = m.style(m.colors().accent).Render(m.wrap(m.message, contentWidth))
	}
	return header + "\n\n" + body + "\n\n" + footer + "\n" + second
}

func (m *Model) expandedBoard() bool {
	return m.width >= 98 && m.height >= 42
}

func cellWidth(expanded bool) int {
	if expanded {
		// With borders, each cell spans 8 columns by 4 rows. Terminal
		// characters are commonly about twice as tall as they are wide.
		return 7
	}
	return 3
}

func (m *Model) boardView(expanded bool) string {
	w, h := cellWidth(expanded), 1
	if expanded {
		h = 3
	}
	var lines []string
	for r := 0; r < 9; r++ {
		lines = append(lines, m.borderLine(r, w))
		for sub := 0; sub < h; sub++ {
			var row strings.Builder
			for c := 0; c < 9; c++ {
				v := "│"
				if c%3 == 0 {
					v = "║"
				}
				row.WriteString(m.style(m.colors().line).Render(v))
				row.WriteString(m.cellView(r*9+c, sub, expanded))
			}
			row.WriteString(m.style(m.colors().line).Render("║"))
			lines = append(lines, row.String())
		}
	}
	lines = append(lines, m.borderLine(9, w))
	return strings.Join(lines, "\n")
}
func (m *Model) borderLine(row, w int) string {
	left, mid, box, right, horiz := "╟", "┼", "╫", "╢", "─"
	if row == 0 {
		left, mid, box, right, horiz = "╔", "╤", "╦", "╗", "═"
	} else if row == 9 {
		left, mid, box, right, horiz = "╚", "╧", "╩", "╝", "═"
	} else if row%3 == 0 {
		left, mid, box, right, horiz = "╠", "╪", "╬", "╣", "═"
	}
	s := left
	for c := 0; c < 9; c++ {
		s += strings.Repeat(horiz, w)
		if c == 8 {
			s += right
		} else if c%3 == 2 {
			s += box
		} else {
			s += mid
		}
	}
	return m.style(m.colors().line).Render(s)
}
func (m *Model) cellView(i, sub int, expanded bool) string {
	g, p := m.game, m.colors()
	cell := g.Board[i]
	selected := i == g.Cursor
	style := m.style(p.text)
	bg := ""
	if puzzle.Sees(i, g.Cursor) {
		bg = p.peer
	}
	if cell.Value != 0 && cell.Value == g.Board[g.Cursor].Value {
		bg = p.match
	}
	if m.screen == hint && m.hint != nil {
		for _, j := range m.hint.Cells {
			if j == i {
				bg = p.hint
			}
		}
		for _, r := range m.hint.Removals {
			if r.Cell == i {
				bg = p.hint
			}
		}
	}
	if selected {
		bg = p.selected
	}
	if !m.plain && bg != "" {
		style = style.Background(lipgloss.Color(bg))
	}
	width := cellWidth(expanded)
	content := strings.Repeat(" ", width)
	if cell.Value != 0 {
		if !expanded || sub == 1 {
			padding := strings.Repeat(" ", (width-1)/2)
			content = fmt.Sprintf("%s%d%s", padding, cell.Value, padding)
		}
		if g.Givens[i] != 0 {
			style = style.Bold(true)
		} else {
			if !m.plain {
				style = style.Foreground(lipgloss.Color(p.accent))
			}
		}
		if g.Wrong(i) {
			style = style.Underline(true).Bold(true)
			if !m.plain {
				style = style.Foreground(lipgloss.Color(p.danger))
			}
		}
	} else if cell.Notes != 0 {
		if !m.plain {
			style = style.Foreground(lipgloss.Color(p.muted))
		}
		if expanded {
			s := ""
			for n := 1; n <= 3; n++ {
				if n > 1 {
					s += " "
				}
				d := uint8(sub*3 + n)
				if cell.Notes&puzzle.Bit(d) != 0 {
					s += fmt.Sprint(d)
				} else {
					s += " "
				}
			}
			content = " " + s + " "
		} else {
			content = " · "
		}
	} else if !expanded {
		content = "   "
	}
	if selected {
		style = style.Bold(true)
		if cell.Value == 0 && !m.plain {
			style = style.Foreground(lipgloss.Color(p.text))
		}
		if m.plain {
			content = "[" + content[1:len(content)-1] + "]"
		}
	}
	return style.Render(content)
}

func (m *Model) panelView(width int) string {
	g := m.game
	i := g.Cursor
	cell := g.Board[i]
	var lines []string
	if m.screen == hint && m.hint != nil {
		s := m.hint
		lines = []string{m.accent("A WAY FORWARD"), "", m.text(s.Technique), "", m.wrap(s.Explanation, width)}
		if s.Digit != 0 {
			lines = append(lines, "", m.accent(fmt.Sprintf("Place %d in %s", s.Digit, puzzle.Cell(s.Cell))))
		} else {
			lines = append(lines, "", m.muted("Candidate removals:"))
			var removals []string
			for _, r := range s.Removals {
				removals = append(removals, fmt.Sprintf("%s: %s", puzzle.Cell(r.Cell), r.Digits))
			}
			// Keep the full explanation visible; a count summarizes long lists.
			list := strings.Join(removals, " · ")
			maxLines := 6
			if m.height >= 42 {
				maxLines = 15
			}
			wrapped := strings.Split(m.wrap(list, width), "\n")
			if len(wrapped) > maxLines {
				wrapped = append(wrapped[:maxLines-1], fmt.Sprintf("%d affected cells", len(s.Removals)))
			}
			lines = append(lines, m.text(strings.Join(wrapped, "\n")))
		}
	} else {
		mode := "ANSWER"
		if g.NotesMode {
			mode = "NOTES"
		}
		lines = []string{m.accent(mode), m.muted("n to switch"), "", m.text(fmt.Sprintf("ROW %d  ·  COL %d", i/9+1, i%9+1)), ""}
		switch {
		case g.Givens[i] != 0:
			lines = append(lines, m.muted("Starting clue"))
		case g.Wrong(i):
			lines = append(lines, m.style(m.colors().danger).Bold(true).Render("Incorrect answer"))
		case cell.Value != 0:
			lines = append(lines, m.text("Your answer"))
		default:
			lines = append(lines, m.muted("Pencil notes"))
		}
		lines = append(lines, "")
		for r := 0; r < 3; r++ {
			s := ""
			for c := 0; c < 3; c++ {
				d := uint8(r*3 + c + 1)
				v := "·"
				if cell.Notes&puzzle.Bit(d) != 0 {
					v = fmt.Sprint(d)
				}
				if c > 0 {
					s += "  "
				}
				s += v
			}
			lines = append(lines, m.text("  "+s))
		}
		filled, total := g.Progress()
		lines = append(lines, "", m.muted(fmt.Sprintf("%d / %d filled", filled, total)))
		if g.Complete() {
			lines = append(lines, m.accent("Puzzle complete"))
		} else if g.Mistakes() > 0 {
			lines = append(lines, m.muted(fmt.Sprintf("%d to revisit", g.Mistakes())))
		}
		lines = append(lines, "", m.muted("i  Show hint"))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}
