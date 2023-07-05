package u

import (
	"net"

	"github.com/getsentry/sentry-go"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func getSafeAddress(addr string, ctx rcontext.RequestContext) (net.IP, string, error) {
	ctx.Log.Debug("Checking address: " + addr)
	realHost, p, err := net.SplitHostPort(addr)
	if err != nil {
		ctx.Log.Debug("Error parsing host and port: ", err)
		sentry.CaptureException(err)
		realHost = addr
	}

	ipAddr := net.IPv4(127, 0, 0, 1)
	if realHost != "localhost" {
		addrs, err := net.LookupIP(realHost)
		if err != nil {
			ctx.Log.Debug("Error looking up DNS record for preview - assuming invalid host:", err)
			return nil, "", common.ErrInvalidHost
		}
		if len(addrs) == 0 {
			return nil, "", common.ErrHostNotFound
		}
		ipAddr = addrs[0]
	}

	allowedCidrs := ctx.Config.UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := ctx.Config.UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}

	// Forcefully append 0.0.0.0 and :: because they are unroutable and resolve to localhost
	deniedCidrs = append(deniedCidrs, "0.0.0.0/32")
	deniedCidrs = append(deniedCidrs, "::/128")

	if !isAllowed(ipAddr, allowedCidrs, deniedCidrs, ctx) {
		return nil, "", common.ErrHostNotAllowed
	}
	return ipAddr, p, nil
}

func isAllowed(ip net.IP, allowed []string, disallowed []string, ctx rcontext.RequestContext) bool {
	ctx.Log.Debug("Validating host")

	// First check if the IP fits the deny list. This should be a much shorter list, and therefore
	// much faster to check.
	ctx.Log.Debug("Checking deny list for host...")
	if inRange(ip, disallowed, ctx) {
		ctx.Log.Debug("Host found on deny list - rejecting")
		return false
	}

	// Now check the allowed list just to make sure the IP is actually allowed
	if inRange(ip, allowed, ctx) {
		ctx.Log.Debug("Host allowed due to allow list")
		return true
	}

	ctx.Log.Debug("Host is not on either allow list or deny list, considering deny listed")
	return false
}

func inRange(ip net.IP, cidrs []string, ctx rcontext.RequestContext) bool {
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
