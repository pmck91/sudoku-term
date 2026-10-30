package game

import (
	"context"
	"math/rand/v2"
	"reflect"
	"testing"

	"sudoku-term/internal/puzzle"
)

func fixture() *Game {
	const input = "530070000600195000098000060800060003400803001700020006060000280000419005000080079"
	var givens puzzle.Grid
	for i, c := range input {
		givens[i] = uint8(c - '0')
	}
	_, solution, _ := puzzle.CountSolutions(context.Background(), givens, 2)
	return New(puzzle.Puzzle{Givens: givens, Solution: solution, Difficulty: puzzle.Easy})
}
func TestNotesCleanupAndHistory(t *testing.T) {
	g := fixture()
	before := g.Board
	if changed, err := g.Enter(0, 4, false); changed || err != ErrFixed {
		t.Fatal("changed clue")
	}
	// r1c3 and r1c4 share a row; 4 belongs in r1c3.
	g.Enter(3, 4, true)
	g.Enter(3, 6, true)
	noted := g.Board
	changed, err := g.Enter(2, 4, false)
	if !changed || err != nil {
		t.Fatal(err)
	}
	if g.Board[3].Notes&puzzle.Bit(4) != 0 || g.Board[3].Notes&puzzle.Bit(6) == 0 {
		t.Fatal("bad cleanup")
	}
	answered := g.Board
	if !g.Undo() || g.Board != noted {
		t.Fatal("undo did not restore peer notes")
	}
	if !g.Redo() || g.Board != answered {
		t.Fatal("redo did not restore answer")
	}
	if _, err := g.Enter(2, 1, true); err != ErrOccupied {
		t.Fatal("notes on occupied cell")
	}
	for g.Undo() {
	}
	if g.Board != before {
		t.Fatal("undo all failed")
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	g.Enter(2, 2, true)
	if len(g.Future) != 0 {
		t.Fatal("edit did not clear redo")
	}
}
func TestWrongAnswersAndClear(t *testing.T) {
	g := fixture()
	g.Enter(3, 2, true)
	g.Enter(2, 2, true)
	g.Enter(2, 2, false)
	if !g.Wrong(2) || g.Mistakes() != 1 || g.Board[2].Notes != 0 {
		t.Fatal("wrong answer feedback")
	}
	if g.Board[3].Notes != puzzle.Bit(2) {
		t.Fatal("wrong answer pruned notes")
	}
	if _, err := g.Hint(); err != ErrMistakes {
		t.Fatal("hint accepted incorrect board")
	}
	if err := g.Validate(); err != nil {
		t.Fatal("wrong answers must survive validation:", err)
	}
	g.Clear(2)
	if g.Board[2].Value != 0 {
		t.Fatal("clear failed")
	}
	g.Undo()
	if !g.Wrong(2) {
		t.Fatal("undo clear")
	}
}
func TestHintPlacementsAndCompletion(t *testing.T) {
	g := fixture()
	for n := 0; !g.Complete(); n++ {
		if n > 100 {
			t.Fatal("hint loop")
		}
		s, err := g.Hint()
		if err != nil || s == nil {
			t.Fatalf("hint: %v", err)
		}
		before := g.Board
		changed, err := g.ApplyHint(*s)
		if !changed || err != nil {
			t.Fatal(err)
		}
		after := g.Board
		if !g.Undo() || g.Board != before {
			t.Fatal("hint not undoable")
		}
		if !g.Redo() || g.Board != after {
			t.Fatal("hint not redoable")
		}
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	g.Undo()
	if g.Complete() {
		t.Fatal("undo did not reopen completed puzzle")
	}
}
func TestEliminationHintState(t *testing.T) {
	p, err := puzzle.Generate(context.Background(), puzzle.Master, rand.New(rand.NewPCG(1, 77)))
	if err != nil {
		t.Fatal(err)
	}
	g := New(p)
	for n := 0; n < 200; n++ {
		s, err := g.Hint()
		if err != nil || s == nil {
			t.Fatal("no elimination hint")
		}
		before := g.Board
		if _, err := g.ApplyHint(*s); err != nil {
			t.Fatal(err)
		}
		if s.Digit != 0 {
			continue
		}
		after := g.Board
		next, err := g.Hint()
		if err != nil || reflect.DeepEqual(next, s) {
			t.Fatal("elimination hint repeated")
		}
		for _, r := range s.Removals {
			if g.Board[r.Cell].Notes&r.Digits != 0 || g.Board[r.Cell].Eliminated&r.Digits != r.Digits {
				t.Fatal("elimination not stored")
			}
		}
		g.Undo()
		if g.Board != before {
			t.Fatal("undo deduction")
		}
		g.Redo()
		if g.Board != after {
			t.Fatal("redo deduction")
		}
		if err := g.Validate(); err != nil {
			t.Fatal(err)
		}
		for i, c := range g.Board {
			if g.Givens[i] == 0 && c.Value == 0 {
				g.Enter(i, g.Solution[i], false)
				break
			}
		}
		for _, c := range g.Board {
			if c.Eliminated != 0 {
				t.Fatal("value edit did not rebuild deductions")
			}
		}
		return
	}
	t.Fatal("never applied an elimination")
}
func TestInvalidHistory(t *testing.T) {
	g := fixture()
	g.Enter(2, 4, false)
	g.History[0].Cells[0].Index = 100
	if g.Validate() == nil {
		t.Fatal("accepted bad history")
	}
}
