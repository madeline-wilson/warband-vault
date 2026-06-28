package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"warband-vault/assets"
	"warband-vault/internal/buildinfo"
	"warband-vault/internal/platform"
	"warband-vault/internal/update"
)

func main() {
	versionFlag := flag.Bool("version", false, "print updater version")
	manifestURL := flag.String("manifest-url", "", "signed update manifest URL")
	currentVersion := flag.String("current-version", buildinfo.Version, "currently running application version")
	launcherVersion := flag.String("launcher-version", buildinfo.Version, "launcher protocol version")
	installRoot := flag.String("install-root", "", "installation root")
	downloadOnly := flag.Bool("download", false, "download and verify the selected package")
	stagePackage := flag.String("stage-package", "", "verified package archive to stage")
	stageVersion := flag.String("stage-version", "", "version directory to stage")
	allowHTTP := flag.Bool("allow-http", false, "allow HTTP URLs for local tests")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("warband-vault-updater %s\n", buildinfo.Version)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if *stagePackage != "" {
		root := requiredRoot(*installRoot)
		version := *stageVersion
		if version == "" {
			fmt.Fprintln(os.Stderr, "--stage-version is required with --stage-package")
			os.Exit(2)
		}
		_, err := update.StageVersion(ctx, update.InstallOptions{
			InstallRoot:       root,
			Version:           version,
			PackagePath:       *stagePackage,
			MainExecutable:    platform.ExecutableName("warband-vault"),
			UpdaterExecutable: platform.ExecutableName("warband-vault-updater"),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "stage update failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *manifestURL == "" {
		fmt.Fprintln(os.Stderr, "--manifest-url is required")
		os.Exit(2)
	}
	keyBytes, err := assets.Files.ReadFile("update_public_key.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read embedded update public key: %v\n", err)
		os.Exit(1)
	}
	publicKey, err := update.DecodePublicKeyB64(string(keyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode update public key: %v\n", err)
		os.Exit(1)
	}
	downloader := update.NewDownloader(30*time.Second, nil)
	manifest, _, err := downloader.FetchSignedManifest(ctx, *manifestURL, publicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check updates failed: %v\n", err)
		os.Exit(1)
	}
	selection, err := manifest.Select(update.SelectionOptions{
		CurrentVersion:  *currentVersion,
		LauncherVersion: *launcherVersion,
		PlatformKey:     platform.PlatformKey(),
		AllowHTTP:       *allowHTTP,
	})
	if err != nil {
		if err == update.ErrNoUpdate {
			fmt.Println("Warband Vault is already up to date.")
			return
		}
		fmt.Fprintf(os.Stderr, "update is not installable: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Update available: %s\n", selection.Version)
	if *downloadOnly {
		root := requiredRoot(*installRoot)
		path, err := downloader.DownloadArtifact(ctx, selection.Asset.URL, filepath.Join(root, "downloads"), selection.Asset.Size, selection.Asset.SHA256, *allowHTTP)
		if err != nil {
			fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(path)
	}
}

func requiredRoot(value string) string {
	if value != "" {
		return value
	}
	if env := os.Getenv(platform.InstallRootEnv); env != "" {
		return env
	}
	fmt.Fprintln(os.Stderr, "--install-root is required")
	os.Exit(2)
	return ""
}
