package puzzle

import (
	"context"
	"fmt"
	"math/rand/v2"
)

type Puzzle struct {
	Givens     Grid
	Solution   Grid
	Difficulty Difficulty
}

// Generate searches fresh grids until one matches the requested logical tier.
// The caller owns the RNG; cancellation never returns a partial puzzle.
func Generate(ctx context.Context, level Difficulty, rng *rand.Rand) (Puzzle, error) {
	if level > Master {
		return Puzzle{}, fmt.Errorf("unknown difficulty %d", level)
	}
	for {
		if err := ctx.Err(); err != nil {
			return Puzzle{}, err
		}
		solution, err := randomSolution(ctx, rng)
		if err != nil {
			return Puzzle{}, err
		}
		g := solution
		for _, i := range rng.Perm(81) {
			previous := g[i]
			g[i] = 0
			count, _, err := CountSolutions(ctx, g, 2)
			if err != nil {
				return Puzzle{}, err
			}
			if count != 1 {
				g[i] = previous
				continue
			}
			// Early grids are almost always singles: defer expensive grading.
			clues := 0
			for _, v := range g {
				if v != 0 {
					clues++
				}
			}
			if clues > 40 {
				continue
			}
			_, rating, solved, err := SolveLogic(ctx, g, level)
			if err != nil {
				return Puzzle{}, err
			}
			if solved && rating == level {
				// Easy puzzles with too many clues feel nearly complete.
				if level != Easy || clues <= 34 {
					return Puzzle{g, solution, level}, nil
				}
			}
			if !solved {
				g[i] = previous
			}
		}
	}
}
