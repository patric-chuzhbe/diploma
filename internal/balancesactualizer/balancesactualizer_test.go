package balancesactualizer

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/patric-chuzhbe/diploma/internal/db/postgresdb"
	"github.com/patric-chuzhbe/diploma/internal/logger"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

type testStorage interface {
	storage

	UpdateOrders(
		ctx context.Context,
		orders map[string]models.Order,
		outerTransaction *sql.Tx,
	) error

	CreateUser(
		ctx context.Context,
		usr *models.User,
	) (string, error)

	SaveNewOrderForUser(
		ctx context.Context,
		userID string,
		orderNumber string,
	) (string, error)
}

const (
	databaseDSN = `host=localhost user=diplomatestuser password=XMXHYLEmMV3Rz1t8nS1onq5xRGxMhK dbname=diplomatest sslmode=disable`

	migrationsDir = `../../migrations`

	authCookieName = `auth`

	authCookieSigningSecretKey = `LduYtmp2gWSRuyQyRHqbog==`

	logLevel = `debug`
)

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

func startMockAccrualServer(t *testing.T, accruals map[string]float32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		order := parts[len(parts)-1]

		accrual, ok := accruals[order]
		require.True(t, ok)

		resp := map[string]interface{}{
			"order":   order,
			"status":  models.RemoteOrderStatusProcessed,
			"accrual": accrual,
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
}

func TestBalancesActualizerRunSuccess(t *testing.T) {
	mockServer := startMockAccrualServer(t, map[string]float32{
		"5857088487": 150.5,
		"2503317444": 200.7,
	})
	defer mockServer.Close()

	err := logger.Init(logLevel)
	require.NoError(t, err)

	db, err := postgresdb.New(databaseDSN, migrationsDir, postgresdb.WithDBPreReset(true))
	require.NoError(t, err)

	userIDs := createUsers(
		db,
		[]models.User{
			models.User{
				Login:          "user1",
				Pass:           "pass1",
				LoyaltyBalance: 0,
			},
		},
		t,
	)

	accrual150_5 := float32(0)
	accrual200_7 := float32(0)
	orders := []models.Order{
		{
			Number:  "5857088487",
			Status:  models.LocalOrderStatusNew,
			Accrual: &accrual150_5,
		},
		{
			Number:  "2503317444",
			Status:  models.LocalOrderStatusNew,
			Accrual: &accrual200_7,
		},
	}

	saveUserOrders(t, db, userIDs["user1"], orders)

	actualizer := New(
		db,
		5*time.Second,
		10,
		10,
		3*time.Second,
		mockServer.URL,
		2,
		2,
		2,
	)

	ctx, cancel := context.WithCancel(context.Background())

	var errorOccurred bool
	actualizer.ListenErrors(func(err error) {
		t.Log("Unexpected error:", err)
		errorOccurred = true
	})

	actualizer.Run(ctx)

	time.Sleep(7 * time.Second)
	cancel()

	// allow goroutines to clean up
	time.Sleep(7 * time.Second)

	assert.False(t, errorOccurred)

	accrual150_5 = float32(150.5)
	accrual200_7 = float32(200.7)
	expectedOrders := []models.Order{
		{
			Number:  "5857088487",
			Status:  models.LocalOrderStatusProcessed,
			Accrual: &accrual150_5,
		},
		{
			Number:  "2503317444",
			Status:  models.LocalOrderStatusProcessed,
			Accrual: &accrual200_7,
		},
	}

	actualOrders, err := db.GetUserOrders(context.Background(), userIDs["user1"])
	require.NoError(t, err)

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
		assert.InDeltaf(
			t,
			*expectedOrders[i].Accrual,
			*actualOrders[i].Accrual,
			0.0001,
			"Results mismatch at index %d",
			i,
		)
	}

	user, err := db.GetUserByID(context.Background(), userIDs["user1"])
	require.NoError(t, err)

	expectedLoyaltyBalance := 351.2

	assert.InDelta(t, expectedLoyaltyBalance, user.LoyaltyBalance, 0.0001)
}
