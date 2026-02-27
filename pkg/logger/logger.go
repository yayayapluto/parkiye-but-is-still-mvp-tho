package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// New constructs a Logger based on provided Config.
//
// Format rules:
//   - stdout / stderr → text format (human-readable in dev)
//   - file paths      → JSON format always (required for Promtail/Loki parsing)
func New(cfg *Config) (Logger, error) {
	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}

	var handlers []slog.Handler
	for _, path := range cfg.OutputPaths {
		handler, err := buildHandler(path, cfg)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, handler)
	}

	var finalHandler slog.Handler
	if len(handlers) == 1 {
		finalHandler = handlers[0]
	} else {
		finalHandler = teeHandler(handlers)
	}

	return &SlogLogger{logger: slog.New(finalHandler)}, nil
}

func buildHandler(path string, cfg *Config) (slog.Handler, error) {
	w, err := buildWriter(path, cfg)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	// stdout/stderr → text (readable), files → JSON (Promtail-parseable)
	var handler slog.Handler
	if isConsole(path) {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	if cfg.Sampling != nil {
		handler = newSamplingHandler(handler, cfg.Sampling)
	}

	return handler, nil
}

func buildWriter(path string, cfg *Config) (io.Writer, error) {
	switch path {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if cfg.FileConfig != nil {
			return &lumberjack.Logger{
				Filename:   path,
				MaxSize:    cfg.FileConfig.MaxSize,
				MaxBackups: cfg.FileConfig.MaxBackups,
				MaxAge:     cfg.FileConfig.MaxAge,
				Compress:   cfg.FileConfig.Compress,
			}, nil
		}
		return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}
}

func isConsole(path string) bool {
	return path == "stdout" || path == "stderr"
}
