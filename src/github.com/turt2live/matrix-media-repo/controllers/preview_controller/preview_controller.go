package preview_controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func GetPreview(urlStr string, onHost string, forUserId string, atTs int64, ctx context.Context, log *logrus.Entry) (*types.UrlPreview, error) {
	log = log.WithFields(logrus.Fields{
		"preview_controller_at_ts": atTs,
	})

	db := storage.GetDatabase().GetUrlStore(ctx, log)

	cached, err := db.GetPreview(urlStr, atTs)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Error getting cached URL preview: ", err.Error())
		return nil, err
	}
	if err != sql.ErrNoRows {
		log.Info("Returning cached URL preview")
		return cachedPreviewToReal(cached)
	}

	now := util.NowMillis()
	if (now - atTs) > 60000 {
		// Because we don't have a cached preview, we'll use the current time as the preview time.
		// We also give a 60 second buffer so we don't cause an infinite loop (considering we're
		// calling ourselves), and to give a lenient opportunity for slow execution.
		return GetPreview(urlStr, onHost, forUserId, now, ctx, log)
	}

	log.Info("Preview not cached - fetching resource")

	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		log.Error("Error parsing URL: ", err.Error())
		db.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}

	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		log.Error("Error getting host info: ", err.Error())
		db.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}
	if len(addrs) == 0 {
		db.InsertPreviewError(urlStr, common.ErrCodeHostNotFound)
		return nil, common.ErrHostNotFound
	}
	addr := addrs[0]

	allowedCidrs := config.Get().UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := config.Get().UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}
	if !isAllowed(addr, allowedCidrs, deniedCidrs, log) {
		db.InsertPreviewError(urlStr, common.ErrCodeHostBlacklisted)
		return nil, common.ErrHostBlacklisted
	}

	result := <-getResourceHandler().GeneratePreview(urlStr, forUserId, onHost)
	return result.preview, result.err
}

func isAllowed(ip net.IP, allowed []string, disallowed []string, log *logrus.Entry) bool {
	log = log.WithFields(logrus.Fields{
		"checkHost":       ip,
		"allowedHosts":    fmt.Sprintf("%v", allowed),
		"disallowedHosts": fmt.Sprintf("%v", allowed),
	})
	log.Info("Validating host")

	// First check if the IP fits the blacklist. This should be a much shorter list, and therefore
	// much faster to check.
	log.Info("Checking blacklist for host...")
	if inRange(ip, disallowed, log) {
		log.Warn("Host found on blacklist - rejecting")
		return false
	}

	// Now check the allowed list just to make sure the IP is actually allowed
	if inRange(ip, allowed, log) {
		log.Info("Host allowed due to whitelist")
		return true
	}

	log.Warn("Host is not on either whitelist or blacklist, considering blacklisted")
	return false
}

func inRange(ip net.IP, cidrs []string, log *logrus.Entry) bool {
	for i := 0; i < len(cidrs); i++ {
		cidr := cidrs[i]
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Error("Error checking host: " + err.Error())
			return false
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func cachedPreviewToReal(cached *types.CachedUrlPreview) (*types.UrlPreview, error) {
	if cached.ErrorCode == common.ErrCodeInvalidHost {
		return nil, common.ErrInvalidHost
	} else if cached.ErrorCode == common.ErrCodeHostNotFound {
		return nil, common.ErrHostNotFound
	} else if cached.ErrorCode == common.ErrCodeHostBlacklisted {
		return nil, common.ErrHostBlacklisted
	} else if cached.ErrorCode == common.ErrCodeNotFound {
		return nil, common.ErrMediaNotFound
	} else if cached.ErrorCode == common.ErrCodeUnknown {
		return nil, errors.New("unknown error")
	}

	return cached.Preview, nil
}
