package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func defaultKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func strictKnownHostsCallback() (ssh.HostKeyCallback, error) {
	path, err := defaultKnownHostsPath()
	if err != nil {
		return nil, err
	}
	return loadKnownHosts(path)
}

func promptingKnownHostsCallback() (ssh.HostKeyCallback, error) {
	path, err := defaultKnownHostsPath()
	if err != nil {
		return nil, err
	}
	callback, err := loadKnownHosts(path)
	if err != nil {
		return nil, err
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
			return fmt.Errorf("verify host key for %s: %w", hostname, err)
		}

		if err := promptTrustHostKey(path, hostname, key); err != nil {
			return err
		}
		return nil
	}, nil
}

func loadKnownHosts(path string) (ssh.HostKeyCallback, error) {
	callback, err := knownhosts.New(path)
	if errors.Is(err, os.ErrNotExist) {
		return knownhosts.New()
	}
	return callback, err
}

func promptTrustHostKey(knownHostsPath, hostname string, key ssh.PublicKey) error {
	accept := false
	description := fmt.Sprintf(
		"Host: %s\nKey type: %s\nFingerprint: %s\n\nAdd this host key to %s?",
		knownhosts.Normalize(hostname),
		key.Type(),
		ssh.FingerprintSHA256(key),
		knownHostsPath,
	)

	if err := huh.NewConfirm().
		Title("Unknown SFTP host key").
		Description(description).
		Affirmative("Add").
		Negative("Cancel").
		Value(&accept).
		Run(); err != nil {
		return err
	}
	if !accept {
		return fmt.Errorf("unknown host key for %s was not trusted", hostname)
	}

	line := knownhosts.Line([]string{hostname}, key)
	if err := appendKnownHostLine(knownHostsPath, line); err != nil {
		return fmt.Errorf("add host key to known_hosts: %w", err)
	}

	fmt.Printf("%s added %s to %s\n", styleOK.Render("✓"), knownhosts.Normalize(hostname), knownHostsPath)
	return nil
}

func appendKnownHostLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 0 {
		if _, err := f.Seek(info.Size()-1, io.SeekStart); err != nil {
			return err
		}
		last := make([]byte, 1)
		if _, err := f.Read(last); err != nil {
			return err
		}
		if last[0] != '\n' {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}

	_, err = f.WriteString(line + "\n")
	return err
}
