package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathsCreatesDirectories(t *testing.T) {
	root := t.TempDir()
	paths, err := ResolvePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{paths.RootDir, paths.BackupsDir, paths.ExportsDir, paths.LogsDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected directory %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
}

func TestInvalidConfigRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid config warning")
	}
	if !settings.UpdateCheckOnStartup {
		t.Fatal("expected default settings after recovery")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected replacement config: %v", statErr)
	}
}
