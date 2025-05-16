package mockauthenticator

import (
	"github.com/stretchr/testify/mock"
	"net/http"
)

type MockAuthenticator struct {
	mock.Mock
}

func (m *MockAuthenticator) SetAuthData(userID string, response http.ResponseWriter) error {
	args := m.Called(userID, response)
	return args.Error(0)
}

func (m *MockAuthenticator) AuthenticateUser(h http.Handler) http.Handler {
	return h
}
