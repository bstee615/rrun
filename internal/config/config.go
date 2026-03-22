package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration for clean YAML marshaling (e.g. "2s", "1m30s").
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q (use e.g. '2s', '1m30s'): %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	if d.Duration == 0 {
		return nil, nil
	}
	return d.String(), nil
}

func (d Duration) IsZero() bool { return d.Duration == 0 }

// RetryConfig controls retry behavior for transient network errors.
type RetryConfig struct {
	MaxAttempts     int      `yaml:"max_attempts,omitempty"`
	InitialInterval Duration `yaml:"initial_interval,omitempty"`
	MaxInterval     Duration `yaml:"max_interval,omitempty"`
	Multiplier      float64  `yaml:"multiplier,omitempty"`
}

// WithDefaults fills unset fields with sensible defaults.
func (r RetryConfig) WithDefaults() RetryConfig {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.InitialInterval.IsZero() {
		r.InitialInterval = Duration{2 * time.Second}
	}
	if r.MaxInterval.IsZero() {
		r.MaxInterval = Duration{30 * time.Second}
	}
	if r.Multiplier == 0 {
		r.Multiplier = 2.0
	}
	return r
}

// PathMap maps a local path prefix to a remote path prefix.
type PathMap struct {
	Local  string `yaml:"local,omitempty"`
	Remote string `yaml:"remote,omitempty"`
}

// Remote represents a named remote machine.
type Remote struct {
	Host    string  `yaml:"host"`
	PathMap PathMap `yaml:"path_map,omitempty"`
}

// Config is the top-level rrun configuration (~/.config/rrun/config.yaml).
type Config struct {
	DefaultRemote       string            `yaml:"default_remote,omitempty"`
	Remotes             map[string]Remote `yaml:"remotes,omitempty"`
	NoState             bool              `yaml:"no_state,omitempty"`
	Quiet               bool              `yaml:"quiet,omitempty"`
	LogPath             string            `yaml:"log_path,omitempty"`
	LargeTransferWarnMB int               `yaml:"large_transfer_warn_mb,omitempty"` // 0=default(100), -1=off
	Retry               RetryConfig       `yaml:"retry,omitempty"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rrun", "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Remotes: make(map[string]Remote)}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Remotes == nil {
		cfg.Remotes = make(map[string]Remote)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
