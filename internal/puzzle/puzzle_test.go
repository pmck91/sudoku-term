package puzzle

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"
)

func Parse(s string) Grid {
	var g Grid
	for i, c := range s {
		if i < 81 && c >= '1' && c <= '9' {
			g[i] = uint8(c - '0')
		}
	}
	return g
}

const easyPuzzle = "530070000600195000098000060800060003400803001700020006060000280000419005000080079"
const easySolution = "534678912672195348198342567859761423426853791713924856961537284287419635345286179"

func TestSolutions(t *testing.T) {
	cases := []struct {
		name string
		g    Grid
		want int
	}{{"unique", Parse(easyPuzzle), 1}, {"ambiguous", Grid{}, 2}, {"invalid", Parse("550000000"), 0}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, solution, err := CountSolutions(context.Background(), tc.g, 2)
			if err != nil || n != tc.want {
				t.Fatalf("count %d, %v", n, err)
			}
			if n == 1 && solution != Parse(easySolution) {
				t.Fatal("wrong solution")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := CountSolutions(ctx, Grid{}, 2); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestGenerateAndLogicalSoundness(t *testing.T) {
	for level := Easy; level <= Master; level++ {
		t.Run(level.String(), func(t *testing.T) {
			for seed := uint64(1); seed <= 3; seed++ {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				start := time.Now()
				p, err := Generate(ctx, level, rand.New(rand.NewPCG(seed, 77)))
				cancel()
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				t.Logf("seed %d: %s", seed, time.Since(start))
				n, solution, err := CountSolutions(context.Background(), p.Givens, 2)
				if err != nil || n != 1 || solution != p.Solution {
					t.Fatal("generation is not unique")
				}
				l := NewLogic(p.Givens, [81]Mask{})
				hardest := Easy
				for step := 0; !l.Grid.Complete(); step++ {
					if step > 1000 {
						t.Fatal("did not converge")
					}
					s := l.Next(Master)
					if s == nil {
						t.Fatal("needs guessing")
					}
					if s.Level > hardest {
						hardest = s.Level
					}
					if s.Digit != 0 && p.Solution[s.Cell] != s.Digit {
						t.Fatalf("unsound placement: %+v", s)
					}
					for _, r := range s.Removals {
						if r.Digits&Bit(p.Solution[r.Cell]) != 0 {
							t.Fatalf("unsound elimination: %+v", s)
						}
					}
					l.Apply(*s)
				}
				if hardest != level {
					t.Fatalf("rated %v, expected %v", hardest, level)
				}
				if level > Easy {
					_, _, ok, err := SolveLogic(context.Background(), p.Givens, level-1)
					if err != nil || ok {
						t.Fatal("lower tier solved puzzle")
					}
				}
			}
		})
	}
}

func mask(digits ...uint8) Mask {
	var m Mask
	for _, d := range digits {
		m |= Bit(d)
	}
	return m
}
func hasRemoval(s *Step, cell int, digit uint8) bool {
	if s == nil {
		return false
	}
	for _, r := range s.Removals {
		if r.Cell == cell && r.Digits&Bit(digit) != 0 {
			return true
		}
	}
	return false
}

func TestTechniqueFixtures(t *testing.T) {
	t.Run("naked pair", func(t *testing.T) {
		var l Logic
		l.Candidates[0] = mask(1, 2)
		l.Candidates[1] = mask(1, 2)
		l.Candidates[2] = mask(1, 2, 3)
		if !hasRemoval(l.subset(2, false), 2, 1) {
			t.Fatal("missed pair")
		}
		l.Candidates[1] |= Bit(4)
		if l.subset(2, false) != nil {
			t.Fatal("false pair")
		}
	})
	t.Run("hidden pair", func(t *testing.T) {
		var l Logic
		for i := 0; i < 9; i++ {
			l.Candidates[i] = mask(3, 4, 5)
		}
		l.Candidates[0] = mask(1, 2, 3)
		l.Candidates[1] = mask(1, 2, 4)
		if !hasRemoval(l.subset(2, true), 0, 3) {
			t.Fatal("missed hidden pair")
		}
		l.Candidates[2] |= mask(1, 2)
		if hasRemoval(l.subset(2, true), 0, 3) {
			t.Fatal("false hidden pair")
		}
	})
	t.Run("naked triple", func(t *testing.T) {
		var l Logic
		l.Candidates[0] = mask(1, 2)
		l.Candidates[1] = mask(2, 3)
		l.Candidates[2] = mask(1, 3)
		l.Candidates[3] = mask(1, 2, 3, 4)
		if !hasRemoval(l.subset(3, false), 3, 3) {
			t.Fatal("missed triple")
		}
		l.Candidates[2] = mask(1, 4)
		if l.subset(3, false) != nil {
			t.Fatal("false triple")
		}
	})
	t.Run("hidden triple", func(t *testing.T) {
		var l Logic
		for i := 0; i < 9; i++ {
			l.Candidates[i] = mask(4, 5, 6)
		}
		l.Candidates[0] = mask(1, 2, 4)
		l.Candidates[1] = mask(2, 3, 5)
		l.Candidates[2] = mask(1, 3, 6)
		if !hasRemoval(l.subset(3, true), 0, 4) {
			t.Fatal("missed hidden triple")
		}
	})
	t.Run("locked", func(t *testing.T) {
		var l Logic
		l.Candidates[0] = mask(1, 2)
		l.Candidates[1] = mask(1, 3)
		l.Candidates[8] = mask(1, 4)
		if !hasRemoval(l.locked(), 8, 1) {
			t.Fatal("missed pointing candidates")
		}
		l.Candidates[9] = mask(1, 5)
		if hasRemoval(l.locked(), 8, 1) {
			t.Fatal("false pointing")
		}
	})
	t.Run("X-Wing", func(t *testing.T) {
		var l Logic
		for _, i := range []int{0, 3, 18, 21, 39} {
			l.Candidates[i] = mask(1, 2, 3)
		}
		if !hasRemoval(l.fish(2), 39, 1) {
			t.Fatal("missed X-Wing")
		}
		l.Candidates[22] = mask(1, 2, 3)
		if hasRemoval(l.fish(2), 39, 1) {
			t.Fatal("false X-Wing")
		}
	})
	t.Run("Swordfish", func(t *testing.T) {
		var l Logic
		for _, i := range []int{0, 3, 21, 24, 36, 42, 57} {
			l.Candidates[i] = mask(1, 2, 3)
		}
		if !hasRemoval(l.fish(3), 57, 1) {
			t.Fatal("missed Swordfish")
		}
		l.Candidates[25] = mask(1, 2, 3)
		if hasRemoval(l.fish(3), 57, 1) {
			t.Fatal("false Swordfish")
		}
	})
	t.Run("XY-Wing", func(t *testing.T) {
		var l Logic
		l.Candidates[0] = mask(1, 2)
		l.Candidates[3] = mask(1, 3)
		l.Candidates[27] = mask(2, 3)
		l.Candidates[30] = mask(3, 4)
		if !hasRemoval(l.xyWing(), 30, 3) {
			t.Fatal("missed XY-Wing")
		}
		l.Candidates[27] = mask(2, 4)
		if l.xyWing() != nil {
			t.Fatal("false XY-Wing")
		}
	})
	t.Run("color wrap", func(t *testing.T) {
		var l Logic
		for _, i := range []int{0, 3, 30, 27, 9} {
			l.Candidates[i] = mask(1, 2)
		}
		s := l.coloring()
		if !hasRemoval(s, 9, 1) {
			t.Fatal("missed coloring")
		}
		for _, r := range s.Removals {
			if l.Candidates[r.Cell]&r.Digits == 0 {
				t.Fatal("eliminated absent candidate")
			}
		}
	})
}

func TestGenerateCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Generate(ctx, Master, rand.New(rand.NewPCG(1, 2)))
	if err != context.Canceled {
		t.Fatalf("%v", err)
	}
}
