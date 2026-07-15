package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mister-gnommer/trust-issues/v2/internal/analysis"
	"github.com/mister-gnommer/trust-issues/v2/internal/config"
	"github.com/mister-gnommer/trust-issues/v2/internal/report"
	"github.com/mister-gnommer/trust-issues/v2/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.toml", "path to TOML config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.LoadForReport(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	st, err := store.New(ctx, cfg.Storage.DatabasePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	return runReportOnce(ctx, logger, cfg, st, time.Now())
}

func runReportOnce(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, now time.Time) error {
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		return fmt.Errorf("load location Europe/Warsaw: %w", err)
	}
	now = now.In(loc)

	if err := os.MkdirAll(cfg.Reports.Dir, 0755); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}
	reportFilename := now.Format("20060102")
	summaryPath := filepath.Join(cfg.Reports.Dir, reportFilename+".md")
	dataPath := filepath.Join(cfg.Reports.Dir, reportFilename+"-data.md")
	_, sumErr := os.Stat(summaryPath)
	_, datErr := os.Stat(dataPath)
	// Idempotent no-op if both report files already exist.
	if sumErr == nil && datErr == nil {
		logger.Info("report already generated", "date", now.Format("2006-01-02"))
		return nil
	}
	// If exactly one file exists the prior run crashed mid-write.
	// Remove the orphan and regenerate.
	if sumErr == nil && datErr != nil {
		logger.Warn("removing partial file from prior failed run", "path", summaryPath)
		os.Remove(summaryPath)
	}
	if datErr == nil && sumErr != nil {
		logger.Warn("removing partial file from prior failed run", "path", dataPath)
		os.Remove(dataPath)
	}

	var results []analysis.Result
	failCount := 0
	for _, account := range cfg.Accounts {
		accountResults, err := analysis.Analyze(ctx, st, account.UserID, account.DisplayName, cfg.Reports.MinPlays, cfg.Reports.ResidualThreshold)
		if err != nil {
			logger.Error("analysis for account", "user_id", account.UserID, "error", err)
			failCount++
			continue
		}
		results = append(results, accountResults...)
	}

	if len(cfg.Accounts) > 0 && failCount == len(cfg.Accounts) {
		return fmt.Errorf("all %d accounts failed analysis", failCount)
	}

	summary := report.RenderSummary(now, now, loc, results, cfg.Reports.ResidualThreshold)
	data := report.RenderData(now, now, loc, results, cfg.Reports.ResidualThreshold)

	if err := report.WriteAll(cfg.Reports.Dir, now, summary, data); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	logger.Info("report written", "dir", cfg.Reports.Dir, "date", now.Format("2006-01-02"))
	return nil
}
