package models

import "errors"

type UserRegisterRequest struct {
	Login string `json:"login" validate:"required,alphanum"`
	Pass  string `json:"password" validate:"required,min=8,max=30,password"`
}

type User struct {
	Login          string
	Pass           string
	LoyaltyBalance float32
}

var ErrUserAlreadyExists = errors.New("user already exists")
