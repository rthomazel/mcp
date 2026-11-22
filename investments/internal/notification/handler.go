package notification

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	handlerNamePrefix    = "notification"
	noChannelHandlerName = handlerNamePrefix + ".none"
)

// Source is the one notification handler, expanded for the current
// configuration. It implements domain.EventHandlerSource.
type Source struct {
	loader        *Loader
	store         domain.Store
	renderers     map[string]map[string]Renderer
	telegramToken string
}

// NewSource builds a Source with the renderers from NewRenderers.
func NewSource(loader *Loader, store domain.Store, telegramToken string) *Source {
	return &Source{loader: loader, store: store, renderers: NewRenderers(store), telegramToken: telegramToken}
}

type channelHandler struct {
	name     string
	renderer Renderer
	channel  Channel
	category string
}

type noChannelHandler struct {
	eventType string
}

// Handlers returns the handlers of each notifying event type. A type without
// channels is handled with a warning, never an error.
func (s *Source) Handlers(_ context.Context) (domain.EventHandlers, error) {
	cfg, err := s.loader.Current()
	if err != nil {
		return nil, err
	}

	channels := make(map[string]Channel, len(cfg.Channels))
	for name, channel := range cfg.Channels {
		channels[name] = newChannel(name, channel, s.telegramToken)
	}

	return func(eventType string) []domain.EventHandler {
		if !domain.IsNotification(eventType) {
			return nil
		}

		notification, ok := cfg.Notifications[eventType]
		if !ok {
			return []domain.EventHandler{&noChannelHandler{eventType: eventType}}
		}

		handlers := make([]domain.EventHandler, 0, len(notification.Channels))

		for _, channelName := range slices.Sorted(maps.Keys(notification.Channels)) {
			category, err := cfg.resolveCategory(eventType, channelName, notification.Channels[channelName])
			if err != nil {
				// Validate already rejected this; a handler that always fails keeps it loud.
				return []domain.EventHandler{failingHandler{err: err}}
			}

			handlers = append(handlers, &channelHandler{
				name:     channelHandlerName(channelName, category),
				renderer: s.renderers[eventType][notification.Format],
				channel:  channels[channelName],
				category: category,
			})
		}

		return handlers
	}, nil
}

func newChannel(name string, cfg ChannelConfig, telegramToken string) Channel {
	if name == implementationTelegram {
		return NewTelegramChannel(name, cfg, telegramToken)
	}

	return &StubChannel{name: name}
}

// channelHandlerName is stored on events, so it never changes meaning.
func channelHandlerName(channel, category string) string {
	return handlerNamePrefix + "." + channel + "." + category
}

func (h *channelHandler) Name() string { return h.name }

func (h *channelHandler) Handle(ctx context.Context, event domain.Event) error {
	text, err := h.renderer.Render(ctx, event)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", event.Type, err)
	}

	if err := h.channel.SendPlainText(ctx, h.category, text); err != nil {
		return fmt.Errorf("sending to %s: %w", h.name, err)
	}

	return nil
}

func (h *noChannelHandler) Name() string { return noChannelHandlerName }

func (h *noChannelHandler) Handle(ctx context.Context, _ domain.Event) error {
	slog.WarnContext(ctx, "notification not delivered: no channel is configured for the event type", "type", h.eventType)

	return nil
}

// failingHandler reports a configuration that passed validation yet cannot be resolved.
type failingHandler struct{ err error }

func (h failingHandler) Name() string { return handlerNamePrefix + ".invalid" }

func (h failingHandler) Handle(context.Context, domain.Event) error { return h.err }
