package exchange

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// OrderSide defines whether we buy or sell.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType identifies the exchange order flavour.
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// OrderRequest contains the required data to submit an exchange order.
type OrderRequest struct {
	Symbol      string
	Notional    float64
	Side        OrderSide
	Type        OrderType
	SlippageBps int
	Metadata    map[string]string
}

// OrderAck represents the exchange acknowledgement.
type OrderAck struct {
	OrderID     string
	SubmittedAt time.Time
}

// Executor abstracts order submission.
type Executor interface {
	Submit(ctx context.Context, req OrderRequest) (OrderAck, error)
	Name() string
}

// DryRunExecutor logs orders without sending anything to the exchange.
type DryRunExecutor struct {
	logger *slog.Logger
}

func NewDryRunExecutor(logger *slog.Logger) *DryRunExecutor {
	return &DryRunExecutor{logger: logger}
}

func (d *DryRunExecutor) Name() string {
	return "dry-run"
}

func (d *DryRunExecutor) Submit(ctx context.Context, req OrderRequest) (OrderAck, error) {
	d.logger.InfoContext(ctx, "dry-run order", "symbol", req.Symbol, "side", req.Side, "notional", req.Notional, "type", req.Type)
	if req.Notional <= 0 {
		return OrderAck{}, fmt.Errorf("invalid notional %f", req.Notional)
	}
	return OrderAck{
		OrderID:     "dry-run",
		SubmittedAt: time.Now(),
	}, nil
}
