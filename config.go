package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	Pairs []SyncPair `json:"pairs"`
}

type SyncPair struct {
	Name      string `json:"name"`
	LocalDir  string `json:"local_dir"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	RemoteDir string `json:"remote_dir"`
	Password  string `json:"password"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dev-sync", "config.json"), nil
}

func pidFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dev-sync", "daemon.pid"), nil
}

func logFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dev-sync", "daemon.log"), nil
}

func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

func SaveConfig(c *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) FindPair(name string) *SyncPair {
	for i := range c.Pairs {
		if c.Pairs[i].Name == name {
			return &c.Pairs[i]
		}
	}
	return nil
}
