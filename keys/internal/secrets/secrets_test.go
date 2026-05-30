package secrets_test

import (
	"os"
	"testing"

	"github.com/rthomazel/mcp/keys/internal/config"
	"github.com/rthomazel/mcp/keys/internal/secrets"
	"github.com/stretchr/testify/require"
)

func makeStore(t *testing.T, values map[string]string) *secrets.Store {
	t.Helper()
	tmpDir := t.TempDir()
	oldDir := secrets.SecretsDir
	secrets.SecretsDir = tmpDir
	t.Cleanup(func() { secrets.SecretsDir = oldDir })

	cfg := make(map[string]config.SecretConfig)
	for name, value := range values {
		require.NoError(t, os.WriteFile(tmpDir+"/"+name, []byte(value), 0o600))
		cfg[name] = config.SecretConfig{DockerSecret: name}
	}

	store, err := secrets.Load(cfg)
	require.NoError(t, err)
	return store
}

func TestLoad(t *testing.T) {
	t.Run("reads file and trims trailing newline", func(t *testing.T) {
		store := makeStore(t, map[string]string{"tok": "myvalue\n"})
		require.Equal(t, "myvalue", store.Get("tok"))
	})

	t.Run("trims crlf and surrounding spaces", func(t *testing.T) {
		store := makeStore(t, map[string]string{"tok": "  myvalue  \r\n"})
		require.Equal(t, "myvalue", store.Get("tok"))
	})

	t.Run("empty value after trim returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldDir := secrets.SecretsDir
		secrets.SecretsDir = tmpDir
		t.Cleanup(func() { secrets.SecretsDir = oldDir })

		require.NoError(t, os.WriteFile(tmpDir+"/tok", []byte("   \n"), 0o600))
		_, err := secrets.Load(map[string]config.SecretConfig{"tok": {DockerSecret: "tok"}})
		require.Error(t, err)
	})

	t.Run("missing file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldDir := secrets.SecretsDir
		secrets.SecretsDir = tmpDir
		t.Cleanup(func() { secrets.SecretsDir = oldDir })

		_, err := secrets.Load(map[string]config.SecretConfig{"tok": {DockerSecret: "nonexistent"}})
		require.Error(t, err)
	})
}

func TestStoreGet(t *testing.T) {
	t.Run("known key returns value", func(t *testing.T) {
		store := makeStore(t, map[string]string{"key": "val"})
		require.Equal(t, "val", store.Get("key"))
	})

	t.Run("unknown key panics with key name in message", func(t *testing.T) {
		store := makeStore(t, map[string]string{"key": "val"})
		require.PanicsWithValue(t, `secret "unknown" not found in store`, func() {
			store.Get("unknown")
		})
	})
}

func TestStoreNames(t *testing.T) {
	store := makeStore(t, map[string]string{"zebra": "z", "apple": "a", "mango": "m"})
	require.Equal(t, []string{"apple", "mango", "zebra"}, store.Names())
}

func TestStoreValues(t *testing.T) {
	store := makeStore(t, map[string]string{"a": "val_a", "b": "val_b", "c": "val_c"})
	require.Equal(t, []string{"val_a", "val_b", "val_c"}, store.Values())
}
