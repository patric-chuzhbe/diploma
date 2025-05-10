package handlers

import "net/http"

type authenticator interface {
	SetAuthData(userID string, response http.ResponseWriter) error
	AuthenticateUser(h http.Handler) http.Handler
}
