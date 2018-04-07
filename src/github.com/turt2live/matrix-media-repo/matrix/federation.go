package matrix

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

var apiUrlCacheInstance *cache.Cache
var apiUrlSingletonLock = &sync.Once{}

func setupCache() {
	if apiUrlCacheInstance == nil {
		apiUrlSingletonLock.Do(func() {
			apiUrlCacheInstance = cache.New(1*time.Hour, 2*time.Hour)
		})
	}
}

func GetServerApiUrl(hostname string) (string, error) {
	logrus.Info("Getting server API URL for " + hostname)

	// Check to see if we've cached this hostname at all
	setupCache()
	record, found := apiUrlCacheInstance.Get(hostname)
	if found {
		url := record.(string)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (cache)")
		return url, nil
	}

	// If not cached, start by seeing if there's a port. If there is a port, use that.
	// Note: we ignore errors because they are parsing errors. Invalid hostnames will fail through elsewhere.
	h, p, _ := net.SplitHostPort(hostname)
	if p != "" {
		url := fmt.Sprintf("https://%s:%d", h, p)
		apiUrlCacheInstance.Set(hostname, url, cache.DefaultExpiration)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (explicit port)")
		return url, nil
	}

	// Try resolving by SRV record. If there's at least one result, use that.
	// Note: we also ignore errors here because the hostname will fail elsewhere.
	_, addrs, _ := net.LookupSRV("matrix", "tcp", hostname)
	if len(addrs) > 0 {
		// Trim off the trailing period if there is one (golang doesn't like this)
		realAddr := addrs[0].Target
		if realAddr[len(realAddr)-1:] == "." {
			realAddr = realAddr[0:len(realAddr)-1]
		}
		url := fmt.Sprintf("https://%s:%d", realAddr, addrs[0].Port)
		apiUrlCacheInstance.Set(hostname, url, cache.DefaultExpiration)
		logrus.Info("Server API URL for " + hostname + " is " + url + " (SRV)")
		return url, nil
	}

	// Lastly fall back to port 8448
	url := fmt.Sprintf("https://%s:%d", hostname, 8448)
	apiUrlCacheInstance.Set(hostname, url, cache.DefaultExpiration)
	logrus.Info("Server API URL for " + hostname + " is " + url + " (fallback)")
	return url, nil
}

func FederatedGet(url string, realHost string) (*http.Response, error) {
	logrus.Info("Doing federated GET to " + url + " with host " + realHost)
	transport := &http.Transport{
		// Based on https://github.com/matrix-org/gomatrixserverlib/blob/51152a681e69a832efcd934b60080b92bc98b286/client.go#L74-L90
		DialTLS: func(network, addr string) (net.Conn, error) {
			rawconn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			// Wrap a raw connection ourselves since tls.Dial defaults the SNI
			conn := tls.Client(rawconn, &tls.Config{
				ServerName: "",
				// TODO: We should be checking that the TLS certificate we see here matches one of the allowed SHA-256 fingerprints for the server.
				InsecureSkipVerify: true,
			})
			if err := conn.Handshake(); err != nil {
				return nil, err
			}
			return conn, nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Override the host to be compliant with the spec
	req.Header.Set("Host", realHost)
	req.Header.Set("User-Agent", "matrix-media-repo")
	req.Host = realHost

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
