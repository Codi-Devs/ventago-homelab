package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultPath  = "configs/config.json"
	DefaultAddr  = ":8080"
	DefaultHost  = "http://ollama:11434"
	DefaultModel = "qwen3.5:4b"
)

type Worker struct {
	Enabled bool `json:"enabled"`
}

type Config struct {
	HTTPAddr         string            `json:"http_addr"`
	OllamaHost       string            `json:"ollama_host"`
	OllamaModel      string            `json:"ollama_model"`
	VentagoAPIBase   string            `json:"ventago_api_base"`
	MySQLDSN         string            `json:"mysql_dsn"`
	PaddleContainer  string            `json:"paddle_container"`
	OCRWorkDir       string            `json:"ocr_work_dir"`
	Workers          map[string]Worker `json:"workers"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = DefaultPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		c.HTTPAddr = DefaultAddr
	}
	if strings.TrimSpace(c.OllamaHost) == "" {
		c.OllamaHost = DefaultHost
	}
	if strings.TrimSpace(c.OllamaModel) == "" {
		c.OllamaModel = DefaultModel
	}
	if strings.TrimSpace(c.VentagoAPIBase) == "" {
		c.VentagoAPIBase = strings.TrimSpace(os.Getenv("VENTAGO_API_BASE"))
	}
	if strings.TrimSpace(c.MySQLDSN) == "" {
		c.MySQLDSN = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if strings.TrimSpace(c.PaddleContainer) == "" {
		c.PaddleContainer = "paddleocr"
	}
	if strings.TrimSpace(c.OCRWorkDir) == "" {
		c.OCRWorkDir = "/opt/ventago-lab/ocr/work"
	}
	if c.Workers == nil {
		c.Workers = map[string]Worker{}
	}
}

func (c Config) validate() error {
	if c.OllamaModel != DefaultModel {
		return fmt.Errorf("ollama_model must be %s (got %q); deepseek and other models are out of architecture", DefaultModel, c.OllamaModel)
	}
	return nil
}

func (c Config) WorkerEnabled(name string) bool {
	w, ok := c.Workers[name]
	return ok && w.Enabled
}
