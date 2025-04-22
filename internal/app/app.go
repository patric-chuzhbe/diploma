package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/accrualsfetcher"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/config"
	"github.com/patric-chuzhbe/diploma/internal/db/postgresdb"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/patric-chuzhbe/diploma/internal/router"
	"github.com/patric-chuzhbe/diploma/internal/userbalancescalculator"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type usersKeeper interface {
	CreateUser(ctx context.Context, usr *models.User) (string, error)

	GetUserIDByLoginAndPassword(
		ctx context.Context,
		usr *models.User,
	) (string, error)

	GetUserByID(
		ctx context.Context,
		userID string,
	) (*models.User, error)

	UpdateUsers(
		ctx context.Context,
		users []models.User,
		outerTransaction *sql.Tx,
	) error
}

type userWithdrawalsKeeper interface {
	GetUserBalanceAndWithdrawals(
		ctx context.Context,
		userID string,
	) (*models.UserBalanceAndWithdrawals, error)

	Withdraw(
		ctx context.Context,
		userID string,
		orderNumber string,
		withdrawSum float32,
	) error

	GetUserWithdrawals(
		ctx context.Context,
		userID string,
	) ([]models.UserWithdrawal, error)
}

type transactioner interface {
	BeginTransaction() (*sql.Tx, error)

	RollbackTransaction(transaction *sql.Tx) error

	CommitTransaction(transaction *sql.Tx) error
}

type userOrdersKeeper interface {
	SaveNewOrderForUser(
		ctx context.Context,
		userID string,
		orderNumber string,
	) (string, error)

	GetUserOrders(
		ctx context.Context,
		userID string,
	) ([]models.Order, error)

	GetOrders(
		ctx context.Context,
		statusFilter []string,
		ordersBatchSize int,
		transaction *sql.Tx,
	) (map[string]models.Order, error)

	GetUsersByOrders(
		ctx context.Context,
		orderNumbers []string,
		transaction *sql.Tx,
	) ([]models.User, map[string][]string, error)

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error
}

type storage interface {
	usersKeeper
	userWithdrawalsKeeper
	transactioner
	userOrdersKeeper

	Close() error
}

type App struct {
	cfg                      *config.Config
	db                       storage
	httpHandler              http.Handler
	userBalancesCalculator   *userbalancescalculator.UserBalancesCalculator
	stopBalancesCalculator   context.CancelFunc
	balancesCalculatorRunCtx context.Context
	accrualsFetcher          *accrualsfetcher.AccrualsFetcher
	stopAccrualsFetcher      context.CancelFunc
	accrualsFetcherRunCtx    context.Context
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

	app.userBalancesCalculator = userbalancescalculator.New(
		app.db,
		app.cfg.DelayBetweenQueueFetchesForBalancesCalculator,
		app.cfg.ErrorChannelCapacity,
		app.cfg.OrdersBatchSizeForBalancesCalculator,
	)
	balancesCalculatorRunCtx, stopBalancesCalculator := context.WithCancel(context.Background())
	app.stopBalancesCalculator = stopBalancesCalculator
	app.balancesCalculatorRunCtx = balancesCalculatorRunCtx

	app.userBalancesCalculator.ListenErrors(func(err error) {
		logger.Log.Debugln("Error passed from the `app.userBalancesCalculator.ListenErrors()`:", zap.Error(err))
	})

	app.accrualsFetcher = accrualsfetcher.New(
		app.db,
		app.cfg.DelayBetweenQueueFetchesForAccrualsFetcher,
		app.cfg.ErrorChannelCapacity,
		app.cfg.OrdersBatchSizeForAccrualsFetcher,
		app.cfg.HttpClientTimeoutForAccrualsFetcher,
		app.cfg.AccrualSystemAddress,
	)
	accrualsFetcherRunCtx, stopAccrualsFetcher := context.WithCancel(context.Background())
	app.stopAccrualsFetcher = stopAccrualsFetcher
	app.accrualsFetcherRunCtx = accrualsFetcherRunCtx

	app.accrualsFetcher.ListenErrors(func(err error) {
		logger.Log.Debugln("Error passed from the `app.accrualsFetcher.ListenErrors()`:", zap.Error(err))
	})

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

	a.userBalancesCalculator.Run(a.balancesCalculatorRunCtx)
	a.accrualsFetcher.Run(a.accrualsFetcherRunCtx)

	select {
	case <-ctx.Done():
		logger.Log.Infoln("Received shutdown signal. Saving database and exiting...")
		a.stopBalancesCalculator()
		a.stopAccrualsFetcher()
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
