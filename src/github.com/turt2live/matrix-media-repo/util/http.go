package util

import (
	"net/http"
)

func GetAccessTokenFromRequest(request *http.Request) (string) {
	token := request.Header.Get("Authorization")
	if token != "" && len(token) > 7 {
		// "Bearer <token>"
		return token[7:]
	}

	return request.URL.Query().Get("access_token")
}