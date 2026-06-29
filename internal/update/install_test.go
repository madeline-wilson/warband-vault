package update

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStageVersionFromFullInstallArchive(t *testing.T) {
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	version := "v1.2.0"
	mainExecutable := "warband-vault" + exe
	updaterExecutable := "warband-vault-updater" + exe
	archive := writeZip(t, map[string][]byte{
		filepath.ToSlash(filepath.Join("WarbandVault", "versions", version, mainExecutable)):    []byte("app"),
		filepath.ToSlash(filepath.Join("WarbandVault", "versions", version, updaterExecutable)): []byte("updater"),
	}, "")
	root := t.TempDir()
	state, err := StageVersion(context.Background(), InstallOptions{
		InstallRoot:       root,
		Version:           version,
		PackagePath:       archive,
		MainExecutable:    mainExecutable,
		UpdaterExecutable: updaterExecutable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.RelativeExecutable != filepath.ToSlash(filepath.Join("versions", version, mainExecutable)) {
		t.Fatalf("unexpected state executable: %#v", state)
	}
	if _, err := os.Stat(filepath.Join(root, "versions", version, mainExecutable)); err != nil {
		t.Fatalf("expected staged app executable: %v", err)
	}
	if _, err := CurrentState(root); err != nil {
		t.Fatalf("expected current state: %v", err)
	}
}

func TestCheckInstallRootWritableAllowsTempDir(t *testing.T) {
	if err := CheckInstallRootWritable(t.TempDir()); err != nil {
		t.Fatalf("expected writable install root: %v", err)
	}
}

func TestCheckInstallRootWritableRejectsAppTranslocation(t *testing.T) {
	root := "/private/var/folders/_7/example/T/AppTranslocation/977D5649-CBEC-42E6-8606-31A8F7BD4186/d/Warband Vault.app/Contents/Resources/WarbandVault"
	err := CheckInstallRootWritable(root)
	if err == nil {
		t.Fatal("expected App Translocation rejection")
	}
	if !strings.Contains(err.Error(), "App Translocation") {
		t.Fatalf("expected App Translocation error, got %v", err)
	}
}

func TestIsAppTranslocationPath(t *testing.T) {
	if !IsAppTranslocationPath("/private/var/folders/x/T/AppTranslocation/id/d/Warband Vault.app/Contents/Resources/WarbandVault") {
		t.Fatal("expected App Translocation path to be detected")
	}
	if IsAppTranslocationPath("/Applications/Warband Vault.app/Contents/Resources/WarbandVault") {
		t.Fatal("expected normal Applications path not to be detected")
	}
}

func TestStageVersionFromMacAppArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS app bundle paths are not portable on Windows")
	}
	version := "v2.0.0"
	archive := writeZip(t, map[string][]byte{
		filepath.ToSlash(filepath.Join("Warband Vault.app", "Contents", "Resources", "WarbandVault", "versions", version, "warband-vault")):         []byte("app"),
		filepath.ToSlash(filepath.Join("Warband Vault.app", "Contents", "Resources", "WarbandVault", "versions", version, "warband-vault-updater")): []byte("updater"),
	}, "")
	root := t.TempDir()
	if _, err := StageVersion(context.Background(), InstallOptions{
		InstallRoot:       root,
		Version:           version,
		PackagePath:       archive,
		MainExecutable:    "warband-vault",
		UpdaterExecutable: "warband-vault-updater",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "versions", version, "warband-vault")); err != nil {
		t.Fatalf("expected staged mac app payload: %v", err)
	}
}
