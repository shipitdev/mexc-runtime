# MEXC Signal Bot (Golang Skeleton)

Low-latency trading bot that ingests templated Telegram pump signals from a user-authenticated session, extracts the target pair from the embedded MEXC link, runs risk checks, and dispatches exchange orders (dry-run or live). The repository ships with full config management, message parsing, MTProto ingestion, risk limits, and a MEXC REST executor.

## Features

- **MTProto ingestion** – connects as a Telegram user via gotd/td, filters the configured channels, and streams matching messages into the engine.
- **Template-aware parser** – validates the pump-signal format, derives the symbol from the exchange link, and normalises it for MEXC.
- **Risk gate** – enforces cooldowns and daily trade limits before handing an order to the exchange layer.
- **Execution** – supports dry-run logging or live MEXC spot market orders with HMAC signing and quote-notional sizing.
- **Configurable everything** – TOML-based configuration controls trading mode, sizing, risk, telemetry, and infrastructure options.

## Layout

- `cmd/bot`: application entrypoint (`main.go`) – loads config, initialises parser/risk/executor, and wires the Telegram listener to the engine.
- `internal/config`: TOML configuration loader with validation and secret helpers.
- `internal/signal`: strict template parser that derives the pair symbol from `https://www.mexc.com/exchange/<PAIR>` links.
- `internal/engine`: orchestrates parse → risk → execution.
- `internal/exchange`: order executor abstractions, including MEXC REST implementation and dry-run fallback.
- `internal/risk`: cooldown-aware risk manager with daily trade limits.
- `internal/telegram`: MTProto listener built on gotd/td that consumes messages from a user-authenticated session.
- `config/example.toml`: reference configuration file.

## Getting Started

1. Copy `config/example.toml` to `config/local.toml` and adjust values (slippage, PnL rules, dry run flag, etc.).
2. Export secrets or create files referenced by `env:` / `file:` entries (e.g. `MEXC_KEY`, `MEXC_SECRET`, Telegram session).
3. Run tests:

   ```bash
   go test ./...
   ```

4. Create or import a Telegram user session:
   - If you already use Telethon/TDesktop, convert the session to the path configured in `telegram.session_storage_path`.
   - Alternatively, use a gotd helper (e.g. `gotdlogin`) to authenticate once and persist the session file.

5. Launch the bot in dry-run mode:

   ```bash
   go run ./cmd/bot -config config/local.toml
   ```

When `telegram.enabled = true` the MTProto client loads the user session file, connects to Telegram, and streams authorised channel posts into the engine. The session must remain valid (2FA/password handled externally). For live trading, disable `debug.dry_run`, provide MEXC API credentials, and ensure the configured user account has access to the target channel.

### Live Trading Checklist

1. Set `mode.environment = "live"` and confirm `mode.market_type` matches the instruments you intend to trade.
2. Populate `auth.api_key` / `auth.api_secret` with production credentials (use `env:` or `file:` indirections).
3. Flip `debug.dry_run = false`.
4. Tail logs to verify latency and fills (`log_level = "info"` or stricter once stable).

## Next Steps

- Extend MEXC executor with futures support and WebSocket order/position listeners.
- Add persistent storage for executions, PnL tracking, and advanced risk controls (drawdown, exposure).
- Wire Prometheus metrics and structured logging sinks as per config.
- Add replay recorder/runner to support backtesting and regression testing against archived signals.
