package matrix

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/t2bot/matrix-media-repo/util"
)

var ErrNoXMatrixAuth = errors.New("no X-Matrix auth headers")
var ErrWrongDestination = errors.New("wrong destination")

func ValidateXMatrixAuth(request *http.Request, expectNoContent bool) (string, error) {
	if !expectNoContent {
		panic("development error: X-Matrix auth validation can only be done with an empty body for now")
	}

	auths, err := util.GetXMatrixAuth(request.Header.Values("Authorization"))
	if err != nil {
		return "", err
	}
	if len(auths) == 0 {
		return "", ErrNoXMatrixAuth
	}

	keys, err := QuerySigningKeys(auths[0].Origin)
	if err != nil {
		return "", err
	}

	uri := request.RequestURI
	if strings.HasSuffix(uri, "?") {
		uri = uri[:len(uri)-1]
	}

	err = ValidateXMatrixAuthHeader(request.Method, uri, nil, auths, keys, request.Host)
	if err != nil {
		return "", err
	}
	return auths[0].Origin, nil
}

func ValidateXMatrixAuthHeader(requestMethod string, requestUri string, content []byte, headers []util.XMatrixAuth, originKeys ServerSigningKeys, destinationHost string) error {
	if len(headers) == 0 {
		return ErrNoXMatrixAuth
	}

	obj := map[string]interface{}{
		"method":      requestMethod,
		"uri":         requestUri,
		"origin":      headers[0].Origin,
		"destination": headers[0].Destination,
	}
	if content != nil {
		obj["content"] = content // we can't modify the content, so don't
	}
	canonical, err := util.EncodeCanonicalJson(obj)
	if err != nil {
		return err
	}

	for i, h := range headers {
		if h.Origin != obj["origin"] {
			return errors.New("auth is from multiple servers")
		}
		if h.Destination != obj["destination"] {
			return errors.New("auth is for multiple servers")
		}
		if h.Destination != "" && (!util.IsServerOurs(h.Destination) || destinationHost != h.Destination) {
			return ErrWrongDestination
		}

		if key, ok := (originKeys)[h.KeyId]; ok {
			if !ed25519.Verify(key, canonical, h.Signature) {
				return fmt.Errorf("failed signatures on '%s', header %d", h.KeyId, i)
			}
		} else {
			return fmt.Errorf("unknown key '%s'", h.KeyId)
		}
	}

	return nil
}

func CreateXMatrixHeader(origin string, destination string, requestMethod string, requestUri string, content []byte, key ed25519.PrivateKey, keyVersion string) (string, error) {
	obj := map[string]interface{}{
		"method":      requestMethod,
		"uri":         requestUri,
		"origin":      origin,
		"destination": destination,
	}
	if content != nil {
		obj["content"] = content
	}
	canonical, err := util.EncodeCanonicalJson(obj)
	if err != nil {
		return "", err
	}

	b := ed25519.Sign(key, canonical)
	sig := util.EncodeUnpaddedBase64ToString(b)

	return fmt.Sprintf("X-Matrix origin=\"%s\",destination=\"%s\",key=\"ed25519:%s\",sig=\"%s\"", origin, destination, keyVersion, sig), nil
}
