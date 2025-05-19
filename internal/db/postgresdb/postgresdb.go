package postgresdb

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type PostgresDB struct {
	database *sql.DB
}

func (db *PostgresDB) resetDB(ctx context.Context) error {
	_, err := db.database.ExecContext(ctx, resetDBQuery)
	if err != nil {
		return fmt.Errorf(resetDBErr, err)
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

func (db *PostgresDB) RollbackTransaction(transaction *sql.Tx) error {
	return transaction.Rollback()
}

func (db *PostgresDB) BeginTransaction() (*sql.Tx, error) {
	return db.database.Begin()
}

type InitOption func(*initOptions)

type initOptions struct {
	DBPreReset bool
}

func WithDBPreReset(value bool) InitOption {
	return func(options *initOptions) {
		options.DBPreReset = value
	}
}

func New(databaseDSN string, migrationsDir string, optionsProto ...InitOption) (*PostgresDB, error) {
	options := &initOptions{
		DBPreReset: false,
	}
	for _, protoOption := range optionsProto {
		protoOption(options)
	}

	database, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return nil, fmt.Errorf(newErr1, err)
	}

	result := &PostgresDB{
		database: database,
	}

	if options.DBPreReset {
		if err := result.resetDB(context.TODO()); err != nil {
			return nil, fmt.Errorf(newErr4, err)
		}
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, fmt.Errorf(newErr2, err)
	}

	if err := goose.Up(result.database, migrationsDir); err != nil {
		return nil, fmt.Errorf(newErr3, err)
	}

	return result, nil
}

func (db *PostgresDB) Close() error {
	err := db.database.Close()
	if err != nil {
		return fmt.Errorf(closeErr1, err)
	}

	return nil
}
