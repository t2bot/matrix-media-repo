package _responses

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type RedirectResponse struct {
	ToUrl string
}

func Redirect(ctx rcontext.RequestContext, toUrl string, auth _apimeta.AuthContext) *RedirectResponse {
	if auth.IsAuthenticated() {
		// Figure out who is authenticated here, as that affects the expiration time
		var expirationTime time.Time
		if auth.Server.ServerName != "" {
			expirationTime = time.Now().Add(time.Minute)
		} else {
			expirationTime = time.Now().Add(time.Minute * 5)
		}

		// Append the expiration time to the URL
		toUrl = appendQueryParam(toUrl, "matrix_exp", strconv.FormatInt(expirationTime.UnixMilli(), 10))

		// Prepare our HMAC message contents as a JSON object
		hmacInput := make(map[string]string)
		hmacInput["url"] = toUrl
		if auth.User.UserId != "" {
			hmacInput["access_token"] = auth.User.AccessToken
		}
		hmacMessage, err := json.Marshal(hmacInput)
		if err != nil {
			panic(err) // "should never happen"
		}

		// Actually do the HMAC
		mac := hmac.New(sha256.New, []byte("THIS_IS_A_SECRET_KEY")) // TODO: @@ Actual secret key
		mac.Write(hmacMessage)
		verifyHmac := mac.Sum(nil)

		// Append the HMAC to the URL
		toUrl = appendQueryParam(toUrl, "matrix_verify", hex.EncodeToString(verifyHmac))
	}
	return &RedirectResponse{ToUrl: toUrl}
}

func appendQueryParam(toUrl string, key string, val string) string {
	parsedUrl, err := url.Parse(toUrl)
	if err != nil {
		panic(err) // it wouldn't have worked anyways
	}
	qs := parsedUrl.Query()
	qs.Set(key, val)
	parsedUrl.RawQuery = qs.Encode()
	return parsedUrl.String()
}
