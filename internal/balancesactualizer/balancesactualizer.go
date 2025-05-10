package balancesactualizer

import (
	"context"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/balancesactualizer/accrualsfetcher"
	"github.com/patric-chuzhbe/diploma/internal/balancesactualizer/balancesupdater"
	"github.com/patric-chuzhbe/diploma/internal/balancesactualizer/ordersupdater"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"time"
)

type userOrdersKeeper interface {
	GetOrdersAndUpdateStatus(
		ctx context.Context,
		statusFilter []string,
		newStatus string,
		ordersBatchSize int,
	) (map[string]models.Order, error)
}

type storage interface {
	userOrdersKeeper
	balancesupdater.Storage
	ordersupdater.Storage
}

type BalancesActualizer struct {
	db                             storage
	delayBetweenOrdersQueueFetches time.Duration
	errorChannel                   chan error
	ordersBatchSize                int
	accrualsFetcher                *accrualsfetcher.AccrualsFetcher
	ordersUpdater                  *ordersupdater.OrdersUpdater
	balancesUpdater                *balancesupdater.BalancesUpdater
}

func (a *BalancesActualizer) generator(
	ctx context.Context,
	doneCh chan struct{},
	input map[string]models.Order,
) chan *models.Order {
	inputCh := make(chan *models.Order)

	go func() {
		defer close(inputCh)

		for _, data := range input {
			select {
			case <-doneCh:
			case <-ctx.Done():
				return
			case inputCh <- &data:
			}
		}
	}()

	return inputCh
}

func (a *BalancesActualizer) pipeline(ctx context.Context) {
	input, err := a.db.GetOrdersAndUpdateStatus(
		ctx,
		[]string{models.LocalOrderStatusNew},
		models.LocalOrderStatusProcessing,
		a.ordersBatchSize,
	)

	if err != nil {
		a.errorChannel <- fmt.Errorf(pipelineErr1, err)
		return
	}

	doneCh := make(chan struct{})
	defer close(doneCh)

	inputCh := a.generator(ctx, doneCh, input)

	fetchAccrualsResultCh := a.accrualsFetcher.Go(ctx, doneCh, inputCh)

	updateOrdersResultCh := a.ordersUpdater.Go(ctx, doneCh, fetchAccrualsResultCh)

	resultCh := a.balancesUpdater.Go(ctx, doneCh, updateOrdersResultCh)

	for resCh := range resultCh {
		if resCh.Err != nil {
			a.errorChannel <- fmt.Errorf(pipelineErr2, resCh.Err)
		}
	}
}

func (a *BalancesActualizer) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(a.delayBetweenOrdersQueueFetches)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Log.Infoln("BalancesActualizer.Run() stopped")
				return
			case <-ticker.C:
				a.pipeline(ctx)
			}
		}
	}()
}

func (a *BalancesActualizer) ListenErrors(callback func(error)) {
	go func() {
		for err := range a.errorChannel {
			callback(err)
		}
	}()
}

func New(
	db storage,
	delayBetweenOrdersQueueFetches time.Duration,
	errorChannelCapacity int,
	ordersBatchSize int,
	httpClientTimeout time.Duration,
	accrualSystemAddress string,
	numFetchAccrualWorkers int,
	numUpdateOrdersWorkers int,
	numUpdateBalancesWorkers int,
) *BalancesActualizer {
	return &BalancesActualizer{
		db:                             db,
		delayBetweenOrdersQueueFetches: delayBetweenOrdersQueueFetches,
		errorChannel:                   make(chan error, errorChannelCapacity),
		ordersBatchSize:                ordersBatchSize,
		accrualsFetcher: accrualsfetcher.New(
			httpClientTimeout,
			accrualSystemAddress,
			numFetchAccrualWorkers,
		),
		ordersUpdater:   ordersupdater.New(db, numUpdateOrdersWorkers),
		balancesUpdater: balancesupdater.New(db, numUpdateBalancesWorkers),
	}
}
