package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/middlewares"
	"github.com/patric-chuzhbe/diploma/internal/router/handlers"
	"net/http"
)

type storage interface {
	handlers.UsersKeeper
	handlers.BalancesKeeper
	handlers.OrdersKeeper
}

type authenticator interface {
	SetAuthData(userID string, response http.ResponseWriter) error
	AuthenticateUser(h http.Handler) http.Handler
}

func New(
	db storage,
	auth authenticator,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(
		logger.WithLoggingHTTPMiddleware,
		middlewares.UngzipJSONAndTextHTMLRequest,
	)

	indexHandler := handlers.NewIndexHandler()
	authHandler := handlers.NewAuthHandler(db, auth)
	ordersHandler := handlers.NewOrdersHandler(db, auth)
	balanceHandler := handlers.NewBalanceHandler(db, auth)

	r.Get(`/`, indexHandler.GetIndex)

	r.Post(`/api/user/register`, authHandler.PostApiuserregister)

	r.Post(`/api/user/login`, authHandler.PostApiuserlogin)

	r.With(
		middlewares.GzipResponse,
		auth.AuthenticateUser,
	).Post(`/api/user/orders`, ordersHandler.PostApiuserorders)

	r.With(
		middlewares.GzipResponse,
		auth.AuthenticateUser,
	).Get(`/api/user/orders`, ordersHandler.GetApiuserorders)

	r.With(
		middlewares.GzipResponse,
		auth.AuthenticateUser,
	).Get(`/api/user/balance`, balanceHandler.GetApiuserbalance)

	r.With(auth.AuthenticateUser).Post(`/api/user/balance/withdraw`, balanceHandler.PostApiuserbalancewithdraw)

	r.With(
		middlewares.GzipResponse,
		auth.AuthenticateUser,
	).Get(`/api/user/withdrawals`, balanceHandler.GetApiuserwithdrawals)

	return r
}
