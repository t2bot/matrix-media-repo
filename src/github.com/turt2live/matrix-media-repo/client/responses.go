package client

type ErrorResponse struct {
	Code string `json:"errcode"`
	Message string `json:"error"`
}

func InternalServerError(message string) *ErrorResponse {
	return &ErrorResponse{"M_UNKNOWN", message}
}

func NotFoundError() *ErrorResponse {
	return &ErrorResponse{"M_NOT_FOUND", "Not found"}
}