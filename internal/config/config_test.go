package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := writeFile(p, content); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestLoad_valid(t *testing.T) {
	p := writeTOML(t, `
[app]
client_id     = "cid"
client_secret = "secret"

[storage]
database_path = "./trust.db"

[[accounts]]
user_id       = "u1"
display_name  = "One"
refresh_token = "r1"

[[accounts]]
user_id       = "u2"
display_name  = "Two"
refresh_token = "r2"
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.App.ClientID != "cid" || c.App.ClientSecret != "secret" {
		t.Errorf("app: %+v", c.App)
	}
	if c.Storage.DatabasePath != "./trust.db" {
		t.Errorf("storage: %+v", c.Storage)
	}
	if len(c.Accounts) != 2 {
		t.Fatalf("accounts: got %d, want 2", len(c.Accounts))
	}
}

func TestLoad_validationErrors(t *testing.T) {
	cases := []struct {
		name    string
		toml    string
		wantSub string
	}{
		{
			name: "missing client_id",
			toml: `
[app]
client_secret = "s"
[storage]
database_path = "x"
[[accounts]]
user_id = "u"
display_name = "n"
refresh_token = "r"`,
			wantSub: "client_id",
		},
		{
			name: "missing client_secret",
			toml: `
[app]
client_id = "c"
[storage]
database_path = "x"
[[accounts]]
user_id = "u"
display_name = "n"
refresh_token = "r"`,
			wantSub: "client_secret",
		},
		{
			name: "missing database_path",
			toml: `
[app]
client_id = "c"
client_secret = "s"
[storage]
[[accounts]]
user_id = "u"
display_name = "n"
refresh_token = "r"`,
			wantSub: "database_path",
		},
		{
			name: "no accounts",
			toml: `
[app]
client_id = "c"
client_secret = "s"
[storage]
database_path = "x"`,
			wantSub: "at least one",
		},
		{
			name: "duplicate user_id",
			toml: `
[app]
client_id = "c"
client_secret = "s"
[storage]
database_path = "x"
[[accounts]]
user_id = "u1"
display_name = "n"
refresh_token = "r"
[[accounts]]
user_id = "u1"
display_name = "n2"
refresh_token = "r2"`,
			wantSub: "duplicates user_id",
		},
		{
			name: "missing refresh_token",
			toml: `
[app]
client_id = "c"
client_secret = "s"
[storage]
database_path = "x"
[[accounts]]
user_id = "u"
display_name = "n"`,
			wantSub: "refresh_token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeTOML(t, tc.toml)
			_, err := Load(p)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoad_missingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadForReport_skipsCreds(t *testing.T) {
	p := writeTOML(t, `
[storage]
database_path = "./trust.db"

[[accounts]]
user_id = "u1"
display_name = "One"
`)
	c, err := LoadForReport(p)
	if err != nil {
		t.Fatalf("LoadForReport: %v", err)
	}
	if c.App.ClientID != "" {
		t.Error("ClientID should be empty")
	}
	if c.App.ClientSecret != "" {
		t.Error("ClientSecret should be empty")
	}
	if len(c.Accounts) != 1 {
		t.Fatalf("accounts: got %d, want 1", len(c.Accounts))
	}
	if c.Accounts[0].RefreshToken != "" {
		t.Error("RefreshToken should be empty in report mode")
	}
}

func TestApplyDefaults(t *testing.T) {
	p := writeTOML(t, `
[storage]
database_path = "./trust.db"

[[accounts]]
user_id = "u1"
display_name = "One"
`)
	c, err := LoadForReport(p)
	if err != nil {
		t.Fatalf("LoadForReport: %v", err)
	}
	if c.Reports.Dir != "./reports" {
		t.Errorf("reports dir default: got %q, want ./reports", c.Reports.Dir)
	}
	if c.Reports.MinPlays != 30 {
		t.Errorf("min_plays default: got %d, want 30", c.Reports.MinPlays)
	}
	if c.Reports.ResidualThreshold != 3 {
		t.Errorf("residual_threshold default: got %d, want 3", c.Reports.ResidualThreshold)
	}
}

func TestLoadForCapture_requiresCreds(t *testing.T) {
	p := writeTOML(t, `
[storage]
database_path = "./trust.db"

[[accounts]]
user_id = "u1"
display_name = "One"
`)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for missing client_id")
	}
	if !strings.Contains(err.Error(), "client_id") {
		t.Errorf("error should mention client_id: %v", err)
	}
}

func TestLoadForReport_commonValidation(t *testing.T) {
	cases := []struct {
		name    string
		toml    string
		wantSub string
	}{
		{
			name: "missing database_path",
			toml: `
[[accounts]]
user_id = "u"
display_name = "n"`,
			wantSub: "database_path",
		},
		{
			name: "no accounts",
			toml: `
[storage]
database_path = "x"`,
			wantSub: "at least one",
		},
		{
			name: "missing user_id",
			toml: `
[storage]
database_path = "x"
[[accounts]]
display_name = "n"`,
			wantSub: "user_id",
		},
		{
			name: "missing display_name",
			toml: `
[storage]
database_path = "x"
[[accounts]]
user_id = "u"`,
			wantSub: "display_name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeTOML(t, tc.toml)
			_, err := LoadForReport(p)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
