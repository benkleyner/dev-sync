package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type daemonPidfile struct {
	PID        int       `json:"pid"`
	Executable string    `json:"executable,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
}

func runDaemonSubcommand() int {
	if err := setupDaemonLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon logger: %v\n", err)
		return 1
	}
	defer removePidfile()
	if err := runDaemon(); err != nil {
		slog.Error("daemon exited", "err", err)
		return 1
	}
	return 0
}

func runStart() error {
	if !daemonControlsSupported() {
		return errDaemonControlsUnsupported
	}

	cfg, err := loadConfigWithMigratedSecrets()
	if err != nil {
		return err
	}
	if len(cfg.Pairs) == 0 {
		return fmt.Errorf("no pairs configured; run `dev-sync init`")
	}

	state, _ := readPidfile()
	if state.PID > 0 {
		if processAlive(state.PID) && processMatchesExecutable(state.PID, state.Executable) {
			return fmt.Errorf("daemon already running (pid %d)", state.PID)
		}
		removePidfile()
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	logPath, err := logFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return err
	}

	cmd := exec.Command(exe, "_daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := configureDaemonCommand(cmd); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	pid := cmd.Process.Pid
	if err := writePidfile(daemonPidfile{
		PID:        pid,
		Executable: exe,
		StartedAt:  time.Now(),
	}); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		removePidfile()
		return fmt.Errorf("release child: %w", err)
	}

	if err := waitForDaemonStartup(pid); err != nil {
		removePidfile()
		return err
	}

	fmt.Printf("dev-sync started (pid %d)\n", pid)
	fmt.Printf("logs: %s\n", logPath)
	return nil
}

func runStop() error {
	if !daemonControlsSupported() {
		return errDaemonControlsUnsupported
	}

	state, err := readPidfile()
	if err != nil {
		return err
	}
	if state.PID <= 0 {
		return fmt.Errorf("not running")
	}
	if !processAlive(state.PID) {
		removePidfile()
		return fmt.Errorf("pid %d not running (stale pidfile removed)", state.PID)
	}
	if !processMatchesExecutable(state.PID, state.Executable) {
		removePidfile()
		return fmt.Errorf("pid %d belongs to a different process (stale pidfile removed)", state.PID)
	}

	proc, err := os.FindProcess(state.PID)
	if err != nil {
		return err
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("signal: %w", err)
	}

	for i := 0; i < 30; i++ {
		if !processAlive(state.PID) {
			removePidfile()
			fmt.Printf("stopped (pid %d)\n", state.PID)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("pid %d did not exit within 3s", state.PID)
}

func runStatus() error {
	if !daemonControlsSupported() {
		return errDaemonControlsUnsupported
	}

	state, _ := readPidfile()
	if state.PID <= 0 || !processAlive(state.PID) {
		removePidfile()
		fmt.Println(styleBad.Render("stopped"))
		return nil
	}
	if !processMatchesExecutable(state.PID, state.Executable) {
		removePidfile()
		fmt.Println(styleBad.Render("stopped"))
		return nil
	}
	fmt.Printf("%s (pid %d)\n", styleOK.Render("running"), state.PID)
	return nil
}

func runLogs(args []string) error {
	n := 50
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 {
			n = v
		}
	}

	path, err := logFilePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Println("no logs yet")
		return nil
	}
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		printLogLine(line)
	}
	return nil
}

func printLogLine(line string) {
	if line == "" {
		return
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		fmt.Println(line)
		return
	}
	ts := stringField(entry, "time")
	level := stringField(entry, "level")
	msg := stringField(entry, "msg")

	short := ts
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		short = t.Local().Format("15:04:05")
	}

	var extras []string
	for k, v := range entry {
		switch k {
		case "time", "level", "msg":
		case "pair":
			extras = append(extras, fmt.Sprintf("%s=%s",
				styleKey.Render(k),
				stylePair.Render(fmt.Sprintf("%v", v))))
		case "path":
			extras = append(extras, fmt.Sprintf("%s=%s",
				styleKey.Render(k),
				stylePath.Render(fmt.Sprintf("%v", v))))
		default:
			extras = append(extras, fmt.Sprintf("%s=%v", styleKey.Render(k), v))
		}
	}

	levelStyled := styleLevel(level).Render(fmt.Sprintf("%-5s", level))
	timeStyled := styleTime.Render(short)

	if len(extras) > 0 {
		fmt.Printf("%s %s %-20s %s\n", timeStyled, levelStyled, msg, strings.Join(extras, " "))
	} else {
		fmt.Printf("%s %s %s\n", timeStyled, levelStyled, msg)
	}
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func readPidfile() (daemonPidfile, error) {
	path, err := pidFilePath()
	if err != nil {
		return daemonPidfile{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return daemonPidfile{}, nil
	}
	if err != nil {
		return daemonPidfile{}, err
	}

	var state daemonPidfile
	if err := json.Unmarshal(data, &state); err == nil && state.PID > 0 {
		return state, nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return daemonPidfile{}, err
	}
	return daemonPidfile{PID: pid}, nil
}

func writePidfile(state daemonPidfile) error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func removePidfile() {
	path, err := pidFilePath()
	if err != nil {
		return
	}
	os.Remove(path)
}

func waitForDaemonStartup(pid int) error {
	const startupWindow = 750 * time.Millisecond
	deadline := time.Now().Add(startupWindow)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return fmt.Errorf("daemon exited during startup; check logs")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func setupDaemonLogger() error {
	path, err := logFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	return nil
}
