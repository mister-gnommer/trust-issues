package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

type Reports struct {
	Dir               string `toml:"dir"`
	MinPlays          int    `toml:"min_plays"`
	ResidualThreshold int    `toml:"residual_threshold"`
}

type Config struct {
	App      App       `toml:"app"`
	Storage  Storage   `toml:"storage"`
	Reports  Reports   `toml:"reports"`
	Accounts []Account `toml:"accounts"`
}

type App struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

type Storage struct {
	DatabasePath string `toml:"database_path"`
}

type Account struct {
	UserID       string `toml:"user_id"`
	DisplayName  string `toml:"display_name"`
	RefreshToken string `toml:"refresh_token"`
}

func load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Reports.Dir == "" {
		c.Reports.Dir = "./reports"
	}
	if c.Reports.MinPlays == 0 {
		c.Reports.MinPlays = 30
	}
	if c.Reports.ResidualThreshold == 0 {
		c.Reports.ResidualThreshold = 3
	}
}

// Load loads and validates a config for capture mode (requires Spotify credentials).
func Load(path string) (*Config, error) {
	c, err := load(path)
	if err != nil {
		return nil, err
	}
	c.applyDefaults()
	if err := c.validateForCapture(); err != nil {
		return nil, err
	}
	return c, nil
}

// LoadForReport loads and validates a config for report-only mode (no Spotify credentials needed).
func LoadForReport(path string) (*Config, error) {
	c, err := load(path)
	if err != nil {
		return nil, err
	}
	c.applyDefaults()
	if err := c.validateForReport(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validateCommon() error {
	if c.Storage.DatabasePath == "" {
		return errors.New("config: storage.database_path is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("config: at least one [[accounts]] entry is required")
	}
	if c.Reports.MinPlays < 1 {
		return errors.New("config: reports.min_plays must be >= 1")
	}
	if c.Reports.ResidualThreshold < 1 {
		return errors.New("config: reports.residual_threshold must be >= 1")
	}
	if c.Reports.Dir == "" {
		return errors.New("config: reports.dir is required")
	}
	seen := make(map[string]struct{}, len(c.Accounts))
	for i, a := range c.Accounts {
		if a.UserID == "" {
			return fmt.Errorf("config: accounts[%d].user_id is required", i)
		}
		if a.DisplayName == "" {
			return fmt.Errorf("config: accounts[%d].display_name is required", i)
		}
		if _, dup := seen[a.UserID]; dup {
			return fmt.Errorf("config: accounts[%d] duplicates user_id %q", i, a.UserID)
		}
		seen[a.UserID] = struct{}{}
	}
	return nil
}

func (c *Config) validateForCapture() error {
	if c.App.ClientID == "" {
		return errors.New("config: app.client_id is required")
	}
	if c.App.ClientSecret == "" {
		return errors.New("config: app.client_secret is required")
	}
	if err := c.validateCommon(); err != nil {
		return err
	}
	for i, a := range c.Accounts {
		if a.RefreshToken == "" {
			return fmt.Errorf("config: accounts[%d].refresh_token is required", i)
		}
	}
	return nil
}

func (c *Config) validateForReport() error {
	return c.validateCommon()
}
