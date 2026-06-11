package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ConfluenceBaseURL string
	ConfluenceUser    string
	ConfluencePass    string
	PageURLOrID       string
	OutputFile        string
}

func Load() (*Config, error) {
	// Load .env file if it exists (best-effort)
	_ = loadDotEnv(".env")

	cfg := &Config{
		ConfluenceBaseURL: getEnv("CONFLUENCE_URL", "https://confluence.bri.co.id"),
		ConfluenceUser:    os.Getenv("CONFLUENCE_USERNAME"),
		ConfluencePass:    os.Getenv("CONFLUENCE_PASSWORD"),
		PageURLOrID:       os.Getenv("CONFLUENCE_PAGE"),
		OutputFile:        getEnv("OUTPUT_FILE", "test_cases.xlsx"),
	}

	var missing []string
	if cfg.ConfluenceUser == "" {
		missing = append(missing, "CONFLUENCE_USERNAME")
	}
	if cfg.ConfluencePass == "" {
		missing = append(missing, "CONFLUENCE_PASSWORD")
	}
	if cfg.PageURLOrID == "" {
		missing = append(missing, "CONFLUENCE_PAGE")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads a simple KEY=VALUE .env file and sets env vars (does not override).
func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return nil
}
