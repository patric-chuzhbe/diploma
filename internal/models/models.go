package models

import "errors"

type UserRegisterRequest struct {
	Login string `json:"login" validate:"required,alphanum"`
	Pass  string `json:"password" validate:"required,min=8,max=40,password"`
}

type User struct {
	ID             string
	Login          string
	Pass           string
	LoyaltyBalance float32
}

var ErrUserAlreadyExists = errors.New("user already exists")

var ErrOrderAlreadyExists = errors.New("order already exists")

type Order struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float32 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

type UserBalanceAndWithdrawals struct {
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}

type BalanceWithdrawRequest struct {
	OrderNumber string  `json:"order"`
	WithdrawSum float32 `json:"sum"`
}

var ErrNotEnoughBalance = errors.New("not enough loyalty balance")

var ErrAlreadyWithdrawn = errors.New("the order has already withdrawn")

type UserWithdrawal struct {
	OrderNumber string  `json:"order"`
	Sum         float32 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}
