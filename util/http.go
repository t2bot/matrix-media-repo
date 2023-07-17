package util

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

func GetAccessTokenFromRequest(request *http.Request) string {
	token := request.Header.Get("Authorization")

	if token != "" {
		if !strings.HasPrefix(token, "Bearer") {
			logrus.Warn("Invalid Authorization header observed: expected a Bearer token, got something else")
			return ""
		}
		if len(token) > 7 {
			// "Bearer <token>"
			return token[7:]
		}
	}

	return request.URL.Query().Get("access_token")
}

func GetAppserviceUserIdFromRequest(request *http.Request) string {
	return request.URL.Query().Get("user_id")
}

func GetLogSafeQueryString(r *http.Request) string {
	qs := r.URL.Query()

	if qs.Get("access_token") != "" {
		qs.Set("access_token", "redacted")
	}

	return qs.Encode()
}

func GetLogSafeUrl(r *http.Request) string {
	copyUrl, _ := url.ParseRequestURI(r.URL.String())
	copyUrl.RawQuery = GetLogSafeQueryString(r)
	return copyUrl.String()
}
