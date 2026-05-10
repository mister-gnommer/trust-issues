// 🤖 AI-generated
package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App      App       `toml:"app"`
	Storage  Storage   `toml:"storage"`
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

func Load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	if c.App.ClientID == "" {
		return errors.New("config: app.client_id is required")
	}
	if c.App.ClientSecret == "" {
		return errors.New("config: app.client_secret is required")
	}
	if c.Storage.DatabasePath == "" {
		return errors.New("config: storage.database_path is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("config: at least one [[accounts]] entry is required")
	}
	seen := make(map[string]struct{}, len(c.Accounts))
	for i, a := range c.Accounts {
		if a.UserID == "" {
			return fmt.Errorf("config: accounts[%d].user_id is required", i)
		}
		if a.DisplayName == "" {
			return fmt.Errorf("config: accounts[%d].display_name is required", i)
		}
		if a.RefreshToken == "" {
			return fmt.Errorf("config: accounts[%d].refresh_token is required", i)
		}
		if _, dup := seen[a.UserID]; dup {
			return fmt.Errorf("config: accounts[%d] duplicates user_id %q", i, a.UserID)
		}
		seen[a.UserID] = struct{}{}
	}
	return nil
}
