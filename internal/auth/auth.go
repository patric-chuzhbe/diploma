package auth

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"github.com/patric-chuzhbe/diploma/internal/models"
	"net/http"
)

type userKeeper interface {
	CreateUser(ctx context.Context, usr *models.User) (string, error)
}

type Auth struct {
	db                         userKeeper
	authCookieName             string
	authCookieSigningSecretKey []byte
}

type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

func New(
	db userKeeper,
	authCookieName string,
	authCookieSigningSecretKey []byte,
) *Auth {
	return &Auth{
		db:                         db,
		authCookieName:             authCookieName,
		authCookieSigningSecretKey: authCookieSigningSecretKey,
	}
}

func (a *Auth) buildJWTString(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, *claims)

	tokenString, err := token.SignedString(a.authCookieSigningSecretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (a *Auth) SetAuthData(userID string, response http.ResponseWriter) error {
	JWTString, err := a.buildJWTString(&Claims{UserID: userID})
	if err != nil {
		return err
	}

	response.Header().Set("Authorization", JWTString)

	http.SetCookie(
		response,
		&http.Cookie{
			Name:  a.authCookieName,
			Value: JWTString,
		},
	)

	return nil
}
