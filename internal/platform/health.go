package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const InstallRootEnv = "WARBAND_VAULT_INSTALL_ROOT"

func WriteHealthMarker(installRoot, version string) error {
	if installRoot == "" || version == "" || version == "dev" {
		return nil
	}
	healthDir := filepath.Join(installRoot, "state", "health")
	if err := os.MkdirAll(healthDir, 0o755); err != nil {
		return fmt.Errorf("create health directory: %w", err)
	}
	path := filepath.Join(healthDir, version+".ok")
	body := []byte(time.Now().UTC().Format(time.RFC3339Nano) + "\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write health marker: %w", err)
	}
	return nil
}

func HealthMarkerPath(installRoot, version string) string {
	return filepath.Join(installRoot, "state", "health", version+".ok")
}
