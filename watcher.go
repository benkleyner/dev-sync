package main

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	ignore "github.com/sabhiram/go-gitignore"
)

type Watcher struct {
	root    string
	matcher *ignore.GitIgnore
	fsw     *fsnotify.Watcher
}

func NewWatcher(root string) (*Watcher, error) {
	matcher, err := loadGitignore(root)
	if err != nil {
		return nil, err
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{root: root, matcher: matcher, fsw: fsw}
	if err := w.addTree(root); err != nil {
		fsw.Close()
		return nil, err
	}
	return w, nil
}

func (w *Watcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if w.matcher != nil && rel != "." && w.matcher.MatchesPath(rel) {
			return filepath.SkipDir
		}
		return w.fsw.Add(path)
	})
}

func (w *Watcher) Events() <-chan fsnotify.Event { return w.fsw.Events }
func (w *Watcher) Errors() <-chan error          { return w.fsw.Errors }
func (w *Watcher) Close() error                  { return w.fsw.Close() }

func (w *Watcher) WatchCreatedDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return w.addTree(path)
}

func (w *Watcher) ShouldIgnore(path string) bool {
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return false
	}
	if w.matcher != nil && w.matcher.MatchesPath(rel) {
		return true
	}
	return false
}
