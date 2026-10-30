package puzzle

import (
	"context"
	"fmt"
)

type Removal struct {
	Cell   int
	Digits Mask
}

// Step is an explanation and its precise, independently applicable effects.
type Step struct {
	Technique   string
	Level       Difficulty
	Explanation string
	Cells       []int
	Cell        int
	Digit       uint8
	Removals    []Removal
}
type Logic struct {
	Grid       Grid
	Candidates [81]Mask
}

func NewLogic(g Grid, eliminated [81]Mask) Logic {
	l := Logic{Grid: g}
	for i := range g {
		l.Candidates[i] = g.Candidates(i) &^ eliminated[i]
	}
	return l
}
func (l *Logic) Apply(s Step) {
	if s.Digit != 0 {
		l.Grid[s.Cell] = s.Digit
		l.Candidates[s.Cell] = 0
		for _, p := range Peers[s.Cell] {
			l.Candidates[p] &^= Bit(s.Digit)
		}
	}
	for _, r := range s.Removals {
		l.Candidates[r.Cell] &^= r.Digits
	}
}
func (l Logic) Next(max Difficulty) *Step {
	for i, m := range l.Candidates {
		if m.Count() == 1 {
			return &Step{Technique: "Naked single", Level: Easy, Cell: i, Digit: m.Digit(), Cells: []int{i}, Explanation: fmt.Sprintf("%s has only one possible value: %d.", Cell(i), m.Digit())}
		}
	}
	for u, unit := range Units {
		for d := uint8(1); d <= 9; d++ {
			cells := l.locations(unit, Bit(d))
			if len(cells) == 1 {
				i := cells[0]
				return &Step{Technique: "Hidden single", Level: Easy, Cell: i, Digit: d, Cells: []int{i}, Explanation: fmt.Sprintf("In %s, %d can only go in %s.", UnitName(u), d, Cell(i))}
			}
		}
	}
	if max < Medium {
		return nil
	}
	if s := l.locked(); s != nil {
		return s
	}
	if s := l.subset(2, false); s != nil {
		return s
	}
	if s := l.subset(2, true); s != nil {
		return s
	}
	if max < Hard {
		return nil
	}
	if s := l.subset(3, false); s != nil {
		return s
	}
	if s := l.subset(3, true); s != nil {
		return s
	}
	if s := l.fish(2); s != nil {
		return s
	}
	if max < Master {
		return nil
	}
	if s := l.xyWing(); s != nil {
		return s
	}
	if s := l.fish(3); s != nil {
		return s
	}
	return l.coloring()
}
func (l Logic) locations(unit [9]int, digit Mask) []int {
	var out []int
	for _, i := range unit {
		if l.Candidates[i]&digit != 0 {
			out = append(out, i)
		}
	}
	return out
}
func (l Logic) locked() *Step {
	for u, unit := range Units {
		for d := uint8(1); d <= 9; d++ {
			cells := l.locations(unit, Bit(d))
			if len(cells) < 2 {
				continue
			}
			for _, target := range []int{cells[0] / 9, 9 + cells[0]%9, 18 + Box(cells[0])} {
				if u == target || (u < 18 && target < 18) || (u >= 18 && target >= 18) {
					continue
				}
				inside := func(i int) bool {
					if target < 9 {
						return i/9 == target
					}
					if target < 18 {
						return 9+i%9 == target
					}
					return 18+Box(i) == target
				}
				all := true
				for _, i := range cells {
					if !inside(i) {
						all = false
						break
					}
				}
				if !all {
					continue
				}
				var rem []Removal
				for _, i := range Units[target] {
					inSource := false
					for _, j := range unit {
						if i == j {
							inSource = true
							break
						}
					}
					if !inSource && l.Candidates[i]&Bit(d) != 0 {
						rem = append(rem, Removal{i, Bit(d)})
					}
				}
				if len(rem) > 0 {
					return &Step{Technique: "Locked candidates", Level: Medium, Cells: cells, Removals: rem, Explanation: fmt.Sprintf("In %s, every %d is also in %s. Remove %d from the rest of %s.", UnitName(u), d, UnitName(target), d, UnitName(target))}
				}
			}
		}
	}
	return nil
}

func combinations(n, k int, fn func([]int) bool) bool {
	var chosen []int
	var walk func(int) bool
	walk = func(start int) bool {
		if len(chosen) == k {
			return fn(chosen)
		}
		for j := start; j <= n-(k-len(chosen)); j++ {
			chosen = append(chosen, j)
			if walk(j + 1) {
				return true
			}
			chosen = chosen[:len(chosen)-1]
		}
		return false
	}
	return walk(0)
}

func (l Logic) subset(size int, hidden bool) *Step {
	level := Medium
	if size == 3 {
		level = Hard
	}
	name := "Naked pair"
	if hidden {
		name = "Hidden pair"
	}
	if size == 3 {
		if hidden {
			name = "Hidden triple"
		} else {
			name = "Naked triple"
		}
	}
	for u, unit := range Units {
		var found *Step
		combinations(9, size, func(indices []int) bool {
			var digits Mask
			var cells []int
			if hidden {
				for _, d := range indices {
					digits |= Bit(uint8(d + 1))
					if len(l.locations(unit, Bit(uint8(d+1)))) == 0 {
						return false
					}
				}
				for _, i := range unit {
					if l.Candidates[i]&digits != 0 {
						cells = append(cells, i)
					}
				}
				if len(cells) != size {
					return false
				}
			} else {
				for _, j := range indices {
					i := unit[j]
					m := l.Candidates[i]
					if m.Count() < 2 || m.Count() > size {
						return false
					}
					digits |= m
					cells = append(cells, i)
				}
				if digits.Count() != size {
					return false
				}
			}
			var rem []Removal
			for _, i := range unit {
				selected := false
				for _, j := range cells {
					if i == j {
						selected = true
						break
					}
				}
				m := Mask(0)
				if hidden && selected {
					m = l.Candidates[i] &^ digits
				} else if !hidden && !selected {
					m = l.Candidates[i] & digits
				}
				if m != 0 {
					rem = append(rem, Removal{i, m})
				}
			}
			if len(rem) == 0 {
				return false
			}
			explanation := fmt.Sprintf("In %s, the highlighted cells are restricted to %s. Remove these digits from its other cells.", UnitName(u), digits)
			if hidden {
				explanation = fmt.Sprintf("In %s, %s can only appear in the highlighted cells. Remove other candidates from those cells.", UnitName(u), digits)
			}
			found = &Step{Technique: name, Level: level, Cells: cells, Removals: rem, Explanation: explanation}
			return true
		})
		if found != nil {
			return found
		}
	}
	return nil
}

func (l Logic) fish(size int) *Step {
	for axis := 0; axis < 2; axis++ {
		for d := uint8(1); d <= 9; d++ {
			var found *Step
			combinations(9, size, func(lines []int) bool {
				var positions Mask
				var cells []int
				for _, line := range lines {
					u := line
					if axis == 1 {
						u += 9
					}
					loc := l.locations(Units[u], Bit(d))
					if len(loc) < 2 || len(loc) > size {
						return false
					}
					for _, i := range loc {
						p := i % 9
						if axis == 1 {
							p = i / 9
						}
						positions |= 1 << p
						cells = append(cells, i)
					}
				}
				if positions.Count() != size {
					return false
				}
				var rem []Removal
				for i, m := range l.Candidates {
					if m&Bit(d) == 0 {
						continue
					}
					line, p := i/9, i%9
					if axis == 1 {
						line, p = p, line
					}
					selected := false
					for _, j := range lines {
						if line == j {
							selected = true
							break
						}
					}
					if !selected && positions&(1<<p) != 0 {
						rem = append(rem, Removal{i, Bit(d)})
					}
				}
				if len(rem) == 0 {
					return false
				}
				name, level := "X-Wing", Hard
				if size == 3 {
					name, level = "Swordfish", Master
				}
				direction := "rows"
				cross := "columns"
				if axis == 1 {
					direction, cross = cross, direction
				}
				found = &Step{Technique: name, Level: level, Cells: cells, Removals: rem, Explanation: fmt.Sprintf("The highlighted %d candidates occupy %d %s and the same %d %s. Remove %d from other cells in those %s.", d, size, direction, size, cross, d, cross)}
				return true
			})
			if found != nil {
				return found
			}
		}
	}
	return nil
}

func (l Logic) xyWing() *Step {
	for pivot, m := range l.Candidates {
		if m.Count() != 2 {
			continue
		}
		peers := Peers[pivot]
		for a, x := range peers {
			xm := l.Candidates[x]
			if xm.Count() != 2 || (xm&m).Count() != 1 {
				continue
			}
			for _, y := range peers[a+1:] {
				ym := l.Candidates[y]
				if ym.Count() != 2 || (ym&m).Count() != 1 || xm&m == ym&m {
					continue
				}
				z := xm & ym &^ m
				if z.Count() != 1 || xm|ym|m != m|z {
					continue
				}
				var rem []Removal
				for _, i := range Peers[x] {
					if i != pivot && i != y && Sees(i, y) && l.Candidates[i]&z != 0 {
						rem = append(rem, Removal{i, z})
					}
				}
				if len(rem) > 0 {
					return &Step{Technique: "XY-Wing", Level: Master, Cells: []int{pivot, x, y}, Removals: rem, Explanation: fmt.Sprintf("Whichever value goes in pivot %s, one wing (%s or %s) must be %d. Remove %d from cells that see both wings.", Cell(pivot), Cell(x), Cell(y), z.Digit(), z.Digit())}
				}
			}
		}
	}
	return nil
}

func (l Logic) coloring() *Step {
	for d := uint8(1); d <= 9; d++ {
		var edges [81][]int
		for _, u := range Units {
			loc := l.locations(u, Bit(d))
			if len(loc) == 2 {
				a, b := loc[0], loc[1]
				edges[a] = append(edges[a], b)
				edges[b] = append(edges[b], a)
			}
		}
		var color [81]int
		for start := range edges {
			if len(edges[start]) == 0 || color[start] != 0 {
				continue
			}
			color[start] = 1
			nodes := []int{start}
			for q := 0; q < len(nodes); q++ {
				a := nodes[q]
				for _, b := range edges[a] {
					if color[b] == 0 {
						color[b] = 3 - color[a]
						nodes = append(nodes, b)
					}
				}
			}
			var rem []Removal
			bad := 0
			for a, i := range nodes {
				for _, j := range nodes[a+1:] {
					if color[i] == color[j] && Sees(i, j) {
						bad = color[i]
					}
				}
			}
			if bad != 0 {
				for _, i := range nodes {
					if color[i] == bad {
						rem = append(rem, Removal{i, Bit(d)})
					}
				}
			} else {
				var in [81]bool
				for _, i := range nodes {
					in[i] = true
				}
				for i, m := range l.Candidates {
					if m&Bit(d) == 0 || in[i] {
						continue
					}
					seen := 0
					for _, j := range nodes {
						if Sees(i, j) {
							seen |= 1 << color[j]
						}
					}
					if seen == 6 {
						rem = append(rem, Removal{i, Bit(d)})
					}
				}
			}
			if len(rem) > 0 {
				explanation := fmt.Sprintf("Alternate two colors along the highlighted strong links for %d. A cell seeing both colors cannot contain %d.", d, d)
				if bad != 0 {
					explanation = fmt.Sprintf("Alternate two colors along strong links for %d. One color puts two %ds in the same unit, so eliminate that color.", d, d)
				}
				return &Step{Technique: "Simple coloring", Level: Master, Cells: nodes, Removals: rem, Explanation: explanation}
			}
		}
	}
	return nil
}

// SolveLogic never guesses. The reported tier is the hardest deduction used.
func SolveLogic(ctx context.Context, g Grid, max Difficulty) (Grid, Difficulty, bool, error) {
	l := NewLogic(g, [81]Mask{})
	level := Easy
	if !g.Valid() {
		return g, level, false, nil
	}
	for n := 0; n < 4096; n++ {
		if err := ctx.Err(); err != nil {
			return l.Grid, level, false, err
		}
		if l.Grid.Complete() {
			return l.Grid, level, true, nil
		}
		s := l.Next(max)
		if s == nil {
			return l.Grid, level, false, nil
		}
		if s.Level > level {
			level = s.Level
		}
		l.Apply(*s)
	}
	return l.Grid, level, false, fmt.Errorf("logical solver did not converge")
}
