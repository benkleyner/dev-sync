package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalSyncer struct {
	srcRoot string
	dstRoot string
}

func NewLocalSyncer(srcRoot, dstRoot string) *LocalSyncer {
	return &LocalSyncer{srcRoot: srcRoot, dstRoot: dstRoot}
}

func (s *LocalSyncer) Upload(relPath string) error {
	srcPath := filepath.Join(s.srcRoot, relPath)
	dstPath := filepath.Join(s.dstRoot, relPath)

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

func (s *LocalSyncer) Delete(relPath string) error {
	dstPath := filepath.Join(s.dstRoot, relPath)
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete path: %w", err)
	}
	return nil
}
