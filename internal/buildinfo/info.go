package buildinfo

import (
	"fmt"
	"strings"
)

const appName = "upwind-cli"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Short() string {
	return valueOrDefault(Version, "dev")
}

func UserAgent() string {
	return fmt.Sprintf("%s/%s", appName, Short())
}

func Details() string {
	return fmt.Sprintf(
		"version: %s\ncommit: %s\ndate: %s\n",
		Short(),
		valueOrDefault(Commit, "none"),
		valueOrDefault(Date, "unknown"),
	)
}

func valueOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}
