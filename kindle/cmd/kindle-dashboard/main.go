package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/app"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	configPath := config.PathFromEnv()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s: %v", configPath, err)
	}
	stopFramework()
	defer restoreFramework()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.New(cfg).Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func stopFramework() {
	if err := command("/usr/sbin/stop", "framework"); err != nil {
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
	if err := command("/usr/sbin/start", "framework"); err != nil {
		log.Printf("start framework: %v", err)
	}
}

func command(path string, args ...string) error {
	if _, err := os.Stat(path); err != nil {
		if resolved, lookErr := exec.LookPath(path); lookErr == nil {
			path = resolved
		} else {
			return err
		}
	}
	return exec.Command(path, args...).Run()
}
