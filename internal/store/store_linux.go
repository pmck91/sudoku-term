// Package store persists one game using ordered, durable atomic replacements.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"sudoku-term/internal/game"
)

type Store struct {
	path string
	lock *os.File
	mu   sync.Mutex
}

func DefaultPath() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "sudoku-term", "game.json"), nil
}
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create save directory: %w", err)
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another Sudoku instance is using this save location")
	}
	return &Store{path: path, lock: file}, nil
}
func (s *Store) Close() error { return s.lock.Close() }
func (s *Store) Load() (*game.Game, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var g game.Game
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&g); err != nil {
		return nil, fmt.Errorf("read save: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("unexpected data after save")
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("invalid save: %w", err)
	}
	return &g, nil
}
func (s *Store) Save(g *game.Game) error {
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return s.SaveBytes(data)
}

// SaveBytes accepts an immutable snapshot, so the UI can keep rendering safely.
func (s *Store) SaveBytes(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.path)
	f, err := os.CreateTemp(dir, ".game-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
