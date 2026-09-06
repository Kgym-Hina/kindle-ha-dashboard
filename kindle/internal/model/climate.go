package model

import (
	"fmt"
	"strings"
)

type ClimateHitAreas struct {
	Reading   Frame
	Power     Frame
	Decrease  Frame
	Increase  Frame
	ModeItems []ClimateModeHitArea
}

type ClimateModeHitArea struct {
	Mode  string
	Frame Frame
}

func ClimateModes(attributes map[string]any) []string {
	modes := make([]string, 0, 6)
	seen := make(map[string]struct{})
	appendMode := func(value any) {
		mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
		if mode == "" || mode == "off" {
			return
		}
		if _, exists := seen[mode]; exists {
			return
		}
		seen[mode] = struct{}{}
		modes = append(modes, mode)
	}
	if attributes != nil {
		switch values := attributes["hvac_modes"].(type) {
		case []any:
			for _, value := range values {
				appendMode(value)
			}
		case []string:
			for _, value := range values {
				appendMode(value)
			}
		}
	}
	if len(modes) == 0 {
		modes = []string{"auto", "heat", "cool"}
	}
	if len(modes) > 6 {
		modes = modes[:6]
	}
	return modes
}

func ClimateCurrentMode(state EntityState) string {
	mode := ""
	if state.Attributes != nil {
		if value, ok := state.Attributes["hvac_mode"]; ok {
			mode = fmt.Sprint(value)
		}
	}
	if strings.TrimSpace(mode) == "" {
		mode = state.State
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func ClimateHitLayout(frame Frame, modes []string) ClimateHitAreas {
	if len(modes) == 0 {
		modes = ClimateModes(nil)
	}
	if len(modes) > 6 {
		modes = modes[:6]
	}
	padding := climateClamp(frame.Width*0.035, 8, 18)
	innerX := frame.X + padding
	innerWidth := climateMax(frame.Width-padding*2, 1)
	headerHeight := climateClamp(frame.Height*0.18, 30, 44)
	modeHeight := climateClamp(frame.Height*0.20, 30, 44)
	temperatureHeight := climateClamp(frame.Height*0.20, 36, 48)
	modeY := frame.Y + frame.Height - padding - modeHeight
	temperatureY := modeY - 8 - temperatureHeight
	readingY := frame.Y + padding + headerHeight + 4
	readingHeight := climateMax(temperatureY-8-readingY, 1)
	powerWidth := climateClamp(frame.Width*0.24, 84, 124)
	powerHeight := climateClamp(headerHeight-6, 28, 34)
	areas := ClimateHitAreas{
		Reading:  Frame{X: innerX, Y: readingY, Width: innerWidth, Height: readingHeight},
		Power:    Frame{X: frame.X + frame.Width - padding - powerWidth, Y: frame.Y + padding + 3, Width: powerWidth, Height: powerHeight},
		Decrease: Frame{X: innerX, Y: temperatureY, Width: innerWidth / 3, Height: temperatureHeight},
		Increase: Frame{X: innerX + innerWidth*2/3, Y: temperatureY, Width: innerWidth / 3, Height: temperatureHeight},
	}
	modeGap := 4.0
	if len(modes) > 0 {
		modeWidth := climateMax((innerWidth-modeGap*float64(len(modes)-1))/float64(len(modes)), 1)
		for index, mode := range modes {
			areas.ModeItems = append(areas.ModeItems, ClimateModeHitArea{
				Mode: mode,
				Frame: Frame{
					X:      innerX + float64(index)*(modeWidth+modeGap),
					Y:      modeY,
					Width:  modeWidth,
					Height: modeHeight,
				},
			})
		}
	}
	return areas
}

func climateClamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func climateMax(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
