package balancesupdater

import (
	"context"
	"database/sql"
	"fmt"
	actualizerModels "github.com/patric-chuzhbe/diploma/internal/balancesactualizer/models"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"sync"
)

type transactioner interface {
	BeginTransaction() (*sql.Tx, error)

	RollbackTransaction(transaction *sql.Tx) error

	CommitTransaction(transaction *sql.Tx) (err error)
}

type ordersKeeper interface {
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

type usersKeeper interface {
	UpdateUsers(
		ctx context.Context,
		users []models.User,
		outerTransaction *sql.Tx,
	) error
}

type Storage interface {
	transactioner
	ordersKeeper
	usersKeeper
}

type BalancesUpdater struct {
	numWorkers int
	db         Storage
}

func (u *BalancesUpdater) fanIn(
	ctx context.Context,
	doneCh chan struct{},
	resultChs ...chan *actualizerModels.UpdateBalanceRes,
) chan *actualizerModels.UpdateBalanceRes {
	finalCh := make(chan *actualizerModels.UpdateBalanceRes)

	var wg sync.WaitGroup

	for _, ch := range resultChs {
		chClosure := ch

		wg.Add(1)

		go func() {
			defer wg.Done()

			for data := range chClosure {
				select {
				case <-doneCh:
				case <-ctx.Done():
					return
				case finalCh <- data:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(finalCh)
	}()

	return finalCh
}

func (u *BalancesUpdater) updateBalance(
	ctx context.Context,
	data *actualizerModels.UpdateOrderRes,
) *actualizerModels.UpdateBalanceRes {
	transaction, err := u.db.BeginTransaction()
	if err != nil {
		return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr1, err)}
	}

	users, _, err := u.db.GetUsersByOrders(
		ctx,
		[]string{data.Number},
		transaction,
	)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr2, err2)}
		}
		return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr3, err)}
	}

	orders := map[string]models.Order{}

	result := &actualizerModels.UpdateBalanceRes{}

	for i, _ := range users {
		if data.Err != nil {
			data.Status = models.LocalOrderStatusNew
			orders[data.Number] = data.Order
			result = &actualizerModels.UpdateBalanceRes{
				Err: fmt.Errorf(updateBalanceErr4, data.Number, data.Err),
				Order: models.Order{
					Number: data.Number,
				},
			}
			continue
		}
		users[i].LoyaltyBalance += *data.Accrual
		data.Status = models.LocalOrderStatusProcessed
		orders[data.Number] = data.Order
	}

	err = u.db.UpdateUsers(ctx, users, transaction)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr2, err2)}
		}
		return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr5, err)}
	}

	err = u.db.UpdateOrders(ctx, orders, transaction)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr2, err2)}
		}
		return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr6, err)}
	}

	err = u.db.CommitTransaction(transaction)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr2, err2)}
		}
		return &actualizerModels.UpdateBalanceRes{Err: fmt.Errorf(updateBalanceErr7, err)}
	}

	return result
}

func (u *BalancesUpdater) updateBalances(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.UpdateOrderRes,
) chan *actualizerModels.UpdateBalanceRes {
	resCh := make(chan *actualizerModels.UpdateBalanceRes)

	go func() {
		defer close(resCh)

		for data := range inputCh {
			result := u.updateBalance(ctx, data)

			select {
			case <-doneCh:
			case <-ctx.Done():
				return
			case resCh <- result:
			}
		}
	}()

	return resCh
}

func (u *BalancesUpdater) fanOut(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.UpdateOrderRes,
) []chan *actualizerModels.UpdateBalanceRes {
	channels := make([]chan *actualizerModels.UpdateBalanceRes, u.numWorkers)

	for i := 0; i < u.numWorkers; i++ {
		updateBalancesResultCh := u.updateBalances(ctx, doneCh, inputCh)
		channels[i] = updateBalancesResultCh
	}

	return channels
}

func (u *BalancesUpdater) Go(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.UpdateOrderRes,
) chan *actualizerModels.UpdateBalanceRes {
	return u.fanIn(
		ctx,
		doneCh,
		u.fanOut(
			ctx,
			doneCh,
			inputCh,
		)...,
	)
}

func New(db Storage, numUpdateBalancesWorkers int) *BalancesUpdater {
	return &BalancesUpdater{
		numWorkers: numUpdateBalancesWorkers,
		db:         db,
	}
}
