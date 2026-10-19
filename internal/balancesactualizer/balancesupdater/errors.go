package balancesupdater

const (
	updateBalanceErr1 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.BeginTransaction()` calling: %w"

	updateBalanceErr2 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.RollbackTransaction()` calling: %w"

	updateBalanceErr3 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.GetUsersByOrders()` calling: %w"

	updateBalanceErr4 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): " +
		"rolled back the order no. %s state to `NEW` " +
		"because of error in data from the previous pipeline operation: %w"

	updateBalanceErr5 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.UpdateUsers()` calling: %w"

	updateBalanceErr6 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.UpdateOrders()` calling: %w"

	updateBalanceErr7 = "in internal/balancesactualizer/balancesupdater/balancesupdater.go/updateBalance(): error while `u.db.CommitTransaction()` calling: %w"
)
