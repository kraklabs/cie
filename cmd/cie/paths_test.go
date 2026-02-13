package main

import (
	"path/filepath"
	"testing"
)

func TestDataRootFromConfig_Default(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CIE_DATA_DIR", "")

	root, err := dataRootFromConfig(&Config{ProjectID: "demo"}, "")
	if err != nil {
		t.Fatalf("dataRootFromConfig() error = %v", err)
	}

	want := filepath.Join(home, ".cie", "data")
	if root != want {
		t.Fatalf("dataRootFromConfig() = %q, want %q", root, want)
	}
}

func TestDataRootFromConfig_EnvOverride(t *testing.T) {
	t.Setenv("CIE_DATA_DIR", "/tmp/custom-cie")

	root, err := dataRootFromConfig(&Config{ProjectID: "demo"}, "")
	if err != nil {
		t.Fatalf("dataRootFromConfig() error = %v", err)
	}
	if root != "/tmp/custom-cie" {
		t.Fatalf("dataRootFromConfig() = %q, want %q", root, "/tmp/custom-cie")
	}
}

func TestDataRootFromConfig_RelativeLocalDataDir(t *testing.T) {
	t.Setenv("CIE_DATA_DIR", "")

	repo := t.TempDir()
	cfg := &Config{
		ProjectID: "demo",
		Indexing: IndexingConfig{
			LocalDataDir: "./.cie/db",
		},
	}

	cfgPath := filepath.Join(repo, ".cie", "project.yaml")
	root, err := dataRootFromConfig(cfg, cfgPath)
	if err != nil {
		t.Fatalf("dataRootFromConfig() error = %v", err)
	}

	want := filepath.Join(repo, ".cie", ".cie", "db")
	if root != want {
		t.Fatalf("dataRootFromConfig() = %q, want %q", root, want)
	}
}

func TestProjectDataDir_AppendsProjectID(t *testing.T) {
	t.Setenv("CIE_DATA_DIR", "/tmp/cie-root")

	dir, err := projectDataDir(&Config{ProjectID: "my-project"}, "")
	if err != nil {
		t.Fatalf("projectDataDir() error = %v", err)
	}
	if dir != "/tmp/cie-root/my-project" {
		t.Fatalf("projectDataDir() = %q, want %q", dir, "/tmp/cie-root/my-project")
	}
}
