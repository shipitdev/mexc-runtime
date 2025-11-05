package mexc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/mexc-bot/internal/config"
	"github.com/user/mexc-bot/internal/exchange"
)

// Executor implements the exchange.Executor interface using MEXC REST endpoints.
type Executor struct {
	logger    *slog.Logger
	client    *http.Client
	apiKey    string
	apiSecret string
	baseURL   string
	market    string
}

// NewExecutor constructs a live MEXC executor.
func NewExecutor(cfg *config.Config, apiKey, apiSecret string, logger *slog.Logger) (*Executor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return nil, fmt.Errorf("api key/secret required for live trading")
	}
	baseURL, err := resolveBaseURL(cfg.Mode.Environment)
	if err != nil {
		return nil, err
	}

	return &Executor{
		logger:    logger,
		client:    &http.Client{Timeout: 5 * time.Second},
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		market:    cfg.Mode.MarketType,
	}, nil
}

func resolveBaseURL(env string) (string, error) {
	switch env {
	case "live":
		return "https://api.mexc.com", nil
	case "testnet":
		return "https://testnet.mexc.com", nil
	default:
		return "", fmt.Errorf("unknown environment %q", env)
	}
}

func (e *Executor) Name() string {
	return "mexc-rest"
}

func (e *Executor) Submit(ctx context.Context, req exchange.OrderRequest) (exchange.OrderAck, error) {
	switch e.market {
	case "spot":
		return e.submitSpot(ctx, req)
	default:
		return exchange.OrderAck{}, fmt.Errorf("market %s not yet supported", e.market)
	}
}

func (e *Executor) submitSpot(ctx context.Context, req exchange.OrderRequest) (exchange.OrderAck, error) {
	params := map[string]string{
		"symbol":     strings.ToUpper(req.Symbol),
		"side":       string(req.Side),
		"type":       string(req.Type),
		"timestamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
		"recvWindow": "5000",
	}

	if req.Type == exchange.OrderTypeMarket {
		// Use quoteOrderQty to target notional size.
		params["quoteOrderQty"] = formatFloat(req.Notional)
	}

	query := canonicalQuery(params)
	signature := e.sign(query)
	body := query + "&signature=" + signature

	endpoint := e.baseURL + "/api/v3/order"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return exchange.OrderAck{}, err
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("X-MEXC-APIKEY", e.apiKey)

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return exchange.OrderAck{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr mexcError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return exchange.OrderAck{}, fmt.Errorf("order rejected status %d", resp.StatusCode)
		}
		return exchange.OrderAck{}, fmt.Errorf("order rejected: %s (%d)", apiErr.Msg, apiErr.Code)
	}

	var payload orderResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return exchange.OrderAck{}, fmt.Errorf("decode order response: %w", err)
	}
	if payload.Code != 0 && payload.Code != 200 {
		return exchange.OrderAck{}, fmt.Errorf("order error: %s (%d)", payload.Msg, payload.Code)
	}

	ack := exchange.OrderAck{
		OrderID:     payload.OrderID,
		SubmittedAt: time.UnixMilli(payload.TransactTime),
	}

	return ack, nil
}

func (e *Executor) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(e.apiSecret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func canonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := url.Values{}
	for _, k := range keys {
		values.Set(k, params[k])
	}
	return values.Encode()
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

type orderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       string `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	TransactTime  int64  `json:"transactTime"`
	Code          int    `json:"code"`
	Msg           string `json:"msg"`
}

type mexcError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
