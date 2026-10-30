package store

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sudoku-term/internal/game"
	"sudoku-term/internal/puzzle"
)

func savedGame() *game.Game {
	const input = "530070000600195000098000060800060003400803001700020006060000280000419005000080079"
	var g puzzle.Grid
	for i, c := range input {
		g[i] = uint8(c - '0')
	}
	_, solution, _ := puzzle.CountSolutions(context.Background(), g, 2)
	return game.New(puzzle.Puzzle{Givens: g, Solution: solution, Difficulty: puzzle.Easy})
}
func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state", "game.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestRoundtripAndLock(t *testing.T) {
	s := openTest(t)
	if g, err := s.Load(); err != nil || g != nil {
		t.Fatal("missing save")
	}
	if second, err := Open(s.path); err == nil {
		second.Close()
		t.Fatal("competing instance obtained lock")
	}
	g := savedGame()
	g.Enter(2, 3, true)
	g.Enter(3, 5, true)
	g.Enter(2, 4, false)
	g.Undo()
	g.Elapsed = 79 * time.Second
	g.Cursor = 30
	g.NotesMode = true
	g.TimerVisible = false
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, loaded) {
		t.Fatal("state or history changed")
	}
	if !loaded.Redo() || loaded.Board[2].Value != 4 {
		t.Fatal("saved redo lost")
	}
	info, err := os.Stat(s.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("save permissions")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), ".game-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary files left behind")
	}
}
func TestInvalidSaveIsPreserved(t *testing.T) {
	for _, contents := range []string{"{", `{"version":999}`, "null"} {
		t.Run(contents, func(t *testing.T) {
			s := openTest(t)
			os.WriteFile(s.path, []byte(contents), 0600)
			if _, err := s.Load(); err == nil {
				t.Fatal("accepted invalid save")
			}
			got, _ := os.ReadFile(s.path)
			if string(got) != contents {
				t.Fatal("invalid save modified")
			}
		})
	}
}
func TestFailedReplacementPreservesExistingData(t *testing.T) {
	s := openTest(t)
	g := savedGame()
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(s.path)
	// A failed destination rename must leave the prior destination untouched.
	s.path = filepath.Join(filepath.Dir(s.path), "destination-directory")
	os.Mkdir(s.path, 0700)
	if err := s.Save(g); err == nil {
		t.Fatal("expected rename failure")
	}
	got, _ := os.ReadFile(filepath.Join(filepath.Dir(s.path), "game.json"))
	if string(got) != string(original) {
		t.Fatal("prior save was lost")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), ".game-*.tmp"))
	if len(files) != 0 {
		t.Fatal("failed write left temporary file")
	}
}
func TestTrailingAndUnsupportedData(t *testing.T) {
	s := openTest(t)
	g := savedGame()
	s.Save(g)
	data, _ := os.ReadFile(s.path)
	os.WriteFile(s.path, append(data, []byte(" {}")...), 0600)
	if _, err := s.Load(); err == nil {
		t.Fatal("accepted trailing data")
	}
	os.WriteFile(s.path, []byte(strings.Replace(string(data), `"version":1`, `"version":2`, 1)), 0600)
	if _, err := s.Load(); err == nil {
		t.Fatal("accepted unknown version")
	}
}
func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	p, err := DefaultPath()
	if err != nil || !strings.HasSuffix(p, "sudoku-term/game.json") {
		t.Fatal(p, err)
	}
	t.Setenv("XDG_STATE_HOME", "relative")
	p, err = DefaultPath()
	home, _ := os.UserHomeDir()
	if err != nil || p != filepath.Join(home, ".local/state/sudoku-term/game.json") {
		t.Fatal(p, err)
	}
}
