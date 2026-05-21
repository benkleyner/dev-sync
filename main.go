package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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
	scanner, err := NewScanner(root)
	if err != nil {
		return err
	}

	fmt.Printf("watching %s (poll every 2s, Ctrl-C to stop)\n", root)

	for {
		changes, err := scanner.Scan()
		if err != nil {
			return err
		}
		for _, path := range changes.Added {
			fmt.Printf(" + %s \n", path)
		}
		for _, path := range changes.Modified {
			fmt.Printf(" ~ %s \n", path)
		}
		for _, path := range changes.Removed {
			fmt.Printf(" - %s \n", path)
		}
		time.Sleep(2 * time.Second)
	}
}
