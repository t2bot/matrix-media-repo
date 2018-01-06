package services

import (
	"fmt"
	"net"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type UrlService struct {
	//store *stores.UrlStore
	i rcontext.RequestInfo
}

func CreateUrlService(i rcontext.RequestInfo) (*UrlService) {
	return &UrlService{i}
}

func (s *UrlService) GetPreview(urlStr string, onHost string, forUserId string) (types.UrlPreview, error) {
	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		s.i.Log.Error("Error parsing url: " + err.Error())
		return types.UrlPreview{}, util.ErrInvalidHost
	}

	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		s.i.Log.Error("Error getting host info: " + err.Error())
		return types.UrlPreview{}, util.ErrInvalidHost
	}
	if len(addrs) == 0 {
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
		return types.UrlPreview{}, util.ErrHostBlacklisted
	}

	s.i.Log = s.i.Log.WithFields(logrus.Fields{
		"previewer": "OpenGraph",
	})

	previewer := &handlers.OpenGraphUrlPreviewer{Info: s.i}
	preview, err := previewer.GeneratePreview(urlStr, onHost, forUserId)
	if err != nil {
		return types.UrlPreview{}, err
	}

	// TODO: Store URL preview in db

	result := &types.UrlPreview{
		Url:         preview.Url,
		SiteName:    preview.SiteName,
		Type:        preview.Type,
		Description: preview.Description,
		Title:       preview.Title,
		ImageMxc:    preview.ImageMxc,
		ImageType:   preview.ImageType,
		ImageSize:   preview.ImageSize,
		ImageWidth:  preview.ImageWidth,
		ImageHeight: preview.ImageHeight,
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
