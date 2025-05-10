package models

import "github.com/patric-chuzhbe/diploma/internal/models"

type APIOrder struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float32 `json:"accrual,omitempty"`
	Err     error
}

type UpdateOrderRes struct {
	models.Order
	Err error
}

type UpdateBalanceRes struct {
	models.Order
	Err error
}
