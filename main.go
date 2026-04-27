package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
}

func runScan(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fmt.Println(path)
		return nil
	})
}
