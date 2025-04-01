package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/config"
	"github.com/patric-chuzhbe/diploma/internal/db/postgresdb"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/patric-chuzhbe/diploma/internal/router"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type userKeeper interface {
	CreateUser(ctx context.Context, usr *models.User) (string, error)
}

type storage interface {
	userKeeper
	Close() error
}

type App struct {
	cfg         *config.Config
	db          storage
	httpHandler http.Handler
}

func New() (*App, error) {
	var err error
	app := &App{}

	app.cfg, err = config.New()
	if err != nil {
		return nil, err
	}

	err = logger.Init(app.cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	app.db, err = postgresdb.New(app.cfg.DatabaseDSN, app.cfg.MigrationsDir)
	if err != nil {
		return nil, err
	}

	authCookieSigningSecretKey, err := base64.URLEncoding.DecodeString(app.cfg.AuthCookieSigningSecretKey)
	if err != nil {
		return nil, err
	}

	app.httpHandler = router.New(
		app.db,
		auth.New(
			app.db,
			app.cfg.AuthCookieName,
			authCookieSigningSecretKey,
		),
	)

	return app, nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Log.Infoln("server running", "RunAddr", a.cfg.RunAddr)

	server := &http.Server{
		Addr:    a.cfg.RunAddr,
		Handler: a.httpHandler,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Log.Infoln("Received shutdown signal. Saving database and exiting...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		return a.db.Close()

	case err := <-serverErrCh:
		return fmt.Errorf("server error: %w", err)
	}
}

func (a *App) Close() {
	if err := logger.Sync(); err != nil {
		fmt.Println("Logger sync error:", err)
	}
}
