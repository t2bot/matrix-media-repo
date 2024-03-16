package matrix

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alioygur/is"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

var (
	apiUrlCacheInstance *cache.Cache
	apiUrlSingletonLock = &sync.Once{}
)

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

func GetServerApiUrl(hostname string) (string, string, error) {
	// dev note: URL lookups are not covered by the breaker because otherwise it might never close.

	logrus.Debug("Getting server API URL for " + hostname)

	// Check to see if we've cached this hostname at all
	setupCache()
	record, found := apiUrlCacheInstance.Get(hostname)
	if found {
		server := record.(cachedServer)
		logrus.Debug("Server API URL for " + hostname + " is " + server.url + " (cache)")
		return server.url, server.hostname, nil
	}

	addrErr := &net.AddrError{}
	host, port, err := net.SplitHostPort(hostname)
	defPort := false
	switch {
	case errors.As(err, &addrErr) && addrErr.Err == "missing port in address":
		host, port, err = net.SplitHostPort(hostname + ":8448")
		if err != nil {
			return "", "", fmt.Errorf("failed to parse hostname: %w", err)
		}
	case err != nil:
		return "", "", fmt.Errorf("failed to parse hostname: %w", err)
	}

	// Step 1 of the discovery process: if the hostname is an IP, use that with explicit or default port
	logrus.Debug("Testing if " + host + " is an IP address")
	if is.IP(host) {
		url := fmt.Sprintf("https://%s", net.JoinHostPort(host, port))
		server := cachedServer{url, hostname}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Debug("Server API URL for " + hostname + " is " + url + " (IP address)")
		return url, hostname, nil
	}

	// Step 2: if the hostname is not an IP address, and an explicit port is given, use that
	logrus.Debug("Testing if a default port was used. Using default = ", defPort)
	if !defPort {
		url := fmt.Sprintf("https://%s", net.JoinHostPort(host, port))
		server := cachedServer{url, host}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Debugf("Server API URL for %s is %s (explicit port)", hostname, url)
		return url, host, nil
	}

	// Step 3: if the hostname is not an IP address and no explicit port is given, do .well-known
	// Note that we have sprawling branches here because we need to fall through to step 4 if parsing fails
	logrus.Debug("Doing .well-known lookup on " + host)
	r, err := http.Get(fmt.Sprintf("https://%s/.well-known/matrix/server", host))
	if r != nil {
		defer r.Body.Close()
	}
	if err == nil && r.StatusCode == http.StatusOK {
		// Try parsing .well-known
		decoder := json.NewDecoder(r.Body)
		wk := &wellknownServerResponse{}
		err3 := decoder.Decode(&wk)
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
					logrus.Debugf("Server API URL for %s is %s (WK; IP address)", hostname, url)
					return url, wk.ServerAddr, nil
				}

				// Step 3b: if the delegated host is not an IP and an explicit port is given, use that
				logrus.Debug("Checking if WK is using default port? ", wkDefPort)
				if !wkDefPort {
					wkHost = net.JoinHostPort(wkHost, wkPort)
					url := fmt.Sprintf("https://%s", wkHost)
					server := cachedServer{url, wkHost}
					apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
					logrus.Debugf("Server API URL for %s is %s (WK; explicit port)", hostname, url)
					return url, wkHost, nil
				}

				// Step 3c: if the delegated host is not an IP and doesn't have a port, start a SRV lookup and use it.
				// Note: we ignore errors here because the hostname will fail elsewhere.
				logrus.Debug("Doing SRV on WK host ", wkHost)
				_, addrs, _ := net.LookupSRV("matrix-fed", "tcp", wkHost)
				if len(addrs) > 0 {
					// Trim off the trailing period if there is one (golang doesn't like this)
					realAddr := addrs[0].Target
					if realAddr[len(realAddr)-1:] == "." {
						realAddr = realAddr[0 : len(realAddr)-1]
					}
					url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
					server := cachedServer{url, wkHost}
					apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
					logrus.Debugf("Server API URL for %s is %s (WK; SRV)", hostname, url)
					return url, wkHost, nil
				}

				// Step 3d: if the delegated host is not an IP and doesn't have a port, start a DEPRECATED SRV
				// lookup and use it.
				// Note: we ignore errors here because the hostname will fail elsewhere.
				logrus.Debug("Doing SRV on WK host ", wkHost)
				_, addrs, _ = net.LookupSRV("matrix", "tcp", wkHost)
				if len(addrs) > 0 {
					// Trim off the trailing period if there is one (golang doesn't like this)
					realAddr := addrs[0].Target
					if realAddr[len(realAddr)-1:] == "." {
						realAddr = realAddr[0 : len(realAddr)-1]
					}
					url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
					server := cachedServer{url, wkHost}
					apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
					logrus.Debugf("Server API URL for %s is %s (WK; SRV-Deprecated)", hostname, url)
					return url, wkHost, nil
				}

				// Step 3d: use the delegated host as-is
				logrus.Debug("Using .well-known as-is for ", wkHost)
				url := fmt.Sprintf("https://%s", net.JoinHostPort(wkHost, wkPort))
				server := cachedServer{url, wkHost}
				apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
				logrus.Debugf("Server API URL for %s is %s (WK; fallback)", hostname, url)
				return url, wkHost, nil
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
	_, addrs, _ := net.LookupSRV("matrix-fed", "tcp", hostname)
	if len(addrs) > 0 {
		// Trim off the trailing period if there is one (golang doesn't like this)
		realAddr := addrs[0].Target
		if realAddr[len(realAddr)-1:] == "." {
			realAddr = realAddr[0 : len(realAddr)-1]
		}
		url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
		server := cachedServer{url, host}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Debugf("Server API URL for %s is %s (SRV)", hostname, url)
		return url, host, nil
	}

	// Step 5: try resolving a hostname using DEPRECATED SRV records and use it
	// Note: we ignore errors here because the hostname will fail elsewhere.
	logrus.Debug("Doing SRV for host ", hostname)
	_, addrs, _ = net.LookupSRV("matrix", "tcp", hostname)
	if len(addrs) > 0 {
		// Trim off the trailing period if there is one (golang doesn't like this)
		realAddr := addrs[0].Target
		if realAddr[len(realAddr)-1:] == "." {
			realAddr = realAddr[0 : len(realAddr)-1]
		}
		url := fmt.Sprintf("https://%s", net.JoinHostPort(realAddr, strconv.Itoa(int(addrs[0].Port))))
		server := cachedServer{url, host}
		apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
		logrus.Debugf("Server API URL for %s is %s (SRV-Deprecated)", hostname, url)
		return url, host, nil
	}

	// Step 6: use the target host as-is
	logrus.Debug("Using host as-is: ", hostname)
	url := fmt.Sprintf("https://%s", net.JoinHostPort(host, port))
	server := cachedServer{url, host}
	apiUrlCacheInstance.Set(hostname, server, cache.DefaultExpiration)
	logrus.Debugf("Server API URL for %s is %s (fallback)", hostname, url)
	return url, host, nil
}
