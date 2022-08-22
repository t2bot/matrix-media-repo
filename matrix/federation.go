package matrix

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alioygur/is"
	"github.com/patrickmn/go-cache"
	circuit "github.com/rubyist/circuitbreaker"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

var apiUrlCacheInstance *cache.Cache
var apiUrlSingletonLock = &sync.Once{}
var federationBreakers = &sync.Map{}

type cachedServer struct {
	url      string
	hostname string
}

func setupCache() {
	if apiUrlCacheInstance == nil {
		apiUrlSingletonLock.Do(func() {
			apiUrlCacheInstance = cache.New(1*time.Hour, 2*time.Hour)
		})
	}
}

func getFederationBreaker(hostname string) *circuit.Breaker {
	var cb *circuit.Breaker
	cbRaw, hasCb := federationBreakers.Load(hostname)
	if !hasCb {
		backoffAt := int64(config.Get().Federation.BackoffAt)
		if backoffAt <= 0 {
			backoffAt = 20 // default to 20 for those who don't have this set
		}
		cb = circuit.NewConsecutiveBreaker(backoffAt)
		federationBreakers.Store(hostname, cb)
	} else {
		cb = cbRaw.(*circuit.Breaker)
	}
	return cb
}

// Note: URL lookups are not covered by the breaker because otherwise it might never close.
func GetServerApiUrl(hostname string) (string, string, error) {
	logrus.Info("Getting server API URL for " + hostname)

	// Check to see if we've cached this hostname at all
	setupCache()
	record, found := apiUrlCacheInstance.Get(hostname)
	if found {
		server := record.(cachedServer)
		logrus.Info("Server API URL for " + hostname + " is " + server.url + " (cache)")
		return server.url, server.hostname, nil
	}

	h, p, err := net.SplitHostPort(hostname)
	defPort := false
	if err != nil && strings.HasSuffix(err.Error(), "missing port in address") {
		h, p, err = net.SplitHostPort(hostname + ":8448")
		defPort = true
	}
	if err != nil {
		return "", "", err
	}

	// Step 1 of the discovery process: if the hostname is an IP, use that with explicit or default port
	logrus.Debug("Testing if " + h + " is an IP address")
	if is.IP(h) {
		url := fmt.Sprintf("https://%s", net.JoinHostPort(h, p))
		server := cachedServer{url, hostname}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (IP address)")
		return url, hostname, nil
	}

	// Step 2: if the hostname is not an IP address, and an explicit port is given, use that
	logrus.Debug("Testing if a default port was used. Using default = ", defPort)
	if !defPort {
		url := fmt.Sprintf("https://%s", net.JoinHostPort(h, p))
		server := cachedServer{url, h}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (explicit port)")
		return url, h, nil
	}

	// Step 3: if the hostname is not an IP address and no explicit port is given, do .well-known
	// Note that we have sprawling branches here because we need to fall through to step 4 if parsing fails
	logrus.Debug("Doing .well-known lookup on " + h)
	r, err := http.Get(fmt.Sprintf("https://%s/.well-known/matrix/server", h))
	if err == nil && r.StatusCode == http.StatusOK {
		// Try parsing .well-known
		c, err2 := ioutil.ReadAll(r.Body)
		if err2 == nil {
			wk := &wellknownServerResponse{}
			err3 := json.Unmarshal(c, wk)
			if err3 == nil && wk.ServerAddr != "" {
				wkHost, wkPort, err4 := net.SplitHostPort(wk.ServerAddr)
				wkDefPort := false
				if err4 != nil && strings.HasSuffix(err4.Error(), "missing port in address") {
					wkHost, wkPort, err4 = net.SplitHostPort(wk.ServerAddr + ":8448")
					wkDefPort = true
				}
				if err4 == nil {
					// Step 3a: if the delegated host is an IP address, use that (regardless of port)
					logrus.Debug("Checking if WK host is an IP: " + wkHost)
					if is.IP(wkHost) {
						url := fmt.Sprintf("https://%s", net.JoinHostPort(wkHost, wkPort))
						server := cachedServer{url, wk.ServerAddr}
						apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
						logrus.Info("Server API URL for " + hostname + " is " + url + " (WK; IP address)")
						return url, wk.ServerAddr, nil
					}

					// Step 3b: if the delegated host is not an IP and an explicit port is given, use that
					logrus.Debug("Checking if WK is using default port? ", wkDefPort)
					if !wkDefPort {
						wkHost = net.JoinHostPort(wkHost, wkPort)
						url := fmt.Sprintf("https://%s", wkHost)
						server := cachedServer{url, wkHost}
						apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
						logrus.Info("Server API URL for " + hostname + " is " + url + " (WK; explicit port)")
						return url, wkHost, nil
					}

					// Step 3c: if the delegated host is not an IP and doesn't have a port, start a SRV lookup and use it
					// Note: we ignore errors here because the hostname will fail elsewhere.
					logrus.Debug("Doing SRV on WK host ", wkHost)
					_, addrs, _ := net.LookupSRV("matrix", "tcp", wkHost)
					if len(addrs) > 0 {
						// Trim off the trailing period if there is one (golang doesn't like this)
						realAddr := addrs[0].Target
						if realAddr[len(realAddr)-1:] == "." {
							realAddr = realAddr[0 : len(realAddr)-1]
						}
						url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
						server := cachedServer{url, wkHost}
						apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
						logrus.Info("Server API URL for " + hostname + " is " + url + " (WK; SRV)")
						return url, wkHost, nil
					}

					// Step 3d: use the delegated host as-is
					logrus.Debug("Using .well-known as-is for ", wkHost)
					url := fmt.Sprintf("https://%s", net.JoinHostPort(wkHost, wkPort))
					server := cachedServer{url, wkHost}
					apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
					logrus.Info("Server API URL for " + hostname + " is " + url + " (WK; fallback)")
					return url, wkHost, nil
				}
			}
		}
	}
	if r != nil {
		logrus.Debug("WK response code was ", r.StatusCode)
	}
	logrus.Debug("WK error: ", err)

	// Step 4: try resolving a hostname using SRV records and use it
	// Note: we ignore errors here because the hostname will fail elsewhere.
	logrus.Debug("Doing SRV for host ", hostname)
	_, addrs, _ := net.LookupSRV("matrix", "tcp", hostname)
	if len(addrs) > 0 {
		// Trim off the trailing period if there is one (golang doesn't like this)
		realAddr := addrs[0].Target
		if realAddr[len(realAddr)-1:] == "." {
			realAddr = realAddr[0 : len(realAddr)-1]
		}
		url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
		server := cachedServer{url, h}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (SRV)")
		return url, h, nil
	}

	// Step 5: use the target host as-is
	logrus.Debug("Using host as-is: ", hostname)
	url := fmt.Sprintf("https://%s", net.JoinHostPort(h, p))
	server := cachedServer{url, h}
	apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
	logrus.Info("Server API URL for " + hostname + " is " + url + " (fallback)")
	return url, h, nil
}

func FederatedGet(url string, realHost string, ctx rcontext.RequestContext) (*http.Response, error) {
	logrus.Info("Doing federated GET to " + url + " with host " + realHost)

	cb := getFederationBreaker(realHost)

	var resp *http.Response
	replyError := cb.CallContext(ctx, func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		// Override the host to be compliant with the spec
		req.Header.Set("Host", realHost)
		req.Header.Set("User-Agent", "matrix-media-repo")
		req.Host = realHost

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
				Timeout: time.Duration(ctx.Config.TimeoutSeconds.Federation) * time.Second,
			}
		} else {
			ctx.Log.Warn("Ignoring any certificate errors while making request")
			tr := &http.Transport{
				DisableKeepAlives: true,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
				// Based on https://github.com/matrix-org/gomatrixserverlib/blob/51152a681e69a832efcd934b60080b92bc98b286/client.go#L74-L90
				DialTLS: func(network, addr string) (net.Conn, error) {
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
				Timeout:   time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
			}
		}

		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return errors.New(fmt.Sprintf("response not ok: %d", resp.StatusCode))
		}
		return nil
	}, 1*time.Minute)

	return resp, replyError
}
