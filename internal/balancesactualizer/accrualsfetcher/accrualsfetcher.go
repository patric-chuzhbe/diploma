package accrualsfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	actualizerModels "github.com/patric-chuzhbe/diploma/internal/balancesactualizer/models"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"net/http"
	"sync"
	"time"
)

type AccrualsFetcher struct {
	client               *http.Client
	accrualSystemAddress string
	numWorkers           int
}

func (f *AccrualsFetcher) fanIn(
	ctx context.Context,
	doneCh chan struct{},
	resultChs ...chan *actualizerModels.ApiOrder,
) chan *actualizerModels.ApiOrder {
	finalCh := make(chan *actualizerModels.ApiOrder)

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

func (f *AccrualsFetcher) parseAndValidateAccrualServiceResponse(
	resp *http.Response,
	responseDTO *actualizerModels.ApiOrder,
) error {
	if err := json.NewDecoder(resp.Body).Decode(responseDTO); err != nil {
		return fmt.Errorf("cannot decode response JSON body: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(responseDTO); err != nil {
		return fmt.Errorf("incorrect response structure: %w", err)
	}

	return nil
}

func (f *AccrualsFetcher) fetchAccrual(ctx context.Context, data *models.Order) *actualizerModels.ApiOrder {
	url := fmt.Sprintf("%s/api/orders/%s", f.accrualSystemAddress, data.Number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &actualizerModels.ApiOrder{
			Err:   fmt.Errorf(fetchAccrualErr1, err),
			Order: data.Number,
		}
	}
	resp, err := f.client.Do(req)
	if err != nil {
		err2 := resp.Body.Close()
		if err2 != nil {
			return &actualizerModels.ApiOrder{
				Err:   fmt.Errorf(fetchAccrualErr2, err),
				Order: data.Number,
			}
		}
		return &actualizerModels.ApiOrder{
			Err:   fmt.Errorf(fetchAccrualErr3, err),
			Order: data.Number,
		}
	}
	defer resp.Body.Close()

	var responseDTO actualizerModels.ApiOrder

	switch resp.StatusCode {
	case http.StatusOK:
		err := f.parseAndValidateAccrualServiceResponse(resp, &responseDTO)
		if err != nil {
			return &actualizerModels.ApiOrder{
				Err:   fmt.Errorf(fetchAccrualErr4, data.Number, err),
				Order: data.Number,
			}
		}
	case http.StatusNoContent:
		// the order is not registered in the external system
		responseDTO = actualizerModels.ApiOrder{
			Order:  data.Number,
			Status: models.RemoteOrderStatusInvalid,
		}
	default:
		return &actualizerModels.ApiOrder{
			Err:   fmt.Errorf(fetchAccrualErr5, resp.StatusCode, data.Number),
			Order: data.Number,
		}
	}

	if responseDTO.Order != data.Number {
		return &actualizerModels.ApiOrder{
			Err:   fmt.Errorf(fetchAccrualErr6, responseDTO.Order, data.Number),
			Order: data.Number,
		}
	}

	return &responseDTO
}

func (f *AccrualsFetcher) fetchAccruals(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *models.Order,
) chan *actualizerModels.ApiOrder {
	resCh := make(chan *actualizerModels.ApiOrder)

	go func() {
		defer close(resCh)

		for data := range inputCh {
			result := f.fetchAccrual(ctx, data)

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

func (f *AccrualsFetcher) fanOut(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *models.Order,
) []chan *actualizerModels.ApiOrder {
	channels := make([]chan *actualizerModels.ApiOrder, f.numWorkers)

	for i := 0; i < f.numWorkers; i++ {
		fetchAccrualResultCh := f.fetchAccruals(ctx, doneCh, inputCh)
		channels[i] = fetchAccrualResultCh
	}

	return channels
}

func (f *AccrualsFetcher) Go(
	ctx context.Context,
	doneCh chan struct{},
	inputCh chan *models.Order,
) chan *actualizerModels.ApiOrder {
	return f.fanIn(
		ctx,
		doneCh,
		f.fanOut(
			ctx,
			doneCh,
			inputCh,
		)...,
	)
}

func New(
	httpClientTimeout time.Duration,
	accrualSystemAddress string,
	numFetchAccrualWorkers int,
) *AccrualsFetcher {
	return &AccrualsFetcher{
		accrualSystemAddress: accrualSystemAddress,
		numWorkers:           numFetchAccrualWorkers,
		client:               &http.Client{Timeout: httpClientTimeout},
	}
}
