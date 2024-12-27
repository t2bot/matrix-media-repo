package _responses

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"os"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

// TODO: Persist and store key (or use some other source of information)
var jwtKey, keyErr = rsa.GenerateKey(rand.Reader, 2048)
var jwtSig, jwtErr = jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("0102030405060708090A0B0C0D0E0F10")}, (&jose.SignerOptions{}).WithType("JWT"))
var jweEnc, jweErr = jose.NewEncrypter(jose.A128GCM, jose.Recipient{Algorithm: jose.RSA_OAEP, Key: &jwtKey.PublicKey}, nil)

func init() {
	// We don't expect these to happen, so just panic
	if keyErr != nil {
		panic(keyErr)
	}
	if jwtErr != nil {
		panic(jwtErr)
	}
	if jweErr != nil {
		panic(jweErr)
	}

	f1, _ := os.Create("./gdpr-data/jwe.rsa")
	f2, _ := os.Create("./gdpr-data/jwe.rsa.pub")
	defer f1.Close()
	defer f2.Close()
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(jwtKey),
	})
	pubPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&jwtKey.PublicKey),
	})
	f1.Write(keyPem)
	f2.Write(pubPem)
}

type RedirectResponse struct {
	ToUrl string
}

func Redirect(ctx rcontext.RequestContext, toUrl string, auth _apimeta.AuthContext) *RedirectResponse {
	if auth.IsAuthenticated() {
		// Figure out who we're authenticating as, and add that as JWT claims
		claims := jwt.Claims{
			Issuer: ctx.Request.Host,
		}
		moreClaims := struct {
			Admin       bool   `json:"adm,omitempty"`
			AccessToken string `json:"tok,omitempty"`
		}{}
		if auth.Server.ServerName != "" {
			claims.Subject = auth.Server.ServerName

			// The server won't necessarily re-auth itself with the redirect, so we just put an expiration on it instead
			claims.Expiry = jwt.NewNumericDate(time.Now().Add(2 * time.Minute))
		} else {
			claims.Subject = auth.User.UserId
			moreClaims.Admin = auth.User.IsShared
			moreClaims.AccessToken = auth.User.AccessToken
		}
		raw, err := jwt.Encrypted(jweEnc).Claims(claims).Claims(moreClaims).Serialize()
		if err != nil {
			panic(err) // should never happen if we've done things correctly
		}

		// Now add the query string
		parsedUrl, err := url.Parse(toUrl)
		if err != nil {
			panic(err) // it wouldn't have worked anyways
		}
		qs := parsedUrl.Query()
		qs.Set("org.matrix.media_auth", raw)
		parsedUrl.RawQuery = qs.Encode()
		toUrl = parsedUrl.String()
	}
	return &RedirectResponse{ToUrl: toUrl}
}
