package matrix

import (
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common"
)

type ErrorResponse struct {
	ErrorCode string `json:"errcode"`
	Message   string `json:"error"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("code=%s message=%s", e.ErrorCode, e.Message)
}

func filterError(err error) (error, error) {
	if err == nil {
		return nil, nil
	}

	// Unknown token errors should be filtered out explicitly to ensure we don't break on bad requests
	var httpErr *ErrorResponse
	if errors.As(err, &httpErr) {
		// We send back our own version of errors to ensure we can filter them out elsewhere
		if httpErr.ErrorCode == common.ErrCodeUnknownToken {
			return nil, ErrInvalidToken
		} else if httpErr.ErrorCode == common.ErrCodeNoGuests {
			return nil, ErrGuestToken
		}
	}

	return err, err
}

type ServerNotAllowedError struct {
	error
	ServerName string
}

func MakeServerNotAllowedError(serverName string) ServerNotAllowedError {
	return ServerNotAllowedError{
		error:      errors.New("server " + serverName + " is not allowed"),
		ServerName: serverName,
	}
}
