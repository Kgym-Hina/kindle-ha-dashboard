package power

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ReadPercent() (int, error) {
	paths, err := filepath.Glob("/sys/class/power_supply/*/capacity")
	if err != nil {
		return 0, err
	}
	if len(paths) == 0 {
		return 0, errors.New("battery capacity sysfs entry not found")
	}
	for _, path := range paths {
		value, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		percent, parseErr := strconv.Atoi(strings.TrimSpace(string(value)))
		if parseErr == nil && percent >= 0 && percent <= 100 {
			return percent, nil
		}
	}
	return 0, fmt.Errorf("unable to parse battery capacity from %d entries", len(paths))
}
