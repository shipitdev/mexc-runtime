package signal

import (
	"testing"
	"time"

	"github.com/user/mexc-bot/internal/config"
)

func baseConfig() config.ParserConfig {
	return config.ParserConfig{
		RequiredTokens: []string{"MEGA PUMP SIGNAL", "Targets"},
		LinkHost:       "www.mexc.com",
		LinkPathPrefix: "/exchange/",
		PairSeparator:  "_",
	}
}

func TestParserParseSuccess(t *testing.T) {
	parser := NewParser(baseConfig())
	msg := Message{
		ID: 1,
		Text: `BUYING #TWIF/USDT

MEGA PUMP SIGNAL: #TWIF

Low cap, bottomed out, rising volume, and it’s starting.. a big pump for #TWIF🚀

Link :- https://www.mexc.com/exchange/TWIF_USDT

Buy and hold for massive profits🚀 Targets: 2000%-5000%`,
		Timestamp: time.Unix(1710000000, 0),
	}

	signal, err := parser.Parse(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if signal.Symbol != "TWIFUSDT" {
		t.Fatalf("expected symbol TWIFUSDT, got %s", signal.Symbol)
	}
	if signal.PairCode != "TWIF_USDT" {
		t.Fatalf("expected pair code TWIF_USDT, got %s", signal.PairCode)
	}
}

func TestParserMissingTokens(t *testing.T) {
	parser := NewParser(baseConfig())
	msg := Message{
		ID:        2,
		Text:      "something else https://www.mexc.com/exchange/TWIF_USDT",
		Timestamp: time.Now(),
	}

	if _, err := parser.Parse(msg); err == nil {
		t.Fatalf("expected error when required tokens missing")
	}
}

func TestParserInvalidLink(t *testing.T) {
	parser := NewParser(baseConfig())
	msg := Message{
		ID:        3,
		Text:      "MEGA PUMP SIGNAL ... Targets ... https://www.other.com/exchange/TWIF_USDT",
		Timestamp: time.Now(),
	}

	if _, err := parser.Parse(msg); err == nil {
		t.Fatalf("expected error when link host is invalid")
	}
}

func TestParserMissingPair(t *testing.T) {
	parser := NewParser(baseConfig())
	msg := Message{
		ID:   4,
		Text: "MEGA PUMP SIGNAL ... Targets ... https://www.mexc.com/exchange/",
	}

	if _, err := parser.Parse(msg); err == nil {
		t.Fatalf("expected error when pair missing")
	}
}
