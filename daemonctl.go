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
	"syscall"
	"time"
)

func runStart() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Pairs) == 0 {
		return fmt.Errorf("no pairs configured; run `dev-sync init`")
	}

	if pid, _ := readPidfile(); pid > 0 && processAlive(pid) {
		return fmt.Errorf("daemon already running (pid %d)", pid)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release child: %w", err)
	}

	if err := writePidfile(cmd.Process.Pid); err != nil {
		return err
	}

	fmt.Printf("dev-sync started (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("logs: %s\n", logPath)
	return nil
}

func runStop() error {
	pid, err := readPidfile()
	if err != nil {
		return err
	}
	if pid <= 0 {
		return fmt.Errorf("not running")
	}
	if !processAlive(pid) {
		removePidfile()
		return fmt.Errorf("pid %d not running (stale pidfile removed)", pid)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("signal: %w", err)
	}

	for i := 0; i < 30; i++ {
		if !processAlive(pid) {
			removePidfile()
			fmt.Printf("stopped (pid %d)\n", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("pid %d did not exit within 3s", pid)
}

func runStatus() error {
	pid, _ := readPidfile()
	if pid <= 0 || !processAlive(pid) {
		fmt.Println(styleBad.Render("stopped"))
		return nil
	}
	fmt.Printf("%s (pid %d)\n", styleOK.Render("running"), pid)
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

func readPidfile() (int, error) {
	path, err := pidFilePath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func writePidfile(pid int) error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600)
}

func removePidfile() {
	path, err := pidFilePath()
	if err != nil {
		return
	}
	os.Remove(path)
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
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
