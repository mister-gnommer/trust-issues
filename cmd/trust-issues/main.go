// 🤖 AI-generated
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

	"github.com/mister-gnommer/trust-issues/internal/config"
	"github.com/mister-gnommer/trust-issues/internal/playback"
	"github.com/mister-gnommer/trust-issues/internal/playlists"
	"github.com/mister-gnommer/trust-issues/internal/spotify"
	"github.com/mister-gnommer/trust-issues/internal/store"
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

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.Storage.DatabasePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	logger.Info("starting", "accounts", len(cfg.Accounts), "db", cfg.Storage.DatabasePath)

	eg, egCtx := errgroup.WithContext(ctx)
	for _, account := range cfg.Accounts {
		client := spotify.NewClient(egCtx, cfg.App.ClientID, cfg.App.ClientSecret, account.RefreshToken)
		// Duplicated Account structs below (playback.Account vs playlists.Account)
		// is intentional — each package owns its own types at its boundary.
		// Sharing a common type would create an import dependency between them.
		// human comm: I don't yet know if above is true :shrug:
		pbAcct := playback.Account{UserID: account.UserID, DisplayName: account.DisplayName}
		plAcct := playlists.Account{UserID: account.UserID, DisplayName: account.DisplayName}

		eg.Go(func() error {
			return playback.Run(egCtx, logger, playback.Config{}, pbAcct, client, st)
		})
		eg.Go(func() error {
			return playlists.Run(egCtx, logger, playlists.Config{}, plAcct, client, st)
		})
	}

	// Watch for shutdown: once ctx is canceled, give goroutines a bounded
	// window to wind down. If they don't, force-exit so a wedged HTTP
	// connection or DB lock can't keep the process alive forever.
	go func() {
		<-ctx.Done()
		t := time.AfterFunc(shutdownDeadline, func() {
			logger.Error("shutdown deadline exceeded; forcing exit")
			os.Exit(2)
		})
		_ = t // referenced for future cancellation if needed
	}()

	err = eg.Wait()
	if errors.Is(err, context.Canceled) {
		logger.Info("shutdown")
		return nil
	}
	return err
}
