package identity

import "errors"

var (
	ErrInvalidCaptcha = errors.New("invalid captcha")
	ErrEmailTaken     = errors.New("email already taken")
	ErrUserNotFound   = errors.New("user not found")
)
