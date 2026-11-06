package fonts

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
)

const cacheTTL = 10 * time.Minute

type Family struct {
	Family string `json:"family"`

	Monospace bool `json:"monospace"`

	HasNerdIcons bool `json:"hasNerdIcons"`

	LocalizedNames map[string]string `json:"localizedNames,omitempty"`
}

type Service struct {
	mu       sync.Mutex
	now      func() time.Time
	scan     func(context.Context) ([]Family, error)
	cached   []Family
	cachedAt time.Time
	started  bool
}

func NewService() *Service {
	return &Service{
		now:  time.Now,
		scan: scanLocalFonts,
	}
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.now == nil || s.scan == nil {
		return ErrServiceNotReady
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errno.New(errno.CodeInvalidArg, "fonts.already_started", "")
	}
	s.started = true
	return nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = false
	s.cached = nil
	s.cachedAt = time.Time{}
	return nil
}

func (s *Service) ListTerminalFonts(ctx context.Context, refresh bool) ([]Family, error) {
	if s == nil {
		return nil, ErrServiceNotReady
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil, ErrServiceNotReady
	}
	if !refresh && s.cached != nil && s.now().Sub(s.cachedAt) < cacheTTL {
		cached := append([]Family(nil), s.cached...)
		s.mu.Unlock()
		return cached, nil
	}
	scan := s.scan
	s.mu.Unlock()

	families, err := scan(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.started {
		s.cached = append([]Family(nil), families...)
		s.cachedAt = s.now()
	}
	s.mu.Unlock()
	return families, nil
}

var ErrServiceNotReady = errno.New(errno.CodeUnknown, "fonts.not_ready", "")

func sortFamilies(families []Family) {
	rank := func(family Family) int {
		switch {
		case family.Monospace && family.HasNerdIcons:
			return 0
		case family.Monospace:
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(families, func(i, j int) bool {
		if rank(families[i]) != rank(families[j]) {
			return rank(families[i]) < rank(families[j])
		}
		return strings.ToLower(families[i].Family) < strings.ToLower(families[j].Family)
	})
}

func scanError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errno.Wrap(errno.CodeUnknown, "fonts.scan_failed", err)
}
