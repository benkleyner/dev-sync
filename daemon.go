package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
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
				fmt.Fprintf(os.Stderr, "[%s] stopped: %v\n", pair.Name, err)
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

	fmt.Printf("[%s] connected; initial sync...\n", p.Name)
	if err := initialSync(ctx, p, syncer); err != nil {
		return fmt.Errorf("initial sync: %w", err)
	}
	fmt.Printf("[%s] watching %s\n", p.Name, p.LocalDir)

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
			fmt.Fprintf(os.Stderr, "[%s] upload %s: %v\n", p.Name, rel, err)
			continue
		}
		fmt.Printf("[%s] ✓ %s\n", p.Name, rel)
	}
	return nil
}

func watchLoop(ctx context.Context, p SyncPair, syncer Syncer) error {
	watcher, err := NewWatcher(p.LocalDir)
	if err != nil {
		return err
	}
	defer watcher.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events():
			if !ok {
				return nil
			}
			if watcher.ShouldIgnore(ev.Name) {
				continue
			}
			handleEvent(p.Name, ev, p.LocalDir, syncer)
		case err, ok := <-watcher.Errors():
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "[%s] watcher error: %v\n", p.Name, err)
		}
	}
}
