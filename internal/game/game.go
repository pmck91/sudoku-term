// Package game owns reversible player actions, independently of the terminal.
package game

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"sudoku-term/internal/puzzle"
)

const SaveVersion = 1

var (
	ErrFixed    = errors.New("Starting clues cannot be changed")
	ErrOccupied = errors.New("Clear the answer before adding notes")
	ErrMistakes = errors.New("Correct the red answers before requesting a hint")
)

type Cell struct {
	Value      uint8       `json:"value"`
	Notes      puzzle.Mask `json:"notes"`
	Eliminated puzzle.Mask `json:"eliminated"`
}
type Board [81]Cell
type CellChange struct {
	Index  int  `json:"index"`
	Before Cell `json:"before"`
	After  Cell `json:"after"`
}
type Change struct {
	Cells []CellChange `json:"cells"`
}

type Game struct {
	Version      int               `json:"version"`
	Givens       puzzle.Grid       `json:"givens"`
	Solution     puzzle.Grid       `json:"solution"`
	Difficulty   puzzle.Difficulty `json:"difficulty"`
	Board        Board             `json:"board"`
	History      []Change          `json:"history"`
	Future       []Change          `json:"future"`
	Elapsed      time.Duration     `json:"elapsed_ns"`
	Cursor       int               `json:"cursor"`
	NotesMode    bool              `json:"notes_mode"`
	TimerVisible bool              `json:"timer_visible"`
}

func New(p puzzle.Puzzle) *Game {
	g := &Game{Version: SaveVersion, Givens: p.Givens, Solution: p.Solution, Difficulty: p.Difficulty, TimerVisible: true}
	for i, v := range p.Givens {
		g.Board[i].Value = v
	}
	for i, v := range p.Givens {
		if v == 0 {
			g.Cursor = i
			break
		}
	}
	return g
}
func (g *Game) Values() puzzle.Grid {
	var v puzzle.Grid
	for i, c := range g.Board {
		v[i] = c.Value
	}
	return v
}
func (g *Game) Complete() bool   { return g.Values() == g.Solution }
func (g *Game) Wrong(i int) bool { return g.Board[i].Value != 0 && g.Board[i].Value != g.Solution[i] }
func (g *Game) Mistakes() int {
	n := 0
	for i := range g.Board {
		if g.Wrong(i) {
			n++
		}
	}
	return n
}
func (g *Game) Progress() (int, int) {
	filled, total := 0, 0
	for i := range g.Board {
		if g.Givens[i] == 0 {
			total++
			if g.Board[i].Value == g.Solution[i] {
				filled++
			}
		}
	}
	return filled, total
}

func (g *Game) commit(before Board) bool {
	c := Change{}
	for i := range g.Board {
		if before[i] != g.Board[i] {
			c.Cells = append(c.Cells, CellChange{i, before[i], g.Board[i]})
		}
	}
	if len(c.Cells) == 0 {
		return false
	}
	g.History = append(g.History, c)
	g.Future = nil
	return true
}
func (g *Game) resetEliminations() {
	for i := range g.Board {
		g.Board[i].Eliminated = 0
	}
}
func (g *Game) Enter(i int, d uint8, notes bool) (bool, error) {
	if i < 0 || i >= 81 || d < 1 || d > 9 {
		return false, errors.New("Invalid cell or digit")
	}
	if g.Givens[i] != 0 {
		return false, ErrFixed
	}
	before := g.Board
	if notes {
		if g.Board[i].Value != 0 {
			return false, ErrOccupied
		}
		g.Board[i].Notes ^= puzzle.Bit(d)
	} else {
		if g.Board[i].Value == d {
			return false, nil
		}
		g.resetEliminations()
		g.Board[i].Value = d
		g.Board[i].Notes = 0
		if d == g.Solution[i] {
			for _, p := range puzzle.Peers[i] {
				g.Board[p].Notes &^= puzzle.Bit(d)
			}
		}
	}
	return g.commit(before), nil
}
func (g *Game) Clear(i int) (bool, error) {
	if i < 0 || i >= 81 {
		return false, errors.New("Invalid cell")
	}
	if g.Givens[i] != 0 {
		return false, ErrFixed
	}
	before := g.Board
	if g.Board[i].Value != 0 {
		g.resetEliminations()
	}
	g.Board[i].Value = 0
	g.Board[i].Notes = 0
	return g.commit(before), nil
}
func (g *Game) Undo() bool {
	if len(g.History) == 0 {
		return false
	}
	c := g.History[len(g.History)-1]
	g.History = g.History[:len(g.History)-1]
	for _, v := range c.Cells {
		g.Board[v.Index] = v.Before
	}
	g.Future = append(g.Future, c)
	return true
}
func (g *Game) Redo() bool {
	if len(g.Future) == 0 {
		return false
	}
	c := g.Future[len(g.Future)-1]
	g.Future = g.Future[:len(g.Future)-1]
	for _, v := range c.Cells {
		g.Board[v.Index] = v.After
	}
	g.History = append(g.History, c)
	return true
}
func (g *Game) Hint() (*puzzle.Step, error) {
	if g.Mistakes() > 0 {
		return nil, ErrMistakes
	}
	var eliminated [81]puzzle.Mask
	for i, c := range g.Board {
		eliminated[i] = c.Eliminated
	}
	l := puzzle.NewLogic(g.Values(), eliminated)
	return l.Next(puzzle.Master), nil
}
func (g *Game) ApplyHint(s puzzle.Step) (bool, error) {
	next, err := g.Hint()
	if err != nil {
		return false, err
	}
	if next == nil || !reflect.DeepEqual(*next, s) {
		return false, errors.New("The board changed; request a fresh hint")
	}
	if s.Digit != 0 {
		return g.Enter(s.Cell, s.Digit, false)
	}
	before := g.Board
	for _, r := range s.Removals {
		c := &g.Board[r.Cell]
		if c.Notes == 0 {
			c.Notes = g.Values().Candidates(r.Cell) &^ c.Eliminated
		}
		c.Notes &^= r.Digits
		c.Eliminated |= r.Digits
	}
	return g.commit(before), nil
}

func (g *Game) validateBoard(b Board) error {
	for i, c := range b {
		if c.Value > 9 || c.Notes&^puzzle.All != 0 || c.Eliminated&^puzzle.All != 0 {
			return fmt.Errorf("invalid cell %s", puzzle.Cell(i))
		}
		if g.Givens[i] != 0 && c.Value != g.Givens[i] {
			return fmt.Errorf("altered starting clue at %s", puzzle.Cell(i))
		}
		if c.Value != 0 && c.Notes != 0 {
			return fmt.Errorf("notes on filled cell %s", puzzle.Cell(i))
		}
		if c.Eliminated&puzzle.Bit(g.Solution[i]) != 0 {
			return fmt.Errorf("invalid deduction at %s", puzzle.Cell(i))
		}
	}
	return nil
}
func (g *Game) Validate() error {
	if g.Version != SaveVersion {
		return fmt.Errorf("unsupported save version %d", g.Version)
	}
	if g.Difficulty > puzzle.Master || g.Cursor < 0 || g.Cursor >= 81 || g.Elapsed < 0 {
		return errors.New("invalid game metadata")
	}
	if !g.Solution.Complete() || !g.Givens.Valid() {
		return errors.New("invalid puzzle or solution")
	}
	for i, v := range g.Givens {
		if v != 0 && v != g.Solution[i] {
			return errors.New("clues do not match solution")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	n, solution, err := puzzle.CountSolutions(ctx, g.Givens, 2)
	if err != nil {
		return fmt.Errorf("validate puzzle: %w", err)
	}
	if n != 1 || solution != g.Solution {
		return errors.New("puzzle does not have its claimed unique solution")
	}
	if err := g.validateBoard(g.Board); err != nil {
		return err
	}
	// Validate the entire history before any saved action can be replayed.
	for _, history := range []struct {
		changes []Change
		undo    bool
	}{{g.History, true}, {g.Future, false}} {
		board := g.Board
		for n := len(history.changes) - 1; n >= 0; n-- {
			c := history.changes[n]
			if len(c.Cells) == 0 {
				return errors.New("empty history action")
			}
			var seen [81]bool
			for _, v := range c.Cells {
				if v.Index < 0 || v.Index >= 81 || seen[v.Index] {
					return errors.New("invalid history cell")
				}
				seen[v.Index] = true
				from, to := v.Before, v.After
				if history.undo {
					from, to = to, from
				}
				if board[v.Index] != from {
					return errors.New("inconsistent undo history")
				}
				board[v.Index] = to
			}
			if err := g.validateBoard(board); err != nil {
				return fmt.Errorf("invalid history: %w", err)
			}
		}
	}
	return nil
}
