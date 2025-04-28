package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/models"
)

func (db *PostgresDB) GetUserBalanceAndWithdrawals(
	ctx context.Context,
	userID string,
) (*models.UserBalanceAndWithdrawals, error) {
	row := db.database.QueryRowContext(ctx, getUserBalanceAndWithdrawalsQuery, userID)
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
				"in internal/db/postgresdb/balances.go/GetUserBalanceAndWithdrawals(): error while `row.Scan()` calling: %w",
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

func (db *PostgresDB) Withdraw(
	ctx context.Context,
	userID string,
	orderNumber string,
	withdrawSum float32,
) error {
	transaction, err := db.database.Begin()
	if err != nil {
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `db.database.Begin()` calling: %w",
			err,
		)
	}

	var loyaltyBalance float32
	err = transaction.QueryRowContext(ctx, selectUserBalanceQuery, userID).Scan(&loyaltyBalance)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.QueryRowContext()` calling: %w",
			err,
		)
	}

	if loyaltyBalance < withdrawSum {
		err = transaction.Rollback()
		if err != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err,
			)
		}
		return models.ErrNotEnoughBalance
	}

	var orderNumberFromDB string
	err = transaction.QueryRowContext(
		ctx,
		insertWithdrawalQuery,
		orderNumber,
		withdrawSum,
	).Scan(&orderNumberFromDB)

	if errors.Is(err, sql.ErrNoRows) {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return models.ErrAlreadyWithdrawn
	}

	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.QueryRowContext()` calling: %w",
			err,
		)
	}

	_, err = transaction.ExecContext(
		ctx,
		insertUserWithdrawalQuery,
		userID,
		orderNumber,
	)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.ExecContext()` calling: %w",
			err,
		)
	}

	loyaltyBalance -= withdrawSum

	_, err = transaction.ExecContext(
		ctx,
		updateUserBalanceQuery,
		loyaltyBalance,
		userID,
	)
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.ExecContext()` calling: %w",
			err,
		)
	}

	err = transaction.Commit()
	if err != nil {
		err2 := transaction.Rollback()
		if err2 != nil {
			return fmt.Errorf(
				"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w",
				err2,
			)
		}
		return fmt.Errorf(
			"in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Commit()` calling: %w",
			err,
		)
	}

	return nil
}

func (db *PostgresDB) GetUserWithdrawals(
	ctx context.Context,
	userID string,
) ([]models.UserWithdrawal, error) {
	rows, err := db.database.QueryContext(ctx, getUserWithdrawalsQuery, userID)
	if err != nil {
		return nil,
			fmt.Errorf(
				"in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `db.database.QueryContext()` calling: %w",
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
					"in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `rows.Scan()` calling: %w",
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
				"in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `rows.Err()` calling: %w",
				err,
			)
	}

	return result, nil
}
