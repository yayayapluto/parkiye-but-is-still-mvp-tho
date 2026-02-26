package bootstrap

import (
	"context"
	"os"
	"os/signal"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
	"syscall"
)

// App represents the application
type App struct {
	Config    *config.Config
	Logger    logger.Logger
	Container *Container
	//Server    *Server
}

// NewApp creates a new application instance
func NewApp() (*App, error) {
	// Load configuration
	config.LoadEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Initialize logger
	//log := logger.New(&cfg.App)

	// Initialize container
	//container, err := NewContainer(cfg, log)
	//if err != nil {
	//	return nil, err
	//}

	// Initialize server
	//server := NewServer(container)

	return &App{
		Config: cfg,
		//Logger:    log,
		//Container: container,
		//Server:    server,
	}, nil
}

// Run starts the application
func (a *App) Run() error {
	// Graceful shutdown channel
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		//addr := a.Config.Server.Host + ":" + a.Config.Server.Port
		//a.Logger.Info(context.Background(), "server starting", "addr", addr, "env", a.Config.App.Env)
		//if err := a.Server.Listen(addr); err != nil {
		//	a.Logger.Error(context.Background(), "server error", "error", err)
		//}
	}()

	// Wait for interrupt signal
	<-quit
	a.Logger.Info(context.Background(), "shutting down server...")

	// Graceful shutdown with timeout
	//ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//defer cancel()

	//if err := a.Server.ShutdownWithContext(ctx); err != nil {
	//	a.Logger.Error(context.Background(), "server shutdown error", "error", err)
	//}

	// Close database connection
	if err := a.Container.Close(); err != nil {
		a.Logger.Error(context.Background(), "database close error", "error", err)
	}

	a.Logger.Info(context.Background(), "server stopped")
	return nil
}
