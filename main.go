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
	case "watch":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dev-sync watch <directory>")
			os.Exit(1)
		}
		if err := runWatch(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "watch failed: %v\n", err)
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

func runWatch(root string) error {
	watcher, err := NewWatcher(root)
	if err != nil {
		return err
	}
	defer watcher.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	fmt.Printf("watching %s (Ctrl-C to stop)\n", root)

	for {
		select {
		case ev, ok := <-watcher.Events():
			if !ok {
				return nil
			}
			if watcher.ShouldIgnore(ev.Name) {
				continue
			}
			printEvent(ev)
		case err, ok := <-watcher.Errors():
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		case <-sigs:
			fmt.Println("\nshutting down...")
			return nil
		}
	}
}

func printEvent(ev fsnotify.Event) {
	switch {
	case ev.Has(fsnotify.Create):
		fmt.Printf(" + %s\n", ev.Name)
	case ev.Has(fsnotify.Write):
		fmt.Printf(" ~ %s\n", ev.Name)
	case ev.Has(fsnotify.Remove):
		fmt.Printf(" - %s\n", ev.Name)
	case ev.Has(fsnotify.Rename):
		fmt.Printf(" > %s\n", ev.Name)
	}
}
