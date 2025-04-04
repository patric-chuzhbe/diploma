package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/pressly/goose/v3"
)

type PostgresDB struct {
	database *sql.DB
}

func (db *PostgresDB) GetUserBalanceAndWithdrawals(
	ctx context.Context,
	userID string,
) (*models.UserBalanceAndWithdrawals, error) {
	row := db.database.QueryRowContext(
		ctx,
		`
			SELECT 
				users.id,
				users.loyalty_balance,
				SUM(withdrawals."sum")
				FROM users
					LEFT JOIN users_withdrawals ON 
						users_withdrawals.user_id = users.id 
					LEFT JOIN withdrawals ON 
						withdrawals.order_number = users_withdrawals.withdraw_order_number
				WHERE users.id = $1
				GROUP BY users.id
				LIMIT 1;
		`,
		userID,
	)
	var userIDFromDB string
	var balance float32
	var withdrawalsSum sql.NullFloat64
	err := row.Scan(
		&userIDFromDB,
		&balance,
		&withdrawalsSum,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	withdrawalsSumValue := float32(0)
	if withdrawalsSum.Valid {
		withdrawalsSumValue = float32(withdrawalsSum.Float64)
	}

	return &models.UserBalanceAndWithdrawals{
		Current:   balance,
		Withdrawn: withdrawalsSumValue,
	}, nil
}

func (db *PostgresDB) GetUserOrders(
	ctx context.Context,
	userID string,
) ([]models.Order, error) {
	rows, err := db.database.QueryContext(
		ctx,
		`
			SELECT 
				orders.id,
				orders.status,
				orders.uploaded_at,
				orders.accrual
				FROM orders
					JOIN users_orders ON 
						users_orders.order_id = orders.id
							AND users_orders.user_id = $1
				ORDER BY orders.uploaded_at DESC;
		`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.Order{}
	for rows.Next() {
		var number,
			status,
			uploadedAt string
		var accrual sql.NullFloat64
		err = rows.Scan(
			&number,
			&status,
			&uploadedAt,
			&accrual,
		)
		if err != nil {
			return nil, err
		}

		var accrualPtr *float32
		if accrual.Valid {
			accrualValue := float32(accrual.Float64)
			accrualPtr = &accrualValue
		}

		result = append(
			result,
			models.Order{
				Number:     number,
				Status:     status,
				Accrual:    accrualPtr,
				UploadedAt: uploadedAt,
			},
		)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (db *PostgresDB) SaveNewOrderForUser(
	ctx context.Context,
	userID string,
	orderNumber string,
) (string, error) {
	var actualUserID string
	query := `
		WITH 
			ins_order AS (
				INSERT INTO orders (id, status)
				   VALUES ($1, 'NEW')
				   ON CONFLICT (id) DO NOTHING 
			), 
			ins_users_orders AS (
				INSERT INTO users_orders (user_id, order_id)
					VALUES ($2, $1)
					ON CONFLICT (order_id) DO NOTHING
			)
		SELECT user_id FROM users_orders WHERE order_id = $1;
	`
	err := db.database.QueryRowContext(ctx, query, orderNumber, userID).Scan(&actualUserID)

	if errors.Is(err, sql.ErrNoRows) {
		// Successfully inserted and linked order
		return userID, nil
	}

	if err != nil {
		return "", err
	}

	return actualUserID, models.ErrOrderAlreadyExists
}

func (db *PostgresDB) GetUserByID(
	ctx context.Context,
	userID string,
) (*models.User, error) {
	if userID == "" {
		return &models.User{}, nil
	}

	row := db.database.QueryRowContext(
		ctx,
		`SELECT id FROM users WHERE id = $1`,
		userID,
	)
	var userIDFromDB string
	err := row.Scan(&userIDFromDB)
	if errors.Is(err, sql.ErrNoRows) {
		return &models.User{}, nil
	}
	if err != nil {
		return &models.User{}, err
	}

	return &models.User{ID: userIDFromDB}, nil
}

func (db *PostgresDB) GetUserIDByLoginAndPassword(
	ctx context.Context,
	usr *models.User,
) (string, error) {
	var userID string
	query := `SELECT id FROM users WHERE login = $1 AND pass = $2`
	err := db.database.QueryRowContext(ctx, query, usr.Login, usr.Pass).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return userID, nil
}

func (db *PostgresDB) CreateUser(
	ctx context.Context,
	usr *models.User,
) (string, error) {
	var userID string
	query := `
		INSERT INTO users (login, pass, loyalty_balance)
		VALUES ($1, $2, $3)
		ON CONFLICT (login) DO NOTHING
		RETURNING id;
	`
	err := db.database.QueryRowContext(ctx, query, usr.Login, usr.Pass, usr.LoyaltyBalance).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		return "", models.ErrUserAlreadyExists
	}

	if err != nil {
		return "", err
	}

	return userID, nil
}

func New(databaseDSN string, migrationsDir string) (*PostgresDB, error) {
	database, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return nil, err
	}

	result := &PostgresDB{
		database: database,
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, fmt.Errorf("failed to set dialect:  %w", err)
	}

	if err := goose.Up(result.database, migrationsDir); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return result, nil
}

func (db *PostgresDB) Close() error {
	err := db.database.Close()
	if err != nil {
		return err
	}

	return nil
}
