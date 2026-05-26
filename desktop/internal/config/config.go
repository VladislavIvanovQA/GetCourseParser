package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultPort = 18765

type Config struct {
	Port             int    `json:"port"`
	Token            string `json:"token"`
	DownloadDir      string `json:"download_dir"`
	MaxParallelJobs  int    `json:"max_parallel_jobs"`
	MaxParallelFiles int    `json:"max_parallel_files"`
}

func AppDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func Path() string {
	return filepath.Join(AppDir(), "config.json")
}

func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return CreateDefault()
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.Token == "" {
		c.Token = newToken()
	}
	if c.DownloadDir == "" {
		c.DownloadDir = "downloads"
	}
	c.applyDefaults()
	return &c, nil
}

func CreateDefault() (*Config, error) {
	c := &Config{
		Port:        DefaultPort,
		Token:       newToken(),
		DownloadDir: "downloads",
	}
	c.applyDefaults()
	return c, c.Save()
}

func (c *Config) applyDefaults() {
	if c.MaxParallelJobs < 1 {
		c.MaxParallelJobs = 2
	}
	if c.MaxParallelFiles < 1 {
		c.MaxParallelFiles = 4
	}
}

func (c *Config) Save() error {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.Token == "" {
		c.Token = newToken()
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0o600)
}

func (c *Config) DownloadsPath() string {
	if filepath.IsAbs(c.DownloadDir) {
		return c.DownloadDir
	}
	return filepath.Join(AppDir(), c.DownloadDir)
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "change-me-token"
	}
	return hex.EncodeToString(b)
}
