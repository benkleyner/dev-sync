package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

func runDaemon() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Pairs) == 0 {
		return fmt.Errorf("no pairs configured; run `dev-sync init`")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var wg sync.WaitGroup
	for _, pair := range cfg.Pairs {
		wg.Go(func() {
			if err := runPair(ctx, pair); err != nil && ctx.Err() == nil {
				slog.Error("pair stopped", "pair", pair.Name, "err", err)
			}
		})
	}

	fmt.Printf("running %d pair(s); Ctrl-C to stop\n", len(cfg.Pairs))
	wg.Wait()
	fmt.Println("all pairs stopped")
	return nil
}

func runPair(ctx context.Context, p SyncPair) error {
	syncer, err := NewSFTPSyncer(
		p.LocalDir,
		p.Host+":"+strconv.Itoa(p.Port),
		p.User,
		p.Password,
		p.RemoteDir,
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer syncer.Close()

	slog.Info("connected", "pair", p.Name)
	if err := initialSync(ctx, p, syncer); err != nil {
		return fmt.Errorf("initial sync: %w", err)
	}
	slog.Info("watching", "pair", p.Name, "dir", p.LocalDir)

	return watchLoop(ctx, p, syncer)
}

func initialSync(ctx context.Context, p SyncPair, syncer Syncer) error {
	scanner, err := NewScanner(p.LocalDir)
	if err != nil {
		return err
	}
	changes, err := scanner.Scan()
	if err != nil {
		return err
	}
	for _, rel := range changes.Added {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncer.Upload(rel); err != nil {
			slog.Error("initial upload failed", "pair", p.Name, "path", rel, "err", err)
			continue
		}
		slog.Info("synced", "pair", p.Name, "path", rel)
	}
	return nil
}

func watchLoop(ctx context.Context, p SyncPair, syncer Syncer) error {
	watcher, err := NewWatcher(p.LocalDir)
	if err != nil {
		return err
	}
	defer watcher.Close()

	scanner, err := NewScanner(p.LocalDir)
	if err != nil {
		return err
	}
	if _, err := scanner.Scan(); err != nil {
		return err
	}

	const debounceWindow = 300 * time.Millisecond
	var debounce <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events():
			if !ok {
				return nil
			}
			if err := watcher.WatchCreatedDir(ev.Name); err != nil {
				slog.Error("watch new directory failed", "pair", p.Name, "path", ev.Name, "err", err)
			}
			debounce = time.After(debounceWindow)
		case err, ok := <-watcher.Errors():
			if !ok {
				return nil
			}
			slog.Error("watcher error", "pair", p.Name, "err", err)
		case <-debounce:
			debounce = nil
			reconcile(ctx, p, scanner, syncer)
		}
	}
}

func reconcile(ctx context.Context, p SyncPair, scanner *Scanner, syncer Syncer) {
	changes, err := scanner.Scan()
	if err != nil {
		slog.Error("scan failed", "pair", p.Name, "err", err)
		return
	}

	for _, rel := range changes.Added {
		if ctx.Err() != nil {
			return
		}
		if err := syncer.Upload(rel); err != nil {
			slog.Error("upload failed", "pair", p.Name, "path", rel, "err", err)
			continue
		}
		slog.Info("created", "pair", p.Name, "path", rel)
	}
	for _, rel := range changes.Modified {
		if ctx.Err() != nil {
			return
		}
		if err := syncer.Upload(rel); err != nil {
			slog.Error("upload failed", "pair", p.Name, "path", rel, "err", err)
			continue
		}
		slog.Info("modified", "pair", p.Name, "path", rel)
	}
	for _, rel := range changes.Removed {
		if ctx.Err() != nil {
			return
		}
		if err := syncer.Delete(rel); err != nil {
			slog.Error("delete failed", "pair", p.Name, "path", rel, "err", err)
			continue
		}
		slog.Info("deleted", "pair", p.Name, "path", rel)
	}
}
