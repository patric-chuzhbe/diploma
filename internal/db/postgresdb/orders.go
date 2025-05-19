package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"strings"
)

func (db *PostgresDB) GetOrderByID(
	ctx context.Context,
	ID string,
	transaction *sql.Tx,
) (*models.Order, error) {
	if ID == "" {
		return &models.Order{}, nil
	}

	row := transaction.QueryRowContext(ctx, getOrderByIDQuery, ID)
	var IDFromDB string
	var status string
	var uploadedAt string
	var accrual sql.NullFloat64
	err := row.Scan(
		&IDFromDB,
		&status,
		&uploadedAt,
		&accrual,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return &models.Order{}, nil
	}

	if err != nil {
		return &models.Order{}, fmt.Errorf(getOrderByIDErr1, err)
	}

	accrualValue := float32(0)
	if accrual.Valid {
		accrualValue = float32(accrual.Float64)
	}

	return &models.Order{
		Number:     IDFromDB,
		Status:     status,
		Accrual:    &accrualValue,
		UploadedAt: uploadedAt,
	}, nil
}

func (db *PostgresDB) GetOrdersAndUpdateStatus(
	ctx context.Context,
	statusFilter []string,
	newStatus string,
	ordersBatchSize int,
) (map[string]models.Order, error) {
	if len(statusFilter) == 0 {
		return map[string]models.Order{}, nil
	}

	statusesPlaceholders := make([]string, len(statusFilter))
	for i := range statusesPlaceholders {
		statusesPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	statusesPlaceholdersAsString := strings.Join(statusesPlaceholders, ",")

	rows, err := db.database.QueryContext(
		ctx,
		fmt.Sprintf(
			getOrdersAndUpdateStatusQuery,
			statusesPlaceholdersAsString,
			ordersBatchSize,
			len(statusFilter)+1,
		),
		append(toInterfaceSlice(statusFilter), newStatus)...,
	)
	if err != nil {
		return nil, fmt.Errorf(getOrdersAndUpdateStatusErr1, err)
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
			return nil, fmt.Errorf(getOrdersAndUpdateStatusErr2, err)
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
		return nil, fmt.Errorf(getOrdersAndUpdateStatusErr3, err)
	}

	return result, nil
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
			return fmt.Errorf(updateOrdersErr1, err)
		}
	}

	for _, order := range orders {
		_, err := innerTransaction.ExecContext(
			ctx,
			updateOrdersQuery,
			order.Status,
			*order.Accrual,
			order.Number,
		)
		if err != nil {
			if outerTransaction == nil {
				err2 := db.RollbackTransaction(innerTransaction)
				if err2 != nil {
					return fmt.Errorf(updateOrdersErr2, err2)
				}
			}
			return fmt.Errorf(updateOrdersErr3, err)
		}
	}

	if outerTransaction == nil {
		err := db.CommitTransaction(innerTransaction)
		if err != nil {
			return fmt.Errorf(updateOrdersErr4, err)
		}
	}

	return nil
}

func (db *PostgresDB) GetOrders(
	ctx context.Context,
	statusFilter []string,
	ordersBatchSize int,
	transaction *sql.Tx,
) (map[string]models.Order, error) {
	if len(statusFilter) == 0 {
		return map[string]models.Order{}, nil
	}

	statusesPlaceholders := make([]string, len(statusFilter))
	for i := range statusesPlaceholders {
		statusesPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	statusesPlaceholdersAsString := strings.Join(statusesPlaceholders, ",")

	rows, err := transaction.QueryContext(
		ctx,
		fmt.Sprintf(
			getOrdersQuery,
			statusesPlaceholdersAsString,
			ordersBatchSize,
		),
		toInterfaceSlice(statusFilter)...,
	)
	if err != nil {
		return nil,
			fmt.Errorf(getOrdersErr1, err)
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
				fmt.Errorf(getOrdersErr2, err)
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
		return nil, fmt.Errorf(getOrdersErr3, err)
	}

	return result, nil
}

func (db *PostgresDB) SaveNewOrderForUser(
	ctx context.Context,
	userID string,
	orderNumber string,
) (string, error) {
	var actualUserID string
	err := db.database.QueryRowContext(ctx, saveNewOrderForUserQuery, orderNumber, userID).Scan(&actualUserID)

	if errors.Is(err, sql.ErrNoRows) {
		// Successfully inserted and linked order
		return userID, nil
	}

	if err != nil {
		return "", fmt.Errorf(saveNewOrderForUserErr1, err)
	}

	return actualUserID, models.ErrOrderAlreadyExists
}

func (db *PostgresDB) GetUserOrders(
	ctx context.Context,
	userID string,
) ([]models.Order, error) {
	rows, err := db.database.QueryContext(ctx, getUserOrdersQuery, userID)
	if err != nil {
		return nil, fmt.Errorf(getUserOrdersErr1, err)
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
			return nil, fmt.Errorf(getUserOrdersErr2, err)
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
		return nil, fmt.Errorf(getUserOrdersErr3, err)
	}

	return result, nil
}
