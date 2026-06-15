//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

var errDaemonControlsUnsupported = errors.New("background daemon control is not supported on this platform")

func daemonControlsSupported() bool { return true }

func configureDaemonCommand(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return nil
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func processMatchesExecutable(pid int, expected string) bool {
	if expected == "" {
		return true
	}

	actual, err := processExecutable(pid)
	if err != nil {
		return false
	}

	if samePath(actual, expected) {
		return true
	}
	return filepath.Base(actual) == filepath.Base(expected)
}

func processExecutable(pid int) (string, error) {
	if runtime.GOOS == "linux" {
		return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	}

	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return "", fmt.Errorf("inspect process %d: %w", pid, err)
	}
	cmd := strings.TrimSpace(string(out))
	if cmd == "" {
		return "", fmt.Errorf("process %d has no command", pid)
	}
	return cmd, nil
}

func samePath(a, b string) bool {
	ai, aErr := os.Stat(a)
	bi, bErr := os.Stat(b)
	if aErr == nil && bErr == nil {
		return os.SameFile(ai, bi)
	}
	return a == b
}
