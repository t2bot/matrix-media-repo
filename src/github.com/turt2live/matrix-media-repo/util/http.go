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

func GetLogSafeQueryString(r *http.Request) (string) {
	qs := r.URL.Query()

	if qs.Get("access_token") != "" {
		qs.Set("access_token", "redacted")
	}

	return qs.Encode()
}
