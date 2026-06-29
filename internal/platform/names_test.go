package platform

import (
	"path/filepath"
	"testing"
)

func TestInstallRootFromMacAppExecutable(t *testing.T) {
	executable := filepath.FromSlash("/Applications/Warband Vault.app/Contents/MacOS/Warband Vault")
	got := installRootFromExecutable("darwin", executable)
	want := filepath.FromSlash("/Applications/Warband Vault.app/Contents/Resources/WarbandVault")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestInstallRootFromPlainExecutable(t *testing.T) {
	executable := filepath.FromSlash("/opt/WarbandVault/warband-vault-launcher")
	got := installRootFromExecutable("linux", executable)
	want := filepath.Dir(executable)
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
