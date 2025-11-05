# MEXC Signal Bot (Golang Skeleton)

Low-latency trading bot scaffold that ingests templated Telegram pump signals, extracts the target pair from the embedded MEXC link, performs risk checks, and fires exchange orders. This repository currently includes config management, message parsing, and the execution pipeline skeleton wired for dry-run mode.

## Layout

- `cmd/bot`: application entrypoint (`main.go`) – loads config, initialises parser/risk/executor, and exposes a channel for future Telegram integration.
- `internal/config`: TOML configuration loader with validation and secret helpers.
- `internal/signal`: strict template parser that derives the pair symbol from `https://www.mexc.com/exchange/<PAIR>` links.
- `internal/engine`: orchestrates parse → risk → execution.
- `internal/exchange`: order executor abstractions and dry-run implementation.
- `internal/risk`: baseline risk manager placeholder ready for stateful controls.
- `config/example.toml`: reference configuration file.

## Getting Started

1. Copy `config/example.toml` to `config/local.toml` and adjust values (slippage, PnL rules, dry run flag, etc.).
2. Export secrets or create files referenced by `env:` / `file:` entries (e.g. `MEXC_KEY`, `MEXC_SECRET`, Telegram session).
3. Run tests:

   ```bash
   go test ./...
   ```

4. Launch the bot in dry-run mode:

   ```bash
   go run ./cmd/bot -config config/local.toml
   ```

The main loop exposes a `msgCh` channel where the forthcoming Telegram listener will push `signal.Message` structs. The engine already enforces the message template and symbol resolution.

## Next Steps

- Integrate Telegram TDLib/GramJS client to feed verified template messages into `msgCh`.
- Implement real MEXC REST/WebSocket executor with pre-authenticated connection pools.
- Extend `risk.SimpleManager` with stateful exposure limits, cooldowns, and PnL tracking.
- Wire Prometheus metrics and structured logging sinks as per config.
- Add replay recorder/runner to support backtesting and regression testing against archived signals.

