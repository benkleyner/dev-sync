package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMigrateConfigPasswordsMovesLegacyPasswordToKeyring(t *testing.T) {
	keyring.MockInit()

	cfg := &Config{Pairs: []SyncPair{{Name: "prod", Password: "secret"}}}
	changed, err := migrateConfigPasswords(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected migration to report a change")
	}
	if cfg.Pairs[0].Password != "" {
		t.Fatalf("expected legacy password to be cleared, got %q", cfg.Pairs[0].Password)
	}
	if cfg.Pairs[0].PasswordRef != "sftp:prod" {
		t.Fatalf("unexpected password ref: %q", cfg.Pairs[0].PasswordRef)
	}

	password, err := keyring.Get(keyringService, "sftp:prod")
	if err != nil {
		t.Fatal(err)
	}
	if password != "secret" {
		t.Fatalf("unexpected keyring password: %q", password)
	}
}

func TestMigrateConfigPasswordsLeavesKeyringOnlyPairUnchanged(t *testing.T) {
	keyring.MockInit()

	cfg := &Config{Pairs: []SyncPair{{Name: "prod", PasswordRef: "sftp:prod"}}}
	changed, err := migrateConfigPasswords(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected keyring-only pair to remain unchanged")
	}
	if cfg.Pairs[0].PasswordRef != "sftp:prod" {
		t.Fatalf("unexpected password ref: %q", cfg.Pairs[0].PasswordRef)
	}
}

func TestPasswordForPairReadsKeyringSecret(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(keyringService, "sftp:prod", "secret"); err != nil {
		t.Fatal(err)
	}

	password, err := passwordForPair(SyncPair{Name: "prod", PasswordRef: "sftp:prod"})
	if err != nil {
		t.Fatal(err)
	}
	if password != "secret" {
		t.Fatalf("unexpected password: %q", password)
	}
}

func TestPasswordForPairReportsMissingKeyringSecret(t *testing.T) {
	keyring.MockInit()

	_, err := passwordForPair(SyncPair{Name: "prod", PasswordRef: "sftp:prod"})
	if err == nil {
		t.Fatal("expected missing keyring secret error")
	}
	want := `password for pair "prod" not found in keyring`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}

func TestLoadConfigWithMigratedSecretsRewritesLegacyConfig(t *testing.T) {
	keyring.MockInit()
	tempHome := t.TempDir()
	tempConfig := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempConfig)

	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{
 "pairs": [
  {
   "name": "prod",
   "local_dir": "/tmp/local",
   "host": "example.com",
   "port": 22,
   "user": "deploy",
   "remote_dir": "/srv/app",
   "password": "secret"
  }
 ]
}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfigWithMigratedSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Pairs) != 1 {
		t.Fatalf("expected one pair, got %d", len(cfg.Pairs))
	}
	if cfg.Pairs[0].PasswordRef != "sftp:prod" {
		t.Fatalf("unexpected password ref: %q", cfg.Pairs[0].PasswordRef)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"password_ref"`) {
		t.Fatalf("expected migrated config to contain password_ref: %s", content)
	}
	if strings.Contains(content, "secret") {
		t.Fatalf("expected migrated config to omit plaintext secret: %s", content)
	}
	if strings.Contains(content, `"password"`) {
		t.Fatalf("expected migrated config to omit legacy password key: %s", content)
	}
}
