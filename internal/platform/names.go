package platform

import (
	"path/filepath"
	"runtime"
	"strings"
)

func ExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func PlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func InstallRootFromExecutable(executable string) string {
	return installRootFromExecutable(runtime.GOOS, executable)
}

func installRootFromExecutable(goos, executable string) string {
	dir := filepath.Dir(executable)
	if goos != "darwin" {
		return dir
	}
	macOSDir := filepath.Dir(filepath.Clean(executable))
	contentsDir := filepath.Dir(macOSDir)
	appDir := filepath.Dir(contentsDir)
	if filepath.Base(macOSDir) == "MacOS" && filepath.Base(contentsDir) == "Contents" && strings.HasSuffix(filepath.Base(appDir), ".app") {
		return filepath.Join(contentsDir, "Resources", "WarbandVault")
	}
	return dir
}
