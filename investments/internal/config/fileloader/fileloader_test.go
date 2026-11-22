package fileloader

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Name string `yaml:"name"`
}

func validateSample(s sample) error {
	if s.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

func writeFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestCurrentReadsAValidFile(t *testing.T) {
	got, err := New(writeFile(t, "name: a"), validateSample).Current()

	require.NoError(t, err)
	assert.Equal(t, sample{Name: "a"}, got)
}

func TestCurrentTreatsAMissingFileAndAnEmptyPathAsUnconfigured(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "absent.yaml")} {
		got, err := New(path, validateSample).Current()

		require.NoError(t, err, path)
		assert.Equal(t, sample{}, got, path)
	}
}

func TestCurrentKeepsTheLastValidResultWhenAnEditBreaksTheFile(t *testing.T) {
	path := writeFile(t, "name: a")
	loader := New(path, validateSample)

	_, err := loader.Current()
	require.NoError(t, err)

	for _, broken := range []string{"name: [", "other: x"} {
		require.NoError(t, os.WriteFile(path, []byte(broken), 0o600))

		got, err := loader.Current()
		require.NoError(t, err, broken)
		assert.Equal(t, sample{Name: "a"}, got, broken)
	}

	fresh, err := New(path, validateSample).Current()
	assert.Error(t, err)
	assert.Equal(t, sample{}, fresh)
}

func TestCurrentForgetsTheLastValidResultWhenTheFileIsRemoved(t *testing.T) {
	path := writeFile(t, "name: a")
	loader := New(path, validateSample)

	_, err := loader.Current()
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	got, err := loader.Current()
	require.NoError(t, err)
	assert.Equal(t, sample{}, got)

	require.NoError(t, os.WriteFile(path, []byte("other: x"), 0o600))

	_, err = loader.Current()
	assert.Error(t, err)
}
