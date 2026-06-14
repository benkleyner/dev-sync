package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestAppendKnownHostLineCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssh", "known_hosts")

	if err := appendKnownHostLine(path, "example.com ssh-ed25519 AAAA"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "example.com ssh-ed25519 AAAA\n" {
		t.Fatalf("unexpected known_hosts content: %q", string(data))
	}
}

func TestAppendKnownHostLinePreservesExistingLineWithoutTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(path, []byte("existing ssh-ed25519 AAAA"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendKnownHostLine(path, "example.com ssh-ed25519 BBBB"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "existing ssh-ed25519 AAAA\nexample.com ssh-ed25519 BBBB\n"
	if string(data) != want {
		t.Fatalf("unexpected known_hosts content: %q", string(data))
	}
}

func TestLoadKnownHostsAcceptsKnownHost(t *testing.T) {
	key := newTestPublicKey(t)
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(knownhosts.Line([]string{"example.com:22"}, key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	callback, err := loadKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := callback("example.com:22", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key); err != nil {
		t.Fatalf("known host rejected: %v", err)
	}
}

func TestLoadKnownHostsMissingFileRejectsUnknownHost(t *testing.T) {
	key := newTestPublicKey(t)
	callback, err := loadKnownHosts(filepath.Join(t.TempDir(), "missing_known_hosts"))
	if err != nil {
		t.Fatal(err)
	}

	err = callback("example.com:22", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key)
	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("expected KeyError, got %T: %v", err, err)
	}
	if len(keyErr.Want) != 0 {
		t.Fatalf("expected unknown host, got %d wanted keys", len(keyErr.Want))
	}
}

func newTestPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return publicKey
}
