package fileloader

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"go.yaml.in/yaml/v3"
)

// Loader reads one YAML file into T and remembers the last valid result.
type Loader[T any] struct {
	path     string
	validate func(T) error
	mu       sync.Mutex
	lastGood *T
}

// New builds a Loader; an empty path means unconfigured.
func New[T any](path string, validate func(T) error) *Loader[T] {
	return &Loader[T]{path: path, validate: validate}
}

// Current reads the file on every call. A missing file is the zero T. An
// invalid file is logged on every call; the last valid result is served when
// there is one, otherwise the error is returned.
func (l *Loader[T]) Current() (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var zero T

	if l.path == "" {
		l.lastGood = nil

		return zero, nil
	}

	raw, err := os.ReadFile(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		l.lastGood = nil

		return zero, nil
	}

	var cfg T

	if err == nil {
		if err = yaml.Unmarshal(raw, &cfg); err != nil {
			err = fmt.Errorf("parsing %s: %w", l.path, err)
		} else if err = l.validate(cfg); err != nil {
			err = fmt.Errorf("invalid %s: %w", l.path, err)
		}
	} else {
		err = fmt.Errorf("reading %s: %w", l.path, err)
	}

	if err != nil {
		slog.Error("configuration file is invalid", "path", l.path, "err", err)

		if l.lastGood != nil {
			return *l.lastGood, nil
		}

		return zero, err
	}

	l.lastGood = &cfg

	return cfg, nil
}
