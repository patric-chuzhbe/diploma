package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joeljunstrom/go-luhn"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
)

type OrdersKeeper interface {
	SaveNewOrderForUser(
		ctx context.Context,
		userID string,
		orderNumber string,
	) (string, error)

	GetUserOrders(
		ctx context.Context,
		userID string,
	) ([]models.Order, error)
}

type OrdersHandler struct {
	db   OrdersKeeper
	auth authenticator
}

var orderNumberPattern = regexp.MustCompile(`^\d+$`)

func (h *OrdersHandler) GetApiuserorders(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	responseDTO, err := h.db.GetUserOrders(request.Context(), userID)
	if err != nil {
		logger.Log.Debugln("Error calling the `h.db.GetUserOrders()`: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(responseDTO) == 0 {
		response.WriteHeader(http.StatusNoContent)
		return
	}

	response.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(response).Encode(responseDTO)
	if err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
		return
	}
}

func (h *OrdersHandler) validateOrderNumber(orderNumber []byte) error {
	if !orderNumberPattern.Match(orderNumber) {
		return errors.New("order number contains invalid characters")
	}

	if !luhn.Valid(string(orderNumber)) {
		return errors.New("order number is invalid")
	}

	return nil
}

func (h *OrdersHandler) PostApiuserorders(response http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Content-Type") != "text/plain" {
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	orderNumber, err := io.ReadAll(request.Body)
	if err != nil {
		logger.Log.Debugln("error while `io.ReadAll()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.validateOrderNumber(orderNumber)
	if err != nil {
		http.Error(
			response,
			fmt.Sprintf(
				"invalid order number: %v",
				err,
			),
			http.StatusUnprocessableEntity,
		)
		return
	}

	actualUserID, err := h.db.SaveNewOrderForUser(request.Context(), userID, string(orderNumber))
	if errors.Is(err, models.ErrOrderAlreadyExists) {
		if actualUserID == userID {
			response.WriteHeader(http.StatusOK)
			return
		}
		response.WriteHeader(http.StatusConflict)
		return
	}
	if err != nil {
		logger.Log.Debugln("error while `h.db.SaveNewOrderForUser()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusAccepted)
}

func NewOrdersHandler(db OrdersKeeper, auth authenticator) *OrdersHandler {
	return &OrdersHandler{db: db, auth: auth}
}
