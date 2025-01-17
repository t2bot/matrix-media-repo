package matrix

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type HttpClientConfig struct {
	Timeout                time.Duration
	AllowUnsafeCertificate bool
	AllowedCIDRs           []string
	DeniedCIDRs            []string
	// Used for federation. Set to the *real* server name, without port.
	TLSServerName   string
	FollowRedirects bool
	Proxy           func(*http.Request) (*url.URL, error)
}

func NewHttpClient(ctx rcontext.RequestContext, clientConfig *HttpClientConfig) *http.Client {
	// Forcefully append 0.0.0.0 and :: because they are unroutable and resolve to localhost
	deniedCidrs := clientConfig.DeniedCIDRs
	deniedCidrs = append(deniedCidrs, "0.0.0.0/32")
	deniedCidrs = append(deniedCidrs, "::/128")
	clientConfig.DeniedCIDRs = deniedCidrs

	// Ensure we're at least able to make requests
	if clientConfig.AllowedCIDRs == nil {
		clientConfig.AllowedCIDRs = []string{"0.0.0.0/0"}
	}

	// Default to environment variable proxy if unspecified
	if clientConfig.Proxy == nil {
		clientConfig.Proxy = http.ProxyFromEnvironment
	}

	// safeDialer and safeTransport are from https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang
	// We add our client config and request context to the safeControl function for logging, primarily.
	safeDialer := &net.Dialer{
		Timeout:   clientConfig.Timeout,
		KeepAlive: 30 * time.Second, // default
		Control: func(network string, address string, conn syscall.RawConn) error {
			return safeControl(ctx, clientConfig, network, address, conn)
		},
	}
	safeTransport := &http.Transport{
		DialContext:           safeDialer.DialContext,
		Proxy:                 clientConfig.Proxy,
		ForceAttemptHTTP2:     true,             // default
		MaxIdleConns:          100,              // default
		IdleConnTimeout:       90 * time.Second, // default
		TLSHandshakeTimeout:   10 * time.Second, // default
		ExpectContinueTimeout: 1 * time.Second,  // default
	}

	// If we're allowing invalid certificates, ensure mistakes can be made. If we've got a
	// TLSServerName, indicating we're handling a Federation request, set that only if we
	// will actually verify it.
	if clientConfig.AllowUnsafeCertificate {
		ctx.Log.Warn("Ignoring any certificate errors while making requests")
		safeTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		safeTransport.DialTLSContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			rawconn, err := safeDialer.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			conn := tls.Client(rawconn, &tls.Config{
				ServerName:         "",
				InsecureSkipVerify: true,
			})
			if err := conn.Handshake(); err != nil {
				return nil, err
			}
			return conn, err
		}
		safeTransport.DisableKeepAlives = true
	} else if clientConfig.TLSServerName != "" {
		safeTransport.TLSClientConfig = &tls.Config{ServerName: clientConfig.TLSServerName}
	}

	// Create a client with our new safeTransport (and safeDialer), and avoid redirecting into infinity.
	// We also remove the server name check from our federation requests once redirected because they're
	// likely to fail.
	safeClient := &http.Client{
		Transport: safeTransport,
		Timeout:   clientConfig.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !clientConfig.FollowRedirects {
				// Normally we'd use the last response instead of erroring, but we want to
				// ensure things break when a redirect is encountered.
				return errors.New("too many redirects (1 > 0)")
			}
			if len(via) > 5 {
				return errors.New("too many redirects")
			}

			if req.URL.Scheme != "https" {
				return errors.New("https downgrades are not allowed")
			}

			// Now that we're past the initial server name validation, clear it
			safeTransport.TLSClientConfig.ServerName = ""

			return nil
		},
	}

	return safeClient
}

func safeControl(ctx rcontext.RequestContext, clientConfig *HttpClientConfig, network string, address string, conn syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" {
		return fmt.Errorf("%s is not a safe network type", network)
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s is not a valid host/port pair: %s", address, err)
	}

	ipaddress := net.ParseIP(host)
	if ipaddress == nil {
		return fmt.Errorf("%s is not a valid IP address", host)
	}

	if !isAllowed(ctx, ipaddress, clientConfig.AllowedCIDRs, clientConfig.DeniedCIDRs) {
		return fmt.Errorf("%s is denied", address)
	}

	return nil // allow connection
}

func isAllowed(ctx rcontext.RequestContext, ip net.IP, allowed []string, disallowed []string) bool {
	ctx.Log.Debug("Validating host")

	// First check if the IP fits the deny list. This should be a much shorter list, and therefore
	// much faster to check.
	ctx.Log.Debug("Checking deny list for host...")
	if inRange(ctx, ip, disallowed) {
		ctx.Log.Debug("Host found on deny list - rejecting")
		return false
	}

	// Now check the allowed list just to make sure the IP is actually allowed
	if inRange(ctx, ip, allowed) {
		ctx.Log.Debug("Host allowed due to allow list")
		return true
	}

	ctx.Log.Debug("Host is not on either allow list or deny list, considering deny listed")
	return false
}

func inRange(ctx rcontext.RequestContext, ip net.IP, cidrs []string) bool {
	for i := 0; i < len(cidrs); i++ {
		cidr := cidrs[i]
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			ctx.Log.Debug("Error checking host: ", err)
			sentry.CaptureException(err)
			return false
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
