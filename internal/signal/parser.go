package signal

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/user/mexc-bot/internal/config"
)

// Message represents the subset of Telegram data the parser cares about.
type Message struct {
	ID        int64
	Text      string
	Timestamp time.Time
}

// Signal is the normalized trading instruction extracted from a Telegram message.
type Signal struct {
	RawMessage Message
	Symbol     string // canonical exchange symbol, e.g. TWIFUSDT
	PairCode   string // raw pair component from the URL, e.g. TWIF_USDT
}

// Parser enforces the message template and extracts actionable data.
type Parser struct {
	cfg config.ParserConfig
}

func NewParser(cfg config.ParserConfig) *Parser {
	return &Parser{cfg: cfg}
}

var (
	errMissingLink      = errors.New("signal link missing")
	errUnsupportedLink  = errors.New("signal link unsupported")
	errMissingSymbol    = errors.New("unable to resolve symbol from link")
	errMissingToken     = errors.New("required template token missing")
	errInvalidSeparator = errors.New("invalid pair separator")
)

// Parse validates the message against the template and returns a trading signal.
func (p *Parser) Parse(msg Message) (*Signal, error) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil, fmt.Errorf("empty message body")
	}

	if err := p.requireTokens(text); err != nil {
		return nil, err
	}

	link, err := p.extractLink(text)
	if err != nil {
		return nil, err
	}

	pairCode, err := p.resolvePair(link)
	if err != nil {
		return nil, err
	}

	symbol := strings.ToUpper(strings.ReplaceAll(pairCode, p.cfg.PairSeparator, ""))
	if symbol == "" {
		return nil, errMissingSymbol
	}

	return &Signal{
		RawMessage: msg,
		Symbol:     symbol,
		PairCode:   strings.ToUpper(pairCode),
	}, nil
}

func (p *Parser) requireTokens(text string) error {
	for _, token := range p.cfg.RequiredTokens {
		if !strings.Contains(text, token) {
			return fmt.Errorf("%w: %s", errMissingToken, token)
		}
	}
	return nil
}

func (p *Parser) extractLink(text string) (*url.URL, error) {
	segments := strings.Fields(text)
	for _, segment := range segments {
		if !strings.Contains(segment, "://") {
			continue
		}

		u, err := url.Parse(strings.Trim(segment, " \t\n\r,.;!"))
		if err != nil {
			continue
		}
		if !strings.EqualFold(u.Host, p.cfg.LinkHost) {
			continue
		}
		if !strings.HasPrefix(u.Path, p.cfg.LinkPathPrefix) {
			continue
		}
		return u, nil
	}
	return nil, errMissingLink
}

func (p *Parser) resolvePair(link *url.URL) (string, error) {
	raw := strings.TrimPrefix(link.Path, p.cfg.LinkPathPrefix)
	if raw == "" {
		return "", errMissingSymbol
	}
	raw = strings.Trim(raw, "/")
	sep := p.cfg.PairSeparator
	if sep == "" {
		return "", errInvalidSeparator
	}

	upper := strings.ToUpper(raw)
	if !strings.Contains(upper, strings.ToUpper(sep)) {
		return "", fmt.Errorf("pair does not contain separator %q: %s", sep, upper)
	}
	// Some links include hyphen or encoded characters; normalize them to the separator.
	upper = strings.ReplaceAll(upper, "-", sep)
	upper = strings.ReplaceAll(upper, "%5F", sep) // URL encoded underscore

	return upper, nil
}
