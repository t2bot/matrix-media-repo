package client

type ErrorResponse struct {
	Code string `json:"errcode"`
	Message string `json:"error"`
	InternalCode string `json:"mr_errcode"`
}

func InternalServerError(message string) *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", message, "M_UNKNOWN"}
}

func NotFoundError() *ErrorResponse {
	return &ErrorResponse{"M_NOT_FOUND", "Not found", "M_NOT_FOUND"}
}

func RequestTooLarge() *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", "Too Large", "M_MEDIA_TOO_LARGE"}
}

func AuthFailed() *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN_TOKEN", "Authentication Failed", "M_UNKNOWN_TOKEN"}
}