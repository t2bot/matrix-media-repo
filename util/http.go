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

func GetXMatrixAuth(headers []string) ([]XMatrixAuth, error) {
	auths := make([]XMatrixAuth, 0)
	for _, h := range headers {
		if !strings.HasPrefix(h, "X-Matrix ") {
			continue
		}

		paramCsv := h[len("X-Matrix "):]
		params := make(map[string]string)

		pairs := strings.Split(paramCsv, ",")
		for _, pair := range pairs {
			csv := strings.SplitN(pair, "=", 2)
			if len(csv) != 2 {
				return nil, fmt.Errorf("invalid auth param pair: %s", pair)
			}

			key := strings.TrimSpace(strings.ToLower(csv[0]))
			value := strings.Trim(strings.TrimSpace(csv[1]), "\"")
			if _, ok := params[key]; ok {
				return nil, fmt.Errorf("duplicate auth param: %s", key)
			}

			params[key] = value
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
