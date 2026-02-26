package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type RequestData struct {
	URL     string            `json:"url"`
	Body    string            `json:"body"`
	Params  map[string]string `json:"params,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type ResponseData struct {
	Body       string `json:"body"`
	StatusCode int    `json:"status_code,omitempty"`
}

type Logger interface {
	Info(msg, ctxName string, data ...map[string]interface{})
	Debug(msg, ctxName string, data ...map[string]interface{})
	Error(msg, ctxName string, data ...map[string]interface{})
	With(data map[string]interface{}) Logger
	WithTraceID(traceID string) Logger
}

type SlogLogger struct {
	handler slog.Logger
}

var (
	instance *SlogLogger
	once     sync.Once
)

func GetInstance() Logger {
	once.Do(func() {
		// 1. Pastikan folder storage/logs ada
		logDir := filepath.Join("storage", "logs")
		_ = os.MkdirAll(logDir, 0755)
		logFilePath := filepath.Join(logDir, "app.log")

		// 2. Setup Lumberjack untuk Log Rotation
		fileLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    50,   // 50 Megabytes
			MaxBackups: 5,    // Simpan 5 file cadangan
			MaxAge:     30,   // Simpan selama 30 hari
			Compress:   true, // Kompres jadi .gz
		}

		// 3. Gabungkan Output: Terminal (Stdout) + File
		multiWriter := io.MultiWriter(os.Stdout, fileLogger)

		// 4. Inisialisasi JSON Handler
		h := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
			Level: slog.LevelDebug,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.LevelKey {
					level := a.Value.Any().(slog.Level)
					switch level {
					case slog.LevelInfo:
						return slog.Int("level", 30)
					case slog.LevelDebug:
						return slog.Int("level", 40)
					case slog.LevelError:
						return slog.Int("level", 50)
					}
				}
				return a
			},
		})
		instance = &SlogLogger{handler: *slog.New(h)}
	})
	return instance
}

func (l *SlogLogger) Info(msg, ctxName string, data ...map[string]interface{}) {
	l.log(slog.LevelInfo, msg, ctxName, data...)
}

func (l *SlogLogger) Debug(msg, ctxName string, data ...map[string]interface{}) {
	l.log(slog.LevelDebug, msg, ctxName, data...)
}

func (l *SlogLogger) Error(msg, ctxName string, data ...map[string]interface{}) {
	l.log(slog.LevelError, msg, ctxName, data...)
}

func (l *SlogLogger) With(data map[string]interface{}) Logger {
	args := make([]any, 0, len(data)*2)
	for k, v := range data {
		args = append(args, k, v)
	}
	return &SlogLogger{handler: *l.handler.With(args...)}
}

func (l *SlogLogger) WithTraceID(traceID string) Logger {
	return &SlogLogger{handler: *l.handler.With("trace_id", traceID)}
}

func (l *SlogLogger) log(level slog.Level, msg, ctxName string, data ...map[string]interface{}) {
	// Menambahkan context secara default
	args := []any{"context", ctxName}

	// Flatten map data ke dalam arguments slog
	for _, d := range data {
		for k, v := range d {
			args = append(args, k, v)
		}
	}

	l.handler.Log(context.Background(), level, msg, args...)
}

func GenerateTraceID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

// Global convenience functions
var defaultLogger = GetInstance()

func Info(msg, context string, data ...map[string]interface{}) {
	defaultLogger.Info(msg, context, data...)
}

func Error(msg, context string, data ...map[string]interface{}) {
	defaultLogger.Error(msg, context, data...)
}

func WithTraceID(traceID string) Logger {
	return defaultLogger.WithTraceID(traceID)
}
