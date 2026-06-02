// Package secrets provides loading and retrieval of secret values from Docker Secrets.
package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rthomazel/mcp/keys/internal/config"
)

// Store holds resolved secret values, keyed by the name defined in config.
type Store struct {
	values map[string]string
}

// Load resolves all secrets defined in cfg.
// Reads each docker_secret from /run/secrets/<name>.
// Trims all leading/trailing whitespace from values.
// Returns an error if any secret file cannot be read or if the trimmed value is empty.
// The caller should treat any error as fatal.
func Load(cfg map[string]config.SecretConfig) (*Store, error) {
	values := make(map[string]string)

	for name, secret := range cfg {
		filePath := filepath.Join(config.SecretsDir, secret.DockerSecret)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read secret %q: %w", name, err)
		}

		value := strings.TrimSpace(string(data))
		if value == "" {
			return nil, fmt.Errorf("secret %q: trimmed value is empty", name)
		}

		values[name] = value
	}

	return &Store{values: values}, nil
}

// Get returns the resolved value for secretName.
// Panics with a clear message if secretName is not found — this should never happen
// after a successful Load, since config validation ensures all inject references exist.
func (s *Store) Get(secretName string) string {
	value, ok := s.values[secretName]
	if !ok {
		panic(fmt.Sprintf("secret %q not found in store", secretName))
	}
	return value
}

// Names returns all secret names in sorted order. Used for safe startup logging (names only, never values).
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.values))
	for name := range s.values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Values returns all secret values. Used internally for response scrubbing. Never log or expose these.
func (s *Store) Values() []string {
	vals := make([]string, 0, len(s.values))
	for _, value := range s.values {
		vals = append(vals, value)
	}
	sort.Strings(vals)
	return vals
}

// NewStoreForTest creates a Store directly from a values map.
// For use in tests only — bypasses file loading.
func NewStoreForTest(values map[string]string) *Store {
	return &Store{values: values}
}
