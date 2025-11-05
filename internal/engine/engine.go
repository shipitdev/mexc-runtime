package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/user/mexc-bot/internal/config"
	"github.com/user/mexc-bot/internal/exchange"
	"github.com/user/mexc-bot/internal/risk"
	"github.com/user/mexc-bot/internal/signal"
)

// Engine ties signal parsing, risk evaluation, and order execution together.
type Engine struct {
	logger   *slog.Logger
	cfg      *config.Config
	parser   *signal.Parser
	risk     risk.Manager
	executor exchange.Executor
}

func New(cfg *config.Config, parser *signal.Parser, riskManager risk.Manager, executor exchange.Executor, logger *slog.Logger) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("config must not be nil")
	}
	if parser == nil {
		return nil, errors.New("parser must not be nil")
	}
	if riskManager == nil {
		return nil, errors.New("risk manager must not be nil")
	}
	if executor == nil {
		return nil, errors.New("executor must not be nil")
	}

	return &Engine{
		logger:   logger,
		cfg:      cfg,
		parser:   parser,
		risk:     riskManager,
		executor: executor,
	}, nil
}

// HandleMessage ingests a Telegram message and attempts to trade it.
func (e *Engine) HandleMessage(ctx context.Context, msg signal.Message) error {
	sig, err := e.parser.Parse(msg)
	if err != nil {
		return fmt.Errorf("parse signal: %w", err)
	}

	notional := e.resolveNotional(sig.Symbol)

	decision, err := e.risk.Evaluate(ctx, *sig, notional)
	if err != nil {
		return fmt.Errorf("risk evaluation failed: %w", err)
	}
	if !decision.Allow {
		e.logger.InfoContext(ctx, "signal skipped by risk", "symbol", sig.Symbol, "reason", decision.Reason)
		return nil
	}

	req := exchange.OrderRequest{
		Symbol:      sig.Symbol,
		Notional:    decision.Notional,
		Side:        exchange.OrderSideBuy,
		Type:        e.resolveOrderType(),
		SlippageBps: e.cfg.Trading.SlippageBps,
		Metadata: map[string]string{
			"source_message_id": fmt.Sprintf("%d", msg.ID),
		},
	}

	ack, err := e.executor.Submit(ctx, req)
	if err != nil {
		return fmt.Errorf("order submission failed: %w", err)
	}

	e.risk.RecordExecution(ctx, *sig, decision.Notional)

	e.logger.InfoContext(ctx, "order submitted", "order_id", ack.OrderID, "executor", e.executor.Name(), "symbol", req.Symbol, "notional", req.Notional)

	return nil
}

func (e *Engine) resolveNotional(symbol string) float64 {
	size := e.cfg.Trading.DefaultBaseNotional
	if ov, ok := e.cfg.Overrides[symbol]; ok {
		if ov.DefaultBaseNotional > 0 {
			size = ov.DefaultBaseNotional
		}
		if ov.MaxNotional > 0 && size > ov.MaxNotional {
			size = ov.MaxNotional
		}
	}
	if size > e.cfg.Trading.MaxNotional {
		size = e.cfg.Trading.MaxNotional
	}
	return size
}

func (e *Engine) resolveOrderType() exchange.OrderType {
	switch e.cfg.Trading.OrderType {
	case "limit":
		return exchange.OrderTypeLimit
	default:
		return exchange.OrderTypeMarket
	}
}
