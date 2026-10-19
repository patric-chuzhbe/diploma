package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"strings"
)

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
			return fmt.Errorf(updateUsersErr1, err)
		}
	}

	for _, user := range users {
		_, err := innerTransaction.ExecContext(
			ctx,
			updateUsersQuery,
			user.Login,
			user.Pass,
			user.LoyaltyBalance,
			user.ID,
		)
		if err != nil {
			if outerTransaction == nil {
				err2 := db.RollbackTransaction(innerTransaction)
				if err2 != nil {
					return fmt.Errorf(updateUsersErr2, err2)
				}
			}
			return fmt.Errorf(updateUsersErr3, err)
		}
	}

	if outerTransaction == nil {
		err := db.CommitTransaction(innerTransaction)
		if err != nil {
			return fmt.Errorf(updateUsersErr4, err)
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

	statusesPlaceholders := make([]string, len(orderNumbers))
	for i := range statusesPlaceholders {
		statusesPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	orderNumbersPlaceholdersAsString := strings.Join(statusesPlaceholders, ",")

	rows, err := transaction.QueryContext(
		ctx,
		fmt.Sprintf(
			getUsersByOrdersQuery,
			orderNumbersPlaceholdersAsString,
		),
		toInterfaceSlice(orderNumbers)...,
	)
	if err != nil {
		return []models.User{}, map[string][]string{}, fmt.Errorf(getUsersByOrdersErr1, err)
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
			return []models.User{}, map[string][]string{}, fmt.Errorf(getUsersByOrdersErr2, err)
		}

		users = append(users, models.User{
			ID:             userID,
			Login:          login,
			Pass:           pass,
			LoyaltyBalance: loyaltyBalance,
		})

		usersToOrdersMapping[userID] = append(usersToOrdersMapping[userID], strings.Split(orderIDs, ",")...)
	}

	err = rows.Err()
	if err != nil {
		return []models.User{}, map[string][]string{}, fmt.Errorf(getUsersByOrdersErr3, err)
	}

	return users, usersToOrdersMapping, nil
}

func (db *PostgresDB) GetUserByID(
	ctx context.Context,
	userID string,
) (*models.User, error) {
	if userID == "" {
		return &models.User{}, nil
	}

	row := db.database.QueryRowContext(ctx, getUserByIDQuery, userID)
	var userIDFromDB string
	var login string
	var pass string
	var loyaltyBalance float32
	err := row.Scan(
		&userIDFromDB,
		&login,
		&pass,
		&loyaltyBalance,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return &models.User{}, nil
	}
	if err != nil {
		return &models.User{}, fmt.Errorf(getUserByIDErr1, err)
	}

	return &models.User{
		ID:             userIDFromDB,
		Login:          login,
		Pass:           pass,
		LoyaltyBalance: loyaltyBalance,
	}, nil
}

func (db *PostgresDB) CreateUser(
	ctx context.Context,
	usr *models.User,
) (string, error) {
	var userID string
	err := db.database.QueryRowContext(ctx, createUserQuery, usr.Login, usr.Pass, usr.LoyaltyBalance).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		return "", models.ErrUserAlreadyExists
	}

	if err != nil {
		return "", fmt.Errorf(createUserErr1, err)
	}

	return userID, nil
}

func (db *PostgresDB) GetUserIDByLoginAndPassword(
	ctx context.Context,
	usr *models.User,
) (string, error) {
	var userID string
	err := db.database.QueryRowContext(ctx, getUserIDByLoginAndPasswordQuery, usr.Login, usr.Pass).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf(getUserIDByLoginAndPasswordErr1, err)
	}

	return userID, nil
}
