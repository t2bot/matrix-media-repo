package matrix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

const NoSigningKey = ""

// Based in part on https://github.com/matrix-org/gomatrix/blob/072b39f7fa6b40257b4eead8c958d71985c28bdd/client.go#L180-L243
func doRequest(ctx rcontext.RequestContext, method string, urlStr string, body interface{}, result interface{}, accessToken string, ipAddr string) error {
	ctx.Log.Debugf("Calling %s %s", method, urlStr)
	var bodyBytes []byte
	if body != nil {
		jsonStr, err := json.Marshal(body)
		if err != nil {
			return err
		}

		bodyBytes = jsonStr
	}

	req, err := http.NewRequest(method, urlStr, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "matrix-media-repo")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if ipAddr != "" {
		req.Header.Set("X-Forwarded-For", ipAddr)
		req.Header.Set("X-Real-IP", ipAddr)
	}

	client := &http.Client{
		Timeout: time.Duration(ctx.Config.TimeoutSeconds.ClientServer) * time.Second,
	}
	res, err := client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}

	contents, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		mtxErr := &ErrorResponse{}
		err = json.Unmarshal(contents, mtxErr)
		if err == nil && mtxErr.ErrorCode != "" {
			return mtxErr
		}
		return errors.New("failed to perform request: " + string(contents))
	}

	if result != nil {
		err = json.Unmarshal(contents, &result)
		if err != nil {
			return err
		}
	}

	return nil
}

func FederatedGet(ctx rcontext.RequestContext, reqUrl string, realHost string, destination string, useSigningKeyPath string) (*http.Response, error) {
	ctx.Log.Debug("Doing federated GET to " + reqUrl + " with host " + realHost)

	cb := getFederationBreaker(realHost)

	var resp *http.Response
	replyError := cb.CallContext(ctx, func() error {
		req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err != nil {
			return err
		}

		// Override the host to be compliant with the spec
		req.Header.Set("Host", realHost)
		req.Header.Set("User-Agent", "matrix-media-repo")
		req.Host = realHost

		if useSigningKeyPath != NoSigningKey {
			ctx.Log.Debug("Reading signing key and adding authentication headers")
			key, err := getLocalSigningKey(useSigningKeyPath)
			if err != nil {
				return err
			}
			parsed, err := url.Parse(reqUrl)
			if err != nil {
				return err
			}
			auth, err := CreateXMatrixHeader(ctx.Request.Host, destination, http.MethodGet, parsed.RequestURI(), nil, key.Key, key.Version)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", auth)
		}

		var client *http.Client
		if os.Getenv("MEDIA_REPO_UNSAFE_FEDERATION") != "true" {
			// This is how we verify the certificate is valid for the host we expect.
			// Previously using `req.URL.Host` we'd end up changing which server we were
			// connecting to (ie: matrix.org instead of matrix.org.cdn.cloudflare.net),
			// which obviously doesn't help us. We needed to do that though because the
			// HTTP client doesn't verify against the req.Host certificate, but it does
			// handle it off the req.URL.Host. So, we need to tell it which certificate
			// to verify.

			h, _, err := net.SplitHostPort(realHost)
			if err == nil {
				// Strip the port first, certs are port-insensitive
				realHost = h
			}
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						ServerName: realHost,
					},
				},
			}
		} else {
			ctx.Log.Warn("Ignoring any certificate errors while making request")
			tr := &http.Transport{
				DisableKeepAlives: true,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
				// Based on https://github.com/matrix-org/gomatrixserverlib/blob/51152a681e69a832efcd934b60080b92bc98b286/client.go#L74-L90
				DialTLSContext: func(ctx2 context.Context, network, addr string) (net.Conn, error) {
					rawconn, err := net.Dial(network, addr)
					if err != nil {
						return nil, err
					}
					// Wrap a raw connection ourselves since tls.Dial defaults the SNI
					conn := tls.Client(rawconn, &tls.Config{
						ServerName:         "",
						InsecureSkipVerify: true,
					})
					if err := conn.Handshake(); err != nil {
						return nil, err
					}
					return conn, nil
				},
			}
			client = &http.Client{
				Transport: tr,
			}
		}

		client.Timeout = time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 { // arbitrary
				return errors.New("too many redirects")
			}
			ctx.Log.Debugf("Redirected to %s", req.URL.String())
			client.Transport = nil // Clear our TLS handler as we're out of the Matrix certificate verification steps
			return nil
		}

		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			b, _ := io.ReadAll(resp.Body)
			ctx.Log.Warn(string(b))
			return fmt.Errorf("response not ok: %d", resp.StatusCode)
		}
		return nil
	}, 1*time.Minute)

	return resp, replyError
}
