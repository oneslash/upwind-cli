package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	EnvOrganizationID = "UPWIND_ORGANIZATION_ID"
	EnvRegion         = "UPWIND_REGION"
	EnvBaseURL        = "UPWIND_BASE_URL"
	EnvAuthURL        = "UPWIND_AUTH_URL"
	EnvAudience       = "UPWIND_AUDIENCE"
	EnvClientID       = "UPWIND_CLIENT_ID"
	EnvClientSecret   = "UPWIND_CLIENT_SECRET"
	EnvAccessToken    = "UPWIND_ACCESS_TOKEN"
	EnvOutput         = "UPWIND_OUTPUT"
	EnvTimeout        = "UPWIND_TIMEOUT"
)

type Options struct {
	OrganizationID string
	Region         string
	BaseURL        string
	AuthURL        string
	Audience       string
	ClientID       string
	ClientSecret   string
	AccessToken    string
	Output         string
	Timeout        time.Duration
}

type Runtime struct {
	OrganizationID string
	Region         string
	BaseURL        string
	AuthURL        string
	Audience       string
	ClientID       string
	ClientSecret   string
	AccessToken    string
	Output         string
	Timeout        time.Duration
}

func LoadDotEnv() error {
	_, err := os.Stat(".env")
	if err == nil {
		return godotenv.Load(".env")
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func Resolve(opts Options) (Runtime, error) {
	region := strings.ToLower(strings.TrimSpace(opts.Region))
	if region == "" {
		region = "us"
	}

	defaults, ok := regionDefaults(region)
	if !ok {
		return Runtime{}, fmt.Errorf("unsupported region %q (expected us, eu, or me)", region)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaults.BaseURL
	}

	authURL := strings.TrimRight(strings.TrimSpace(opts.AuthURL), "/")
	if authURL == "" {
		authURL = defaults.AuthURL
	}

	audience := strings.TrimSpace(opts.Audience)
	if audience == "" {
		audience = baseURL
	}

	output := strings.ToLower(strings.TrimSpace(opts.Output))
	if output == "" {
		output = "table"
	}
	if output != "table" && output != "json" {
		return Runtime{}, fmt.Errorf("unsupported output format %q (expected table or json)", output)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return Runtime{
		OrganizationID: strings.TrimSpace(opts.OrganizationID),
		Region:         region,
		BaseURL:        baseURL,
		AuthURL:        authURL,
		Audience:       audience,
		ClientID:       strings.TrimSpace(opts.ClientID),
		ClientSecret:   strings.TrimSpace(opts.ClientSecret),
		AccessToken:    strings.TrimSpace(opts.AccessToken),
		Output:         output,
		Timeout:        timeout,
	}, nil
}

func EnvDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return value
}

type RegionDefaults struct {
	BaseURL string
	AuthURL string
}

func regionDefaults(region string) (RegionDefaults, bool) {
	switch region {
	case "us":
		return RegionDefaults{
			BaseURL: "https://api.upwind.io",
			AuthURL: "https://auth.upwind.io",
		}, true
	case "eu":
		return RegionDefaults{
			BaseURL: "https://api.eu.upwind.io",
			AuthURL: "https://auth.upwind.io",
		}, true
	case "me":
		return RegionDefaults{
			BaseURL: "https://api.me.upwind.io",
			AuthURL: "https://auth.upwind.io",
		}, true
	default:
		return RegionDefaults{}, false
	}
}
