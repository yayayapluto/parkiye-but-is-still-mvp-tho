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

type Config struct {
	App       AppConfig
	Storage   StorageConfig
	S3        S3Config
	OCR       OCRConfig
	Midtrans  MidtransConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Server    ServerConfig
	Tracing   TracingConfig
	Profiling ProfilingConfig
	Logger    LoggerConfig
}

type MidtransConfig struct {
	ServerKey string // MIDTRANS_SERVER_KEY
	ClientKey string // MIDTRANS_CLIENT_KEY
	Env       string // MIDTRANS_ENV: "sandbox" | "production", default "sandbox"
}

type AppConfig struct {
	Name      string
	Env       string
	Debug     bool
	URL       string // base URL, e.g. http://localhost:8080
	PlaceName string // PLACE_NAME, shown on parking ticket
	QRSecret  string // QR_SECRET, used to obfuscate ticket filenames
}

type StorageConfig struct {
	Dir           string // STORAGE_DIR: local fallback path
	OCRPathPrefix string // OCR_STORAGE_PREFIX: path prefix as seen inside the OCR container
}

type S3Config struct {
	Endpoint      string // S3_ENDPOINT, e.g. https://s3.nevaobjects.id
	Bucket        string // S3_BUCKET
	AccessKey     string // S3_ACCESS_KEY
	SecretKey     string // S3_SECRET_KEY
	Region        string // S3_REGION
	PublicBaseURL string // S3_PUBLIC_BASE_URL, used to build public file URLs
}

type OCRConfig struct {
	Enabled             bool          // OCR_ENABLED
	APIURL              string        // OCR_API_URL, e.g. http://python-ocr:8000
	Timeout             time.Duration // OCR_TIMEOUT, e.g. 10s
	MaxRetries          int           // OCR_MAX_RETRIES
	AutoAcceptThreshold float64       // OCR_AUTO_ACCEPT_THRESHOLD: min confidence to auto-verify (default 0.80)
}

type DatabaseConfig struct {
	Host                   string
	Port                   int
	User                   string
	Password               string
	Name                   string
	SSLMode                string
	LogLevel               string
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	GateTokenTTL    time.Duration // default 365 hari — token untuk screen gate
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	PrintRoutes  bool
}

type TracingConfig struct {
	Enabled  bool
	Endpoint string
}

type ProfilingConfig struct {
	Enabled           bool
	PyroscopeEndpoint string
}

type LoggerConfig struct {
	Level          string
	AddSource      bool
	JSONFormat     bool
	OutputPaths    []string
	FileMaxSize    int
	FileMaxBackups int
	FileMaxAge     int
	FileCompress   bool
}

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
	log.Println("config: no .env file found, using OS environment variables")
}

func Load() (*Config, error) {
	cfg := &Config{
		Storage: StorageConfig{
			Dir:           getEnv("STORAGE_DIR", "./storage/photos"),
			OCRPathPrefix: getEnv("OCR_STORAGE_PREFIX", ""),
		},
		S3: S3Config{
			Endpoint:      getEnv("S3_ENDPOINT", ""),
			Bucket:        getEnv("S3_BUCKET", ""),
			AccessKey:     getEnv("S3_ACCESS_KEY", ""),
			SecretKey:     getEnv("S3_SECRET_KEY", ""),
			Region:        getEnv("S3_REGION", "us-east-1"),
			PublicBaseURL: getEnv("S3_PUBLIC_BASE_URL", ""),
		},
		Midtrans: MidtransConfig{
			ServerKey: getEnv("MIDTRANS_SERVER_KEY", ""),
			ClientKey: getEnv("MIDTRANS_CLIENT_KEY", ""),
			Env:       getEnv("MIDTRANS_ENV", "sandbox"),
		},
		OCR: OCRConfig{
			Enabled:             getEnvBool("OCR_ENABLED", true),
			APIURL:              getEnv("OCR_API_URL", "http://localhost:8000"),
			Timeout:             getEnvDuration("OCR_TIMEOUT", 15*time.Second),
			MaxRetries:          getEnvInt("OCR_MAX_RETRIES", 2),
			AutoAcceptThreshold: getEnvFloat64("OCR_AUTO_ACCEPT_THRESHOLD", 0.80),
		},
		App: AppConfig{
			Name:      getEnv("APP_NAME", "parkieee"),
			Env:       getEnv("APP_ENV", "development"),
			Debug:     getEnvBool("APP_DEBUG", true),
			URL:       getEnv("APP_URL", ""),
			PlaceName: getEnv("PLACE_NAME", "Parkir"),
			QRSecret:  getEnvRequired("QR_SECRET"),
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
			GateTokenTTL:    getEnvDuration("JWT_GATE_TOKEN_TTL", 365*24*time.Hour),
		},
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			PrintRoutes:  getEnvBool("SERVER_PRINT_ROUTES", true),
		},
		Tracing: TracingConfig{
			Enabled:  getEnvBool("ENABLE_TRACING", false),
			Endpoint: getEnv("TEMPO_ENDPOINT", "localhost:4318"),
		},
		Profiling: ProfilingConfig{
			Enabled:           getEnvBool("ENABLE_PROFILING", false),
			PyroscopeEndpoint: getEnv("PYROSCOPE_ENDPOINT", "http://localhost:4040"),
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

func (c *Config) IsDevelopment() bool { return c.App.Env == "development" }
func (c *Config) IsProduction() bool  { return c.App.Env == "production" }

// BaseURL returns APP_URL if set, otherwise constructs one from server host/port.
// SERVER_HOST 0.0.0.0 is normalized to localhost for external URLs.
func (c *Config) BaseURL() string {
	if c.App.URL != "" {
		return strings.TrimRight(c.App.URL, "/")
	}
	host := c.Server.Host
	if host == "0.0.0.0" || host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, c.Server.Port)
}

func (c *Config) ToLoggerConfig() *logger.Config {
	lvl := slog.LevelInfo
	switch strings.ToLower(c.Logger.Level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}

	cfg := &logger.Config{
		Level:       lvl,
		AddSource:   c.Logger.AddSource,
		OutputPaths: c.Logger.OutputPaths,
	}

	for _, p := range c.Logger.OutputPaths {
		if p != "stdout" && p != "stderr" {
			cfg.FileConfig = &logger.FileConfig{
				Path:       p,
				MaxSize:    c.Logger.FileMaxSize,
				MaxBackups: c.Logger.FileMaxBackups,
				MaxAge:     c.Logger.FileMaxAge,
				Compress:   c.Logger.FileCompress,
			}
			break
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: required env variable %s is not set", key)
	}
	return v
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

func getEnvFloat64(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func getEnvStringSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return strings.Split(v, ",")
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Name, c.Database.SSLMode,
	)
}
