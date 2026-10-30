// Package tui presents the game using Bubble Tea's event loop.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"sudoku-term/internal/game"
	"sudoku-term/internal/puzzle"
)

type Persistence interface{ SaveBytes([]byte) error }
type screen uint8

const (
	home screen = iota
	playing
	difficulty
	confirm
	paused
	help
	hint
	generating
	victory
)

type tickMsg time.Time
type saveResult struct{ err error }
type generatedMsg struct {
	id     int
	puzzle puzzle.Puzzle
	err    error
}
type saveJob struct {
	bytes       []byte
	replacement *game.Game
}

type Model struct {
	game                 *game.Game
	store                Persistence
	screen               screen
	width, height        int
	dark, plain          bool
	menu                 int
	returnTo, helpReturn screen
	selectedLevel        puzzle.Difficulty
	hint                 *puzzle.Step
	loadErr, errorSave   error
	message              string
	messageUntil         time.Time
	queue                []saveJob
	saving, quitting     bool
	pendingNew           bool
	genCancel            context.CancelFunc
	genID                int
	frame                int
	lastTick             time.Time
	focused              bool
}

func New(g *game.Game, s Persistence, loadErr error) *Model {
	return &Model{game: g, store: s, loadErr: loadErr, dark: true, plain: os.Getenv("NO_COLOR") != "", focused: true, lastTick: time.Now()}
}
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m *Model) Init() tea.Cmd { return tea.Batch(tea.RequestBackgroundColor, tick()) }
func (m *Model) active() bool {
	return m.screen == playing && m.game != nil && !m.game.Complete() && m.width >= 80 && m.height >= 24 && m.focused && m.errorSave == nil && !m.quitting
}
func (m *Model) accountTime(now time.Time) {
	if m.active() && !m.lastTick.IsZero() {
		m.game.Elapsed += now.Sub(m.lastTick)
	}
	m.lastTick = now
}
func (m *Model) notify(s string)       { m.message = s; m.messageUntil = time.Now().Add(4 * time.Second) }
func (m *Model) changeScreen(s screen) { m.screen = s; m.menu = 0; m.lastTick = time.Now() }
func (m *Model) persist(replacement *game.Game) tea.Cmd {
	g := m.game
	if replacement != nil {
		g = replacement
	}
	if g == nil {
		return nil
	}
	b, err := json.Marshal(g)
	if err != nil {
		m.errorSave = err
		return nil
	}
	m.queue = append(m.queue, saveJob{b, replacement})
	return m.nextSave()
}
func (m *Model) nextSave() tea.Cmd {
	if m.saving || m.errorSave != nil || len(m.queue) == 0 {
		return nil
	}
	m.saving = true
	data := m.queue[0].bytes
	return func() tea.Msg { return saveResult{m.store.SaveBytes(data)} }
}
func (m *Model) quit() tea.Cmd {
	if m.genCancel != nil {
		m.genCancel()
		m.genID++
	}
	m.quitting = true
	if m.pendingNew {
		return nil
	}
	cmd := m.persist(nil)
	if !m.saving && len(m.queue) == 0 {
		return tea.Quit
	}
	return cmd
}
func (m *Model) startGeneration() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.genCancel = cancel
	m.genID++
	id := m.genID
	level := m.selectedLevel
	m.changeScreen(generating)
	return func() tea.Msg {
		rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		p, err := puzzle.Generate(ctx, level, rng)
		return generatedMsg{id, p, err}
	}
}
func (m *Model) openDifficulty(from screen) { m.returnTo = from; m.changeScreen(difficulty) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.accountTime(time.Now())
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
	case tea.FocusMsg:
		m.focused = true
	case tea.BlurMsg:
		m.focused = false
	case tickMsg:
		m.frame++
		return m, tick()
	case saveResult:
		m.saving = false
		if msg.err != nil {
			m.errorSave = msg.err
			return m, nil
		}
		job := m.queue[0]
		m.queue = m.queue[1:]
		if job.replacement != nil {
			m.game = job.replacement
			m.pendingNew = false
			m.loadErr = nil
			m.changeScreen(playing)
		}
		if len(m.queue) > 0 {
			return m, m.nextSave()
		}
		if m.quitting {
			return m, tea.Quit
		}
	case generatedMsg:
		if msg.id != m.genID || m.screen != generating {
			return m, nil
		}
		m.genCancel = nil
		if msg.err != nil {
			m.changeScreen(m.returnTo)
			m.notify(fmt.Sprintf("Generation failed: %v", msg.err))
			return m, nil
		}
		m.pendingNew = true
		return m, m.persist(game.New(msg.puzzle))
	case tea.KeyPressMsg:
		key := msg.String()
		if m.errorSave != nil {
			switch key {
			case "r", "enter":
				m.errorSave = nil
				return m, m.nextSave()
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.quitting {
			return m, nil
		}
		if key == "q" || key == "ctrl+c" {
			return m, m.quit()
		}
		if m.width < 80 || m.height < 24 {
			return m, nil
		}
		if m.screen == generating {
			if key == "esc" && !m.pendingNew {
				m.genCancel()
				m.genCancel = nil
				m.genID++
				m.changeScreen(m.returnTo)
			}
			return m, nil
		}
		if m.screen == playing {
			return m, m.playKey(key)
		}
		if m.screen == hint {
			if key == "esc" || key == "i" {
				m.hint = nil
				m.changeScreen(playing)
			} else if key == "enter" {
				changed, err := m.game.ApplyHint(*m.hint)
				m.hint = nil
				m.changeScreen(playing)
				if err != nil {
					m.notify(err.Error())
				} else if changed {
					if m.game.Complete() {
						m.changeScreen(victory)
					}
					return m, m.persist(nil)
				}
			}
			return m, nil
		}
		if m.screen == help {
			if key == "esc" || key == "?" || key == "enter" {
				m.changeScreen(m.helpReturn)
			}
			return m, nil
		}
		if key == "?" {
			m.helpReturn = m.screen
			m.changeScreen(help)
			return m, nil
		}
		if key == "esc" {
			switch m.screen {
			case paused:
				m.changeScreen(playing)
			case difficulty:
				m.changeScreen(m.returnTo)
			case confirm:
				m.changeScreen(difficulty)
			case victory:
				m.changeScreen(playing)
			}
			return m, nil
		}
		options := m.options()
		switch key {
		case "up", "k":
			m.menu = (m.menu + len(options) - 1) % len(options)
		case "down", "j":
			m.menu = (m.menu + 1) % len(options)
		case "enter", " ":
			return m, m.choose()
		}
	}
	return m, nil
}

func (m *Model) playKey(key string) tea.Cmd {
	g := m.game
	i := g.Cursor
	switch key {
	case "left", "h":
		if i%9 > 0 {
			g.Cursor--
		}
		return nil
	case "right", "l":
		if i%9 < 8 {
			g.Cursor++
		}
		return nil
	case "up", "k":
		if i/9 > 0 {
			g.Cursor -= 9
		}
		return nil
	case "down", "j":
		if i/9 < 8 {
			g.Cursor += 9
		}
		return nil
	case "esc":
		m.changeScreen(paused)
		return m.persist(nil)
	case "?":
		m.helpReturn = playing
		m.changeScreen(help)
		return nil
	case "n":
		g.NotesMode = !g.NotesMode
		return m.persist(nil)
	case "t":
		g.TimerVisible = !g.TimerVisible
		return m.persist(nil)
	case "i":
		s, err := g.Hint()
		if err != nil {
			m.notify(err.Error())
		} else if s != nil {
			m.hint = s
			m.changeScreen(hint)
		} else {
			m.notify("No hint needed — the puzzle is complete")
		}
		return nil
	}
	wasComplete := g.Complete()
	changed := false
	var err error
	switch {
	case key == "u":
		changed = g.Undo()
		if !changed {
			m.notify("Nothing to undo")
		}
	case key == "ctrl+r":
		changed = g.Redo()
		if !changed {
			m.notify("Nothing to redo")
		}
	case key == "backspace" || key == "delete" || key == "0":
		changed, err = g.Clear(i)
	case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
		changed, err = g.Enter(i, key[0]-'0', g.NotesMode)
	}
	if err != nil {
		m.notify(err.Error())
	}
	if changed {
		m.message = ""
		if !wasComplete && g.Complete() {
			m.changeScreen(victory)
		}
		return m.persist(nil)
	}
	return nil
}

func (m *Model) options() []string {
	switch m.screen {
	case home:
		out := []string{}
		if m.game != nil {
			label := "Resume game"
			if m.game.Complete() {
				label = "Review completed game"
			}
			out = append(out, label)
		}
		return append(out, "New game", "How to play", "Quit")
	case paused:
		timer := "Hide timer"
		if !m.game.TimerVisible {
			timer = "Show timer"
		}
		return []string{"Resume game", "New game", "How to play", timer, "Save & quit"}
	case difficulty:
		return []string{"Easy", "Medium", "Hard", "Master"}
	case confirm:
		return []string{"Keep current game", "Replace game"}
	case victory:
		return []string{"Review board", "New game", "Save & quit"}
	}
	return nil
}
func (m *Model) choose() tea.Cmd {
	switch m.screen {
	case home:
		n := m.menu
		if m.game != nil {
			if n == 0 {
				m.changeScreen(playing)
				return nil
			}
			n--
		}
		switch n {
		case 0:
			m.openDifficulty(home)
		case 1:
			m.helpReturn = home
			m.changeScreen(help)
		case 2:
			return m.quit()
		}
	case paused:
		switch m.menu {
		case 0:
			m.changeScreen(playing)
		case 1:
			m.openDifficulty(paused)
		case 2:
			m.helpReturn = paused
			m.changeScreen(help)
		case 3:
			m.game.TimerVisible = !m.game.TimerVisible
			return m.persist(nil)
		case 4:
			return m.quit()
		}
	case difficulty:
		m.selectedLevel = puzzle.Difficulty(m.menu)
		if m.loadErr != nil || (m.game != nil && !m.game.Complete()) {
			m.changeScreen(confirm)
		} else {
			return m.startGeneration()
		}
	case confirm:
		if m.menu == 0 {
			m.changeScreen(m.returnTo)
		} else {
			return m.startGeneration()
		}
	case victory:
		switch m.menu {
		case 0:
			m.changeScreen(playing)
		case 1:
			m.openDifficulty(victory)
		case 2:
			return m.quit()
		}
	}
	return nil
}
