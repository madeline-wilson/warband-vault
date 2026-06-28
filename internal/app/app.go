package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"warband-vault/assets"
	"warband-vault/internal/buildinfo"
	"warband-vault/internal/campaign"
	"warband-vault/internal/config"
	"warband-vault/internal/logging"
	"warband-vault/internal/persistence"
)

type Options struct {
	DataDir       string
	NoUpdateCheck bool
}

type Services struct {
	BuildInfo buildinfo.Info
	Paths     config.Paths
	Settings  config.Settings
	Logger    *slog.Logger
	LogClose  logging.CloseFunc
	Store     *persistence.Store
}

func Initialize(ctx context.Context, opts Options) (*Services, error) {
	paths, err := config.ResolvePaths(opts.DataDir)
	if err != nil {
		return nil, err
	}
	info := buildinfo.Current()
	logger, closeLog, err := logging.Init(paths.LogsDir, info)
	if err != nil {
		return nil, err
	}
	settings, err := config.Load(paths.ConfigFile)
	if err != nil {
		logger.Warn("configuration recovered or failed to load", "error", err)
		if strings.Contains(err.Error(), "reset to defaults") {
			err = nil
		}
	}
	if opts.NoUpdateCheck {
		settings.UpdateCheckOnStartup = false
	}
	if err != nil {
		closeLog()
		return nil, err
	}
	store, err := persistence.Open(ctx, paths, logger)
	if err != nil {
		closeLog()
		return nil, err
	}
	return &Services{
		BuildInfo: info,
		Paths:     paths,
		Settings:  settings,
		Logger:    logger,
		LogClose:  closeLog,
		Store:     store,
	}, nil
}

func (s *Services) Close() error {
	var first error
	if s == nil {
		return nil
	}
	if s.Store != nil {
		if err := s.Store.Close(); err != nil && first == nil {
			first = err
		}
	}
	if s.LogClose != nil {
		if err := s.LogClose(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Services) CreateExampleCampaign(ctx context.Context) (*campaign.Campaign, error) {
	example := campaign.ExampleBlackwaterExpedition()
	if err := s.Store.Campaigns.Create(ctx, &example); err != nil {
		return nil, fmt.Errorf("create example campaign: %w", err)
	}
	return &example, nil
}

func CheckEmbeddedAssets() error {
	if _, err := assets.Files.ReadFile("update_public_key.txt"); err != nil {
		return fmt.Errorf("read embedded update public key: %w", err)
	}
	return nil
}
