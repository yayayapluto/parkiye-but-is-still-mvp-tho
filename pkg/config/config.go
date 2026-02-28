package config

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"parkieee/pkg/logger"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the top-level application configuration.
type Config struct {
	App           AppConfig
	Database      DatabaseConfig
	JWT           JWTConfig
	Server        ServerConfig
	Observability ObservabilityConfig // ← ADD THIS
	Logger        LoggerConfig
}

type AppConfig struct {
	Name  string
	Env   string // "development" | "staging" | "production"
	Debug bool
}

type DatabaseConfig struct {
	Host                   string
	Port                   int
	User                   string
	Password               string
	Name                   string
	SSLMode                string
	LogLevel               string // "silent"|"error"|"warn"|"info"
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// ADD THIS NEW STRUCT
type ObservabilityConfig struct {
	EnableTracing   bool
	GrafanaPassword string
	PrometheusPort  int
	GrafanaPort     int
	LokiPort        int
	TempoPort       int
	LogLevel        string
}

type LoggerConfig struct {
	Level       string // "debug"|"info"|"warn"|"error"
	AddSource   bool
	JSONFormat  bool
	OutputPaths []string // e.g. ["stdout", "/var/log/app.log"]

	// File rotation (opsional, hanya aktif kalau OutputPaths ada file path)
	FileMaxSize    int // MB, default 100
	FileMaxBackups int // default 3
	FileMaxAge     int // days, default 28
	FileCompress   bool
}

// LoadEnv loads .env file from the given paths (first found wins).
// Call this once at the very start of main() before calling Load().
// Safe to call in production — if no .env file is found, it silently skips.
func LoadEnv(files ...string) {
	if len(files) == 0 {
		files = []string{".env"}
	}
	for _, f := range files {
		if err := godotenv.Load(f); err == nil {
			log.Printf("config: loaded env from %s", f)
			return
		}
	}
	// No .env file found — rely on OS environment (normal for production containers)
	log.Println("config: no .env file found, using OS environment variables")
}

// Load reads configuration from environment variables.
// Call LoadEnv() before Load() to populate variables from a .env file.
func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:  getEnv("APP_NAME", "parkieee"),
			Env:   getEnv("APP_ENV", "development"),
			Debug: getEnvBool("APP_DEBUG", true),
		},
		Database: DatabaseConfig{
			Host:                   getEnv("DB_HOST", "localhost"),
			Port:                   getEnvInt("DB_PORT", 5432),
			User:                   getEnv("DB_USER", "postgres"),
			Password:               getEnv("DB_PASSWORD", "postgres"),
			Name:                   getEnv("DB_NAME", "parkieee"),
			SSLMode:                getEnv("DB_SSLMODE", "disable"),
			LogLevel:               getEnv("DB_LOG_LEVEL", "warn"),
			MaxOpenConns:           getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:           getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetimeSeconds: getEnvInt("DB_CONN_MAX_LIFETIME_SECONDS", 300),
			ConnMaxIdleTimeSeconds: getEnvInt("DB_CONN_MAX_IDLE_TIME_SECONDS", 60),
		},
		JWT: JWTConfig{
			SecretKey:       getEnvRequired("JWT_SECRET_KEY"),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour),
		},
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		// ADD THIS SECTION
		Observability: ObservabilityConfig{
			EnableTracing:   getEnvBool("ENABLE_TRACING", false),
			GrafanaPassword: getEnv("GRAFANA_PASSWORD", "admin"),
			PrometheusPort:  getEnvInt("PROMETHEUS_PORT", 9090),
			GrafanaPort:     getEnvInt("GRAFANA_PORT", 3000),
			LokiPort:        getEnvInt("LOKI_PORT", 3100),
			TempoPort:       getEnvInt("TEMPO_PORT", 4318), // ← tambah ini (OTLP HTTP)
			LogLevel:        getEnv("OBSERVABILITY_LOG_LEVEL", "info"),
		},
		Logger: LoggerConfig{
			Level:          getEnv("LOG_LEVEL", "info"),
			AddSource:      getEnvBool("LOG_ADD_SOURCE", false),
			JSONFormat:     getEnvBool("LOG_JSON_FORMAT", false),
			OutputPaths:    getEnvStringSlice("LOG_OUTPUT_PATHS", []string{"stdout"}),
			FileMaxSize:    getEnvInt("LOG_FILE_MAX_SIZE", 100),
			FileMaxBackups: getEnvInt("LOG_FILE_MAX_BACKUPS", 3),
			FileMaxAge:     getEnvInt("LOG_FILE_MAX_AGE", 28),
			FileCompress:   getEnvBool("LOG_FILE_COMPRESS", true),
		},
	}

	return cfg, nil
}

// IsDevelopment returns true when APP_ENV=development.
func (c *Config) IsDevelopment() bool { return c.App.Env == "development" }

// IsProduction returns true when APP_ENV=production.
func (c *Config) IsProduction() bool { return c.App.Env == "production" }

// ADD THESE HELPER METHODS
func (c *Config) IsTracingEnabled() bool { return c.Observability.EnableTracing }

func (c *Config) GetObservabilityURLs() map[string]string {
	return map[string]string{
		"prometheus": fmt.Sprintf("http://localhost:%d", c.Observability.PrometheusPort),
		"grafana":    fmt.Sprintf("http://localhost:%d", c.Observability.GrafanaPort),
		"loki":       fmt.Sprintf("http://localhost:%d", c.Observability.LokiPort),
	}
}

func (c *Config) ToLoggerConfig() *logger.Config {
	level := slog.LevelInfo
	switch c.Logger.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var fileConfig *logger.FileConfig
	for _, path := range c.Logger.OutputPaths {
		if path != "stdout" && path != "stderr" {
			fileConfig = &logger.FileConfig{
				Path:       path,
				MaxSize:    c.Logger.FileMaxSize,
				MaxBackups: c.Logger.FileMaxBackups,
				MaxAge:     c.Logger.FileMaxAge,
				Compress:   c.Logger.FileCompress,
			}
			break
		}
	}

	return &logger.Config{
		Level:       level,
		AddSource:   c.Logger.AddSource,
		JSONFormat:  c.Logger.JSONFormat,
		OutputPaths: c.Logger.OutputPaths,
		FileConfig:  fileConfig,
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getEnvStringSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
