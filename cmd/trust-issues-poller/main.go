package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mister-gnommer/trust-issues/v2/internal/config"
	"github.com/mister-gnommer/trust-issues/v2/internal/playback"
	"github.com/mister-gnommer/trust-issues/v2/internal/playlists"
	"github.com/mister-gnommer/trust-issues/v2/internal/spotify"
	"github.com/mister-gnommer/trust-issues/v2/internal/store"
)

// shutdownDeadline is the hard cap on how long we wait for goroutines after
// the root context is canceled. If something is wedged past this, we exit non-zero
// and let systemd restart us rather than hang forever.
const shutdownDeadline = 10 * time.Second

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.toml", "path to TOML config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.New(ctx, cfg.Storage.DatabasePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	logger.Info("starting", "accounts", len(cfg.Accounts), "db", cfg.Storage.DatabasePath)

	return runPollers(ctx, logger, cfg, st)
}

func runPollers(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for _, account := range cfg.Accounts {
		client := spotify.NewClient(egCtx, cfg.App.ClientID, cfg.App.ClientSecret, account.RefreshToken)
		pbAcct := playback.Account{UserID: account.UserID, DisplayName: account.DisplayName}
		plAcct := playlists.Account{UserID: account.UserID, DisplayName: account.DisplayName}

		eg.Go(func() error {
			return playback.Run(egCtx, logger, playback.Config{}, pbAcct, client, st)
		})
		eg.Go(func() error {
			return playlists.Run(egCtx, logger, playlists.Config{}, plAcct, client, st)
		})
	}

	go func() {
		<-ctx.Done()
		time.AfterFunc(shutdownDeadline, func() {
			logger.Error("shutdown deadline exceeded; forcing exit")
			os.Exit(2)
		})
	}()

	err := eg.Wait()
	if errors.Is(err, context.Canceled) {
		logger.Info("shutdown")
		return nil
	}
	return err
}
