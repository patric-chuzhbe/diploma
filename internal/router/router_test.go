package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/patric-chuzhbe/diploma/internal/auth"
	"github.com/patric-chuzhbe/diploma/internal/db/postgresdb"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/mockauthenticator"
	"github.com/patric-chuzhbe/diploma/internal/mockstorage"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

type testStorage interface {
	storage

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error
}

const (
	databaseDSN = `host=localhost user=diplomauser password=Ga6V0W0Ukn2s2tFn3uku7AAp2GAoy5 dbname=diploma sslmode=disable`

	migrationsDir = `../../migrations`

	authCookieName = `auth`

	authCookieSigningSecretKey = `LduYtmp2gWSRuyQyRHqbog==`

	logLevel = `debug`
)

func withdraw(
	t *testing.T,
	db testStorage,
	userID string,
	withdrawals map[string]float32,
) {
	for orderNum, withdrawal := range withdrawals {
		err := db.Withdraw(
			context.Background(),
			userID,
			orderNum,
			withdrawal,
		)
		require.NoError(t, err)
	}
}

func saveUserOrders(t *testing.T, db testStorage, userID string, orders []models.Order) {
	for _, order := range orders {
		_, err := db.SaveNewOrderForUser(
			context.Background(),
			userID,
			order.Number,
		)
		require.NoError(t, err)

		err = db.UpdateOrders(
			context.Background(),
			map[string]models.Order{order.Number: order},
			nil,
		)
		require.NoError(t, err)
	}
}

func createUsers(
	db testStorage,
	users []models.User,
	t *testing.T,
) map[string]string {
	result := map[string]string{}
	for _, user := range users {
		userID, err := db.CreateUser(context.Background(), &user)
		require.NoError(t, err)
		result[user.Login] = userID
	}
	return result
}

func TestIndexRoute(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	r := New(
		db,
		auth.New(
			db,
			authCookieName,
			[]byte(authCookieSigningSecretKey),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Hello from the Gophermart!")
}

func TestRegisterLoginCycle(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	r := New(
		db,
		auth.New(
			db,
			authCookieName,
			[]byte(authCookieSigningSecretKey),
		),
	)

	// Register
	body := []byte(`{"login":"testuser","password":"pass123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Login
	req = httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestPostApiUserRegisterValidationError(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	r := New(
		db,
		auth.New(
			db,
			authCookieName,
			[]byte(authCookieSigningSecretKey),
		),
	)

	reqBody := []byte(`{"login": "", "password": "bad_пароль"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestPostApiUserRegisterConflict(t *testing.T) {
	db := new(mockstorage.MockStorage)
	mockAuth := new(mockauthenticator.MockAuthenticator)
	r := New(db, mockAuth)

	body := models.UserRegisterRequest{Login: "user", Pass: "Password1!"}
	jsonBody, _ := json.Marshal(body)
	db.On("CreateUser", mock.Anything, mock.Anything).Return("", models.ErrUserAlreadyExists)

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusConflict, resp.Code)
}

func TestPostApiUserLoginEmptyCredentials(t *testing.T) {
	db := new(mockstorage.MockStorage)
	mockAuth := new(mockauthenticator.MockAuthenticator)
	r := New(db, mockAuth)

	reqBody := []byte(`{"login": "", "password": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestPostApiUserOrders(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100,
			},
			models.User{
				Login:          "user2",
				Pass:           "pass2",
				LoyaltyBalance: 200,
			},
		},
		t,
	)

	tests := []struct {
		name           string
		orderNumber    string
		expectedStatus int
		user           string
		mockErr        error
	}{
		{
			name:           "success",
			orderNumber:    "5857088487",
			expectedStatus: http.StatusAccepted,
			user:           "user1",
		},
		{
			name:           "order already uploaded by same user",
			orderNumber:    "5857088487",
			expectedStatus: http.StatusOK,
			user:           "user1",
		},
		{
			name:           "order uploaded by another user",
			orderNumber:    "5857088487",
			expectedStatus: http.StatusConflict,
			user:           "user2",
		},
		{
			name:           "invalid order number",
			orderNumber:    "abc",
			expectedStatus: http.StatusUnprocessableEntity,
			user:           "user1",
		},
		{
			name:           "internal error",
			orderNumber:    "2503317444",
			mockErr:        errors.New("db error"),
			expectedStatus: http.StatusInternalServerError,
			user:           "user1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rMock *chi.Mux = nil
			if tt.mockErr != nil {
				db := new(mockstorage.MockStorage)
				db.On("SaveNewOrderForUser", mock.Anything, mock.Anything, mock.Anything).
					Return("", tt.mockErr)
				rMock = New(db, a)
			}

			reqBody := []byte(tt.orderNumber)
			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader(reqBody))
			req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs[tt.user]))
			req.Header.Set("Content-Type", "text/plain")

			resp := httptest.NewRecorder()

			if tt.mockErr != nil {
				rMock.ServeHTTP(resp, req)
			} else {
				r.ServeHTTP(resp, req)
			}

			require.Equal(t, tt.expectedStatus, resp.Code)
		})
	}
}

func TestPostApiUserOrdersBadContentType(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	r := New(
		db,
		auth.New(
			db,
			authCookieName,
			[]byte(authCookieSigningSecretKey),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte("1234567890")))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetApiUserOrdersSuccess(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100,
			},
			models.User{
				Login:          "user2",
				Pass:           "pass2",
				LoyaltyBalance: 200,
			},
		},
		t,
	)

	accrual150_5 := float32(150.5)
	accrual200_7 := float32(200.7)
	expectedOrders := []models.Order{
		{
			Number:     "5857088487",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual150_5,
			UploadedAt: "2025-05-13T18:01:13.229315Z",
		},
		{
			Number:     "2503317444",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual200_7,
			UploadedAt: "2025-05-13T18:02:13.229315Z",
		},
	}

	saveUserOrders(t, db, userIDs["user1"], expectedOrders)

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))

	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var actualOrders []models.Order
	err = json.NewDecoder(rr.Body).Decode(&actualOrders)
	assert.NoError(t, err)

	require.Len(t, actualOrders, len(expectedOrders))

	sort.Slice(actualOrders, func(i, j int) bool {
		return actualOrders[i].Number < actualOrders[j].Number
	})

	sort.Slice(expectedOrders, func(i, j int) bool {
		return expectedOrders[i].Number < expectedOrders[j].Number
	})

	for i := range expectedOrders {
		assert.Equal(t, expectedOrders[i].Number, actualOrders[i].Number)
		assert.Equal(t, expectedOrders[i].Status, actualOrders[i].Status)
		assert.InDeltaf(t,
			*expectedOrders[i].Accrual,
			*actualOrders[i].Accrual,
			0.00001,
			"Accrual mismatch at index %d", i,
		)
	}
}

func TestGetApiUserBalanceSuccess(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	accrual150_5 := float32(150.5)
	accrual200_7 := float32(200.7)
	orders := []models.Order{
		{
			Number:     "5857088487",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual150_5,
			UploadedAt: "2025-05-13T18:01:13.229315Z",
		},
		{
			Number:     "2503317444",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual200_7,
			UploadedAt: "2025-05-13T18:02:13.229315Z",
		},
	}

	saveUserOrders(t, db, userIDs["user1"], orders)

	withdraw(
		t,
		db,
		userIDs["user1"],
		map[string]float32{
			"5857088487": 25,
			"2503317444": 25.7,
		},
	)

	expected := models.UserBalanceAndWithdrawals{
		Current:   49.8,
		Withdrawn: 50.7,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var actual models.UserBalanceAndWithdrawals
	err = json.NewDecoder(rec.Body).Decode(&actual)
	require.NoError(t, err)

	require.InDelta(t, expected.Current, actual.Current, 0.00001)
	require.InDelta(t, expected.Withdrawn, actual.Withdrawn, 0.00001)
}

func TestGetApiUserBalanceUnauthorized(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	accrual150_5 := float32(150.5)
	accrual200_7 := float32(200.7)
	orders := []models.Order{
		{
			Number:     "5857088487",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual150_5,
			UploadedAt: "2025-05-13T18:01:13.229315Z",
		},
		{
			Number:     "2503317444",
			Status:     models.LocalOrderStatusProcessed,
			Accrual:    &accrual200_7,
			UploadedAt: "2025-05-13T18:02:13.229315Z",
		},
	}

	saveUserOrders(t, db, userIDs["user1"], orders)

	withdraw(
		t,
		db,
		userIDs["user1"],
		map[string]float32{
			"5857088487": 25,
			"2503317444": 25.7,
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetApiUserBalanceInternalServerError(t *testing.T) {
	db := new(mockstorage.MockStorage)
	mockAuth := new(mockauthenticator.MockAuthenticator)
	r := New(db, mockAuth)

	db.On(
		"GetUserBalanceAndWithdrawals",
		mock.Anything,
		"test-user",
	).
		Return(
			&models.UserBalanceAndWithdrawals{},
			errors.New("db error"),
		)

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "test-user"))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestWithdrawSuccess(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	reqBody, err := json.Marshal(models.BalanceWithdrawRequest{
		OrderNumber: "3376308833",
		WithdrawSum: 50.2,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWithdrawUnauthorized(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	_ = createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWithdrawInvalidJSON(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader("{invalid json"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWithdrawValidationFail(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	reqBody := `{"order": "5857088487"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWithdrawInvalidOrderFormat(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	reqBody := `{"order": "invalid-order", "sum": 100}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestWithdrawNotEnoughBalance(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	reqBody, _ := json.Marshal(models.BalanceWithdrawRequest{
		OrderNumber: "3376308833",
		WithdrawSum: 100.6,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
}

func TestWithdrawAlreadyWithdrawn(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	reqBody, _ := json.Marshal(models.BalanceWithdrawRequest{
		OrderNumber: "3376308833",
		WithdrawSum: 25,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	reqBody2, _ := json.Marshal(models.BalanceWithdrawRequest{
		OrderNumber: "3376308833",
		WithdrawSum: 30,
	})

	req = httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(reqBody2))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec2.Code)
}

func TestWithdrawInternalError(t *testing.T) {
	db := new(mockstorage.MockStorage)
	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userID := "346d2d98-dd86-48fd-a676-92e85907d046"
	order := "5857088487"
	withdrawSum := float32(30)

	db.On("Withdraw", mock.Anything, userID, order, withdrawSum).
		Return(errors.New("db error"))

	reqBody, _ := json.Marshal(models.BalanceWithdrawRequest{
		OrderNumber: order,
		WithdrawSum: withdrawSum,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "346d2d98-dd86-48fd-a676-92e85907d046"))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWithdrawalsHandlerSuccess(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	withdraw(
		t,
		db,
		userIDs["user1"],
		map[string]float32{
			"5857088487": 25,
			"2503317444": 25.7,
		},
	)

	expectedResult := []models.UserWithdrawal{
		{
			OrderNumber: "5857088487",
			Sum:         25,
		},
		{
			OrderNumber: "2503317444",
			Sum:         25.7,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var result []models.UserWithdrawal
	err = json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, len(expectedResult))

	sort.Slice(result, func(i, j int) bool {
		return result[i].OrderNumber < result[j].OrderNumber
	})

	sort.Slice(expectedResult, func(i, j int) bool {
		return expectedResult[i].OrderNumber < expectedResult[j].OrderNumber
	})

	for i := range expectedResult {
		assert.Equal(t, expectedResult[i].OrderNumber, result[i].OrderNumber)
		assert.InDeltaf(
			t,
			expectedResult[i].Sum,
			result[i].Sum,
			0.00001,
			"Results mismatch at index %d",
			i,
		)
	}
}

func TestWithdrawalsHandlerNoWithdrawals(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 100.5,
			},
		},
		t,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userIDs["user1"]))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNoContent, res.StatusCode)
}

func TestWithdrawalsHandlerStorageError(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db := new(mockstorage.MockStorage)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	db.On("GetUserWithdrawals", mock.Anything, mock.Anything).
		Return([]models.UserWithdrawal{}, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "test-user1"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func TestWithdrawalsHandlerUnauthorized(t *testing.T) {
	err := logger.Init(logLevel)
	require.NoError(t, err)

	err = logger.Init(logLevel)
	require.NoError(t, err)
	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	a := new(mockauthenticator.MockAuthenticator)

	r := New(db, a)

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}
