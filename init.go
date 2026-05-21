package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/huh"
)

func runInit() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	var name, localDir, host, user, remoteDir, password string
	portStr := "22"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Pair name").
				Description("A short identifer (e.g. my-project)").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					if cfg.FindPair(s) != nil {
						return fmt.Errorf("a pair named %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Local directory").
				Value(&localDir).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					abs, err := filepath.Abs(s)
					if err != nil {
						return err
					}
					info, err := os.Stat(abs)
					if err != nil {
						return err
					}
					if !info.IsDir() {
						return fmt.Errorf("Not a directory")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().Title("SFTP host").Value(&host).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().Title("SFTP port").Value(&portStr).
				Validate(func(s string) error {
					p, err := strconv.Atoi(s)
					if err != nil || p <= 0 || p > 65535 {
						return fmt.Errorf("invalid port")
					}
					return nil
				}),
			huh.NewInput().Title("SFTP user").Value(&user).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("SFTP password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Remote directory").
				Description("Absolute path on SFTP server").
				Value(&remoteDir).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	port, _ := strconv.Atoi(portStr)
	localAbs, _ := filepath.Abs(localDir)

	fmt.Println("Verifying SFTP connection...")
	syncer, err := NewSFTPSyncer(localAbs, host+":"+portStr, user, password, remoteDir)
	if err != nil {
		return fmt.Errorf("verication failed: %w", err)
	}
	syncer.Close()

	cfg.Pairs = append(cfg.Pairs, SyncPair{
		Name:      name,
		LocalDir:  localAbs,
		Host:      host,
		Port:      port,
		User:      user,
		RemoteDir: remoteDir,
		Password:  password,
	})

	if err := SaveConfig(cfg); err != nil {
		return err
	}

	cfgPath, _ := configPath()
	fmt.Printf("%s saved pair %q to %s\n", styleOK.Render("✓"), name, cfgPath)
	return nil
}

func runList() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Pairs) == 0 {
		fmt.Println("No sync pairs configured. Run `dev-sync init` to add one.")
		return nil
	}
	for _, p := range cfg.Pairs {
		fmt.Println(stylePair.Render(p.Name))
		target := fmt.Sprintf("%s@%s:%s", p.User, p.Host, p.RemoteDir)
		fmt.Printf(" %s %s %s\n", stylePath.Render(p.LocalDir), styleKey.Render("->"), stylePath.Render(target))
	}
	return nil
}
