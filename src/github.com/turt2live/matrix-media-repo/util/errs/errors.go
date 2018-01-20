package errs

import (
	"errors"
)

var ErrMediaNotFound = errors.New("media not found")
var ErrMediaTooLarge = errors.New("media too large")
var ErrInvalidHost = errors.New("invalid host")
var ErrHostNotFound = errors.New("host not found")
var ErrHostBlacklisted = errors.New("host not allowed")
var ErrMediaNotAllowed = errors.New("media content type not allowed")
