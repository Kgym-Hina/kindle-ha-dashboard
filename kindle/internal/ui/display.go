package ui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Display struct {
	TempDir string
}

func (d Display) Show(canvas image.Image) error {
	if canvas == nil {
		return errors.New("display canvas is nil")
	}
	if err := os.MkdirAll(d.TempDir, 0o755); err != nil {
		return err
	}
	temporary := filepath.Join(d.TempDir, fmt.Sprintf("frame-%d.png", time.Now().UnixNano()))
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	if err := png.Encode(file, canvas); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := showImage(temporary); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	_ = os.Remove(temporary)
	return nil
}

func showImage(path string) error {
	for _, candidate := range []string{"/usr/bin/fbink", "/usr/sbin/fbink", "fbink"} {
		if candidate != "fbink" {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		if err := exec.Command(candidate, "-q", "-c", "-f", "-i", path).Run(); err == nil {
			return nil
		}
	}
	for _, candidate := range []string{"/usr/bin/eips", "eips"} {
		if candidate != "eips" {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		if err := exec.Command(candidate, "-g", path).Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no working framebuffer image command for %s (tried fbink/eips)", path)
}

func FindCommand(name string) string {
	for _, candidate := range []string{filepath.Join("/usr/bin", name), filepath.Join("/usr/sbin", name), name} {
		if strings.Contains(candidate, "/") {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}
