package ordersupdater

const (
	updateOrderErr1 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error in data from the previous pipeline operation: %w"

	updateOrderErr2 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.BeginTransaction()` calling: %w"

	updateOrderErr3 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.RollbackTransaction()` calling: %w"

	updateOrderErr4 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.GetOrderByID()` calling: %w"

	updateOrderErr5 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.RollbackTransaction()` calling: %w"

	updateOrderErr6 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.UpdateOrders()` calling: %w"

	updateOrderErr7 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.RollbackTransaction()` calling: %w"

	updateOrderErr8 = "in internal/balancesactualizer/ordersupdater/ordersupdater.go/updateOrder(): error while `u.db.CommitTransaction()` calling: %w"
)
