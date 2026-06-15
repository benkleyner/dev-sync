package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPidfileRoundTripsDaemonState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	startedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	want := daemonPidfile{PID: 1234, Executable: "/usr/local/bin/dev-sync", StartedAt: startedAt}
	if err := writePidfile(want); err != nil {
		t.Fatal(err)
	}

	got, err := readPidfile()
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != want.PID || got.Executable != want.Executable || !got.StartedAt.Equal(want.StartedAt) {
		t.Fatalf("unexpected pidfile state: %#v", got)
	}
}

func TestReadPidfileAcceptsLegacyPlainPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := pidFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("1234\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readPidfile()
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != 1234 {
		t.Fatalf("expected legacy pid 1234, got %#v", got)
	}
}
