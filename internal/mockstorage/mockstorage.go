package mockstorage

import (
	"context"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/stretchr/testify/mock"
)

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) CreateUser(ctx context.Context, usr *models.User) (string, error) {
	args := m.Called(ctx, usr)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetUserIDByLoginAndPassword(ctx context.Context, usr *models.User) (string, error) {
	args := m.Called(ctx, usr)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) SaveNewOrderForUser(ctx context.Context, userID string, orderNumber string) (string, error) {
	args := m.Called(ctx, userID, orderNumber)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetUserOrders(ctx context.Context, userID string) ([]models.Order, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Order), args.Error(1)
}

func (m *MockStorage) GetUserBalanceAndWithdrawals(ctx context.Context, userID string) (*models.UserBalanceAndWithdrawals, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*models.UserBalanceAndWithdrawals), args.Error(1)
}

func (m *MockStorage) Withdraw(ctx context.Context, userID string, orderNumber string, withdrawSum float32) error {
	args := m.Called(ctx, userID, orderNumber, withdrawSum)
	return args.Error(0)
}

func (m *MockStorage) GetUserWithdrawals(ctx context.Context, userID string) ([]models.UserWithdrawal, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.UserWithdrawal), args.Error(1)
}
