package api

import "github.com/turt2live/matrix-media-repo/common"

type EmptyResponse struct{}

type DoNotCacheResponse struct {
	Payload interface{}
}

type HtmlResponse struct {
	HTML string
}

type ErrorResponse struct {
	Code         string `json:"errcode"`
	Message      string `json:"error"`
	InternalCode string `json:"mr_errcode"`
}

func InternalServerError(message string) *ErrorResponse {
	return &ErrorResponse{common.ErrCodeUnknown, message, common.ErrCodeUnknown}
}

func MethodNotAllowed() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeUnknown, "Method Not Allowed", common.ErrCodeMethodNotAllowed}
}

func RateLimitReached() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeRateLimitExceeded, "Rate Limited", common.ErrCodeRateLimitExceeded}
}

func NotFoundError() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeNotFound, "Not found", common.ErrCodeNotFound}
}

func RequestTooLarge() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeTooLarge, "Too Large", common.ErrCodeMediaTooLarge}
}

func RequestTooSmall() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeUnknown, "Body too small or not provided", common.ErrCodeMediaTooSmall}
}

func AuthFailed() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeUnknownToken, "Authentication Failed", common.ErrCodeUnknownToken}
}

func BadRequest(message string) *ErrorResponse {
	return &ErrorResponse{common.ErrCodeUnknown, message, common.ErrCodeBadRequest}
}

func QuotaExceeded() *ErrorResponse {
	return &ErrorResponse{common.ErrCodeForbidden, "Quota Exceeded", common.ErrCodeQuotaExceeded}
}
