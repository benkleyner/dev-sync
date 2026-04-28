package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPSyncer struct {
	srcRoot string
	dstRoot string
	conn    *ssh.Client
	client  *sftp.Client
}

func NewSFTPSyncer(srcRoot, host, user, password, dstRoot string) (*SFTPSyncer, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %w", err)
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sftp client: %w", err)
	}

	if strings.HasPrefix(dstRoot, "~/") {
		home, err := client.Getwd()
		if err != nil {
			client.Close()
			conn.Close()
			return nil, fmt.Errorf("get remote home: %w", err)
		}
		dstRoot = path.Join(home, dstRoot[2:])
	}

	info, err := client.Stat(dstRoot)
	if err != nil {
		client.Close()
		conn.Close()
		return nil, fmt.Errorf("stat remote dir %q: %w", dstRoot, err)
	}
	if !info.IsDir() {
		client.Close()
		conn.Close()
		return nil, fmt.Errorf("remote path %q is not a directory", dstRoot)
	}

	return &SFTPSyncer{srcRoot: srcRoot, dstRoot: dstRoot, conn: conn, client: client}, nil
}

func (s *SFTPSyncer) Close() error {
	s.client.Close()
	return s.conn.Close()
}

func (s *SFTPSyncer) Upload(relPath string) error {
	src, err := os.Open(path.Join(s.srcRoot, relPath))
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	remotePath := path.Join(s.dstRoot, filepath.ToSlash(relPath))
	if err := s.client.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("remote mkdir: %w", err)
	}

	dst, err := s.client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("remote create: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

func (s *SFTPSyncer) Delete(relPath string) error {
	remotePath := path.Join(s.dstRoot, filepath.ToSlash(relPath))
	if err := s.client.Remove(remotePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remote remove: %w", err)
	}
	return nil
}
