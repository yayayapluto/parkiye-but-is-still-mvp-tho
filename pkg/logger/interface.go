package logger

import "context"

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
