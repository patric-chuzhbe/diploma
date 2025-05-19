package postgresdb

const (
	updateOrdersQuery = `
		UPDATE orders
			SET
				status = $1,
				accrual = $2
			WHERE id = $3;
	`

	updateUsersQuery = `
		UPDATE users 
			SET 
				login = $1,
				pass = $2,
				loyalty_balance = $3
			WHERE id = $4
	`

	getUsersByOrdersQuery = `
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
	`

	getOrdersQuery = `
		SELECT
			id,
			status,
			accrual,
			uploaded_at
			FROM orders
			WHERE status IN (%s)
			ORDER BY uploaded_at ASC
			LIMIT %d;
	`

	getUserWithdrawalsQuery = `
		SELECT
			withdrawals.order_number,
			withdrawals.sum,
			withdrawals.processed_at
			FROM withdrawals
				JOIN users_withdrawals ON 
					users_withdrawals.withdraw_order_number = withdrawals.order_number
						AND users_withdrawals.user_id = $1
			ORDER BY withdrawals.processed_at DESC;
	`

	selectUserBalanceQuery = `SELECT loyalty_balance FROM users WHERE id = $1;`

	insertWithdrawalQuery = `
		INSERT INTO withdrawals (order_number, sum)
			VALUES ($1, $2)
			ON CONFLICT (order_number) DO NOTHING
			RETURNING order_number;
	`

	insertUserWithdrawalQuery = `
		INSERT INTO users_withdrawals(user_id, withdraw_order_number)
			VALUES ($1, $2);
	`

	updateUserBalanceQuery = `UPDATE users SET loyalty_balance = $1 WHERE id = $2;`

	getUserBalanceAndWithdrawalsQuery = `
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
	`

	getUserOrdersQuery = `
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
	`

	saveNewOrderForUserQuery = `
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

	getUserByIDQuery = `SELECT id, login, pass, loyalty_balance FROM users WHERE id = $1`

	getUserIDByLoginAndPasswordQuery = `SELECT id FROM users WHERE login = $1 AND pass = $2`

	createUserQuery = `
		INSERT INTO users (login, pass, loyalty_balance)
		VALUES ($1, $2, $3)
		ON CONFLICT (login) DO NOTHING
		RETURNING id;
	`

	getOrderByIDQuery = `
		SELECT 
			id,
			status,
			uploaded_at,
			accrual
			FROM orders 
			WHERE id = $1;
	`

	getOrdersAndUpdateStatusQuery = `
		WITH ordersForUpdate AS (
			SELECT id
				FROM orders
				WHERE status IN (%s)
				ORDER BY uploaded_at ASC
				LIMIT %d
				FOR UPDATE SKIP LOCKED
		)
		UPDATE orders
			SET status = $%d
			FROM ordersForUpdate
			WHERE orders.id = ordersForUpdate.id
			RETURNING
				orders.id,
				orders.status,
				orders.accrual,
				orders.uploaded_at;
	`

	resetDBQuery = `
		DO $$
		DECLARE
			r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
		END $$;
	`
)
