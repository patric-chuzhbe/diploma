package ordersupdater

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
	GetOrderByID(
		ctx context.Context,
		ID string,
		transaction *sql.Tx,
	) (*models.Order, error)

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error
}

type Storage interface {
	transactioner
	ordersKeeper
}

type OrdersUpdater struct {
	db         Storage
	numWorkers int
}

var remoteToLocalStatusMapping = map[string]string{
	models.RemoteOrderStatusRegistered: models.LocalOrderStatusNew,
	models.RemoteOrderStatusProcessing: models.LocalOrderStatusNew,
	models.RemoteOrderStatusProcessed:  models.LocalOrderStatusProcessing,
	models.RemoteOrderStatusInvalid:    models.LocalOrderStatusInvalid,
}

func (u *OrdersUpdater) fanIn(
	ctx context.Context,
	doneCh chan struct{},
	resultChs ...chan *actualizerModels.UpdateOrderRes,
) chan *actualizerModels.UpdateOrderRes {
	finalCh := make(chan *actualizerModels.UpdateOrderRes)

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

func (u *OrdersUpdater) updateOrder(
	ctx context.Context,
	data *actualizerModels.ApiOrder,
) *actualizerModels.UpdateOrderRes {
	if data.Err != nil {
		return &actualizerModels.UpdateOrderRes{
			Err: fmt.Errorf(updateOrderErr1, data.Err),
			Order: models.Order{
				Number: data.Order,
			},
		}
	}

	transaction, err := u.db.BeginTransaction()
	if err != nil {
		return &actualizerModels.UpdateOrderRes{
			Err: fmt.Errorf(updateOrderErr2, err),
			Order: models.Order{
				Number: data.Order,
			},
		}
	}

	order, err := u.db.GetOrderByID(ctx, data.Order, transaction)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateOrderRes{
				Err: fmt.Errorf(updateOrderErr3, err2),
				Order: models.Order{
					Number: data.Order,
				},
			}
		}
		return &actualizerModels.UpdateOrderRes{
			Err: fmt.Errorf(updateOrderErr4, err),
			Order: models.Order{
				Number: data.Order,
			},
		}
	}
	order.Accrual = &data.Accrual
	order.Status = remoteToLocalStatusMapping[data.Status]
	err = u.db.UpdateOrders(
		ctx,
		map[string]models.Order{order.Number: *order},
		transaction,
	)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateOrderRes{
				Err: fmt.Errorf(updateOrderErr5, err2),
				Order: models.Order{
					Number: data.Order,
				},
			}
		}
		return &actualizerModels.UpdateOrderRes{
			Err: fmt.Errorf(updateOrderErr6, err),
			Order: models.Order{
				Number: data.Order,
			},
		}
	}

	err = u.db.CommitTransaction(transaction)
	if err != nil {
		err2 := u.db.RollbackTransaction(transaction)
		if err2 != nil {
			return &actualizerModels.UpdateOrderRes{
				Err: fmt.Errorf(updateOrderErr7, err2),
				Order: models.Order{
					Number: data.Order,
				},
			}
		}
		return &actualizerModels.UpdateOrderRes{
			Err: fmt.Errorf(updateOrderErr8, err),
			Order: models.Order{
				Number: data.Order,
			},
		}
	}

	return &actualizerModels.UpdateOrderRes{
		Order: *order,
	}
}

func (u *OrdersUpdater) updateOrders(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.ApiOrder,
) chan *actualizerModels.UpdateOrderRes {
	resCh := make(chan *actualizerModels.UpdateOrderRes)

	go func() {
		defer close(resCh)

		for data := range inputCh {
			result := u.updateOrder(ctx, data)

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

func (u *OrdersUpdater) fanOut(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.ApiOrder,
) []chan *actualizerModels.UpdateOrderRes {
	channels := make([]chan *actualizerModels.UpdateOrderRes, u.numWorkers)

	for i := 0; i < u.numWorkers; i++ {
		updateOrdersResultCh := u.updateOrders(ctx, doneCh, inputCh)
		channels[i] = updateOrdersResultCh
	}

	return channels
}

func (u *OrdersUpdater) Go(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *actualizerModels.ApiOrder,
) chan *actualizerModels.UpdateOrderRes {
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

func New(db Storage, numUpdateOrdersWorkers int) *OrdersUpdater {
	return &OrdersUpdater{
		db:         db,
		numWorkers: numUpdateOrdersWorkers,
	}
}
