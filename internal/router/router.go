package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/joeljunstrom/go-luhn"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
)

type storage interface {
	CreateUser(
		ctx context.Context,
		usr *models.User,
	) (string, error)

	GetUserIDByLoginAndPassword(
		ctx context.Context,
		usr *models.User,
	) (string, error)

	SaveNewOrderForUser(
		ctx context.Context,
		userID string,
		orderNumber string,
	) (string, error)

	GetUserOrders(
		ctx context.Context,
		userID string,
	) ([]models.Order, error)

	GetUserBalanceAndWithdrawals(
		ctx context.Context,
		userID string,
	) (*models.UserBalanceAndWithdrawals, error)
}

type authenticator interface {
	SetAuthData(userID string, response http.ResponseWriter) error
	AuthenticateUser(h http.Handler) http.Handler
}

type router struct {
	db   storage
	auth authenticator
}

var pwdPattern = regexp.MustCompile(`^[a-zA-Z0-9~!@#$%^*]+$`)

var orderNumberPattern = regexp.MustCompile(`^\d+$`)

func (theRouter router) GetApiuserbalance(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	responseDTO, err := theRouter.db.GetUserBalanceAndWithdrawals(request.Context(), userID)
	if err != nil {
		logger.Log.Debugln("Error calling the `theRouter.db.GetUserBalanceAndWithdrawals()`: ", zap.Error(err))
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

func (theRouter router) GetApiuserorders(response http.ResponseWriter, request *http.Request) {
	userID, ok := request.Context().Value(auth.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	responseDTO, err := theRouter.db.GetUserOrders(request.Context(), userID)
	if err != nil {
		logger.Log.Debugln("Error calling the `theRouter.db.GetUserOrders()`: ", zap.Error(err))
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

func (theRouter router) validateOrderNumber(orderNumber []byte) error {
	if !orderNumberPattern.Match(orderNumber) {
		return errors.New("order number contains invalid characters")
	}

	if !luhn.Valid(string(orderNumber)) {
		return errors.New("order number is invalid")
	}

	return nil
}

func (theRouter router) PostApiuserorders(response http.ResponseWriter, request *http.Request) {
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

	err = theRouter.validateOrderNumber(orderNumber)
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

	actualUserID, err := theRouter.db.SaveNewOrderForUser(request.Context(), userID, string(orderNumber))
	if errors.Is(err, models.ErrOrderAlreadyExists) {
		if actualUserID == userID {
			response.WriteHeader(http.StatusOK)
			return
		}
		response.WriteHeader(http.StatusConflict)
		return
	}
	if err != nil {
		logger.Log.Debugln("error while `theRouter.db.SaveNewOrderForUser()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusAccepted)
}

func (theRouter router) PostApiuserlogin(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		logger.Log.Debug("got request with bad method", zap.String("method", request.Method))
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	var requestDTO models.UserRegisterRequest
	if err := json.NewDecoder(request.Body).Decode(&requestDTO); err != nil {
		logger.Log.Debugln("cannot decode request JSON body", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)

		return
	}

	validate := validator.New()
	err := validate.RegisterValidation("password", validatePassword)
	if err != nil {
		logger.Log.Debugln("error while `validate.RegisterValidation()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)

		return
	}
	if err := validate.Struct(requestDTO); err != nil {
		logger.Log.Debugln("incorrect request structure", zap.Error(err))
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	userId, err := theRouter.db.GetUserIDByLoginAndPassword(
		request.Context(),
		&models.User{
			Login: requestDTO.Login,
			Pass:  requestDTO.Pass,
		},
	)
	if err != nil {
		logger.Log.Debugln("error while `theRouter.db.GetUserIDByLoginAndPassword()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	if userId == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = theRouter.auth.SetAuthData(userId, response)
	if err != nil {
		logger.Log.Debugln("error while setting auth data", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusOK)
}

func validatePassword(fl validator.FieldLevel) bool {
	return pwdPattern.MatchString(fl.Field().String())
}

func (theRouter router) PostApiuserregister(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		logger.Log.Debug("got request with bad method", zap.String("method", request.Method))
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	var requestDTO models.UserRegisterRequest
	if err := json.NewDecoder(request.Body).Decode(&requestDTO); err != nil {
		logger.Log.Debugln("cannot decode request JSON body", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)

		return
	}

	validate := validator.New()
	err := validate.RegisterValidation("password", validatePassword)
	if err != nil {
		logger.Log.Debugln("error while `validate.RegisterValidation()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)

		return
	}
	if err := validate.Struct(requestDTO); err != nil {
		logger.Log.Debugln("incorrect request structure", zap.Error(err))
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	userId, err := theRouter.db.CreateUser(
		request.Context(),
		&models.User{
			Login: requestDTO.Login,
			Pass:  requestDTO.Pass,
		},
	)
	if errors.Is(err, models.ErrUserAlreadyExists) {
		logger.Log.Debugf("registering user `%s` already exists", requestDTO.Login)
		response.WriteHeader(http.StatusConflict)
		return
	}
	if err != nil {
		logger.Log.Debugln("error while `theRouter.db.CreateUser()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = theRouter.auth.SetAuthData(userId, response)
	if err != nil {
		logger.Log.Debugln("error while setting auth data", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusOK)
}

func (theRouter router) GetIndex(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusOK)

	_, err := response.Write([]byte(`Hello from the Gophermart!`))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
}

func New(
	database storage,
	auth authenticator,
) *chi.Mux {
	myRouter := router{
		db:   database,
		auth: auth,
	}
	r := chi.NewRouter()

	r.Use(
		logger.WithLoggingHTTPMiddleware,
	)

	r.Get(`/`, myRouter.GetIndex)

	r.Post(`/api/user/register`, myRouter.PostApiuserregister)

	r.Post(`/api/user/login`, myRouter.PostApiuserlogin)

	r.With(auth.AuthenticateUser).Post(`/api/user/orders`, myRouter.PostApiuserorders)

	r.With(auth.AuthenticateUser).Get(`/api/user/orders`, myRouter.GetApiuserorders)

	r.With(auth.AuthenticateUser).Get(`/api/user/balance`, myRouter.GetApiuserbalance)

	return r
}
