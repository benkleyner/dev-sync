package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	ignore "github.com/sabhiram/go-gitignore"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "hello":
		fmt.Println("hello from dev-sync")
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
			fmt.Fprintln(os.Stderr, "usage dev-sync mirror <src> <dest>")
			os.Exit(1)
		}
		if err := runMirror(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "mirror failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dev-sync <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, " hello     print a greeting")
	fmt.Fprintln(os.Stderr, " version   print the version")
	fmt.Fprintln(os.Stderr, " scan      list all files under a directory")
	fmt.Fprintln(os.Stderr, " watch     watch a directory and print changes")
	fmt.Fprintln(os.Stderr, " mirror    mirror one directory to another")
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

func runMirror(src, dst string) error {
	watcher, err := NewWatcher(src)
	if err != nil {
		return err
	}
	defer watcher.Close()

	syncer := NewLocalSyncer(src, dst)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	fmt.Printf("mirroring %s -> %s (Ctrl-C to stop)\n", src, dst)

	for {
		select {
		case ev, ok := <-watcher.Events():
			if !ok {
				return nil
			}
			if watcher.ShouldIgnore(ev.Name) {
				continue
			}
			handleEvent(ev, src, syncer)
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

func handleEvent(ev fsnotify.Event, srcRoot string, syncer *LocalSyncer) {
	rel, err := filepath.Rel(srcRoot, ev.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rel path: %v\n", err)
		return
	}

	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		if err := syncer.Delete(rel); err != nil {
			fmt.Fprintf(os.Stderr, "delete %s: %v\n", rel, err)
			return
		}
		fmt.Printf(" - %s\n", rel)
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
		fmt.Fprintf(os.Stderr, "upload %s: %v\n", rel, err)
		return
	}
	fmt.Printf("  ✓ %s \n", rel)
}
