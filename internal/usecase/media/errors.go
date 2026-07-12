package media

import "errors"

var ErrMediaValidationFailed = errors.New("media validation failed")

var ErrMediaUploadExpired = errors.New("media upload expired")
