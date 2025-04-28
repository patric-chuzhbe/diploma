package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/joeljunstrom/go-luhn"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"go.uber.org/zap"
	"net/http"
)

type BalancesKeeper interface {
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

type BalanceHandler struct {
	db   BalancesKeeper
	auth authenticator
}

func (h *BalanceHandler) GetApiuserwithdrawals(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	responseDTO, err := h.db.GetUserWithdrawals(request.Context(), userID)
	if err != nil {
		logger.Log.Debugln("Error calling the `h.db.GetUserWithdrawals()`: ", zap.Error(err))
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

func (h *BalanceHandler) validateOrderNumber(orderNumber []byte) error {
	if !orderNumberPattern.Match(orderNumber) {
		return errors.New("order number contains invalid characters")
	}

	if !luhn.Valid(string(orderNumber)) {
		return errors.New("order number is invalid")
	}

	return nil
}

func (h *BalanceHandler) PostApiuserbalancewithdraw(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	var requestDTO models.BalanceWithdrawRequest
	if err := json.NewDecoder(request.Body).Decode(&requestDTO); err != nil {
		logger.Log.Debugln("cannot decode request JSON body", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	validate := validator.New()
	if err := validate.Struct(requestDTO); err != nil {
		logger.Log.Debugln("incorrect request structure", zap.Error(err))
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	if h.validateOrderNumber([]byte(requestDTO.OrderNumber)) != nil {
		response.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	err := h.db.Withdraw(request.Context(), userID, requestDTO.OrderNumber, requestDTO.WithdrawSum)

	if errors.Is(err, models.ErrNotEnoughBalance) {
		response.WriteHeader(http.StatusPaymentRequired)
		return
	}

	if errors.Is(err, models.ErrAlreadyWithdrawn) {
		response.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if err != nil {
		logger.Log.Debugln("Error calling the `h.db.Withdraw()`: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusOK)
}

func (h *BalanceHandler) GetApiuserbalance(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	responseDTO, err := h.db.GetUserBalanceAndWithdrawals(request.Context(), userID)
	if err != nil {
		logger.Log.Debugln("Error calling the `h.db.GetUserBalanceAndWithdrawals()`: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(response).Encode(responseDTO)
	if err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
		return
	}
}

func NewBalanceHandler(db BalancesKeeper, auth authenticator) *BalanceHandler {
	return &BalanceHandler{db: db, auth: auth}
}
