package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type PathMap struct {
	Local  string `yaml:"local,omitempty"`
	Remote string `yaml:"remote,omitempty"`
}

type Remote struct {
	Host    string  `yaml:"host"`
	PathMap PathMap `yaml:"path_map,omitempty"`
}

type Config struct {
	DefaultRemote string            `yaml:"default_remote,omitempty"`
	Remotes       map[string]Remote `yaml:"remotes,omitempty"`
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
