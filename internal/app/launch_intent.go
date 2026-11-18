package app

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	launchIntentEvent = "qterm:launch-intent"

	maxPendingLaunchIntents = 32

	launchIntentDedupWindow = 10 * time.Second

	maxLaunchIntentLength = 2048
)

var supportedLaunchSchemes = []string{"qterm://", "ssh://", "telnet://"}

type launchIntentState struct {
	mu      sync.Mutex
	pending []model.LaunchIntent

	seen map[string]time.Time

	ready bool
}

func parseLaunchIntentArgs(args []string) []model.LaunchIntent {
	intents := make([]model.LaunchIntent, 0, 2)
	for index := 1; index < len(args); index++ {
		arg := strings.TrimSpace(args[index])
		if arg == "" {
			continue
		}
		if intent, ok := launchIntentFromURL(arg); ok {
			intents = append(intents, intent)
			continue
		}
		if arg != "open" {
			continue
		}

		for next := index + 1; next < len(args); next++ {
			target := strings.TrimSpace(args[next])
			if target == "" || strings.HasPrefix(target, "-") {
				continue
			}
			if strings.Contains(target, "://") {

				if intent, ok := launchIntentFromURL(target); ok {
					intents = append(intents, intent)
				}
			} else if len(target) <= maxLaunchIntentLength {
				intents = append(intents, model.LaunchIntent{Kind: model.LaunchIntentOpen, Target: target})
			}
			index = next
			break
		}
	}
	return intents
}

func launchIntentFromURL(candidate string) (model.LaunchIntent, bool) {
	if len(candidate) > maxLaunchIntentLength {
		return model.LaunchIntent{}, false
	}
	lowered := strings.ToLower(candidate)
	for _, scheme := range supportedLaunchSchemes {
		if strings.HasPrefix(lowered, scheme) {
			return model.LaunchIntent{Kind: model.LaunchIntentURL, Target: candidate}, true
		}
	}
	return model.LaunchIntent{}, false
}

func (s *Service) acceptLaunchIntents(intents []model.LaunchIntent) bool {
	if s == nil || len(intents) == 0 {
		return false
	}
	added := s.launchIntents.enqueue(intents)
	if !added {
		return false
	}
	if app := application.Get(); app != nil {
		if payload := s.launchIntents.deliver(); len(payload) > 0 {
			app.Event.Emit(launchIntentEvent, payload)
		}
	}
	return true
}

func (state *launchIntentState) enqueue(intents []model.LaunchIntent) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	now := time.Now()
	if state.seen == nil {
		state.seen = make(map[string]time.Time, len(intents))
	}

	for key, stamped := range state.seen {
		if now.Sub(stamped) > launchIntentDedupWindow {
			delete(state.seen, key)
		}
	}
	added := false
	for _, intent := range intents {
		if intent.Target == "" || len(intent.Target) > maxLaunchIntentLength {
			continue
		}
		key := intent.Kind + "|" + intent.Target
		if _, duplicated := state.seen[key]; duplicated {
			continue
		}
		state.seen[key] = now
		state.pending = append(state.pending, intent)
		added = true
	}
	if overflow := len(state.pending) - maxPendingLaunchIntents; overflow > 0 {
		state.pending = append([]model.LaunchIntent(nil), state.pending[overflow:]...)
		slog.Warn("dropped oldest launch intents", "count", overflow)
	}
	return added
}

func (state *launchIntentState) take() []model.LaunchIntent {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.ready = true
	if len(state.pending) == 0 {
		return nil
	}
	taken := state.pending
	state.pending = nil
	return taken
}

func (s *Service) TakeLaunchIntents() []model.LaunchIntent {
	if s == nil {
		return nil
	}
	return s.launchIntents.take()
}

func (state *launchIntentState) deliver() []model.LaunchIntent {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.pending) == 0 {
		return nil
	}
	if !state.ready {
		return append([]model.LaunchIntent(nil), state.pending...)
	}
	taken := state.pending
	state.pending = nil
	return taken
}

func RegisterLaunchIntents(service *Service, app *application.App) {
	if service == nil || app == nil {
		return
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
		if event == nil {
			return
		}
		url := strings.TrimSpace(event.Context().URL())
		intent, ok := launchIntentFromURL(url)
		if !ok {
			return
		}

		service.focusWorkspaceWindow()
		service.acceptLaunchIntents([]model.LaunchIntent{intent})
	})

	service.acceptLaunchIntents(parseLaunchIntentArgs(os.Args))
}
