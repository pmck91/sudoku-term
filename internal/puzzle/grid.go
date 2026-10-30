// Package puzzle implements standard 9x9 Sudoku, logical deductions, and generation.
package puzzle

import (
	"context"
	"fmt"
	"math/bits"
	"math/rand/v2"
)

type Grid [81]uint8
type Mask uint16

const All Mask = 0x1ff

func Bit(d uint8) Mask {
	if d < 1 || d > 9 {
		return 0
	}
	return 1 << (d - 1)
}
func (m Mask) Count() int { return bits.OnesCount16(uint16(m)) }
func (m Mask) Digit() uint8 {
	if m.Count() != 1 {
		return 0
	}
	return uint8(bits.TrailingZeros16(uint16(m)) + 1)
}
func (m Mask) String() string {
	s := ""
	for d := uint8(1); d <= 9; d++ {
		if m&Bit(d) != 0 {
			s += fmt.Sprint(d)
		}
	}
	return s
}

type Difficulty uint8

const (
	Easy Difficulty = iota
	Medium
	Hard
	Master
)

func (d Difficulty) String() string {
	if d > Master {
		return "Unknown"
	}
	return [...]string{"Easy", "Medium", "Hard", "Master"}[d]
}

var Units [27][9]int
var Peers [81][]int

func init() {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			i := r*9 + c
			Units[r][c] = i
			Units[9+c][r] = i
			Units[18+(r/3)*3+c/3][(r%3)*3+c%3] = i
		}
	}
	for i := range Peers {
		for j := 0; j < 81; j++ {
			if i != j && Sees(i, j) {
				Peers[i] = append(Peers[i], j)
			}
		}
	}
}
func Sees(i, j int) bool { return i/9 == j/9 || i%9 == j%9 || Box(i) == Box(j) }
func Box(i int) int      { return (i/27)*3 + (i%9)/3 }
func Cell(i int) string  { return fmt.Sprintf("r%dc%d", i/9+1, i%9+1) }
func UnitName(u int) string {
	if u < 9 {
		return fmt.Sprintf("row %d", u+1)
	}
	if u < 18 {
		return fmt.Sprintf("column %d", u-8)
	}
	return fmt.Sprintf("box %d", u-17)
}
func (g Grid) Candidates(i int) Mask {
	if g[i] != 0 {
		return 0
	}
	m := All
	for _, p := range Peers[i] {
		m &^= Bit(g[p])
	}
	return m
}
func (g Grid) Valid() bool {
	for _, u := range Units {
		var seen Mask
		for _, i := range u {
			if g[i] > 9 {
				return false
			}
			b := Bit(g[i])
			if b&seen != 0 {
				return false
			}
			seen |= b
		}
	}
	return true
}
func (g Grid) Complete() bool {
	for _, v := range g {
		if v == 0 {
			return false
		}
	}
	return g.Valid()
}

// CountSolutions stops at limit. Every recursive branch observes cancellation.
func CountSolutions(ctx context.Context, g Grid, limit int) (int, Grid, error) {
	if !g.Valid() {
		return 0, Grid{}, nil
	}
	var solution Grid
	count := 0
	var visit func(Grid) error
	visit = func(board Grid) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		best, min, mask := -1, 10, Mask(0)
		for i, v := range board {
			if v != 0 {
				continue
			}
			m := board.Candidates(i)
			n := m.Count()
			if n == 0 {
				return nil
			}
			if n < min {
				best, min, mask = i, n, m
				if n == 1 {
					break
				}
			}
		}
		if best < 0 {
			count++
			solution = board
			return nil
		}
		for d := uint8(1); d <= 9; d++ {
			if mask&Bit(d) == 0 {
				continue
			}
			board[best] = d
			if err := visit(board); err != nil {
				return err
			}
			if count >= limit {
				break
			}
		}
		return nil
	}
	err := visit(g)
	return count, solution, err
}

func randomSolution(ctx context.Context, rng *rand.Rand) (Grid, error) {
	var fill func(Grid) (Grid, bool, error)
	fill = func(g Grid) (Grid, bool, error) {
		if err := ctx.Err(); err != nil {
			return Grid{}, false, err
		}
		best, min, mask := -1, 10, Mask(0)
		for i, v := range g {
			if v != 0 {
				continue
			}
			m := g.Candidates(i)
			n := m.Count()
			if n == 0 {
				return Grid{}, false, nil
			}
			if n < min {
				best, min, mask = i, n, m
			}
		}
		if best < 0 {
			return g, true, nil
		}
		for _, n := range rng.Perm(9) {
			d := uint8(n + 1)
			if mask&Bit(d) == 0 {
				continue
			}
			g[best] = d
			result, ok, err := fill(g)
			if err != nil || ok {
				return result, ok, err
			}
		}
		return Grid{}, false, nil
	}
	g, _, err := fill(Grid{})
	return g, err
}
