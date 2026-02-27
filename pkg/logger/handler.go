package logger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// teeHandler duplicates records across multiple handlers.
type teeHandler []slog.Handler

func (t teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range t {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range t {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (t teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(t))
	for i, h := range t {
		handlers[i] = h.WithAttrs(attrs)
	}
	return teeHandler(handlers)
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(t))
	for i, h := range t {
		handlers[i] = h.WithGroup(name)
	}
	return teeHandler(handlers)
}

// samplingHandler implements a simple rate-based sampler.
type samplingHandler struct {
	inner  slog.Handler
	cfg    *SamplingConfig
	counts sync.Map
}

type samplingCounter struct {
	mu    sync.Mutex
	count int
	last  time.Time
}

func newSamplingHandler(inner slog.Handler, cfg *SamplingConfig) slog.Handler {
	return &samplingHandler{inner: inner, cfg: cfg}
}

func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *samplingHandler) Handle(ctx context.Context, r slog.Record) error {
	key := fmt.Sprintf("%s:%s", r.Level, r.Message)
	counterI, _ := h.counts.LoadOrStore(key, &samplingCounter{last: time.Now()})
	counter := counterI.(*samplingCounter)
	counter.mu.Lock()
	defer counter.mu.Unlock()
	if time.Since(counter.last) > time.Second {
		counter.count = 0
		counter.last = time.Now()
	}
	counter.count++
	shouldLog := counter.count <= h.cfg.Initial || (counter.count-h.cfg.Initial)%h.cfg.Thereafter == 0
	if !shouldLog {
		return nil
	}
	return h.inner.Handle(ctx, r)
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{inner: h.inner.WithAttrs(attrs), cfg: h.cfg}
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{inner: h.inner.WithGroup(name), cfg: h.cfg}
}
