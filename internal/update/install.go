package update

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"warband-vault/internal/platform"
)

type InstallOptions struct {
	InstallRoot        string
	Version            string
	PackagePath        string
	MainExecutable     string
	UpdaterExecutable  string
	StartupTimeout     time.Duration
	ExpectedHealthFile string
	Logger             *slog.Logger
}

func StageVersion(ctx context.Context, opts InstallOptions) (State, error) {
	if opts.InstallRoot == "" {
		return State{}, fmt.Errorf("install root is required")
	}
	if opts.Version == "" {
		return State{}, fmt.Errorf("version is required")
	}
	if opts.MainExecutable == "" {
		opts.MainExecutable = platform.ExecutableName("warband-vault")
	}
	if opts.UpdaterExecutable == "" {
		opts.UpdaterExecutable = platform.ExecutableName("warband-vault-updater")
	}
	lock, err := AcquireLock(opts.InstallRoot)
	if err != nil {
		return State{}, err
	}
	defer lock.Release()
	staging := filepath.Join(opts.InstallRoot, "versions", "."+opts.Version+".staging")
	finalDir := filepath.Join(opts.InstallRoot, "versions", opts.Version)
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return State{}, fmt.Errorf("create staging directory: %w", err)
	}
	extractOpts := DefaultExtractionOptions()
	if err := ExtractArchive(ctx, opts.PackagePath, staging, extractOpts); err != nil {
		_ = os.RemoveAll(staging)
		return State{}, err
	}
	payloadDir, err := findVersionPayload(staging, opts.Version, opts.MainExecutable, opts.UpdaterExecutable)
	if err != nil {
		_ = os.RemoveAll(staging)
		return State{}, err
	}
	if _, err := os.Stat(finalDir); err == nil {
		_ = os.RemoveAll(staging)
		return State{}, fmt.Errorf("version %s is already installed", opts.Version)
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		_ = os.RemoveAll(staging)
		return State{}, fmt.Errorf("create versions directory: %w", err)
	}
	if err := os.Rename(payloadDir, finalDir); err != nil {
		_ = os.RemoveAll(staging)
		return State{}, fmt.Errorf("activate staged version: %w", err)
	}
	if payloadDir != staging {
		_ = os.RemoveAll(staging)
	}
	next := State{
		Version:            normalizeVersion(opts.Version),
		RelativeExecutable: filepath.ToSlash(filepath.Join("versions", opts.Version, opts.MainExecutable)),
	}
	if err := WriteCurrentState(opts.InstallRoot, next); err != nil {
		return State{}, err
	}
	if opts.Logger != nil {
		opts.Logger.Info("version staged", "version", opts.Version, "executable", next.RelativeExecutable)
	}
	return next, nil
}

func findVersionPayload(staging, version, mainExecutable, updaterExecutable string) (string, error) {
	candidates := []string{
		staging,
		filepath.Join(staging, "versions", version),
		filepath.Join(staging, "WarbandVault", "versions", version),
		filepath.Join(staging, "Warband Vault.app", "Contents", "Resources", "WarbandVault", "versions", version),
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		clean, err := filepath.Abs(candidate)
		if err != nil {
			return "", fmt.Errorf("resolve staged payload: %w", err)
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		if err := validateVersionPayload(clean, mainExecutable, updaterExecutable); err == nil {
			return clean, nil
		}
	}
	return "", fmt.Errorf("archive does not contain installable version payload %s", version)
}

func validateVersionPayload(dir, mainExecutable, updaterExecutable string) error {
	for _, executable := range []string{mainExecutable, updaterExecutable} {
		path := filepath.Join(dir, executable)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("expected executable %s is absent: %w", executable, err)
		}
		if info.IsDir() {
			return fmt.Errorf("expected executable %s is a directory", executable)
		}
		if err := os.Chmod(path, 0o755); err != nil {
			return fmt.Errorf("set executable permissions on %s: %w", executable, err)
		}
	}
	return nil
}

func WaitForHealth(ctx context.Context, marker string, timeout time.Duration) error {
	if marker == "" {
		return fmt.Errorf("health marker path is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("startup health marker did not appear: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
