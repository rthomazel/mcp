package config_test

import (
	"os"
	"testing"

	"github.com/rthomazel/mcp/keys/internal/config"
	"github.com/stretchr/testify/require"
)

func validYAML() string {
	return `
timeout_seconds: 30
max_response_bytes: 1048576
max_request_bytes: 102400
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
    inject:
      Authorization:
        secret: my_secret
        format: "Bearer {value}"
`
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		createFile bool
		wantErr    bool
		check      func(*testing.T, *config.Config)
	}{
		{
			name:       "valid full config",
			yaml:       validYAML(),
			createFile: true,
			check: func(t *testing.T, cfg *config.Config) {
				require.Equal(t, 30, cfg.TimeoutSeconds)
				require.Equal(t, int64(1048576), cfg.MaxResponseBytes)
				require.Equal(t, int64(102400), cfg.MaxRequestBytes)
				require.Len(t, cfg.Secrets, 1)
				require.Len(t, cfg.Tools, 1)
			},
		},
		{
			name: "defaults applied",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
`,
			createFile: true,
			check: func(t *testing.T, cfg *config.Config) {
				require.Equal(t, 30, cfg.TimeoutSeconds)
				require.Equal(t, int64(1048576), cfg.MaxResponseBytes)
				require.Equal(t, int64(102400), cfg.MaxRequestBytes)
			},
		},
		{
			name:       "empty secrets block",
			yaml:       `secrets: {}\ntools:\n  t:\n    description: x\n    base_url: https://api.example.com\n`,
			createFile: true,
			wantErr:    true,
		},
		{
			name:       "empty tools block",
			yaml:       `secrets:\n  s:\n    docker_secret: s\ntools: {}\n`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "docker_secret name empty",
			yaml: `
secrets:
  my_secret:
    docker_secret: ""
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "description empty",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: ""
    base_url: "https://api.example.com"
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "base_url empty",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: ""
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "base_url contains path component",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com/v1"
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "base_url non-https without http:true",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "http://api.example.com"
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "base_url http with http:true",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "http://api.example.com"
    http: true
`,
			createFile: true,
		},
		{
			name: "base_url https with http:true",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
    http: true
`,
			createFile: true,
		},
		{
			name: "inject references unknown secret",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
    inject:
      Authorization:
        secret: unknown_secret
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name: "inject secret field empty",
			yaml: `
secrets:
  my_secret:
    docker_secret: secret_name
tools:
  my_tool:
    description: "A tool"
    base_url: "https://api.example.com"
    inject:
      Authorization:
        secret: ""
`,
			createFile: true,
			wantErr:    true,
		},
		{
			name:       "malformed YAML",
			yaml:       `{invalid yaml: [`,
			createFile: true,
			wantErr:    true,
		},
		{
			name:       "file not found",
			createFile: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := dir + "/config.yaml"

			if tt.createFile {
				require.NoError(t, os.WriteFile(filePath, []byte(tt.yaml), 0o644))
			}

			cfg, err := config.Load(filePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}
