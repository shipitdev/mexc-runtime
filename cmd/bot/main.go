package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	osSignal "os/signal"
	"strings"
	"syscall"

	"github.com/user/mexc-bot/internal/config"
	"github.com/user/mexc-bot/internal/engine"
	"github.com/user/mexc-bot/internal/exchange"
	"github.com/user/mexc-bot/internal/risk"
	signalpkg "github.com/user/mexc-bot/internal/signal"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config/example.toml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Telemetry.LogLevel)
	logger.Info("configuration loaded", "path", cfg.ConfigPath, "environment", cfg.Mode.Environment, "market_type", cfg.Mode.MarketType, "dry_run", cfg.Debug.DryRun)

	parser := signalpkg.NewParser(cfg.Parser)
	riskManager := risk.NewSimpleManager(logger, cfg.Risk)

	var executor exchange.Executor
	if cfg.Debug.DryRun {
		executor = exchange.NewDryRunExecutor(logger)
	} else {
		// Placeholder until the real MEXC executor is implemented.
		executor = exchange.NewDryRunExecutor(logger)
	}

	core, err := engine.New(cfg, parser, riskManager, executor, logger)
	if err != nil {
		logger.Error("initialise engine", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listenForShutdown(cancel)

	// TODO: wire incoming Telegram updates into msgCh.
	msgCh := make(chan signalpkg.Message)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgCh:
				if err := core.HandleMessage(ctx, msg); err != nil {
					logger.Error("handle message", "error", err)
				}
			}
		}
	}()

	logger.Info("bot ready", "executor", executor.Name())

	<-ctx.Done()
	logger.Info("shutdown complete")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})
	return slog.New(handler)
}

func listenForShutdown(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	osSignal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	cancel()
}
