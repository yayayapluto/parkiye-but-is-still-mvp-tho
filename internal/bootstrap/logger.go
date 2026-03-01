package bootstrap

import (
	"log/slog"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
)

func newLogger(cfg *config.Config) (logger.Logger, error) {
	level := slog.LevelInfo
	switch cfg.Logger.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	outputPaths := cfg.Logger.OutputPaths
	if len(outputPaths) == 0 {
		outputPaths = []string{"stdout"}
	}

	var fileConfig *logger.FileConfig
	for _, path := range outputPaths {
		if path != "stdout" && path != "stderr" {
			fileConfig = &logger.FileConfig{
				Path:       path,
				MaxSize:    cfg.Logger.FileMaxSize,
				MaxBackups: cfg.Logger.FileMaxBackups,
				MaxAge:     cfg.Logger.FileMaxAge,
				Compress:   cfg.Logger.FileCompress,
			}
			break
		}
	}

	return logger.New(&logger.Config{
		Level:       level,
		AddSource:   cfg.Logger.AddSource,
		JSONFormat:  cfg.Logger.JSONFormat,
		OutputPaths: outputPaths,
		FileConfig:  fileConfig,
	})
}
