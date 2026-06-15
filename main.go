package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	ignore "github.com/sabhiram/go-gitignore"
)

var version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "scan":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dev-sync scan <directory>")
			os.Exit(1)
		}
		if err := runScan(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
	case "mirror":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage dev-sync mirror <src> <user@host:remoteDir>")
			os.Exit(1)
		}
		if err := runSFTPMirror(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "sftp-mirror failed: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := runList(); err != nil {
			fmt.Fprintf(os.Stderr, "list failed: %v\n", err)
			os.Exit(1)
		}
	case "run":
		if err := runDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
			os.Exit(1)
		}
	case "start":
		if err := runStart(); err != nil {
			fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
			os.Exit(1)
		}
	case "stop":
		if err := runStop(); err != nil {
			fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
			os.Exit(1)
		}
	case "logs":
		if err := runLogs(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "logs failed: %v\n", err)
			os.Exit(1)
		}
	case "_daemon":
		os.Exit(runDaemonSubcommand())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: dev-sync <command> [args...]

dev-sync watches local directories and syncs changes to remote SFTP destinations.

configuration:
  init                              interactively configure a new local→remote sync pair
                                    (prompts for SFTP details, verifies the remote dir exists)
  list                              show configured sync pairs

running:
  run                               run all configured pairs in the foreground (Ctrl-C to stop)
  start                             launch the sync daemon in the background
  stop                              stop the running background daemon
  status                            show whether the daemon is running, and its pid
  logs [n]                          print the last n log entries (default 50)

ad-hoc / debug:
  scan <directory>                  list every file under <directory>, respecting .gitignore
  mirror <src> <user@host:dir>      one-off SFTP mirror without touching the config
                                    (requires DEV_SYNC_PASSWORD env var)

misc:
  version                           print the version
`)
}

func runScan(root string) error {
	matcher, err := loadGitignore(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			if matcher != nil && matcher.MatchesPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		if matcher != nil && matcher.MatchesPath(rel) {
			return nil
		}
		fmt.Println(path)
		return nil
	})
}

func loadGitignore(root string) (*ignore.GitIgnore, error) {
	path := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return ignore.CompileIgnoreFile(path)
}

func handleEvent(label string, ev fsnotify.Event, srcRoot string, syncer Syncer) {
	rel, err := filepath.Rel(srcRoot, ev.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] rel path: %v\n", label, err)
		return
	}

	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		if err := syncer.Delete(rel); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] delete %s: %v\n", label, rel, err)
			return
		}
		fmt.Printf("[%s] - %s\n", label, rel)
		return
	}

	info, err := os.Stat(ev.Name)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}

	if err := syncer.Upload(rel); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] upload %s: %v\n", label, rel, err)
		return
	}
	fmt.Printf("[%s]  ✓ %s \n", label, rel)
}

func runSFTPMirror(src, target string) error {
	user, host, dstRoot, err := parseSFTPTarget(target)
	if err != nil {
		return err
	}
	password := os.Getenv("DEV_SYNC_PASSWORD")
	if password == "" {
		return fmt.Errorf("set DEV_SYNC_PASSWORD env var")
	}

	syncer, err := NewSFTPSyncer(src, host, user, password, dstRoot)
	if err != nil {
		return err
	}
	defer syncer.Close()

	return runSyncLoop(src, syncer, target)
}

func runSyncLoop(src string, syncer Syncer, label string) error {
	watcher, err := NewWatcher(src)
	if err != nil {
		return err
	}
	defer watcher.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	fmt.Printf("mirroring %s -> %s (Ctrl-C to stop)\n", src, label)

	for {
		select {
		case ev, ok := <-watcher.Events():
			if !ok {
				return nil
			}
			if err := watcher.WatchCreatedDir(ev.Name); err != nil {
				fmt.Fprintf(os.Stderr, "watch new directory: %v\n", err)
			}
			if watcher.ShouldIgnore(ev.Name) {
				continue
			}
			handleEvent(label, ev, src, syncer)
		case err, ok := <-watcher.Errors():
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		case <-sigs:
			fmt.Println("\nshutting down...")
			return nil
		}
	}
}

func parseSFTPTarget(target string) (user, hostport, dstRoot string, err error) {
	at := strings.Index(target, "@")
	if at < 0 {
		return "", "", "", fmt.Errorf("expected user@host:path, got %q", target)
	}
	user = target[:at]
	rest := target[at+1:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", "", "", fmt.Errorf("expected user@host:path, got %q", target)
	}
	host := rest[:colon]
	dstRoot = rest[colon+1:]
	return user, host + ":22", dstRoot, nil
}
