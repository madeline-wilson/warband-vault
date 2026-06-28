package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"warband-vault/internal/app"
	"warband-vault/internal/buildinfo"
	"warband-vault/internal/logging"
	"warband-vault/internal/platform"
	"warband-vault/ui"
)

func main() {
	version := flag.Bool("version", false, "print version information")
	smokeTest := flag.Bool("smoke-test", false, "initialize services and GUI framework, then exit")
	dataDir := flag.String("data-dir", "", "override user data directory")
	noUpdateCheck := flag.Bool("no-update-check", false, "disable update checks for this run")
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.String())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	services, err := app.Initialize(ctx, app.Options{DataDir: *dataDir, NoUpdateCheck: *noUpdateCheck})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize Warband Vault: %v\n", err)
		os.Exit(1)
	}
	defer services.Close()
	defer logging.RecoverCommand(services.Logger)

	if err := app.CheckEmbeddedAssets(); err != nil {
		services.Logger.Error("embedded asset check failed", "error", err)
		fmt.Fprintf(os.Stderr, "embedded asset check failed: %v\n", err)
		os.Exit(1)
	}

	if *smokeTest {
		smoke := fyneapp.NewWithID("com.warbandvault.app.smoke")
		w := smoke.NewWindow("Warband Vault Smoke Test")
		w.Resize(fyne.NewSize(1, 1))
		smoke.Quit()
		if err := platform.WriteHealthMarker(os.Getenv(platform.InstallRootEnv), buildinfo.Version); err != nil {
			services.Logger.Warn("health marker write failed", "error", err)
		}
		return
	}

	if err := platform.WriteHealthMarker(os.Getenv(platform.InstallRootEnv), buildinfo.Version); err != nil {
		services.Logger.Warn("health marker write failed", "error", err)
	}
	ui.Run(services)
}
