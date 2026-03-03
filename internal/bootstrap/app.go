package bootstrap

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"os"
	"os/signal"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
	"parkieee/pkg/photo"
	"syscall"
	"time"
)

// App represents the application
type App struct {
	Config    *config.Config
	Logger    logger.Logger
	Container *Container
	Server    *fiber.App
}

// NewApp creates a new application instance
func NewApp() (*App, error) {
	config.LoadEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log, err := newLogger(cfg)
	if err != nil {
		return nil, err
	}

	container, err := NewContainer(cfg, log)
	if err != nil {
		return nil, err
	}

	server := NewServer(container)

	// Ensure S3 bucket is publicly readable on every startup
	if err := photo.EnsureBucketPolicy(cfg.S3); err != nil {
		log.Warn(context.Background(), "s3 bucket policy: could not set public-read", "error", err)
	}

	return &App{
		Config:    cfg,
		Logger:    log,
		Container: container,
		Server:    server,
	}, nil
}

// Run starts the application
func (a *App) Run() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf("%s:%d", a.Config.Server.Host, a.Config.Server.Port)
		a.Logger.Info(context.Background(), "server starting", "addr", addr, "env", a.Config.App.Env)
		if err := a.Server.Listen(addr); err != nil {
			a.Logger.Error(context.Background(), "server error", "error", err)
		}
	}()

	<-quit
	a.Logger.Info(context.Background(), "shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.Server.ShutdownWithContext(ctx); err != nil {
		a.Logger.Error(context.Background(), "server shutdown error", "error", err)
	}

	if err := a.Container.Close(); err != nil {
		a.Logger.Error(context.Background(), "database close error", "error", err)
	}

	a.Logger.Info(context.Background(), "server stopped")
	return nil
}
