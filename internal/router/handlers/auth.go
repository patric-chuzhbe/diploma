package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"go.uber.org/zap"
	"net/http"
	"regexp"
)

type UsersKeeper interface {
	CreateUser(
		ctx context.Context,
		usr *models.User,
	) (string, error)

	GetUserIDByLoginAndPassword(
		ctx context.Context,
		usr *models.User,
	) (string, error)
}

type AuthHandler struct {
	db   UsersKeeper
	auth authenticator
}

func (h *AuthHandler) PostApiuserlogin(response http.ResponseWriter, request *http.Request) {
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

	userID, err := h.db.GetUserIDByLoginAndPassword(
		request.Context(),
		&models.User{
			Login: requestDTO.Login,
			Pass:  requestDTO.Pass,
		},
	)
	if err != nil {
		logger.Log.Debugln("error while `h.db.GetUserIDByLoginAndPassword()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	if userID == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = h.auth.SetAuthData(userID, response)
	if err != nil {
		logger.Log.Debugln("error while setting auth data", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusOK)
}

var pwdPattern = regexp.MustCompile(`^[a-zA-Z0-9~!@#$%^*]+$`)

func validatePassword(fl validator.FieldLevel) bool {
	return pwdPattern.MatchString(fl.Field().String())
}

func (h *AuthHandler) PostApiuserregister(response http.ResponseWriter, request *http.Request) {
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

	userID, err := h.db.CreateUser(
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
		logger.Log.Debugln("error while `h.db.CreateUser()` calling: ", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.auth.SetAuthData(userID, response)
	if err != nil {
		logger.Log.Debugln("error while setting auth data", zap.Error(err))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusOK)
}

func NewAuthHandler(db UsersKeeper, auth authenticator) *AuthHandler {
	return &AuthHandler{db: db, auth: auth}
}
