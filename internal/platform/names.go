package platform

import "runtime"

func ExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func PlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}
