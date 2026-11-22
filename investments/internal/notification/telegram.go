package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	telegramBaseURL = "https://api.telegram.org"
	telegramTimeout = 10 * time.Second
	telegramMaxText = 4096

	settingChatID = "chat_id"
)

// TelegramChannel sends through the Telegram Bot API, one chat per category.
type TelegramChannel struct {
	name    string
	token   string
	chats   map[string]string
	client  *http.Client
	baseURL string
}

// NewTelegramChannel builds the channel with the bot token from the server
// configuration; the notifications file never holds a credential.
func NewTelegramChannel(cfg ChannelConfig, token string) *TelegramChannel {
	chats := make(map[string]string, len(cfg.Categories))

	for _, category := range cfg.Categories {
		if chat := category.Settings[settingChatID]; chat != "" {
			chats[category.Name] = chat
		}
	}

	return &TelegramChannel{
		name: cfg.Name, token: token, chats: chats,
		client: &http.Client{Timeout: telegramTimeout}, baseURL: telegramBaseURL,
	}
}

// SendPlainText posts text to the category's chat. Without a token or chat it warns
// like the stub and succeeds.
func (c *TelegramChannel) SendPlainText(ctx context.Context, category, text string) error {
	chat := c.chats[category]
	if c.token == "" || chat == "" {
		warnUndelivered(ctx, c.name, category)

		return nil
	}

	body, err := json.Marshal(map[string]string{"chat_id": chat, "text": truncate(text, telegramMaxText)})
	if err != nil {
		return fmt.Errorf("encoding telegram message: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("building telegram request")
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		// The request URL carries the token: keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}

		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var failure struct {
		Description string `json:"description"`
	}

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	_ = json.Unmarshal(raw, &failure)

	return fmt.Errorf("telegram responded %d: %s", response.StatusCode, failure.Description)
}

// truncate cuts text to limit characters, ending with an ellipsis when cut.
func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit-1]) + "…"
}
