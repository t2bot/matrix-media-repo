package matrix

import (
	"errors"
	"fmt"

	"github.com/turt2live/matrix-media-repo/common"
)

type errorResponse struct {
	ErrorCode string `json:"errcode"`
	Message   string `json:"error"`
}

func (e errorResponse) Error() string {
	return fmt.Sprintf("code=%s message=%s", e.ErrorCode, e.Message)
}

func filterError(err error) (error, error) {
	if err == nil {
		return nil, nil
	}

	// Unknown token errors should be filtered out explicitly to ensure we don't break on bad requests
	var httpErr *errorResponse
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
