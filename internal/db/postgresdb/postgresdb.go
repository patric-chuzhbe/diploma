package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"github.com/pressly/goose/v3"
	"strings"
)

type PostgresDB struct {
	database *sql.DB
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (db *PostgresDB) UpdateOrders(
	ctx context.Context,
	orders map[string]models.Order,
	outerTransaction *sql.Tx,
) error {
	innerTransaction := outerTransaction
	var err error
	if outerTransaction == nil {
		innerTransaction, err = db.BeginTransaction()
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateOrders(): error while `db.BeginTransaction()` calling: %w",
				err,
			)
		}
	}

	for _, order := range orders {
		_, err := innerTransaction.ExecContext(
			ctx,
			`
				UPDATE orders
					SET
						status = $1,
						accrual = $2
					WHERE id = $3;
			`,
			order.Status,
			*order.Accrual,
			order.Number,
		)
		if err != nil {
			if outerTransaction == nil {
				err2 := db.RollbackTransaction(innerTransaction)
				if err2 != nil {
					return fmt.Errorf(
						"in internal/db/postgresdb/postgresdb.go/UpdateOrders(): error while `db.RollbackTransaction()` calling: %w",
						err2,
					)
				}
			}
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateOrders(): error while `innerTransaction.ExecContext()` calling: %w",
				err,
			)
		}
	}

	if outerTransaction == nil {
		err := db.CommitTransaction(innerTransaction)
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateOrders(): error while `db.CommitTransaction()` calling: %w",
				err,
			)
		}
	}

	return nil
}

func (db *PostgresDB) CommitTransaction(transaction *sql.Tx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred while committing transaction: %v", r)
		}
	}()

	return transaction.Commit()
}

func (db *PostgresDB) UpdateUsers(
	ctx context.Context,
	users []models.User,
	outerTransaction *sql.Tx,
) error {
	innerTransaction := outerTransaction
	var err error
	if outerTransaction == nil {
		innerTransaction, err = db.BeginTransaction()
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateUsers(): error while `db.BeginTransaction()` calling: %w",
				err,
			)
		}
	}

	for _, user := range users {
		_, err := innerTransaction.ExecContext(
			ctx,
			`
				UPDATE users 
					SET 
						login = $1,
						pass = $2,
						loyalty_balance = $3
					WHERE id = $4
			`,
			user.Login,
			user.Pass,
			user.LoyaltyBalance,
			user.ID,
		)
		if err != nil {
			if outerTransaction == nil {
				err2 := db.RollbackTransaction(innerTransaction)
				if err2 != nil {
					return fmt.Errorf(
						"in internal/db/postgresdb/postgresdb.go/UpdateUsers(): error while `db.RollbackTransaction()` calling: %w",
						err2,
					)
				}
			}
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateUsers(): error while `innerTransaction.ExecContext()` calling: %w",
				err,
			)
		}
	}

	if outerTransaction == nil {
		err := db.CommitTransaction(innerTransaction)
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/UpdateUsers(): error while `db.CommitTransaction()` calling: %w",
				err,
			)
		}
	}

	return nil
}

func (db *PostgresDB) GetUsersByOrders(
	ctx context.Context,
	orderNumbers []string,
	transaction *sql.Tx,
) ([]models.User, map[string][]string, error) {
	if len(orderNumbers) == 0 {
		return []models.User{}, map[string][]string{}, nil
	}

	var database querier

	if transaction == nil {
		database = db.database
	} else {
		database = transaction
	}

	statusesPlaceholders := make([]string, len(orderNumbers))
	for i := range statusesPlaceholders {
		statusesPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	orderNumbersPlaceholdersAsString := strings.Join(statusesPlaceholders, ",")

	rows, err := database.QueryContext(
		ctx,
		fmt.Sprintf(
			`
				SELECT DISTINCT
					users.id,
					users.login,
					users.pass,
					users.loyalty_balance,
					STRING_AGG(users_orders.order_id, ',') AS order_ids
					FROM users
						JOIN users_orders ON 
							users_orders.user_id = users.id
								AND users_orders.order_id IN (%s)
					GROUP BY users.id;
			`,
			orderNumbersPlaceholdersAsString,
		),
		func(strSlice []string) []interface{} {
			result := make([]interface{}, len(strSlice))
			for i, v := range strSlice {
				result[i] = v
			}
			return result
		}(orderNumbers)...,
	)
	if err != nil {
		return []models.User{}, map[string][]string{},
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUsersByOrders(): error while `database.QueryContext()` calling: %w",
				err,
			)
	}
	defer rows.Close()

	users := []models.User{}
	usersToOrdersMapping := map[string][]string{}
	for rows.Next() {
		var userID string
		var login string
		var pass string
		var loyaltyBalance float32
		var orderIDs string
		err = rows.Scan(
			&userID,
			&login,
			&pass,
			&loyaltyBalance,
			&orderIDs,
		)
		if err != nil {
			return []models.User{}, map[string][]string{},
				fmt.Errorf(
					"in internal/db/postgresdb/postgresdb.go/GetUsersByOrders(): error while `rows.Scan()` calling: %w",
					err,
				)
		}

		users = append(users, models.User{
			ID:             userID,
			Login:          login,
			Pass:           pass,
			LoyaltyBalance: loyaltyBalance,
		})

		for _, orderID := range strings.Split(orderIDs, ",") {
			usersToOrdersMapping[userID] = append(usersToOrdersMapping[userID], orderID)
		}
	}

	err = rows.Err()
	if err != nil {
		return []models.User{}, map[string][]string{},
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUsersByOrders(): error while `rows.Err()` calling: %w",
				err,
			)
	}

	return users, usersToOrdersMapping, nil
}

func (db *PostgresDB) RollbackTransaction(transaction *sql.Tx) error {
	return transaction.Rollback()
}

func (db *PostgresDB) GetOrders(
	ctx context.Context,
	statusFilter []string,
	ordersBatchSize int,
	transaction *sql.Tx,
) (map[string]models.Order, error) {
	var database querier

	if transaction == nil {
		database = db.database
	} else {
		database = transaction
	}

	if len(statusFilter) == 0 {
		return map[string]models.Order{}, nil
	}

	statusesPlaceholders := make([]string, len(statusFilter))
	for i := range statusesPlaceholders {
		statusesPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	statusesPlaceholdersAsString := strings.Join(statusesPlaceholders, ",")

	rows, err := database.QueryContext(
		ctx,
		fmt.Sprintf(
			`
				SELECT 
				    id,
					status,
					accrual,
					uploaded_at
					FROM orders
					WHERE status IN (%s)
					ORDER BY uploaded_at ASC
					LIMIT %d;
			`,
			statusesPlaceholdersAsString,
			ordersBatchSize,
		),
		func(strSlice []string) []interface{} {
			result := make([]interface{}, len(strSlice))
			for i, v := range strSlice {
				result[i] = v
			}
			return result
		}(statusFilter)...,
	)
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetOrders(): error while `database.QueryContext()` calling: %w",
				err,
			)
	}
	defer rows.Close()

	result := map[string]models.Order{}
	for rows.Next() {
		var number string
		var status string
		var accrual sql.NullFloat64
		var uploadedAt string
		err = rows.Scan(
			&number,
			&status,
			&accrual,
			&uploadedAt,
		)
		if err != nil {
			return nil,
				fmt.Errorf(
					"in internal/db/postgresdb/postgresdb.go/GetOrders(): error while `rows.Scan()` calling: %w",
					err,
				)
		}

		accrualValue := float32(0)
		if accrual.Valid {
			accrualValue = float32(accrual.Float64)
		}

		result[number] = models.Order{
			Number:     number,
			Status:     status,
			Accrual:    &accrualValue,
			UploadedAt: uploadedAt,
		}
	}

	err = rows.Err()
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetOrders(): error while `rows.Err()` calling: %w",
				err,
			)
	}

	return result, nil
}

func (db *PostgresDB) BeginTransaction() (*sql.Tx, error) {
	return db.database.Begin()
}

func (db *PostgresDB) GetUserWithdrawals(
	ctx context.Context,
	userID string,
) ([]models.UserWithdrawal, error) {
	rows, err := db.database.QueryContext(
		ctx,
		`
			SELECT
				withdrawals.order_number,
				withdrawals.sum,
				withdrawals.processed_at
				FROM withdrawals
					JOIN users_withdrawals ON 
						users_withdrawals.withdraw_order_number = withdrawals.order_number
							AND users_withdrawals.user_id = $1
				ORDER BY withdrawals.processed_at DESC;
		`,
		userID,
	)
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserWithdrawals(): error while `db.database.QueryContext()` calling: %w",
				err,
			)
	}
	defer rows.Close()

	result := []models.UserWithdrawal{}
	for rows.Next() {
		var orderNumber string
		var sum float32
		var processedAt string
		err = rows.Scan(
			&orderNumber,
			&sum,
			&processedAt,
		)
		if err != nil {
			return nil,
				fmt.Errorf(
					"in internal/db/postgresdb/postgresdb.go/GetUserWithdrawals(): error while `rows.Scan()` calling: %w",
					err,
				)
		}

		result = append(
			result,
			models.UserWithdrawal{
				OrderNumber: orderNumber,
				Sum:         sum,
				ProcessedAt: processedAt,
			},
		)
	}

	err = rows.Err()
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserWithdrawals(): error while `rows.Err()` calling: %w",
				err,
			)
	}

	return result, nil
}

func (db *PostgresDB) Withdraw(
	ctx context.Context,
	userID string,
	orderNumber string,
	withdrawSum float32,
) error {
	transaction, err := db.database.Begin()
	if err != nil {
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `db.database.Begin()` calling: %w",
			err,
		)
	}

	var loyaltyBalance float32
	err = transaction.QueryRowContext(
		ctx,
		`SELECT loyalty_balance FROM users WHERE id = $1;`,
		userID,
	).Scan(&loyaltyBalance)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.QueryRowContext()` calling: %w",
			err,
		)
	}

	if loyaltyBalance < withdrawSum {
		err = transaction.Rollback()
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err,
			)
		}
		return models.ErrNotEnoughBalance
	}

	var orderNumberFromDB string
	err = transaction.QueryRowContext(
		ctx,
		`
			INSERT INTO withdrawals (order_number, sum)
				VALUES ($1, $2)
				ON CONFLICT (order_number) DO NOTHING
				RETURNING order_number;
		`,
		orderNumber,
		withdrawSum,
	).Scan(&orderNumberFromDB)

	if errors.Is(err, sql.ErrNoRows) {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return models.ErrAlreadyWithdrawn
	}

	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.QueryRowContext()` calling: %w",
			err,
		)
	}

	_, err = transaction.ExecContext(
		ctx,
		`
			INSERT INTO users_withdrawals(user_id, withdraw_order_number)
				VALUES ($1, $2);
		`,
		userID,
		orderNumber,
	)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.ExecContext()` calling: %w",
			err,
		)
	}

	loyaltyBalance -= withdrawSum

	_, err = transaction.ExecContext(
		ctx,
		`UPDATE users SET loyalty_balance = $1 WHERE id = $2;`,
		loyaltyBalance,
		userID,
	)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.ExecContext()` calling: %w",
			err,
		)
	}

	err = transaction.Commit()
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Withdraw(): error while `transaction.Commit()` calling: %w",
			err,
		)
	}

	return nil
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
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserBalanceAndWithdrawals(): error while `row.Scan()` calling: %w",
				err,
			)
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
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserOrders(): error while `db.database.QueryContext()` calling: %w",
				err,
			)
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
			return nil,
				fmt.Errorf(
					"in internal/db/postgresdb/postgresdb.go/GetUserOrders(): error while `rows.Scan()` calling: %w",
					err,
				)
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
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserOrders(): error while `rows.Err()` calling: %w",
				err,
			)
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
		return "",
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/SaveNewOrderForUser(): error while `db.database.QueryRowContext()` calling: %w",
				err,
			)
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
		return &models.User{},
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserByID(): error while `row.Scan()` calling: %w",
				err,
			)
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
		return "",
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/GetUserIDByLoginAndPassword(): error while `db.database.QueryRowContext()` calling: %w",
				err,
			)
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
		return "",
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/CreateUser(): error while `db.database.QueryRowContext()` calling: %w",
				err,
			)
	}

	return userID, nil
}

func New(databaseDSN string, migrationsDir string) (*PostgresDB, error) {
	database, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/New(): error while `sql.Open()` calling: %w",
				err,
			)
	}

	result := &PostgresDB{
		database: database,
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/New(): error while `goose.SetDialect()` calling: %w",
				err,
			)
	}

	if err := goose.Up(result.database, migrationsDir); err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/postgresdb.go/New(): error while `goose.Up()` calling: %w",
				err,
			)
	}

	return result, nil
}

func (db *PostgresDB) Close() error {
	err := db.database.Close()
	if err != nil {
		return fmt.Errorf(
			"in internal/db/postgresdb/postgresdb.go/Close(): error while `db.database.Close()` calling: %w",
			err,
		)
	}

	return nil
}
