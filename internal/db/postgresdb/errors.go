package postgresdb

const (
	getUserBalanceAndWithdrawalsErr1 = "in internal/db/postgresdb/balances.go/GetUserBalanceAndWithdrawals(): error while `row.Scan()` calling: %w"

	withdrawErr1 = "in internal/db/postgresdb/balances.go/Withdraw(): error while `db.database.Begin()` calling: %w"

	withdrawErr2 = "in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Rollback()` calling: %w"

	withdrawErr3 = "in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.QueryRowContext()` calling: %w"

	withdrawErr4 = "in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.ExecContext()` calling: %w"

	withdrawErr5 = "in internal/db/postgresdb/balances.go/Withdraw(): error while `transaction.Commit()` calling: %w"

	getUserWithdrawalsErr1 = "in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `db.database.QueryContext()` calling: %w"

	getUserWithdrawalsErr2 = "in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `rows.Scan()` calling: %w"

	getUserWithdrawalsErr3 = "in internal/db/postgresdb/balances.go/GetUserWithdrawals(): error while `rows.Err()` calling: %w"

	getOrderByIDErr1 = "in internal/db/postgresdb/orders.go/GetOrderByID(): error while `row.Scan()` calling: %w"

	getOrdersAndUpdateStatusErr1 = "in internal/db/postgresdb/orders.go/GetOrdersAndUpdateStatus(): error while `db.database.QueryContext()` calling: %w"

	getOrdersAndUpdateStatusErr2 = "in internal/db/postgresdb/orders.go/GetOrdersAndUpdateStatus(): error while `rows.Scan()` calling: %w"

	getOrdersAndUpdateStatusErr3 = "in internal/db/postgresdb/orders.go/GetOrdersAndUpdateStatus(): error while `rows.Err()` calling: %w"

	updateOrdersErr1 = "in internal/db/postgresdb/orders.go/UpdateOrders(): error while `db.BeginTransaction()` calling: %w"

	updateOrdersErr2 = "in internal/db/postgresdb/orders.go/UpdateOrders(): error while `db.RollbackTransaction()` calling: %w"

	updateOrdersErr3 = "in internal/db/postgresdb/orders.go/UpdateOrders(): error while `innerTransaction.ExecContext()` calling: %w"

	updateOrdersErr4 = "in internal/db/postgresdb/orders.go/UpdateOrders(): error while `db.CommitTransaction()` calling: %w"

	getOrdersErr1 = "in internal/db/postgresdb/orders.go/GetOrders(): error while `transaction.QueryContext()` calling: %w"

	getOrdersErr2 = "in internal/db/postgresdb/orders.go/GetOrders(): error while `rows.Scan()` calling: %w"

	getOrdersErr3 = "in internal/db/postgresdb/orders.go/GetOrders(): error while `rows.Err()` calling: %w"

	saveNewOrderForUserErr1 = "in internal/db/postgresdb/orders.go/SaveNewOrderForUser(): error while `db.database.QueryRowContext()` calling: %w"

	getUserOrdersErr1 = "in internal/db/postgresdb/orders.go/GetUserOrders(): error while `db.database.QueryContext()` calling: %w"

	getUserOrdersErr2 = "in internal/db/postgresdb/orders.go/GetUserOrders(): error while `rows.Scan()` calling: %w"

	getUserOrdersErr3 = "in internal/db/postgresdb/orders.go/GetUserOrders(): error while `rows.Err()` calling: %w"

	newErr1 = "in internal/db/postgresdb/postgresdb.go/New(): error while `sql.Open()` calling: %w"

	newErr2 = "in internal/db/postgresdb/postgresdb.go/New(): error while `goose.SetDialect()` calling: %w"

	newErr3 = "in internal/db/postgresdb/postgresdb.go/New(): error while `goose.Up()` calling: %w"

	closeErr1 = "in internal/db/postgresdb/postgresdb.go/Close(): error while `db.database.Close()` calling: %w"

	updateUsersErr1 = "in internal/db/postgresdb/users.go/UpdateUsers(): error while `db.BeginTransaction()` calling: %w"

	updateUsersErr2 = "in internal/db/postgresdb/users.go/UpdateUsers(): error while `db.RollbackTransaction()` calling: %w"

	updateUsersErr3 = "in internal/db/postgresdb/users.go/UpdateUsers(): error while `innerTransaction.ExecContext()` calling: %w"

	updateUsersErr4 = "in internal/db/postgresdb/users.go/UpdateUsers(): error while `db.CommitTransaction()` calling: %w"

	getUsersByOrdersErr1 = "in internal/db/postgresdb/users.go/GetUsersByOrders(): error while `transaction.QueryContext()` calling: %w"

	getUsersByOrdersErr2 = "in internal/db/postgresdb/users.go/GetUsersByOrders(): error while `rows.Scan()` calling: %w"

	getUsersByOrdersErr3 = "in internal/db/postgresdb/users.go/GetUsersByOrders(): error while `rows.Err()` calling: %w"

	getUserByIDErr1 = "in internal/db/postgresdb/users.go/GetUserByID(): error while `row.Scan()` calling: %w"

	createUserErr1 = "in internal/db/postgresdb/users.go/CreateUser(): error while `db.database.QueryRowContext()` calling: %w"

	getUserIDByLoginAndPasswordErr1 = "in internal/db/postgresdb/users.go/GetUserIDByLoginAndPassword(): error while `db.database.QueryRowContext()` calling: %w"
)
