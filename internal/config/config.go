package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	DataDir   string
	ConfigDir string

	PollInterval    time.Duration
	ZoxideCap      int
	MaxShow        int
	GitConcurrency int
	ProcCacheTTL   time.Duration
	PruneCutoff    time.Duration
}

func Default() *Config {
	return &Config{
		PollInterval:    10 * time.Second,
		ZoxideCap:      40,
		MaxShow:         12,
		GitConcurrency:  4,
		ProcCacheTTL:    2 * time.Second,
		PruneCutoff:     30 * 24 * time.Hour,
	}
}

func (c *Config) ResolveDataDir() string {
	if c.DataDir != "" {
		return c.DataDir
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gotomux")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gotomux")
}

func (c *Config) ResolveConfigDir() string {
	if c.ConfigDir != "" {
		return c.ConfigDir
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gotomux")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gotomux")
}
