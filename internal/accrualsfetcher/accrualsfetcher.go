package accrualsfetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"net/http"
	"strconv"
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

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error

	CommitTransaction(transaction *sql.Tx) (err error)
}

type AccrualsFetcher struct {
	db                       storage
	delayBetweenQueueFetches time.Duration
	errorChannel             chan error
	ordersBatchSize          int
	schema                   string
	host                     string
	port                     string
	client                   *http.Client
	accrualSystemAddress     string
}

type apiOrder struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float32 `json:"accrual,omitempty"`
}

var remoteToLocalStatusMapping = map[string]string{
	"REGISTERED": "NEW",
	"PROCESSING": "NEW",
	"PROCESSED":  "PROCESSING",
	"INVALID":    "INVALID",
}

func (f *AccrualsFetcher) parseAndValidateAccrualServiceResponse(resp *http.Response, responseDTO *apiOrder) error {
	if err := json.NewDecoder(resp.Body).Decode(responseDTO); err != nil {
		return fmt.Errorf("cannot decode response JSON body: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(responseDTO); err != nil {
		return fmt.Errorf("incorrect response structure: %w", err)
	}

	return nil
}

func (f *AccrualsFetcher) getNewThrottleTimeout(
	throttleTimeout time.Duration,
	resp *http.Response,
) (time.Duration, error) {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		retryAfterAsInt, err := strconv.Atoi(retryAfter)
		if err != nil {
			return throttleTimeout, err
		}
		return time.Duration(retryAfterAsInt) * time.Second, nil
	}
	return throttleTimeout, errors.New("there is no `Retry-After` header in the 429 HTTP response")
}

func (f *AccrualsFetcher) actualizeOrder(order *models.Order, throttleTimeout time.Duration) (time.Duration, error) {
	url := fmt.Sprintf("%s/api/orders/%s", f.accrualSystemAddress, order.Number)
	resp, err := f.client.Get(url)
	if err != nil {
		f.errorChannel <- fmt.Errorf("failed to make GET request for order %s: %w", order.Number, err)
		err = resp.Body.Close()
		if err != nil {
			return throttleTimeout, fmt.Errorf("failed to close response body for order %s: %w", order.Number, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		throttleTimeout, err = f.getNewThrottleTimeout(throttleTimeout, resp)
		if err != nil {
			return throttleTimeout,
				fmt.Errorf(
					"failed to get new throttle timeout actualizing the order %s: %w",
					order.Number,
					err,
				)
		}
		return throttleTimeout, nil
	}

	var responseDTO apiOrder

	switch resp.StatusCode {
	case http.StatusOK:
		err := f.parseAndValidateAccrualServiceResponse(resp, &responseDTO)
		if err != nil {
			return throttleTimeout, fmt.Errorf("failed to parse response for order %s: %w", order.Number, err)
		}
	case http.StatusNoContent:
		// the order is not registered in the external system
		responseDTO = apiOrder{
			Order:  order.Number,
			Status: "INVALID",
		}
	default:
		return throttleTimeout,
			fmt.Errorf("unexpected status code %d for order %s", resp.StatusCode, order.Number)
	}

	if responseDTO.Order != order.Number {
		return throttleTimeout,
			fmt.Errorf(
				"the response order number (%s) is not equal to the target order number (%s)",
				responseDTO.Order,
				order.Number,
			)
	}

	*order = models.Order{
		Number:     order.Number,
		Status:     remoteToLocalStatusMapping[responseDTO.Status],
		Accrual:    &responseDTO.Accrual,
		UploadedAt: order.UploadedAt,
	}

	return throttleTimeout, nil
}

func (f *AccrualsFetcher) actualizeOrders(orders map[string]models.Order) {
	throttleTimeout := 0 * time.Second
	var err error
	for orderNum, order := range orders {
		time.Sleep(throttleTimeout)
		throttleTimeout, err = f.actualizeOrder(&order, throttleTimeout)
		if err != nil {
			f.errorChannel <- fmt.Errorf(
				"in internal/accrualsfetcher/accrualsfetcher.go/actualizeOrders(): error while `f.actualizeOrder()` calling: %w",
				err,
			)
			continue
		}
		orders[orderNum] = order
	}
}

func (f *AccrualsFetcher) fetchAccruals(ctx context.Context) {
	transaction, err := f.db.BeginTransaction()
	if err != nil {
		f.errorChannel <- fmt.Errorf(
			"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.BeginTransaction()` calling: %w",
			err,
		)
		return
	}

	orders, err := f.db.GetOrders(ctx, []string{"NEW"}, f.ordersBatchSize, transaction)
	if err != nil {
		err2 := f.db.RollbackTransaction(transaction)
		if err2 != nil {
			f.errorChannel <- fmt.Errorf(
				"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		f.errorChannel <- fmt.Errorf(
			"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.GetOrders()` calling: %w",
			err,
		)
		return
	}

	f.actualizeOrders(orders)

	err = f.db.UpdateOrders(ctx, orders, transaction)
	if err != nil {
		err2 := f.db.RollbackTransaction(transaction)
		if err2 != nil {
			f.errorChannel <- fmt.Errorf(
				"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		f.errorChannel <- fmt.Errorf(
			"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.UpdateOrders()` calling: %w",
			err,
		)
		return
	}

	err = f.db.CommitTransaction(transaction)
	if err != nil {
		err2 := f.db.RollbackTransaction(transaction)
		if err2 != nil {
			f.errorChannel <- fmt.Errorf(
				"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.RollbackTransaction()` calling: %w",
				err2,
			)
			return
		}
		f.errorChannel <- fmt.Errorf(
			"in internal/accrualsfetcher/accrualsfetcher.go/fetchAccruals(): error while `f.db.CommitTransaction()` calling: %w",
			err,
		)
		return
	}
}

func (f *AccrualsFetcher) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(f.delayBetweenQueueFetches)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Log.Infoln("AccrualsFetcher.Run() stopped")
				return
			case <-ticker.C:
				f.fetchAccruals(ctx)
			}
		}
	}()
}

func (f *AccrualsFetcher) ListenErrors(callback func(error)) {
	go func() {
		for err := range f.errorChannel {
			callback(err)
		}
	}()
}

func New(
	db storage,
	delayBetweenQueueFetchesForAccrualsFetcher time.Duration,
	errorChannelCapacity int,
	ordersBatchSize int,
	httpClientTimeout time.Duration,
	accrualSystemAddress string,
) *AccrualsFetcher {
	return &AccrualsFetcher{
		db:                       db,
		delayBetweenQueueFetches: delayBetweenQueueFetchesForAccrualsFetcher,
		errorChannel:             make(chan error, errorChannelCapacity),
		ordersBatchSize:          ordersBatchSize,
		client:                   &http.Client{Timeout: httpClientTimeout},
		accrualSystemAddress:     accrualSystemAddress,
	}
}
