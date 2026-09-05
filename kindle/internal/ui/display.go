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

	xdraw "golang.org/x/image/draw"
)

type Display struct {
	TempDir string
	Width   int
	Height  int
}

func (d Display) Show(canvas image.Image) error {
	if canvas == nil {
		return errors.New("display canvas is nil")
	}
	width, height := canvas.Bounds().Dx(), canvas.Bounds().Dy()
	if d.Width > 0 && d.Height > 0 {
		width, height = d.Width, d.Height
		if canvas.Bounds().Dx() != width || canvas.Bounds().Dy() != height {
			normalized := image.NewGray(image.Rect(0, 0, width, height))
			xdraw.ApproxBiLinear.Scale(normalized, normalized.Bounds(), canvas, canvas.Bounds(), xdraw.Src, nil)
			canvas = normalized
		}
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
	if err := showImage(temporary, width, height); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	_ = os.Remove(temporary)
	return nil
}

func showImage(path string, width, height int) error {
	imageSpec := fmt.Sprintf("file=%s,x=0,y=0,w=%d,h=%d", path, width, height)
	for _, candidate := range []string{"/usr/bin/fbink", "/usr/sbin/fbink", "/mnt/us/linkss/bin/fbink", "fbink"} {
		if candidate != "fbink" {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		for _, args := range [][]string{
			{"-q", "-c", "-f", "-V", "-g", imageSpec},
			{"-q", "-c", "-f", "-g", imageSpec},
			{"-q", "-c", "-f", "-i", path},
		} {
			if err := exec.Command(candidate, args...).Run(); err == nil {
				return nil
			}
		}
	}
	for _, candidate := range []string{"/usr/sbin/eips", "/usr/bin/eips", "eips"} {
		if candidate != "eips" {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		for _, args := range [][]string{
			{"-f", "-g", path, "-x", "0", "-y", "0"},
			{"-f", "-g", path},
			{"-g", path},
		} {
			if err := exec.Command(candidate, args...).Run(); err == nil {
				return nil
			}
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
