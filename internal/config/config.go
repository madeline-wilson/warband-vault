package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	appDirName     = "WarbandVault"
	configFileName = "config.json"
	maxConfigBytes = 1 << 20
)

const DefaultUpdateManifestURL = "https://github.com/madeline-wilson/warband-vault/releases/latest/download/update-manifest.json"

type Paths struct {
	RootDir    string
	Database   string
	ConfigFile string
	BackupsDir string
	ExportsDir string
	LogsDir    string
}

type Settings struct {
	UpdateCheckOnStartup bool   `json:"update_check_on_startup"`
	UpdateManifestURL    string `json:"update_manifest_url"`
}

func DefaultSettings() Settings {
	return Settings{
		UpdateCheckOnStartup: true,
		UpdateManifestURL:    DefaultUpdateManifestURL,
	}
}

func ResolvePaths(dataDir string) (Paths, error) {
	root := dataDir
	if root == "" {
		userConfig, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
		}
		root = filepath.Join(userConfig, appDirName)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory: %w", err)
	}
	paths := Paths{
		RootDir:    root,
		Database:   filepath.Join(root, "warband-vault.db"),
		ConfigFile: filepath.Join(root, configFileName),
		BackupsDir: filepath.Join(root, "backups"),
		ExportsDir: filepath.Join(root, "exports"),
		LogsDir:    filepath.Join(root, "logs"),
	}
	for _, dir := range []string{paths.RootDir, paths.BackupsDir, paths.ExportsDir, paths.LogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return paths, nil
}

func Load(path string) (Settings, error) {
	settings := DefaultSettings()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := Save(path, settings); err != nil {
			return settings, err
		}
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("stat config: %w", err)
	}
	if info.Size() > maxConfigBytes {
		return settings, fmt.Errorf("config exceeds %d bytes", maxConfigBytes)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return settings, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		recovered := path + ".invalid-" + time.Now().UTC().Format("20060102T150405")
		_ = os.Rename(path, recovered)
		if saveErr := Save(path, settings); saveErr != nil {
			return settings, fmt.Errorf("recover invalid config: %w", saveErr)
		}
		return settings, fmt.Errorf("config was invalid and reset to defaults: %w", err)
	}
	if normalizeSettings(&settings) {
		if err := Save(path, settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func normalizeSettings(settings *Settings) bool {
	if settings == nil {
		return false
	}
	switch settings.UpdateManifestURL {
	case "", "https://github.com/example/warband-vault/releases/latest/download/update-manifest.json":
		settings.UpdateManifestURL = DefaultUpdateManifestURL
		return true
	default:
		return false
	}
}

func Save(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	body = append(body, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
