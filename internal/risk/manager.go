package risk

import (
	"context"
	"log/slog"

	"github.com/user/mexc-bot/internal/config"
	"github.com/user/mexc-bot/internal/signal"
)

// Decision captures the outcome of a risk evaluation.
type Decision struct {
	Allow    bool
	Reason   string
	Notional float64
}

// Manager evaluates whether a signal can be traded.
type Manager interface {
	Evaluate(ctx context.Context, sig signal.Signal, desiredNotional float64) (Decision, error)
}

// SimpleManager implements baseline risk checks; extend with stateful limits later.
type SimpleManager struct {
	logger *slog.Logger
	cfg    config.RiskConfig
}

func NewSimpleManager(logger *slog.Logger, cfg config.RiskConfig) *SimpleManager {
	return &SimpleManager{
		logger: logger,
		cfg:    cfg,
	}
}

func (m *SimpleManager) Evaluate(ctx context.Context, sig signal.Signal, desiredNotional float64) (Decision, error) {
	if desiredNotional <= 0 {
		return Decision{Allow: false, Reason: "non-positive notional"}, nil
	}

	// Placeholder: enforce future drawdown/cooldown tracking here.
	m.logger.DebugContext(ctx, "risk accepted signal", "symbol", sig.Symbol, "notional", desiredNotional)

	return Decision{
		Allow:    true,
		Notional: desiredNotional,
	}, nil
}
