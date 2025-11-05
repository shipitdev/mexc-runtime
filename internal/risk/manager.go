package risk

import (
	"context"
	"log/slog"
	"sync"
	"time"

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
	RecordExecution(ctx context.Context, sig signal.Signal, executedNotional float64)
}

// SimpleManager implements baseline risk checks; extend with stateful limits later.
type SimpleManager struct {
	logger *slog.Logger
	cfg    config.RiskConfig

	mu          sync.Mutex
	lastTrade   map[string]time.Time
	dailyTrades int
	dayAnchor   time.Time
}

func NewSimpleManager(logger *slog.Logger, cfg config.RiskConfig) *SimpleManager {
	return &SimpleManager{
		logger:    logger,
		cfg:       cfg,
		lastTrade: make(map[string]time.Time),
	}
}

func (m *SimpleManager) Evaluate(ctx context.Context, sig signal.Signal, desiredNotional float64) (Decision, error) {
	if desiredNotional <= 0 {
		return Decision{Allow: false, Reason: "non-positive notional"}, nil
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resetDay(now)

	if m.cfg.CooldownSeconds > 0 {
		if last, ok := m.lastTrade[sig.Symbol]; ok {
			cooldown := time.Duration(m.cfg.CooldownSeconds) * time.Second
			if since := now.Sub(last); since < cooldown {
				return Decision{Allow: false, Reason: "cooldown_active"}, nil
			}
		}
	}

	if m.cfg.MaxDailyTrades > 0 && m.dailyTrades >= m.cfg.MaxDailyTrades {
		return Decision{Allow: false, Reason: "daily_trade_limit"}, nil
	}

	return Decision{
		Allow:    true,
		Notional: desiredNotional,
	}, nil
}

func (m *SimpleManager) RecordExecution(ctx context.Context, sig signal.Signal, executedNotional float64) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resetDay(now)
	m.lastTrade[sig.Symbol] = now
	m.dailyTrades++

	m.logger.DebugContext(ctx, "risk recorded execution", "symbol", sig.Symbol, "notional", executedNotional, "daily_trades", m.dailyTrades)
}

func (m *SimpleManager) resetDay(now time.Time) {
	// Reset daily counters if we crossed UTC day boundary.
	if m.dayAnchor.IsZero() {
		m.dayAnchor = now.UTC().Truncate(24 * time.Hour)
		return
	}

	currentDay := now.UTC().Truncate(24 * time.Hour)
	if currentDay.After(m.dayAnchor) {
		m.dayAnchor = currentDay
		m.dailyTrades = 0
		m.lastTrade = make(map[string]time.Time)
	}
}
