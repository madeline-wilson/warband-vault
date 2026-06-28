package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	"warband-vault/internal/buildinfo"
)

type CloseFunc func() error

func Init(logDir string, info buildinfo.Info) (*slog.Logger, CloseFunc, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	if err := cleanup(logDir, 5); err != nil {
		return nil, nil, fmt.Errorf("clean logs: %w", err)
	}
	path := filepath.Join(logDir, "warband-vault-"+time.Now().UTC().Format("20060102T150405Z")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	handler := slog.NewJSONHandler(io.MultiWriter(os.Stderr, file), &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	logger.Info("application startup",
		"version", info.Version,
		"commit", info.Commit,
		"build_date", info.BuildDate,
		"channel", info.Channel,
		"log_file", path,
	)
	return logger, file.Close, nil
}

func cleanup(logDir string, retain int) error {
	matches, err := filepath.Glob(filepath.Join(logDir, "warband-vault-*.log"))
	if err != nil {
		return err
	}
	sort.Strings(matches)
	remove := len(matches) - retain
	for i := 0; i < remove; i++ {
		if err := os.Remove(matches[i]); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func RecoverCommand(logger *slog.Logger) {
	if value := recover(); value != nil {
		if logger != nil {
			logger.Error("panic recovered at command entry", "panic", value, "stack", string(debug.Stack()))
		} else {
			fmt.Fprintf(os.Stderr, "panic: %v\n%s\n", value, debug.Stack())
		}
		os.Exit(2)
	}
}
