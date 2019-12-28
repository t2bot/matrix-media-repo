package acl

import (
	"fmt"
	"net"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/preview_controller/preview_types"
	"github.com/turt2live/matrix-media-repo/storage"
)

func ValidateUrlForPreview(urlStr string, ctx rcontext.RequestContext) (*preview_types.UrlPayload, error) {
	db := storage.GetDatabase().GetUrlStore(ctx)

	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		ctx.Log.Error("Error parsing URL: ", err.Error())
		db.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}
	realHost, _, err := net.SplitHostPort(parsedUrl.Host)
	if err != nil {
		ctx.Log.Error("Error parsing host and port: ", err.Error())
		realHost = parsedUrl.Host
	}

	addr := net.IPv4(127, 0, 0, 1)
	if realHost != "localhost" {
		addrs, err := net.LookupIP(realHost)
		if err != nil {
			ctx.Log.Error("Error getting host info: ", err.Error())
			db.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
			return nil, common.ErrInvalidHost
		}
		if len(addrs) == 0 {
			db.InsertPreviewError(urlStr, common.ErrCodeHostNotFound)
			return nil, common.ErrHostNotFound
		}
		addr = addrs[0]
	}

	allowedCidrs := config.Get().UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := config.Get().UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}

	// Forcefully append 0.0.0.0 and :: because they are unroutable and resolve to localhost
	deniedCidrs = append(deniedCidrs, "0.0.0.0/32")
	deniedCidrs = append(deniedCidrs, "::/128")

	if !isAllowed(addr, allowedCidrs, deniedCidrs, ctx) {
		db.InsertPreviewError(urlStr, common.ErrCodeHostBlacklisted)
		return nil, common.ErrHostBlacklisted
	}

	urlToPreview := &preview_types.UrlPayload{
		UrlString: urlStr,
		ParsedUrl: parsedUrl,
		Address:   addr,
	}
	return urlToPreview, nil
}

func isAllowed(ip net.IP, allowed []string, disallowed []string, ctx rcontext.RequestContext) bool {
	ctx = ctx.LogWithFields(logrus.Fields{
		"checkHost":       ip,
		"allowedHosts":    fmt.Sprintf("%v", allowed),
		"disallowedHosts": fmt.Sprintf("%v", allowed),
	})
	ctx.Log.Info("Validating host")

	// First check if the IP fits the blacklist. This should be a much shorter list, and therefore
	// much faster to check.
	ctx.Log.Info("Checking blacklist for host...")
	if inRange(ip, disallowed, ctx) {
		ctx.Log.Warn("Host found on blacklist - rejecting")
		return false
	}

	// Now check the allowed list just to make sure the IP is actually allowed
	if inRange(ip, allowed, ctx) {
		ctx.Log.Info("Host allowed due to whitelist")
		return true
	}

	ctx.Log.Warn("Host is not on either whitelist or blacklist, considering blacklisted")
	return false
}

func inRange(ip net.IP, cidrs []string, ctx rcontext.RequestContext) bool {
	for i := 0; i < len(cidrs); i++ {
		cidr := cidrs[i]
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			ctx.Log.Error("Error checking host: " + err.Error())
			return false
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
