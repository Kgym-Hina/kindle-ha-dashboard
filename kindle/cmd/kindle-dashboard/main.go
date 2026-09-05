package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/app"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	stopFramework()
	defer restoreFramework()
	configPath := config.PathFromEnv()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s: %v", configPath, err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.New(cfg).Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func stopFramework() {
	if err := command("stop", "framework"); err != nil {
		log.Printf("stop framework: %v", err)
	}
	if err := command("/usr/bin/lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "1"); err != nil {
		log.Printf("disable screensaver: %v", err)
	}
}

func restoreFramework() {
	if err := command("/usr/bin/lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "0"); err != nil {
		log.Printf("enable screensaver: %v", err)
	}
	if err := command("start", "framework"); err != nil {
		log.Printf("start framework: %v", err)
	}
}

func command(name string, args ...string) error {
	path, err := resolveCommand(name)
	if err != nil {
		return err
	}
	return exec.Command(path, args...).Run()
}

func resolveCommand(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("command name is empty")
	}
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}
	for _, candidate := range []string{"/sbin/" + name, "/usr/sbin/" + name, "/bin/" + name, "/usr/bin/" + name} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if resolved, err := exec.LookPath(name); err == nil {
		return resolved, nil
	}
	return "", fmt.Errorf("command %q not found", name)
}
