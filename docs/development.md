# Development and releases

## Build and test

Use Linux and Go 1.26 or newer. Run commands from the repository root.

```sh
make build                              # Build ./sudoku
make run                                # Run from source
make check                              # Formatting and go vet
make test                               # go test -race ./...
python3 scripts/pty_smoke.py ./sudoku     # Requires a built executable
```

The terminal smoke test uses Python's standard library and a temporary save
directory. It exercises new games, notes, wrong answers, resume, undo/redo,
hints, resizing, quit, SIGTERM, and terminal restoration.

Local builds report `dev` with `./sudoku --version`. Override it with
`make build VERSION=0.1.0`.

## Code layout

| Package | Responsibility |
| --- | --- |
| [`internal/puzzle`](../internal/puzzle) | Grids, candidates, generation, solution counting, and logical deductions |
| [`internal/game`](../internal/game) | Player actions, notes, hints, undo/redo, and state validation |
| [`internal/store`](../internal/store) | Versioned JSON saves, atomic replacement, and the Linux save lock |
| [`internal/tui`](../internal/tui) | Bubble Tea event loop, layout, input, timer, and save queue |
| [`cmd/sudoku`](../cmd/sudoku) | CLI flags, startup, and shutdown signals |

Algorithm details: [generation](generator.md), [solving](solver.md), and [hints](hints.md).

## Layout snapshots

Golden files in `internal/tui/testdata` cover compact, expanded, hint, error,
home, help, and resize screens. Tests also check that the rendered output fits
the configured terminal dimensions.

Compact cells use three columns and one row. Expanded cells use seven columns
and three rows, with spaces between pencil-mark digits. Including the border
spacing, both layouts use two columns for each row to account for typical
terminal characters being about twice as tall as they are wide. Expanded mode
requires at least 98×42; other supported sizes use the compact board.

After reviewing an intentional layout change, regenerate the snapshots:

```sh
UPDATE_GOLDEN=1 go test ./internal/tui -run TestLayoutSnapshots
```

## Save behavior

The app queues an immutable JSON snapshot after each cell edit, undo/redo, or
applied hint. Writes run in order. **Saved** appears after the queue drains;
quitting waits for pending writes and saves session state such as cursor and time.

The path is `$XDG_STATE_HOME/sudoku-term/game.json`. An unset, empty, or relative
`XDG_STATE_HOME` uses `~/.local/state` instead. The app creates its directory with
mode 0700 and save files with mode 0600. An advisory lock prevents competing writers.

Each write synchronizes a temporary file, renames it over the save, then
synchronizes the directory. A write failure pauses editing and offers Retry or
an explicit quit without saving pending changes. Abrupt termination can lose
updates that have not reached disk.

Invalid or unsupported saves remain in place until the player explicitly
chooses to replace them. Completed games remain available for review until replaced.

## Continuous integration

[`ci.yml`](../.github/workflows/ci.yml) runs on every pull request and every push
to `main`. It runs formatting checks, vet, race-enabled tests, a build, and the
terminal smoke test. Successful runs attach a Linux amd64 archive, retained for
seven days. The release workflow also calls this workflow against its tagged commit.

## Releases

[`release.yml`](../.github/workflows/release.yml) runs when a `v*` tag is pushed.
It publishes only after CI succeeds. Use `v0.1.0` for a stable release or
`v0.2.0-rc.1` for a prerelease. The built-in GitHub token publishes the release;
no additional secret is required.

Tag a commit containing the workflows and push the tag:

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

[`package-release.sh`](../scripts/package-release.sh) validates the version tag,
builds Linux amd64 and arm64 binaries with CGO disabled, and packages each binary
with the README and LICENSE. It embeds the version without the leading `v` in `--version`.

Release assets are:

- `sudoku_<tag>_linux_amd64.tar.gz`
- `sudoku_<tag>_linux_arm64.tar.gz`
- `checksums.txt`, containing SHA-256 hashes of both archives

GitHub generates the release notes. Tags with a prerelease suffix are marked as
prereleases and do not become the latest stable release.

Test packaging locally without publishing:

```sh
bash scripts/package-release.sh v0.1.0
(cd dist && sha256sum -c checksums.txt)
```
