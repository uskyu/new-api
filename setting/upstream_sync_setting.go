package setting

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	UpstreamSyncEnabledOptionKey         = "UpstreamSyncEnabled"
	UpstreamSyncIntervalMinutesOptionKey = "UpstreamSyncIntervalMinutes"

	DefaultUpstreamSyncIntervalMinutes = 5
	MinUpstreamSyncIntervalMinutes     = 1
	MaxUpstreamSyncIntervalMinutes     = 1440
)

// UpstreamSyncEnabled and UpstreamSyncIntervalMinutes are the process-local
// values mirrored from the option table. The unified upstream model/price sync
// is opt-in so an existing deployment never starts calling /api/pricing until a
// root administrator turns it on.
var (
	UpstreamSyncEnabled         = false
	UpstreamSyncIntervalMinutes = DefaultUpstreamSyncIntervalMinutes
)

func GetUpstreamSyncEnabled() bool {
	return UpstreamSyncEnabled
}

func GetUpstreamSyncIntervalMinutes() int {
	if UpstreamSyncIntervalMinutes < MinUpstreamSyncIntervalMinutes ||
		UpstreamSyncIntervalMinutes > MaxUpstreamSyncIntervalMinutes {
		return DefaultUpstreamSyncIntervalMinutes
	}
	return UpstreamSyncIntervalMinutes
}

// ValidateUpstreamSyncOption validates values coming through the generic
// option update API so invalid state can never be persisted.
func ValidateUpstreamSyncOption(key string, value string) error {
	switch key {
	case UpstreamSyncEnabledOptionKey:
		if _, err := parseUpstreamSyncBool(value); err != nil {
			return err
		}
	case UpstreamSyncIntervalMinutesOptionKey:
		if _, err := parseUpstreamSyncInterval(value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported upstream sync option: %s", key)
	}
	return nil
}

// UpdateUpstreamSyncEnabled applies an already validated option value.
func UpdateUpstreamSyncEnabled(value string) error {
	enabled, err := parseUpstreamSyncBool(value)
	if err != nil {
		return err
	}
	UpstreamSyncEnabled = enabled
	return nil
}

// UpdateUpstreamSyncIntervalMinutes applies an already validated option value.
func UpdateUpstreamSyncIntervalMinutes(value string) error {
	minutes, err := parseUpstreamSyncInterval(value)
	if err != nil {
		return err
	}
	UpstreamSyncIntervalMinutes = minutes
	return nil
}

func parseUpstreamSyncBool(value string) (bool, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("upstream sync enabled must be true or false")
	}
}

func parseUpstreamSyncInterval(value string) (int, error) {
	minutes, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("upstream sync interval must be an integer number of minutes")
	}
	if minutes < MinUpstreamSyncIntervalMinutes || minutes > MaxUpstreamSyncIntervalMinutes {
		return 0, fmt.Errorf(
			"upstream sync interval must be between %d and %d minutes",
			MinUpstreamSyncIntervalMinutes,
			MaxUpstreamSyncIntervalMinutes,
		)
	}
	return minutes, nil
}
