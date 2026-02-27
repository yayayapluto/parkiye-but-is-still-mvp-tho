package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// New constructs a Logger based on provided Config.
func New(cfg *Config) (Logger, error) {
	var handlers []slog.Handler
	for _, path := range cfg.OutputPaths {
		var w io.Writer
		var err error

		switch path {
		case "stdout":
			w = os.Stdout
		case "stderr":
			w = os.Stderr
		default:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, err
			}
			if cfg.FileConfig != nil {
				w = &lumberjack.Logger{
					Filename:   path,
					MaxSize:    cfg.FileConfig.MaxSize,
					MaxBackups: cfg.FileConfig.MaxBackups,
					MaxAge:     cfg.FileConfig.MaxAge,
					Compress:   cfg.FileConfig.Compress,
				}
			} else {
				w, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				if err != nil {
					return nil, err
				}
			}
		}

		handlerOpts := &slog.HandlerOptions{
			Level:     cfg.Level,
			AddSource: cfg.AddSource,
		}
		var handler slog.Handler
		if cfg.JSONFormat {
			handler = slog.NewJSONHandler(w, handlerOpts)
		} else {
			handler = slog.NewTextHandler(w, handlerOpts)
		}
		if cfg.Sampling != nil {
			handler = newSamplingHandler(handler, cfg.Sampling)
		}
		handlers = append(handlers, handler)
	}

	var finalHandler slog.Handler
	if len(handlers) == 1 {
		finalHandler = handlers[0]
	} else {
		finalHandler = slog.Handler(teeHandler(handlers))
	}

	return &SlogLogger{logger: slog.New(finalHandler)}, nil
}
