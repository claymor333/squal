package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Profile struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Socket   string `json:"socket,omitempty"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	Charset  string `json:"charset,omitempty"`
	SSL      bool   `json:"ssl"`
	Timeout  int    `json:"timeout_seconds"`
}

type AI struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
}

type Config struct {
	Profiles []Profile `json:"profiles"`
	AI       AI        `json:"ai"`
}

const (
	configEnv  = "SQUAL_CONFIG"
	configName = "config.json"
)

func defaultDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "squal"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "squal"), nil
}

func Path() (string, error) {
	if p := os.Getenv(configEnv); p != "" {
		return p, nil
	}
	dir, err := defaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configName), nil
}

func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	return nil
}

func (c *Config) AddProfile(p Profile) {
	for i := range c.Profiles {
		if c.Profiles[i].Name == p.Name {
			c.Profiles[i] = p
			return
		}
	}
	c.Profiles = append(c.Profiles, p)
}

func (c *Config) RemoveProfile(name string) {
	out := c.Profiles[:0]
	for _, p := range c.Profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	c.Profiles = out
}

func (c *Config) APIKey() string {
	if k := os.Getenv("SQUAL_OPENAI_API_KEY"); k != "" {
		return k
	}
	return c.AI.APIKey
}
