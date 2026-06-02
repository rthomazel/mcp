// Package config provides configuration parsing, defaults, and validation for the keys MCP server.
package config

import (
	"fmt"
	"net/url"
	"os"

	"go.yaml.in/yaml/v3"
)

const (
	// DefaultConfigPath is the default path to the YAML config file.
	// It is relative to the working directory; in the Docker image WORKDIR is /config.
	DefaultConfigPath = "config.yaml"
)

// SecretsDir is the directory Docker Secrets are mounted into.
// Override in tests via t.TempDir().
var SecretsDir = "/run/secrets"

// Config is the top-level config parsed from YAML.
type Config struct {
	TimeoutSeconds   int                     `yaml:"timeout_seconds"`
	MaxResponseBytes int64                   `yaml:"max_response_bytes"`
	MaxRequestBytes  int64                   `yaml:"max_request_bytes"`
	Secrets          map[string]SecretConfig `yaml:"secrets"`
	Tools            map[string]ToolConfig   `yaml:"mcp_tools"`
}

// SecretConfig defines a single secret source. Only docker_secret is supported in v1.
type SecretConfig struct {
	DockerSecret string `yaml:"docker_secret"`
}

// ToolConfig defines a single API tool.
type ToolConfig struct {
	Description string                  `yaml:"description"`
	Docs        []string                `yaml:"docs"`
	BaseURL     string                  `yaml:"base_url"`
	HTTP        bool                    `yaml:"http"`
	Inject      map[string]InjectConfig `yaml:"inject"`
}

// InjectConfig maps a header name to a secret reference and optional format string.
// Format uses {value} as the placeholder for the secret value, e.g. "Bearer {value}".
// If Format is empty, the raw secret value is used as-is.
type InjectConfig struct {
	Secret string `yaml:"secret"`
	Format string `yaml:"format"`
}

// Load reads the YAML file at path, applies defaults, and validates.
// Returns a non-nil error for any validation failure. The caller should treat
// any error as fatal — the server must not start with invalid config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 30
	}
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = 1048576
	}
	if cfg.MaxRequestBytes == 0 {
		cfg.MaxRequestBytes = 102400
	}

	if len(cfg.Secrets) == 0 {
		return nil, fmt.Errorf("secrets must not be empty")
	}
	if len(cfg.Tools) == 0 {
		return nil, fmt.Errorf("tools must not be empty")
	}

	for name, secret := range cfg.Secrets {
		if secret.DockerSecret == "" {
			return nil, fmt.Errorf("secret %q: docker_secret must not be empty", name)
		}
	}

	for toolName, tool := range cfg.Tools {
		if tool.Description == "" {
			return nil, fmt.Errorf("tool %q: description must not be empty", toolName)
		}
		if tool.BaseURL == "" {
			return nil, fmt.Errorf("tool %q: base_url must not be empty", toolName)
		}

		parsed, err := url.Parse(tool.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("tool %q: base_url invalid: %w", toolName, err)
		}

		// http:true allows http or https; http:false requires https.
		if !tool.HTTP && parsed.Scheme != "https" {
			return nil, fmt.Errorf("tool %q: base_url must use https scheme (set http: true to allow plain http)", toolName)
		}
		if tool.HTTP && parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("tool %q: base_url must use http or https scheme", toolName)
		}

		for headerName, inject := range tool.Inject {
			if inject.Secret == "" {
				return nil, fmt.Errorf("tool %q: inject %q: secret must not be empty", toolName, headerName)
			}
			if _, exists := cfg.Secrets[inject.Secret]; !exists {
				return nil, fmt.Errorf("tool %q: inject %q: secret %q not found in secrets", toolName, headerName, inject.Secret)
			}
		}
	}

	return cfg, nil
}
