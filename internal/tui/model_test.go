package tui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"sudoku-term/internal/game"
	"sudoku-term/internal/puzzle"
)

type memoryStore struct {
	snapshots [][]byte
	fail      bool
}

func (s *memoryStore) SaveBytes(b []byte) error {
	if s.fail {
		return errors.New("disk is full")
	}
	s.snapshots = append(s.snapshots, append([]byte(nil), b...))
	return nil
}
func testModel() *Model {
	const input = "530070000600195000098000060800060003400803001700020006060000280000419005000080079"
	var grid puzzle.Grid
	for i, c := range input {
		grid[i] = uint8(c - '0')
	}
	_, solution, _ := puzzle.CountSolutions(context.Background(), grid, 2)
	m := New(game.New(puzzle.Puzzle{Givens: grid, Solution: solution, Difficulty: puzzle.Easy}), &memoryStore{}, nil)
	m.width, m.height = 80, 24
	m.screen = playing
	m.game.Elapsed = 77 * time.Second
	m.plain = false
	return m
}
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}
func press(m *Model, s string) tea.Cmd { _, cmd := m.Update(key(s)); return cmd }
func drain(m *Model, cmd tea.Cmd) bool {
	for cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
		_, cmd = m.Update(msg)
	}
	return false
}

func TestOrderedSavesAndQuit(t *testing.T) {
	m := testModel()
	s := m.store.(*memoryStore)
	m.game.Cursor = 2
	cmd := press(m, "4")
	press(m, "l")
	press(m, "n")
	press(m, "6")
	press(m, "q")
	if len(m.queue) != 4 || !m.quitting || !m.saving {
		t.Fatalf("queue: %d", len(m.queue))
	}
	if !drain(m, cmd) {
		t.Fatal("quit did not wait for writes")
	}
	if len(s.snapshots) != 4 {
		t.Fatal("dropped a cell update")
	}
	var first, last game.Game
	json.Unmarshal(s.snapshots[0], &first)
	json.Unmarshal(s.snapshots[3], &last)
	if first.Board[2].Value != 4 || first.NotesMode {
		t.Fatal("incorrect first snapshot")
	}
	if last.Board[3].Notes != puzzle.Bit(6) || !last.NotesMode || last.Cursor != 3 {
		t.Fatal("incorrect final snapshot")
	}
}
func TestSaveFailurePausesEdits(t *testing.T) {
	m := testModel()
	s := m.store.(*memoryStore)
	s.fail = true
	drain(m, press(m, "4"))
	before := m.game.Board
	if m.errorSave == nil || !strings.Contains(ansi.Strip(m.View().Content), "Couldn't save") {
		t.Fatal("failure not visible")
	}
	press(m, "5")
	if m.game.Board != before {
		t.Fatal("accepted edit during failure")
	}
	s.fail = false
	drain(m, press(m, "r"))
	if m.errorSave != nil || len(m.queue) != 0 || len(s.snapshots) != 1 {
		t.Fatal("retry failed")
	}
}
func TestReplacementAndCancellationPreserveGame(t *testing.T) {
	m := testModel()
	old := m.game
	m.returnTo = paused
	cmd := m.startGeneration()
	press(m, "esc")
	drain(m, cmd)
	if m.game != old || m.screen != paused || len(m.queue) != 0 {
		t.Fatal("cancel replaced game")
	}
	m.screen = generating
	m.genID++
	replacement := puzzle.Puzzle{Givens: old.Givens, Solution: old.Solution, Difficulty: puzzle.Easy}
	_, cmd = m.Update(generatedMsg{m.genID, replacement, nil})
	if m.game != old || !m.pendingNew {
		t.Fatal("replaced before save")
	}
	s := m.store.(*memoryStore)
	s.fail = true
	drain(m, cmd)
	if m.game != old {
		t.Fatal("failed save replaced current game")
	}
	s.fail = false
	drain(m, press(m, "r"))
	if m.game == old || m.screen != playing || m.pendingNew {
		t.Fatal("replacement not committed")
	}
}
func TestHintAndNavigation(t *testing.T) {
	m := testModel()
	m.game.Cursor = 0
	press(m, "h")
	press(m, "k")
	if m.game.Cursor != 0 {
		t.Fatal("wrapped at edge")
	}
	press(m, "j")
	press(m, "l")
	if m.game.Cursor != 10 {
		t.Fatal("vim navigation")
	}
	press(m, "i")
	if m.screen != hint || m.hint == nil {
		t.Fatal("hint not shown")
	}
	before := m.game.Board
	drain(m, press(m, "enter"))
	if m.game.Board == before {
		t.Fatal("hint did not apply")
	}
	drain(m, press(m, "u"))
	if m.game.Board != before {
		t.Fatal("hint undo failed")
	}
	drain(m, press(m, "ctrl+r"))
	if m.game.Board == before {
		t.Fatal("hint redo failed")
	}
}
func TestTimerPausesAndHiddenTimerCounts(t *testing.T) {
	m := testModel()
	m.game.TimerVisible = false
	start := m.game.Elapsed
	m.lastTick = time.Now().Add(-2 * time.Second)
	m.Update(tea.BlurMsg{})
	if m.game.Elapsed < start+2*time.Second {
		t.Fatal("hidden timer stopped")
	}
	start = m.game.Elapsed
	m.lastTick = time.Now().Add(-time.Hour)
	m.Update(tickMsg(time.Now()))
	if m.game.Elapsed != start {
		t.Fatal("unfocused timer advanced")
	}
	m.Update(tea.FocusMsg{})
	press(m, "esc")
	start = m.game.Elapsed
	m.lastTick = time.Now().Add(-time.Hour)
	m.Update(tickMsg(time.Now()))
	if m.game.Elapsed != start {
		t.Fatal("paused timer advanced")
	}
}
func TestVictoryAndUndo(t *testing.T) {
	m := testModel()
	for i, v := range m.game.Givens {
		if v == 0 && i != 2 {
			m.game.Enter(i, m.game.Solution[i], false)
		}
	}
	m.game.Cursor = 2
	drain(m, press(m, "4"))
	if m.screen != victory || !m.game.Complete() {
		t.Fatal("no completion screen")
	}
	press(m, "enter")
	drain(m, press(m, "u"))
	if m.screen != playing || m.game.Complete() {
		t.Fatal("undo did not reopen game")
	}
}

func normalize(s string) string {
	lines := strings.Split(ansi.Strip(s), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n") + "\n"
}
func TestLayoutSnapshots(t *testing.T) {
	for _, name := range []string{"compact", "expanded", "hint", "expanded-hint", "errors", "resize", "home", "help"} {
		t.Run(name, func(t *testing.T) {
			m := testModel()
			m.game.Cursor = 2
			m.game.Enter(2, 1, true)
			m.game.Enter(2, 4, true)
			m.game.Enter(2, 7, true)
			m.game.NotesMode = true
			m.lastTick = time.Time{}
			switch name {
			case "expanded":
				m.width, m.height = 100, 45
			case "hint":
				m.hint, _ = m.game.Hint()
				m.screen = hint
			case "expanded-hint":
				m.width, m.height = 100, 45
				m.hint, _ = m.game.Hint()
				m.screen = hint
			case "errors":
				m.game.Enter(2, 2, false)
			case "resize":
				m.width, m.height = 60, 20
			case "home":
				m.screen = home
			case "help":
				m.screen = help
			}
			view := m.View().Content
			if lipgloss.Height(view) > m.height {
				t.Fatalf("%s exceeds screen height: %d > %d", name, lipgloss.Height(view), m.height)
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > m.width {
					t.Fatalf("line exceeds width: %d", lipgloss.Width(line))
				}
			}
			path := filepath.Join("testdata", name+".golden")
			got := normalize(view)
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != got {
				t.Fatalf("snapshot differs: %s", path)
			}
		})
	}
}

func TestBoardProportionsAndResize(t *testing.T) {
	m := testModel()
	m.game.Cursor = 2
	m.game.Enter(2, 1, true)
	m.game.Enter(2, 4, true)
	before := m.game.Board
	for _, tc := range []struct {
		width, height int
		expanded      bool
	}{
		{80, 24, false},
		{80, 45, false},
		{97, 42, false},
		{98, 41, false},
		{98, 42, true},
		{100, 45, true},
		{80, 24, false},
	} {
		m.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
		if m.expandedBoard() != tc.expanded {
			t.Fatalf("%dx%d: wrong board layout", tc.width, tc.height)
		}
		board := m.boardView(tc.expanded)
		// At a 2:1 character height/width ratio, the outer board should
		// be within one character column of a physical square.
		if difference := lipgloss.Width(board) - 2*lipgloss.Height(board); difference < -1 || difference > 1 {
			t.Fatalf("%dx%d: board proportions are %dx%d", tc.width, tc.height, lipgloss.Width(board), lipgloss.Height(board))
		}
		view := m.View().Content
		if lipgloss.Height(view) > tc.height || lipgloss.Width(view) > tc.width {
			t.Fatalf("%dx%d: layout overflows the terminal", tc.width, tc.height)
		}
		if m.game.Board != before || m.game.Cursor != 2 {
			t.Fatal("resize changed game state")
		}
	}
}

func TestExpandedCellSpacing(t *testing.T) {
	m := testModel()
	for _, digit := range []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9} {
		m.game.Enter(2, digit, true)
	}
	for row, want := range []string{" 1 2 3 ", " 4 5 6 ", " 7 8 9 "} {
		if got := ansi.Strip(m.cellView(2, row, true)); got != want {
			t.Fatalf("notes row %d: %q, want %q", row, got, want)
		}
	}
	m.game.Enter(2, 4, false)
	for row, want := range []string{"       ", "   4   ", "       "} {
		if got := ansi.Strip(m.cellView(2, row, true)); got != want {
			t.Fatalf("answer row %d: %q, want %q", row, got, want)
		}
	}
}

func TestErrorStylingAndNoColorSelection(t *testing.T) {
	m := testModel()
	m.game.Cursor = 2
	m.game.Enter(2, 2, false)
	for _, dark := range []bool{true, false} {
		m.dark = dark
		s := m.cellView(2, 0, false)
		if !strings.Contains(s, "4;") && !strings.Contains(s, ";4") {
			t.Fatalf("incorrect selection lost underline: %q", s)
		}
		if !strings.Contains(s, "38;2;") {
			t.Fatal("missing error color")
		}
	}
	m.plain = true
	if !strings.Contains(ansi.Strip(m.cellView(2, 0, false)), "[2]") {
		t.Fatal("no-color selection invisible")
	}
}
