package r0

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/dyatlov/go-opengraph/opengraph"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/media_handler"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MatrixOpenGraph struct {
	Url string `json:"og:url,omitempty"`
	SiteName string `json:"og:site_name,omitempty"`
	Type string `json:"og:type,omitempty"`
	Description string `json:"og:description,omitempty"`
	Title string `json:"og:title,omitempty"`
	ImageMxc string `json:"og:image,omitempty"`
	ImageType string `json:"og:image:type,omitempty"`
	ImageSize int64 `json:"matrix:image:size,omitempty"`
	ImageWidth int `json:"og:image:width,omitempty"`
	ImageHeight int `json:"og:image:height,omitempty"`
}

func PreviewUrl(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig, log *logrus.Entry) interface{} {
	if !c.UrlPreviews.Enabled {
		return client.NotFoundError()
	}

	accessToken := util.GetAccessTokenFromRequest(r)
	userId, err := util.GetUserIdFromToken(r.Context(), r.Host, accessToken, c)
	if err != nil || userId == "" {
		return client.AuthFailed()
	}

	params := r.URL.Query()

	// Parse the parameters
	urlStr := params.Get("url")
	tsStr := params.Get("ts")
	if tsStr == "" {
		tsStr = "0"
	}
	//ts, err := strconv.ParseInt(tsStr, 10, 64)
	//if err != nil {
	//	log.Error("Error parsing ts: " + err.Error())
	//	return client.BadRequest(err.Error())
	//}

	// Validate the URL
	if urlStr == "" {
		return client.BadRequest("No url provided")
	}
	if strings.Index(urlStr, "http://") != 0 && strings.Index(urlStr, "https://") != 0 {
		return client.BadRequest("Scheme not accepted")
	}

	// Parse the URL
	parsedUrl, err := url.ParseRequestURI(urlStr)
	if err != nil {
		log.Error("Error parsing url: " + err.Error())
		return client.BadRequest(err.Error())
	}

	// Get the IP of the host
	addrs, err := net.LookupIP(parsedUrl.Host)
	if err != nil {
		log.Error("Error getting host info: " + err.Error())
		return client.BadRequest(err.Error())
	}
	if len(addrs) == 0 {
		return client.NotFoundError()
	}
	addr := addrs[0]
	addrStr := fmt.Sprintf("%v", addr)[1:]
	addrStr = addrStr[:len(addrStr)-1]

	// Verify the host is in the allowed range
	allowedCidrs := c.UrlPreviews.AllowedNetworks
	if allowedCidrs == nil {
		allowedCidrs = []string{"0.0.0.0/0"}
	}
	deniedCidrs := c.UrlPreviews.DisallowedNetworks
	if deniedCidrs == nil {
		deniedCidrs = []string{}
	}
	if !isAllowed(addr, allowedCidrs, deniedCidrs, log) {
		return client.BadRequest("Host not allowed")
	}

	// Now we can actually parse the URL
	html,err := downloadContent(urlStr, c, log)
	if err != nil {
		log.Error("Error downloading content: " + err.Error())

		// We consider it not found for the sake of processing.
		return client.NotFoundError()
	}
	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(strings.NewReader(html))
	if err != nil {
		log.Error("Error getting OpenGraph: " + err.Error())
		return client.InternalServerError("error getting OpenGraph")
	}

	// Build a response
	resp := &MatrixOpenGraph{
		Type: og.Type,
		Url: og.URL,
		Title: og.Title,
		Description: og.Description,
		SiteName: og.SiteName,
	}

	if og.Images != nil && len(og.Images) > 0 {
		media, err := downloadImage(r.Context(), og.Images[0].URL, r.Host, userId, c, db, log)
		if err != nil {
			log.Error("Non-fatal error getting thumbnail: " + err.Error())
		} else {
			img, err := imaging.Open(media.Location)
			if err != nil {
				log.Error("Non-fatal error getting thumbnail dimensions: " + err.Error())
			} else {
				resp.ImageMxc = util.MediaToMxc(&media)
				resp.ImageSize = media.SizeBytes
				resp.ImageType = media.ContentType
				resp.ImageWidth = img.Bounds().Max.X
				resp.ImageHeight = img.Bounds().Max.Y
			}
		}
	}

	return resp
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

func downloadContent(urlStr string, c config.MediaRepoConfig, log *logrus.Entry) (string, error) {
	log.Info("Fetching remote content...")
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return "", errors.New("error during transfer")
	}

	var reader io.Reader
	reader = resp.Body
	if c.UrlPreviews.MaxPageSizeBytes > 0 {
		reader = io.LimitReader(resp.Body, c.UrlPreviews.MaxPageSizeBytes)
	}

	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}

	html := string(bytes)
	defer resp.Body.Close()

	return html, nil
}

func downloadImage(ctx context.Context, imageUrl string, host string, userId string, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (types.Media, error) {
	log.Info("Getting image from " + imageUrl)
	resp, err := http.Get(imageUrl)
	if err != nil {
		return types.Media{}, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return types.Media{}, errors.New("error during transfer")
	}

	var reader io.Reader
	reader = resp.Body
	if c.Uploads.MaxSizeBytes > 0 {
		reader = io.LimitReader(resp.Body, c.Uploads.MaxSizeBytes)
	}

	request := &media_handler.MediaUploadRequest{
		Host: host,
		UploadedBy: userId,
		ContentType: resp.Header.Get("Content-Type"),
		Contents: reader,
	}
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	return request.StoreMedia(ctx, c, db, log)
}