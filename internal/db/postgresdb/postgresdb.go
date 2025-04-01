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
