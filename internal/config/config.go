package config

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"math"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Timeout        time.Duration `yaml:"timeout"`
	SampleInterval time.Duration `yaml:"sample_interval"`
	MemoryPercent  float64       `yaml:"memory_percent"`
	DiskPercent    float64       `yaml:"disk_percent"`
	CPUPercent     float64       `yaml:"cpu_percent"`
	LogLimit       int           `yaml:"log_limit"`
	Concurrency    int           `yaml:"concurrency"`
	Disabled       []string      `yaml:"disabled_collectors"`
	Format         string        `yaml:"format"`
	AI             AI            `yaml:"ai"`
}
type AI struct {
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
	Consent  bool   `yaml:"external_consent"`
	KeyEnv   string `yaml:"key_env"`
}

func Default() Config {
	return Config{Timeout: 10 * time.Second, SampleInterval: time.Second, MemoryPercent: 90, DiskPercent: 90, CPUPercent: 90, LogLimit: 50, Concurrency: 4, Format: "text", AI: AI{Provider: "ollama", Endpoint: "http://localhost:11434", Model: "llama3.2", KeyEnv: "SSYNC_API_KEY"}}
}
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "systemsync", "config.yaml"), nil
}
func (c Config) Validate() error {
	if c.Timeout < 100*time.Millisecond || c.Timeout > 5*time.Minute {
		return errors.New("timeout must be between 100ms and 5m")
	}
	if c.SampleInterval < 100*time.Millisecond || c.SampleInterval > 30*time.Second || c.SampleInterval >= c.Timeout {
		return errors.New("sample_interval must be 100ms..30s and less than timeout")
	}
	for _, v := range []float64{c.MemoryPercent, c.DiskPercent, c.CPUPercent} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 100 {
			return errors.New("thresholds must be in (0,100]")
		}
	}
	if c.Concurrency < 1 || c.Concurrency > 16 || c.LogLimit < 1 || c.LogLimit > 1000 {
		return errors.New("concurrency must be 1..16 and log_limit 1..1000")
	}
	if c.Format != "text" && c.Format != "json" {
		return errors.New("format must be text or json")
	}
	for _, v := range c.Disabled {
		switch v {
		case "system", "process", "network", "services", "containers":
		default:
			return fmt.Errorf("unknown disabled collector %q", v)
		}
	}
	if c.AI.Provider != "ollama" && c.AI.Provider != "openai" {
		return errors.New("AI provider must be ollama or openai")
	}
	return nil
}
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)
	if err = decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("configuration: %w", err)
	}
	return c, c.Validate()
}
func Init(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(Default())
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
