package services

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/urlpreview"
)

type UrlService struct {
	//store *stores.UrlStore
	i rcontext.RequestInfo
}

func CreateUrlService(i rcontext.RequestInfo) (*UrlService) {
	return &UrlService{i}
}

func returnCachedPreview(cached *types.CachedUrlPreview) (types.UrlPreview, error) {
	if cached.ErrorCode == urlpreview.ErrCodeInvalidHost {
		return types.UrlPreview{}, util.ErrInvalidHost
	} else if cached.ErrorCode == urlpreview.ErrCodeHostNotFound {
		return types.UrlPreview{}, util.ErrHostNotFound
	} else if cached.ErrorCode == urlpreview.ErrCodeHostBlacklisted {
		return types.UrlPreview{}, util.ErrHostBlacklisted
	} else if cached.ErrorCode == urlpreview.ErrCodeNotFound {
		return types.UrlPreview{}, util.ErrMediaNotFound
	} else if cached.ErrorCode == urlpreview.ErrCodeUnknown {
		return types.UrlPreview{}, errors.New("unknown error")
	}

	return *cached.Preview, nil
}

func (s *UrlService) GetPreview(urlStr string, onHost string, forUserId string, atTs int64) (types.UrlPreview, error) {
	s.i.Log = s.i.Log.WithFields(logrus.Fields{
		"urlService_ts": atTs,
	})

	urlStore := s.i.Db.GetUrlStore(s.i.Context, s.i.Log)
	cached, err := urlStore.GetPreview(urlStr, atTs)
	if err != nil {
		s.i.Log.Error("Error getting cached URL: " + err.Error())
	}
	if err != nil && err != sql.ErrNoRows {
		return types.UrlPreview{}, err
	}
	if err != sql.ErrNoRows {
		s.i.Log.Info("Returning cached URL preview")
		return returnCachedPreview(cached)
	}

	now := util.NowMillis()
	if (now - atTs) > 60000 { // this is to avoid infinite loops (60s window for slow execution)
		// Original bucket failed - try getting the preview for "now"
		return s.GetPreview(urlStr, onHost, forUserId, now)
	}

	s.i.Log.Info("URL preview not cached - fetching resource")

	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		s.i.Log.Error("Error parsing url: " + err.Error())
		urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeInvalidHost)
		return types.UrlPreview{}, util.ErrInvalidHost
	}

	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		s.i.Log.Error("Error getting host info: " + err.Error())
		urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeInvalidHost)
		return types.UrlPreview{}, util.ErrInvalidHost
	}
	if len(addrs) == 0 {
		urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeHostNotFound)
		return types.UrlPreview{}, util.ErrHostNotFound
	}
	addr := addrs[0]
	addrStr := fmt.Sprintf("%v", addr)[1:]
	addrStr = addrStr[:len(addrStr)-1]

	// Verify the host is in the allowed range
	allowedCidrs := s.i.Config.UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := s.i.Config.UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}
	if !isAllowed(addr, allowedCidrs, deniedCidrs, s.i.Log) {
		urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeHostBlacklisted)
		return types.UrlPreview{}, util.ErrHostBlacklisted
	}

	s.i.Log = s.i.Log.WithFields(logrus.Fields{
		"previewer": "OpenGraph",
	})

	previewer := &handlers.OpenGraphUrlPreviewer{Info: s.i}
	preview, err := previewer.GeneratePreview(urlStr)
	if err != nil {
		if err == util.ErrMediaNotFound {
			urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeNotFound)
		} else {
			urlStore.InsertPreviewError(urlStr, urlpreview.ErrCodeUnknown)
		}
		return types.UrlPreview{}, err
	}

	result := &types.UrlPreview{
		Url:         preview.Url,
		SiteName:    preview.SiteName,
		Type:        preview.Type,
		Description: preview.Description,
		Title:       preview.Title,
	}

	// Store the thumbnail, if there is one
	if preview.HasImage {
		mediaSvc := CreateMediaService(s.i)
		media, err := mediaSvc.UploadMedia(preview.Image.Data, preview.Image.ContentType, preview.Image.Filename, forUserId, onHost)
		if err != nil {
			s.i.Log.Warn("Non-fatal error storing preview thumbnail: " + err.Error())
		} else {
			img, err := imaging.Open(media.Location)
			if err != nil {
				s.i.Log.Warn("Non-fatal error getting thumbnail dimensions: " + err.Error())
			} else {
				result.ImageMxc = util.MediaToMxc(&media)
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
	err = urlStore.InsertPreview(dbRecord)
	if err != nil {
		s.i.Log.Warn("Error caching URL preview: " + err.Error())
	}

	return *result, nil
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
