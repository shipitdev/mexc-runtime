package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config captures runtime configuration for the trading bot.
type Config struct {
	Mode       ModeConfig             `toml:"mode"`
	Auth       AuthConfig             `toml:"auth"`
	Trading    TradingConfig          `toml:"trading"`
	Parser     ParserConfig           `toml:"parser"`
	Telegram   TelegramConfig         `toml:"telegram"`
	Risk       RiskConfig             `toml:"risk"`
	PnLExit    PnLExitConfig          `toml:"pnl_exit"`
	Latency    LatencyBudget          `toml:"latency"`
	Telemetry  TelemetryConfig        `toml:"telemetry"`
	Infra      InfraConfig            `toml:"infra"`
	Debug      DebugConfig            `toml:"debug"`
	Overrides  map[string]AssetConfig `toml:"target_overrides"`
	ConfigPath string                 `toml:"-"`
}

type ModeConfig struct {
	Exchange    string `toml:"exchange"`
	Environment string `toml:"environment"`
	MarketType  string `toml:"market_type"`
}

type AuthConfig struct {
	APIKey          SecretRef `toml:"api_key"`
	APISecret       SecretRef `toml:"api_secret"`
	TelegramSession SecretRef `toml:"telegram_session"`
}

type TradingConfig struct {
	DefaultBaseNotional float64 `toml:"default_base_notional"`
	MaxNotional         float64 `toml:"max_notional"`
	MaxOpenPositions    int     `toml:"max_open_positions"`
	OrderType           string  `toml:"order_type"`
	SlippageBps         int     `toml:"slippage_bps"`
}

type ParserConfig struct {
	RequiredTokens []string `toml:"required_tokens"`
	LinkHost       string   `toml:"link_host"`
	LinkPathPrefix string   `toml:"link_path_prefix"`
	PairSeparator  string   `toml:"pair_separator"`
}

type TelegramConfig struct {
	Enabled        bool      `toml:"enabled"`
	AllowedChatIDs []int64   `toml:"allowed_chat_ids"`
	PollTimeoutSec int       `toml:"poll_timeout_seconds"`
	MaxUpdateBatch int       `toml:"max_update_batch"`
	BotToken       SecretRef `toml:"bot_token"`
}

type RiskConfig struct {
	TakeProfitPct    float64 `toml:"take_profit_pct"`
	StopLossPct      float64 `toml:"stop_loss_pct"`
	BreakevenArmPct  float64 `toml:"breakeven_arm_pct"`
	CooldownSeconds  int     `toml:"cooldown_seconds"`
	MaxDailyLoss     float64 `toml:"max_daily_loss"`
	MaxDailyTrades   int     `toml:"max_daily_trades"`
	MaxPositionHours int     `toml:"max_position_hours"`
}

type PnLExitConfig struct {
	TrailingEnable bool    `toml:"trailing_enable"`
	TrailStartPct  float64 `toml:"trail_start_pct"`
	TrailStepPct   float64 `toml:"trail_step_pct"`
}

type LatencyBudget struct {
	ParseBudgetMS      int `toml:"parse_budget_ms"`
	OrderBudgetMS      int `toml:"order_budget_ms"`
	WebsocketTimeoutMS int `toml:"websocket_timeout_ms"`
}

type TelemetryConfig struct {
	LogLevel        string `toml:"log_level"`
	TraceTimings    bool   `toml:"trace_timings"`
	MetricsPush     bool   `toml:"metrics_push"`
	MetricsEndpoint string `toml:"metrics_endpoint"`
	ProfilingEnable bool   `toml:"profiling_enable"`
}

type InfraConfig struct {
	RedisURL string `toml:"redis_url"`
	PgDSN    string `toml:"pg_dsn"`
}

type DebugConfig struct {
	DryRun       bool   `toml:"dry_run"`
	RecordReplay bool   `toml:"record_replay"`
	ReplayDir    string `toml:"replay_dir"`
}

type AssetConfig struct {
	MaxNotional         float64 `toml:"max_notional"`
	DefaultBaseNotional float64 `toml:"default_base_notional"`
}

// SecretRef encodes how to resolve secret material (env:, file:, literal).
type SecretRef struct {
	Value string
}

func (r *SecretRef) UnmarshalText(text []byte) error {
	r.Value = strings.TrimSpace(string(text))
	return nil
}

// Resolve resolves the secret reference into a usable byte slice.
func (r SecretRef) Resolve() ([]byte, error) {
	switch {
	case strings.HasPrefix(r.Value, "env:"):
		key := strings.TrimPrefix(r.Value, "env:")
		if key == "" {
			return nil, errors.New("empty env key")
		}
		val, ok := os.LookupEnv(key)
		if !ok {
			return nil, fmt.Errorf("env var %s not set", key)
		}
		return []byte(val), nil
	case strings.HasPrefix(r.Value, "file:"):
		path := strings.TrimPrefix(r.Value, "file:")
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("read secret file %s: %w", path, err)
		}
		return data, nil
	default:
		return []byte(r.Value), nil
	}
}

// Load reads and validates configuration from a TOML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err = toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ConfigPath = path

	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate ensures core configuration is sane before boot.
func (c *Config) Validate() error {
	if c.Mode.Exchange != "mexc" {
		return fmt.Errorf("unsupported exchange %q", c.Mode.Exchange)
	}
	switch c.Mode.Environment {
	case "testnet", "live":
	default:
		return fmt.Errorf("environment must be testnet or live, got %q", c.Mode.Environment)
	}
	switch c.Mode.MarketType {
	case "spot", "futures":
	default:
		return fmt.Errorf("market_type must be spot or futures, got %q", c.Mode.MarketType)
	}

	if c.Trading.DefaultBaseNotional <= 0 {
		return errors.New("default_base_notional must be positive")
	}
	if c.Trading.MaxNotional < c.Trading.DefaultBaseNotional {
		return errors.New("max_notional must be >= default_base_notional")
	}
	if c.Trading.MaxOpenPositions <= 0 {
		return errors.New("max_open_positions must be > 0")
	}
	if c.Trading.SlippageBps < 0 {
		return errors.New("slippage_bps must be >= 0")
	}
	if c.Parser.LinkHost == "" || c.Parser.LinkPathPrefix == "" {
		return errors.New("parser link_host and link_path_prefix required")
	}
	if c.Parser.PairSeparator == "" {
		return errors.New("parser pair_separator required")
	}
	if c.Telegram.Enabled {
		if c.Telegram.PollTimeoutSec <= 0 {
			return errors.New("telegram poll_timeout_seconds must be > 0 when enabled")
		}
		if c.Telegram.MaxUpdateBatch < 0 {
			return errors.New("telegram max_update_batch must be >= 0")
		}
		if strings.TrimSpace(c.Telegram.BotToken.Value) == "" {
			return errors.New("telegram bot_token must be provided when enabled")
		}
	}
	if c.Risk.TakeProfitPct <= 0 || c.Risk.StopLossPct <= 0 {
		return errors.New("risk take_profit_pct and stop_loss_pct must be > 0")
	}
	if c.Risk.MaxDailyTrades <= 0 {
		return errors.New("risk max_daily_trades must be > 0")
	}
	if c.Risk.MaxPositionHours < 0 {
		return errors.New("risk max_position_hours must be >= 0")
	}
	if c.PnLExit.TrailStepPct < 0 {
		return errors.New("pnl_exit trail_step_pct must be >= 0")
	}
	return nil
}
