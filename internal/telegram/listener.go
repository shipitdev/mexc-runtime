package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/user/mexc-bot/internal/config"
	signalpkg "github.com/user/mexc-bot/internal/signal"
)

// Listener consumes Telegram updates via the Bot API long-polling mechanism.
type Listener struct {
	logger        *slog.Logger
	cfg           config.TelegramConfig
	token         string
	client        *http.Client
	allowedChats  map[int64]struct{}
	baseURLPrefix string
}

// NewListener prepares a long-polling listener.
func NewListener(cfg config.TelegramConfig, token string, logger *slog.Logger) (*Listener, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("telegram token empty")
	}

	timeout := cfg.PollTimeoutSec
	if timeout <= 0 {
		timeout = 10
	}
	client := &http.Client{
		Timeout: time.Duration(timeout+5) * time.Second,
	}

	allowed := make(map[int64]struct{}, len(cfg.AllowedChatIDs))
	for _, id := range cfg.AllowedChatIDs {
		allowed[id] = struct{}{}
	}

	return &Listener{
		logger:        logger,
		cfg:           cfg,
		token:         token,
		client:        client,
		allowedChats:  allowed,
		baseURLPrefix: fmt.Sprintf("https://api.telegram.org/bot%s", token),
	}, nil
}

// Run begins consuming updates until context cancellation.
func (l *Listener) Run(ctx context.Context, out chan<- signalpkg.Message) error {
	var offset int64
	longPollTimeout := l.cfg.PollTimeoutSec
	if longPollTimeout <= 0 {
		longPollTimeout = 10
	}
	maxBatch := l.cfg.MaxUpdateBatch
	if maxBatch <= 0 {
		maxBatch = 100
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := l.fetchUpdates(ctx, offset, longPollTimeout, maxBatch)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			l.logger.Error("telegram fetch failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			msg := upd.ExtractMessage()
			if msg == nil {
				continue
			}
			if len(l.allowedChats) > 0 {
				if _, ok := l.allowedChats[msg.Chat.ID]; !ok {
					continue
				}
			}
			if msg.Text == "" {
				continue
			}

			out <- signalpkg.Message{
				ID:        msg.ID,
				Text:      msg.Text,
				Timestamp: msg.Timestamp,
			}
		}
	}
}

func (l *Listener) fetchUpdates(ctx context.Context, offset int64, timeoutSec, limit int) ([]update, error) {
	values := url.Values{}
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	values.Set("timeout", strconv.Itoa(timeoutSec))
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	values.Set("allowed_updates", `["message","channel_post"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURLPrefix+"/getUpdates?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram getUpdates status %d", resp.StatusCode)
	}

	var parsed updatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode telegram response: %w", err)
	}
	if !parsed.Ok {
		return nil, fmt.Errorf("telegram getUpdates not ok: %s", parsed.Description)
	}

	return parsed.Result, nil
}

// Supporting types extracted from Telegram responses.

type updatesResponse struct {
	Ok          bool     `json:"ok"`
	Result      []update `json:"result"`
	Description string   `json:"description"`
}

type update struct {
	UpdateID    int64      `json:"update_id"`
	Message     *tgMessage `json:"message"`
	ChannelPost *tgMessage `json:"channel_post"`
}

func (u update) ExtractMessage() *normalizedMessage {
	var src *tgMessage
	switch {
	case u.ChannelPost != nil:
		src = u.ChannelPost
	case u.Message != nil:
		src = u.Message
	default:
		return nil
	}

	if src.Chat == nil {
		return nil
	}

	return &normalizedMessage{
		ID:        src.MessageID,
		Text:      src.Text,
		Timestamp: time.Unix(int64(src.Date), 0),
		Chat:      src.Chat,
	}
}

type tgMessage struct {
	MessageID int64   `json:"message_id"`
	Date      int     `json:"date"`
	Text      string  `json:"text"`
	Chat      *tgChat `json:"chat"`
}

type tgChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type normalizedMessage struct {
	ID        int64
	Text      string
	Timestamp time.Time
	Chat      *tgChat
}
