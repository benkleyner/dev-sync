package main

import (
	"io/fs"
	"path/filepath"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
)

type Scanner struct {
	root     string
	matcher  *ignore.GitIgnore
	snapshot map[string]time.Time
}

type Changes struct {
	Added    []string
	Modified []string
	Removed  []string
}

func (c Changes) Empty() bool {
	return len(c.Added) == 0 && len(c.Modified) == 0 && len(c.Removed) == 0
}

func NewScanner(root string) (*Scanner, error) {
	matcher, err := loadGitignore(root)
	if err != nil {
		return nil, err
	}
	return &Scanner{
		root:     root,
		matcher:  matcher,
		snapshot: make(map[string]time.Time),
	}, nil
}

func (s *Scanner) Scan() (Changes, error) {
	current := make(map[string]time.Time)

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			if s.matcher != nil && s.matcher.MatchesPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if s.matcher != nil && s.matcher.MatchesPath(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		current[rel] = info.ModTime()
		return nil
	})
	if err != nil {
		return Changes{}, err
	}

	var changes Changes
	for path, modTime := range current {
		prev, existed := s.snapshot[path]
		if !existed {
			changes.Added = append(changes.Added, path)
		} else if !modTime.Equal(prev) {
			changes.Modified = append(changes.Modified, path)
		}
	}
	for path := range s.snapshot {
		if _, exists := current[path]; !exists {
			changes.Removed = append(changes.Removed, path)
		}
	}

	s.snapshot = current
	return changes, nil
}
