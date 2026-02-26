package logger

import "log/slog"

// Config holds top-level logger configuration.
type Config struct {
	Level        slog.Level
	AddSource    bool
	JSONFormat   bool
	OutputPaths  []string
	FileConfig   *FileConfig
	Sampling     *SamplingConfig
}

// FileConfig configures file-based log rotation.
type FileConfig struct {
	Path       string
	MaxSize    int  // MB
	MaxBackups int
	MaxAge     int  // days
	Compress   bool
}

// SamplingConfig controls log sampling rules.
type SamplingConfig struct {
	Initial    int // First N events per second
	Thereafter int // Then log every Nth event
}
