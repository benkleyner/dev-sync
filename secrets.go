package main

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "dev-sync"
const sftpPasswordRefPrefix = "sftp:"

func passwordRefForPairName(name string) string {
	return sftpPasswordRefPrefix + name
}

func storePairPassword(pairName, password string) (string, error) {
	ref := passwordRefForPairName(pairName)
	if err := keyring.Set(keyringService, ref, password); err != nil {
		return "", fmt.Errorf("store password in keyring: %w", err)
	}
	return ref, nil
}

func passwordForPair(p SyncPair) (string, error) {
	ref := p.PasswordRef
	if ref == "" {
		ref = passwordRefForPairName(p.Name)
	}

	password, err := keyring.Get(keyringService, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("password for pair %q not found in keyring", p.Name)
	}
	if err != nil {
		return "", fmt.Errorf("get password for pair %q from keyring: %w", p.Name, err)
	}
	return password, nil
}

func migrateConfigPasswords(cfg *Config) (bool, error) {
	changed := false
	for i := range cfg.Pairs {
		p := &cfg.Pairs[i]
		if p.Password == "" {
			continue
		}

		ref := p.PasswordRef
		if ref == "" {
			ref = passwordRefForPairName(p.Name)
		}
		if err := keyring.Set(keyringService, ref, p.Password); err != nil {
			return false, fmt.Errorf("migrate password for pair %q: %w", p.Name, err)
		}
		p.PasswordRef = ref
		p.Password = ""
		changed = true
	}
	return changed, nil
}

func loadConfigWithMigratedSecrets() (*Config, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	changed, err := migrateConfigPasswords(cfg)
	if err != nil {
		return nil, fmt.Errorf("migrate config secrets: %w", err)
	}
	if changed {
		if err := SaveConfig(cfg); err != nil {
			return nil, fmt.Errorf("save migrated config: %w", err)
		}
	}
	return cfg, nil
}
