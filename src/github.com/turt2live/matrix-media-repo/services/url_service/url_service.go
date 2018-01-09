package url_service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
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
	if cached.ErrorCode == errs.ErrCodeInvalidHost {
		return nil, errs.ErrInvalidHost
	} else if cached.ErrorCode == errs.ErrCodeHostNotFound {
		return nil, errs.ErrHostNotFound
	} else if cached.ErrorCode == errs.ErrCodeHostBlacklisted {
		return nil, errs.ErrHostBlacklisted
	} else if cached.ErrorCode == errs.ErrCodeNotFound {
		return nil, errs.ErrMediaNotFound
	} else if cached.ErrorCode == errs.ErrCodeUnknown {
		return nil, errors.New("unknown error")
	}

	return cached.Preview, nil
}

func (s *urlService) GetPreview(urlStr string, onHost string, forUserId string, atTs int64) (*types.UrlPreview, error) {
	s.log = s.log.WithFields(logrus.Fields{
		"urlService_ts": atTs,
	})

	cached, err := s.store.GetPreview(urlStr, atTs)
	if err != nil {
		s.log.Error("Error getting cached URL: " + err.Error())
	}
	if err != nil && err != sql.ErrNoRows {
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
		s.store.InsertPreviewError(urlStr, errs.ErrCodeInvalidHost)
		return nil, errs.ErrInvalidHost
	}

	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		s.log.Error("Error getting host info: " + err.Error())
		s.store.InsertPreviewError(urlStr, errs.ErrCodeInvalidHost)
		return nil, errs.ErrInvalidHost
	}
	if len(addrs) == 0 {
		s.store.InsertPreviewError(urlStr, errs.ErrCodeHostNotFound)
		return nil, errs.ErrHostNotFound
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
		s.store.InsertPreviewError(urlStr, errs.ErrCodeHostBlacklisted)
		return nil, errs.ErrHostBlacklisted
	}

	s.log = s.log.WithFields(logrus.Fields{
		"previewer": "OpenGraph",
	})

	previewer := NewOpenGraphPreviewer(s.ctx, s.log)
	preview, err := previewer.GeneratePreview(urlStr)
	if err != nil {
		if err == errs.ErrMediaNotFound {
			s.store.InsertPreviewError(urlStr, errs.ErrCodeNotFound)
		} else {
			s.store.InsertPreviewError(urlStr, errs.ErrCodeUnknown)
		}
		return nil, err
	}

	result := &types.UrlPreview{
		Url:         preview.Url,
		SiteName:    preview.SiteName,
		Type:        preview.Type,
		Description: preview.Description,
		Title:       preview.Title,
	}

	// Store the thumbnail, if there is one
	mediaSvc := media_service.New(s.ctx, s.log)
	if preview.Image != nil && !mediaSvc.IsTooLarge(preview.Image.ContentLength, preview.Image.ContentLengthHeader) {
		// UploadMedia will close the read stream for the thumbnail
		media, err := mediaSvc.UploadMedia(preview.Image.Data, preview.Image.ContentType, preview.Image.Filename, forUserId, onHost)
		if err != nil {
			s.log.Warn("Non-fatal error storing preview thumbnail: " + err.Error())
		} else {
			img, err := imaging.Open(media.Location)
			if err != nil {
				s.log.Warn("Non-fatal error getting thumbnail dimensions: " + err.Error())
			} else {
				result.ImageMxc = media.MxcUri()
				result.ImageType = media.ContentType
				result.ImageSize = media.SizeBytes
				result.ImageWidth = img.Bounds().Max.X
				result.ImageHeight = img.Bounds().Max.Y
			}
		}
	}

	dbRecord := &types.CachedUrlPreview{
		Preview:   result,
		SearchUrl: urlStr,
		ErrorCode: "",
		FetchedTs: util.NowMillis(),
	}
	err = s.store.InsertPreview(dbRecord)
	if err != nil {
		s.log.Warn("Error caching URL preview: " + err.Error())
	}

	return result, nil
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
