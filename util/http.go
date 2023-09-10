package util

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type XMatrixAuth struct {
	Origin      string
	Destination string
	KeyId       string
	Signature   []byte
}

func GetAccessTokenFromRequest(request *http.Request) string {
	token := request.Header.Get("Authorization")

	if token != "" {
		if !strings.HasPrefix(token, "Bearer ") { // including space
			// It's probably an X-Matrix authentication header (federation)
			//logrus.Warn("Invalid Authorization header observed: expected a Bearer token, got something else")
			return ""
		}
		return token[len("Bearer "):] // including space
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

func GetXMatrixAuth(request *http.Request) ([]XMatrixAuth, error) {
	headers := request.Header.Values("Authorization")
	auths := make([]XMatrixAuth, 0)
	for _, h := range headers {
		if !strings.HasPrefix(h, "X-Matrix ") {
			continue
		}

		paramCsv := h[len("X-Matrix "):]
		params := make(map[string]string)
		isKey := true
		keyName := ""
		keyValue := ""
		escape := false
		for _, c := range paramCsv {
			if c == ',' && isKey {
				params[strings.TrimSpace(strings.ToLower(keyName))] = keyValue
				keyName = ""
				keyValue = ""
				continue
			}
			if c == '=' {
				isKey = false
				continue
			}

			if isKey {
				keyName = fmt.Sprintf("%s%s", keyName, string(c))
			} else {
				if c == '\\' && !escape {
					escape = true
					continue
				}
				if c == '"' && !escape {
					escape = false
					if len(keyValue) > 0 {
						isKey = true
					}
					continue
				}
				if escape {
					escape = false
				}
				keyValue = fmt.Sprintf("%s%s", keyValue, string(c))
			}
		}
		if len(keyName) > 0 && isKey {
			params[strings.TrimSpace(strings.ToLower(keyName))] = keyValue
		}

		sig, err := DecodeUnpaddedBase64String(params["sig"])
		if err != nil {
			return nil, err
		}
		auth := XMatrixAuth{
			Origin:      params["origin"],
			Destination: params["destination"],
			KeyId:       params["key"],
			Signature:   sig,
		}
		if auth.Origin == "" || auth.KeyId == "" || len(auth.Signature) == 0 {
			continue
		}
		auths = append(auths, auth)
	}

	return auths, nil
}
