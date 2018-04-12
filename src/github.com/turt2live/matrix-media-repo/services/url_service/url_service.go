package url_service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type urlService struct {
	store *stores.UrlStore
	ctx   context.Context
	log   *logrus.Entry
}

func New(ctx context.Context, log *logrus.Entry) (*urlService) {
	store := storage.GetDatabase().GetUrlStore(ctx, log)
	return &urlService{store, ctx, log}
}

func returnCachedPreview(cached *types.CachedUrlPreview) (*types.UrlPreview, error) {
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

func (s *urlService) GetPreview(urlStr string, onHost string, forUserId string, atTs int64) (*types.UrlPreview, error) {
	s.log = s.log.WithFields(logrus.Fields{
		"urlService_ts": atTs,
	})

	cached, err := s.store.GetPreview(urlStr, atTs)
	if err != nil && err != sql.ErrNoRows {
		s.log.Error("Error getting cached URL: " + err.Error())
		return nil, err
	}
	if err != sql.ErrNoRows {
		s.log.Info("Returning cached URL preview")
		return returnCachedPreview(cached)
	}

	now := util.NowMillis()
	if (now - atTs) > 60000 { // this is to avoid infinite loops (60s window for slow execution)
		// Original bucket failed - try getting the preview for "now"
		return s.GetPreview(urlStr, onHost, forUserId, now)
	}

	s.log.Info("URL preview not cached - fetching resource")

	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		s.log.Error("Error parsing url: " + err.Error())
		s.store.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}

	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		s.log.Error("Error getting host info: " + err.Error())
		s.store.InsertPreviewError(urlStr, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}
	if len(addrs) == 0 {
		s.store.InsertPreviewError(urlStr, common.ErrCodeHostNotFound)
		return nil, common.ErrHostNotFound
	}
	addr := addrs[0]
	addrStr := fmt.Sprintf("%v", addr)[1:]
	addrStr = addrStr[:len(addrStr)-1]

	// Verify the host is in the allowed range
	allowedCidrs := config.Get().UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := config.Get().UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}
	if !isAllowed(addr, allowedCidrs, deniedCidrs, s.log) {
		s.store.InsertPreviewError(urlStr, common.ErrCodeHostBlacklisted)
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
