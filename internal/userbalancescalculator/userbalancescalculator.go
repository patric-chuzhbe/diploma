package userbalancescalculator

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/thoas/go-funk"
	"time"
)

type storage interface {
	BeginTransaction() (*sql.Tx, error)

	GetOrders(
		ctx context.Context,
		statusFilter []string,
		ordersBatchSize int,
		transaction *sql.Tx,
	) (map[string]models.Order, error)

	RollbackTransaction(transaction *sql.Tx) error

	GetUsersByOrders(
		ctx context.Context,
		orderNumbers []string,
		transaction *sql.Tx,
	) ([]models.User, map[string][]string, error)

	UpdateUsers(
		ctx context.Context,
		users []models.User,
		outerTransaction *sql.Tx,
	) error

	CommitTransaction(transaction *sql.Tx) (err error)

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error
}

type UserBalancesCalculator struct {
	db                       storage
	delayBetweenQueueFetches time.Duration
	errorChannel             chan error
	ordersBatchSize          int
}

func (c *UserBalancesCalculator) calcBalances(ctx context.Context) {
	transaction, err := c.db.BeginTransaction()
	if err != nil {
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.BeginTransaction()` calling: %w",
			err,
		)
		return
	}

	orders, err := c.db.GetOrders(ctx, []string{"PROCESSING"}, c.ordersBatchSize, transaction)
	if err != nil {
		err2 := c.db.RollbackTransaction(transaction)
		if err2 != nil {
			c.errorChannel <- fmt.Errorf(
				"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.GetOrders()` calling: %w",
			err,
		)
		return
	}

	users, usersToOrdersMapping, err := c.db.GetUsersByOrders(
		ctx,
		funk.Map(orders, func(key string, it models.Order) string {
			return it.Number
		}).([]string),
		transaction,
	)
	if err != nil {
		err2 := c.db.RollbackTransaction(transaction)
		if err2 != nil {
			c.errorChannel <- fmt.Errorf(
				"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.GetUsersByOrders()` calling: %w",
			err,
		)
		return
	}

	for i, user := range users {
		for _, orderNum := range usersToOrdersMapping[user.ID] {
			if order, ok := orders[orderNum]; ok {
				users[i].LoyaltyBalance += *order.Accrual
				order.Status = "PROCESSED"
				orders[orderNum] = order
			}
		}
	}

	err = c.db.UpdateUsers(ctx, users, transaction)
	if err != nil {
		err2 := c.db.RollbackTransaction(transaction)
		if err2 != nil {
			c.errorChannel <- fmt.Errorf(
				"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.UpdateUsers()` calling: %w",
			err,
		)
		return
	}

	err = c.db.UpdateOrders(ctx, orders, transaction)
	if err != nil {
		err2 := c.db.RollbackTransaction(transaction)
		if err2 != nil {
			c.errorChannel <- fmt.Errorf(
				"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.UpdateOrders()` calling: %w",
			err,
		)
		return
	}

	err = c.db.CommitTransaction(transaction)
	if err != nil {
		err2 := c.db.RollbackTransaction(transaction)
		if err2 != nil {
			c.errorChannel <- fmt.Errorf(
				"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		c.errorChannel <- fmt.Errorf(
			"in internal/userbalancescalculator/userbalancescalculator.go/calcBalances(): error while `c.db.CommitTransaction()` calling: %w",
			err,
		)
		return
	}
}

func (c *UserBalancesCalculator) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.delayBetweenQueueFetches)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Log.Infoln("UserBalancesCalculator.Run() stopped")
				return
			case <-ticker.C:
				c.calcBalances(ctx)
			}
		}
	}()
}

func (c *UserBalancesCalculator) ListenErrors(callback func(error)) {
	go func() {
		for err := range c.errorChannel {
			callback(err)
		}
	}()
}

func New(
	db storage,
	delayBetweenQueueFetches time.Duration,
	errorChannelCapacity int,
	ordersBatchSize int,
) *UserBalancesCalculator {
	return &UserBalancesCalculator{
		db:                       db,
		delayBetweenQueueFetches: delayBetweenQueueFetches,
		errorChannel:             make(chan error, errorChannelCapacity),
		ordersBatchSize:          ordersBatchSize,
	}
}
