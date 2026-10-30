package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"sudoku-term/internal/store"
	"sudoku-term/internal/tui"
)

// Release builds set this with -ldflags "-X main.version=...".
var version = "dev"

func run() error {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: sudoku [--version]\n\nA quiet, keyboard-driven Sudoku game.\n\nArrows / hjkl move · 1–9 enter · n notes · i hint · ? help · q quit\n\nOne game is saved automatically in $XDG_STATE_HOME/sudoku-term/game.json\n(default: ~/.local/state/sudoku-term/game.json).")
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("sudoku", version)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q; use sudoku --help", flag.Arg(0))
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("an interactive terminal is required; use sudoku --help for controls")
	}
	path, err := store.DefaultPath()
	if err != nil {
		return err
	}
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	g, loadErr := s.Load()
	m := tui.New(g, s, loadErr)
	p := tea.NewProgram(m, tea.WithoutSignalHandler())
	// Route operating-system shutdown through the same ordered save path as q.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-signals:
			p.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		case <-done:
		}
	}()
	_, err = p.Run()
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sudoku:", err)
		os.Exit(1)
	}
}
