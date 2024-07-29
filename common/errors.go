package common

import (
	"errors"
)

var ErrMediaNotFound = errors.New("media not found")
var ErrMediaTooLarge = errors.New("media too large")
var ErrInvalidHost = errors.New("invalid host")
var ErrHostNotFound = errors.New("host not found")
var ErrHostNotAllowed = errors.New("host not allowed")
var ErrMediaQuarantined = errors.New("media quarantined")
var ErrQuotaExceeded = errors.New("quota exceeded")
var ErrWrongUser = errors.New("wrong user")
var ErrExpired = errors.New("expired")
var ErrAlreadyUploaded = errors.New("already uploaded")
var ErrMediaNotYetUploaded = errors.New("media not yet uploaded")
var ErrMediaDimensionsTooSmall = errors.New("media is too small dimensionally")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")
var ErrRestrictedAuth = errors.New("authentication is required to download this media")
