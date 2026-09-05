package input

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
)

type TouchEvent struct {
	X        int
	Y        int
	Pressed  bool
	Released bool
}

type Reader struct {
	DevicePath string
	MaxX       int
	MaxY       int
}

func (r Reader) Read(ctx context.Context, output chan<- TouchEvent) error {
	file, err := os.Open(r.DevicePath)
	if err != nil {
		return err
	}
	defer file.Close()
	go func() {
		<-ctx.Done()
		_ = file.Close()
	}()
	if err := grab(file); err != nil {
		return fmt.Errorf("grab input device: %w", err)
	}
	defer func() { _ = ungrab(file) }()

	type rawEvent struct {
		Sec   int32
		Usec  int32
		Type  uint16
		Code  uint16
		Value int32
	}
	const (
		evSyn         = 0x00
		evKey         = 0x01
		evAbs         = 0x03
		synReport     = 0
		btnTouch      = 0x14a
		absX          = 0x00
		absY          = 0x01
		absMTX        = 0x35
		absMTY        = 0x36
		absMTTracking = 0x39
	)
	var (
		x, y           int
		haveX, haveY   bool
		tracking       bool
		pendingPress   bool
		pendingRelease bool
		event          rawEvent
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := binary.Read(file, binary.LittleEndian, &event); err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			return err
		}
		switch event.Type {
		case evAbs:
			switch event.Code {
			case absX, absMTX:
				x, haveX = clamp(int(event.Value), 0, r.MaxX), true
			case absY, absMTY:
				y, haveY = clamp(int(event.Value), 0, r.MaxY), true
			case absMTTracking:
				tracking = event.Value >= 0
				if !tracking {
					pendingRelease = true
				}
			}
		case evKey:
			if event.Code == btnTouch {
				if event.Value != 0 {
					pendingPress = true
				} else {
					pendingRelease = true
				}
			}
		case evSyn:
			if event.Code != synReport {
				continue
			}
			if pendingPress && (haveX || haveY) {
				if err := send(ctx, output, TouchEvent{X: x, Y: y, Pressed: true}); err != nil {
					return err
				}
				pendingPress = false
			}
			if tracking && (haveX || haveY) {
				pendingRelease = false
			}
			if pendingRelease {
				if err := send(ctx, output, TouchEvent{X: x, Y: y, Released: true}); err != nil {
					return err
				}
				pendingRelease, pendingPress = false, false
			}
			haveX, haveY = false, false
		}
	}
}

func send(ctx context.Context, output chan<- TouchEvent, event TouchEvent) error {
	select {
	case output <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func grab(file *os.File) error {
	const evGrab = 1074021776
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(evGrab), 1)
	if errno != 0 {
		return errno
	}
	return nil
}

func ungrab(file *os.File) error {
	const evGrab = 1074021776
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(evGrab), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
