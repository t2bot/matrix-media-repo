package common

import (
	"errors"
)

var ErrMediaNotFound = errors.New("media not found")
var ErrMediaTooLarge = errors.New("media too large")
var ErrInvalidHost = errors.New("invalid host")
var ErrHostNotFound = errors.New("host not found")
var ErrHostBlacklisted = errors.New("host not allowed")
var ErrMediaQuarantined = errors.New("media quarantined")
var ErrCannotOverwriteMedia = errors.New("cannot overwrite media")
var ErrNotYetUploaded = errors.New("not yet uploaded")
