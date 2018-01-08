package client

type ErrorResponse struct {
	Code         string `json:"errcode"`
	Message      string `json:"error"`
	InternalCode string `json:"mr_errcode"`
}

func InternalServerError(message string) *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", message, "M_UNKNOWN"}
}

func MethodNotAllowed() *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", "Method Not Allowed", "M_METHOD_NOT_ALLOWED"}
}

func RateLimitReached() *ErrorResponse {
	return &ErrorResponse{"M_LIMIT_EXCEEDED", "Rate Limited", "M_LIMIT_EXCEEDED"}
}

func NotFoundError() *ErrorResponse {
	return &ErrorResponse{"M_NOT_FOUND", "Not found", "M_NOT_FOUND"}
}

func RequestTooLarge() *ErrorResponse {
	return &ErrorResponse{"M_TOO_LARGE", "Too Large", "M_MEDIA_TOO_LARGE"}
}

func AuthFailed() *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN_TOKEN", "Authentication Failed", "M_UNKNOWN_TOKEN"}
}

func BadRequest(message string) *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", message, "M_BAD_REQUEST"}
}
