package logger

import (
	"context"
	"log/slog"
	"os"
)

// Logger defines the minimal logging interface used by the application.
type Logger interface {
	Info(ctx context.Context, msg string, args ...any)
	Debug(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	Fatal(ctx context.Context, msg string, args ...any)
	With(args ...any) Logger
	WithGroup(name string) Logger
}

// SlogLogger is the concrete implementation backed by slog.Logger.
type SlogLogger struct {
	logger *slog.Logger
}

func (l *SlogLogger) Info(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l *SlogLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

func (l *SlogLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l *SlogLogger) Error(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

func (l *SlogLogger) Fatal(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
	os.Exit(1)
}

func (l *SlogLogger) With(args ...any) Logger {
	return &SlogLogger{logger: l.logger.With(args...)}
}

func (l *SlogLogger) WithGroup(name string) Logger {
	return &SlogLogger{logger: l.logger.WithGroup(name)}
}
