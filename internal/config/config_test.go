package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsNonCanonicalModel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte(`{"ollama_model":"deepseek-r1:8b"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for non-canonical model")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != DefaultAddr || cfg.OllamaModel != DefaultModel {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.WorkerEnabled("ocr") {
		t.Fatal("ocr should be disabled by default")
	}
}
