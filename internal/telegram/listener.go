package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/user/mexc-bot/internal/config"
	signalpkg "github.com/user/mexc-bot/internal/signal"
)

// Listener consumes Telegram updates via a user session authenticated through MTProto.
type Listener struct {
	logger       *slog.Logger
	cfg          config.TelegramConfig
	client       *telegram.Client
	storage      *session.FileStorage
	allowedChats map[int64]struct{}

	outMu sync.RWMutex
	out   chan<- signalpkg.Message
}

// NewListener prepares a MTProto-based listener using gotd/td.
func NewListener(cfg config.TelegramConfig, apiHash string, logger *slog.Logger) (*Listener, error) {
	if cfg.APIID <= 0 {
		return nil, errors.New("telegram api_id must be positive")
	}
	if strings.TrimSpace(apiHash) == "" {
		return nil, errors.New("telegram api_hash must be provided")
	}
	if strings.TrimSpace(cfg.SessionStoragePath) == "" {
		return nil, errors.New("telegram session_storage_path must be provided")
	}

	sessionPath := filepath.Clean(cfg.SessionStoragePath)
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		return nil, fmt.Errorf("prepare session directory: %w", err)
	}

	storage := &session.FileStorage{Path: sessionPath}
	allowed := make(map[int64]struct{}, len(cfg.AllowedChatIDs))
	for _, id := range cfg.AllowedChatIDs {
		allowed[id] = struct{}{}
	}

	device := telegram.DeviceConfig{
		DeviceModel:    fallback(cfg.DeviceModel, "mexc-bot"),
		SystemLangCode: fallback(cfg.SystemLanguage, "en"),
		LangCode:       fallback(cfg.SystemLanguage, "en"),
		AppVersion:     fallback(cfg.ApplicationVersion, "0.1.0"),
	}

	listener := &Listener{
		logger:       logger,
		cfg:          cfg,
		storage:      storage,
		allowedChats: allowed,
	}

	client := telegram.NewClient(cfg.APIID, strings.TrimSpace(apiHash), telegram.Options{
		SessionStorage: storage,
		UpdateHandler: telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
			return listener.handleUpdate(ctx, u)
		}),
		Device: device,
	})
	listener.client = client

	return listener, nil
}

// Run begins consuming updates until the provided context is cancelled.
// The session must already be authorised; use an external tool to create the session file.
func (l *Listener) Run(ctx context.Context, out chan<- signalpkg.Message) error {
	l.outMu.Lock()
	l.out = out
	l.outMu.Unlock()
	defer func() {
		l.outMu.Lock()
		l.out = nil
		l.outMu.Unlock()
	}()

	return l.client.Run(ctx, func(runCtx context.Context) error {
		status, err := l.client.Auth().Status(runCtx)
		if err != nil {
			return fmt.Errorf("telegram auth status: %w", err)
		}
		if !status.Authorized {
			return errors.New("telegram session is not authorised; import a user session first")
		}
		if status.User != nil {
			l.logger.Info("telegram session ready", "user_id", status.User.ID, "username", status.User.Username)
		} else {
			l.logger.Info("telegram session ready")
		}

		<-runCtx.Done()
		return runCtx.Err()
	})
}

func (l *Listener) handleUpdate(ctx context.Context, upd tg.UpdatesClass) error {
	switch u := upd.(type) {
	case *tg.Updates:
		for _, item := range u.Updates {
			l.handleUpdateClass(ctx, item)
		}
	case *tg.UpdatesCombined:
		for _, item := range u.Updates {
			l.handleUpdateClass(ctx, item)
		}
	case *tg.UpdateShort:
		l.handleUpdateClass(ctx, u.Update)
	case *tg.UpdateShortMessage, *tg.UpdateShortChatMessage, *tg.UpdateShortSentMessage:
		// Short updates omit Peer/Chat metadata we rely on; ignore them.
	default:
		// Unhandled update types are ignored.
	}
	return nil
}

func (l *Listener) handleUpdateClass(ctx context.Context, upd tg.UpdateClass) {
	switch u := upd.(type) {
	case *tg.UpdateNewMessage:
		l.consumeMessage(ctx, u.Message)
	case *tg.UpdateNewChannelMessage:
		l.consumeMessage(ctx, u.Message)
	case *tg.UpdateEditMessage:
		l.consumeMessage(ctx, u.Message)
	case *tg.UpdateEditChannelMessage:
		l.consumeMessage(ctx, u.Message)
	default:
	}
}

func (l *Listener) consumeMessage(ctx context.Context, msg tg.MessageClass) {
	m, ok := msg.(*tg.Message)
	if !ok {
		return
	}
	if strings.TrimSpace(m.Message) == "" {
		return
	}

	chatID, ok := peerID(m.PeerID)
	if !ok {
		return
	}
	if len(l.allowedChats) > 0 {
		if _, allowed := l.allowedChats[chatID]; !allowed {
			return
		}
	}

	signalMsg := signalpkg.Message{
		ID:        int64(m.ID),
		Text:      m.Message,
		Timestamp: time.Unix(int64(m.Date), 0),
	}

	l.outMu.RLock()
	out := l.out
	l.outMu.RUnlock()
	if out == nil {
		return
	}

	select {
	case <-ctx.Done():
	case out <- signalMsg:
	}
}

func peerID(peer tg.PeerClass) (int64, bool) {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		return int64(p.ChannelID), true
	case *tg.PeerChat:
		return int64(p.ChatID), true
	case *tg.PeerUser:
		return int64(p.UserID), true
	default:
		return 0, false
	}
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}
