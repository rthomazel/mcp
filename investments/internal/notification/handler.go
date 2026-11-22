package notification

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	handlerNamePrefix   = "notification"
	noTargetHandlerName = handlerNamePrefix + ":none"
)

// Source is the one notification handler, expanded for the current
// configuration. It implements outbox.HandlerSource.
type Source struct {
	loader    *Loader
	store     domain.Store
	renderers map[string]map[string]Renderer
	lookupEnv func(string) string
}

// NewSource builds a Source with the renderers from NewRenderers.
func NewSource(loader *Loader, store domain.Store, lookupEnv func(string) string) *Source {
	return &Source{loader: loader, store: store, renderers: NewRenderers(store), lookupEnv: lookupEnv}
}

type targetHandler struct {
	name     string
	renderer Renderer
	channel  Channel
	category string
}

type noTargetHandler struct {
	eventType string
}

// Handlers returns the handlers of each notifying event type. A type without
// targets is handled with a warning, never an error.
func (s *Source) Handlers(_ context.Context) (map[string][]domain.EventHandler, error) {
	cfg, err := s.loader.Current()
	if err != nil {
		return nil, err
	}

	channels := make(map[string]Channel, len(cfg.Channels))
	for _, channel := range cfg.Channels {
		channels[channel.Name] = newChannel(channel, s.lookupEnv)
	}

	routes := make(map[string]Route, len(cfg.Routes))
	for _, route := range cfg.Routes {
		routes[route.EventType] = route
	}

	handlers := make(map[string][]domain.EventHandler, len(domain.NotifyingEvents))

	for _, eventType := range domain.NotifyingEvents {
		route := routes[eventType]
		if len(route.Targets) == 0 {
			handlers[eventType] = []domain.EventHandler{&noTargetHandler{eventType: eventType}}

			continue
		}

		format := route.Format
		if format == "" {
			format = defaultFormat
		}

		for _, target := range route.Targets {
			handlers[eventType] = append(handlers[eventType], &targetHandler{
				name:     targetHandlerName(target.Channel, target.Category),
				renderer: s.renderers[eventType][format],
				channel:  channels[target.Channel],
				category: target.Category,
			})
		}
	}

	return handlers, nil
}

func newChannel(cfg ChannelConfig, lookupEnv func(string) string) Channel {
	if cfg.Implementation == implementationTelegram {
		return NewTelegramChannel(cfg, lookupEnv)
	}

	return &StubChannel{name: cfg.Name}
}

// targetHandlerName is stored on events, so it never changes meaning.
func targetHandlerName(channel, category string) string {
	return handlerNamePrefix + ":" + channel + ":" + category
}

func (h *targetHandler) Name() string { return h.name }

func (h *targetHandler) Handle(ctx context.Context, event domain.Event) error {
	text, err := h.renderer.Render(ctx, event)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", event.Type, err)
	}

	if err := h.channel.SendPlainText(ctx, h.category, text); err != nil {
		return fmt.Errorf("sending to %s: %w", h.name, err)
	}

	return nil
}

func (h *noTargetHandler) Name() string { return noTargetHandlerName }

func (h *noTargetHandler) Handle(ctx context.Context, _ domain.Event) error {
	slog.WarnContext(ctx, "notification not delivered: no target is configured for the event type", "type", h.eventType)

	return nil
}
