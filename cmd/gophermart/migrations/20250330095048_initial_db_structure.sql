-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders
(
    id          VARCHAR(255) NOT NULL,
    status      VARCHAR(255) NOT NULL
        CONSTRAINT CKC_STATUS_ORDERS CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    uploaded_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accrual     REAL NULL,
    CONSTRAINT PK_ORDERS PRIMARY KEY (id)
);

CREATE TABLE users
(
    id              UUID         NOT NULL DEFAULT gen_random_uuid(),
    login           VARCHAR(255) NOT NULL,
    pass            VARCHAR(255) NOT NULL,
    loyalty_balance REAL NULL DEFAULT 0,
    CONSTRAINT PK_USERS PRIMARY KEY (id)
);

CREATE UNIQUE INDEX Index_1 ON users (login);

CREATE TABLE users_orders
(
    user_id  UUID         NOT NULL DEFAULT gen_random_uuid(),
    order_id VARCHAR(255) NOT NULL,
    CONSTRAINT PK_USERS_ORDERS PRIMARY KEY (order_id)
);

CREATE TABLE users_withdrawals
(
    user_id               UUID NULL DEFAULT gen_random_uuid(),
    withdraw_order_number VARCHAR(255) NOT NULL,
    CONSTRAINT PK_USERS_WITHDRAWALS PRIMARY KEY (withdraw_order_number)
);

CREATE TABLE withdrawals
(
    order_number VARCHAR(255) NOT NULL,
    sum          REAL         NOT NULL DEFAULT 0,
    processed_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT PK_WITHDRAWALS PRIMARY KEY (order_number)
);

ALTER TABLE users_orders
    ADD CONSTRAINT FK_USERS_OR_REFERENCE_USERS FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE users_orders
    ADD CONSTRAINT FK_USERS_OR_REFERENCE_ORDERS FOREIGN KEY (order_id)
        REFERENCES orders (id)
        ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE users_withdrawals
    ADD CONSTRAINT FK_USERS_WI_REFERENCE_USERS FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE users_withdrawals
    ADD CONSTRAINT FK_USERS_WI_REFERENCE_WITHDRAW FOREIGN KEY (withdraw_order_number)
        REFERENCES withdrawals (order_number)
        ON DELETE CASCADE ON UPDATE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users_withdrawals DROP CONSTRAINT FK_USERS_WI_REFERENCE_WITHDRAW;
ALTER TABLE users_withdrawals DROP CONSTRAINT FK_USERS_WI_REFERENCE_USERS;
ALTER TABLE users_orders DROP CONSTRAINT FK_USERS_OR_REFERENCE_ORDERS;
ALTER TABLE users_orders DROP CONSTRAINT FK_USERS_OR_REFERENCE_USERS;

DROP TABLE withdrawals;
DROP TABLE users_withdrawals;
DROP TABLE users_orders;
DROP TABLE users;
DROP TABLE orders;
-- +goose StatementEnd
