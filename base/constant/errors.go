package constant

import (
	"errors"
)

var (
	ErrAuthFailed     = errors.New("auth failed")
	ErrNoPermission   = errors.New("access forbidden")
	ErrLoginFailed    = errors.New("account or password does not match	")
	ErrUserNotFound   = errors.New("user not found")
	ErrPasswordWrong  = errors.New("password incorrect")
	ErrRefreshExpired = errors.New("refresh token expired or revoked")
	ErrRefreshRevoked = errors.New("token revoked (password changed)")
	ErrRefreshInvalid = errors.New("invalid refresh token")
)
