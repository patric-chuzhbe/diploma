package router

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"go.uber.org/zap"
	"net/http"
	"regexp"
)

type storage interface {
	CreateUser(
		ctx context.Context,
		usr *models.User,
	) (string, error)
}

type authenticator interface {
	SetAuthData(userID string, response http.ResponseWriter) error
}

type router struct {
	db   storage
	auth authenticator
}

var pwdPattern = regexp.MustCompile(`^[a-zA-Z0-9~!@#$%^*]+$`)

func validatePassword(fl validator.FieldLevel) bool {
	matched := pwdPattern.MatchString(fl.Field().String())
	return matched
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

func (theRouter router) getUnexistentFullsToShortsMap(unexistentFulls []string) map[string]string {
	result := map[string]string{}
	for _, full := range unexistentFulls {
		result[full] = uuid.New().String()
	}

	return result
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

	return r
}
