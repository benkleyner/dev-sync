//go:build windows

package main

import (
	"errors"
	"os/exec"
)

var errDaemonControlsUnsupported = errors.New("background daemon control is not supported on Windows")

func daemonControlsSupported() bool { return false }

func configureDaemonCommand(cmd *exec.Cmd) error {
	return errDaemonControlsUnsupported
}

func processAlive(pid int) bool { return false }

func processMatchesExecutable(pid int, expected string) bool { return false }
