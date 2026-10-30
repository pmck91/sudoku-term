# Sudoku

A quiet Sudoku game for the Linux terminal. Fresh puzzles, pencil notes,
hints, and a game that's waiting where you left it.

## Run

Requires **Linux**, **Go 1.26 or newer**, and an interactive terminal at least
**80 columns × 24 rows**.

```sh
go run ./cmd/sudoku
```

Or build a standalone executable:

```sh
make build
./sudoku
```

The first build downloads dependencies. Playing and generating puzzles work
entirely offline. No special fonts are needed. Colors adapt to light and dark
terminals; set `NO_COLOR=1` for a layout with a bracketed selection and no colors.

## Playing

Use **Resume game** or choose a difficulty under **New game**. Every row,
column, and 3×3 box must contain the digits 1–9 exactly once.

| Key | Action |
| --- | --- |
| Arrows or `h j k l` | Move one cell, stopping at board edges |
| `1`–`9` | Enter or replace an answer; toggle a note in Notes mode |
| `n` | Switch between Answer and Notes modes |
| Backspace, Delete, or `0` | Clear the selected editable cell |
| `u` / Ctrl+R | Undo / redo |
| `i` | Explain and highlight a logical hint |
| Enter / Esc while viewing a hint | Apply / dismiss the hint |
| `t` | Show or hide the timer |
| `?` | Help |
| Esc | Pause menu / return |
| `q` or Ctrl+C | Save and quit |

Starting clues cannot be changed. Notes are entered only in empty cells.
Incorrect answers appear red and underlined, even if they do not duplicate
another digit. They remain editable, and there are no penalties or mistake limits.

A correct answer removes matching notes in its row, column, and box. Undo
restores the answer and all affected notes together. New edits discard redo history.

At 80×24 the board shows a dot for cells containing notes; the selected cell's
complete pencil notes appear beside it. At 80×42 or larger, every cell has room
for its own 3×3 pencil notes. Smaller terminals show a resize message and pause play.

The optional timer measures active play. Menus, help, hints, resizing below the
minimum, and time outside the app are excluded. Losing terminal focus pauses it
when the terminal reports focus events. Hiding the timer does not stop it.
You can review a completed board and undo the final move to continue playing.

## Difficulties and hints

Each puzzle is freshly generated with a randomized search, verified to have
exactly one solution, and graded by the logical techniques required by the app's
solver. The levels are specific to this game, rather than an industry standard.

| Level | Techniques added to earlier levels |
| --- | --- |
| Easy | Naked and hidden singles |
| Medium | Locked candidates; naked and hidden pairs |
| Hard | Naked and hidden triples; X-Wing |
| Master | XY-Wing, Swordfish, simple coloring |

The generator keeps searching until a puzzle matches the requested level.
Generation times vary; Esc cancels without replacing the previous game. There
is no puzzle-bank fallback and no guessing in the logical solving path.

Hints use the same deductions as grading. They explain one step before changing
anything. Enter applies it as one undoable action. Eliminations update notes;
if an affected cell has no notes, its remaining logical candidates are populated.
Manual notes are annotations, not constraints on the hint solver. Applied logical
eliminations persist across restarts and are rebuilt when an answer changes.
Wrong answers must be corrected before requesting another logical hint.

## Saving

There is **one saved game**. Every cell edit, undo/redo, and applied hint queues
its own atomic save. Quitting waits for pending writes and also saves the cursor,
input mode, timer preference, and elapsed time. Undo/redo history survives restarts.
**Saved** appears only after the queued writes have completed.

The save is `$XDG_STATE_HOME/sudoku-term/game.json`, or
`~/.local/state/sudoku-term/game.json` when that variable is unset, empty, or
relative. The directory is created with mode 0700 and saves with mode 0600.
An advisory lock prevents two running instances from overwriting one another.

Writes synchronize a temporary file, rename it over the save, then synchronize
the directory. On a write failure, editing pauses: retry with `r` or explicitly
quit without saving pending changes with `q`. Progress since the last successful
save can be lost on abrupt process termination or a failed write.

An invalid or unsupported save is preserved until you explicitly choose to
replace it. Starting a new game replaces an unfinished game only after confirmation,
successful generation, and a successful save. A completed game remains available
for review until replaced.

## Development

The implementation separates the logical puzzle engine, reversible game actions,
atomic Linux storage, and the Bubble Tea / Lip Gloss presentation. The versioned
JSON format is internal to the app; unsupported versions are never silently loaded.

```sh
make test       # Race-enabled solver, generation, game, storage, and UI tests
make check      # Vet and formatting checks
make build
python3 scripts/pty_smoke.py ./sudoku
```

UI golden files cover compact, expanded, hint, error, home, help, and resize
screens. After reviewing an intentional layout change, regenerate them with:

```sh
UPDATE_GOLDEN=1 go test ./internal/tui -run TestLayoutSnapshots
```
