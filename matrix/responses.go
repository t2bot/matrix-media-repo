package matrix

import (
	"fmt"
)

type emptyResponse struct {
}

type userIdResponse struct {
	UserId string `json:"user_id"`
}

type whoisResponse struct {
	// We don't actually care about any of the fields here
}

type mediaListResponse struct {
	LocalMxcs  []string `json:"local"`
	RemoteMxcs []string `json:"remote"`
}

type wellknownServerResponse struct {
	ServerAddr string `json:"m.server"`
}

type errorResponse struct {
	ErrorCode string `json:"errcode"`
	Message   string `json:"error"`
}

func (e errorResponse) Error() string {
	return fmt.Sprintf("code=%s message=%s", e.ErrorCode, e.Message)
}
