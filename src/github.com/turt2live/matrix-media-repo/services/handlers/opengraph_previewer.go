package handlers

import (
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/dyatlov/go-opengraph/opengraph"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type OpenGraphResult struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	ImageMxc    string
	ImageType   string
	ImageSize   int64
	ImageWidth  int
	ImageHeight int
}

type OpenGraphUrlPreviewer struct {
	Info rcontext.RequestInfo
}

func (p *OpenGraphUrlPreviewer) GeneratePreview(url string, onHost string, forUserId string) (OpenGraphResult, error) {
	html, err := downloadContent(url, p.Info.Config, p.Info.Log)
	if err != nil {
		p.Info.Log.Error("Error downloading content: " + err.Error())

		// We'll consider it not found for the sake of processing
		return OpenGraphResult{}, util.ErrMediaNotFound
	}

	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(strings.NewReader(html))
	if err != nil {
		p.Info.Log.Error("Error getting OpenGraph: " + err.Error())
		return OpenGraphResult{}, err
	}

	graph := &OpenGraphResult{
		Type:        og.Type,
		Url:         og.URL,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}

	if og.Images != nil && len(og.Images) > 0 {
		media, err := downloadImage(og.Images[0].URL, onHost, forUserId, p.Info)
		if err != nil {
			p.Info.Log.Error("Non-fatal error getting thumbnail: " + err.Error())
		} else {
			img, err := imaging.Open(media.Location)
			if err != nil {
				p.Info.Log.Error("Non-fatal error getting thumbnail dimensions: " + err.Error())
			} else {
				graph.ImageMxc = util.MediaToMxc(&media)
				graph.ImageSize = media.SizeBytes
				graph.ImageType = media.ContentType
				graph.ImageWidth = img.Bounds().Max.X
				graph.ImageHeight = img.Bounds().Max.Y
			}
		}
	}

	return *graph, nil
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

func downloadImage(imageUrl string, host string, userId string, i rcontext.RequestInfo) (types.Media, error) {
	i.Log.Info("Getting image from " + imageUrl)
	resp, err := http.Get(imageUrl)
	if err != nil {
		return types.Media{}, err
	}
	if resp.StatusCode != http.StatusOK {
		i.Log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return types.Media{}, errors.New("error during transfer")
	}

	var reader io.Reader
	reader = resp.Body
	if i.Config.Uploads.MaxSizeBytes > 0 {
		reader = io.LimitReader(resp.Body, i.Config.Uploads.MaxSizeBytes)
	}

	request := &MediaUploadRequest{
		ContentType: resp.Header.Get("Content-Type"),
		Host:        host,
		Contents:    reader,
		UploadedBy:  userId,
	}
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	return request.StoreMedia(i)
}
