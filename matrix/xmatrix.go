package matrix

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"github.com/turt2live/matrix-media-repo/util"
	"net/http"
)

var ErrNoXMatrixAuth = errors.New("no X-Matrix auth headers")

func ValidateXMatrixAuth(request *http.Request, expectNoContent bool) (string, error) {
	if !expectNoContent {
		panic("development error: X-Matrix auth validation can only be done with an empty body for now")
	}

	auths, err := util.GetXMatrixAuth(request)
	if err != nil {
		return "", err
	}

	if len(auths) == 0 {
		return "", ErrNoXMatrixAuth
	}

	obj := map[string]interface{}{
		"method":      request.Method,
		"uri":         request.RequestURI,
		"origin":      auths[0].Origin,
		"destination": auths[0].Destination,
		"content":     "{}",
	}
	canonical, err := util.EncodeCanonicalJson(obj)
	if err != nil {
		return "", err
	}

	keys, err := QuerySigningKeys(auths[0].Origin)
	if err != nil {
		return "", err
	}

	for _, h := range auths {
		if h.Origin != obj["origin"] {
			return "", errors.New("auth is from multiple servers")
		}
		if h.Destination != obj["destination"] {
			return "", errors.New("auth is for multiple servers")
		}
		if h.Destination != "" && !util.IsServerOurs(h.Destination) {
			return "", errors.New("unknown destination")
		}

		if key, ok := (*keys)[h.KeyId]; ok {
			if !ed25519.Verify(key, canonical, h.Signature) {
				return "", fmt.Errorf("failed signatures on '%s'", h.KeyId)
			}
		} else {
			return "", fmt.Errorf("unknown key '%s'", h.KeyId)
		}
	}

	return auths[0].Origin, nil
}
