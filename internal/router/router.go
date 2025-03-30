package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"net/http"
)

type storage interface{}

type router struct {
	db storage
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
) *chi.Mux {
	myRouter := router{
		db: database,
	}
	router := chi.NewRouter()

	router.Use(
		logger.WithLoggingHTTPMiddleware,
	)

	router.Get(`/`, myRouter.GetIndex)

	return router
}
