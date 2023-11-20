package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/turt2live/matrix-media-repo/plugins/plugin_common"
	"github.com/turt2live/matrix-media-repo/plugins/plugin_interfaces"
	"github.com/turt2live/matrix-media-repo/util"
)

type AntispamOCR struct {
	logger hclog.Logger
	config map[string]interface{}

	userIdRegex   *regexp.Regexp
	contentTypes  []string
	minSize       int
	maxSize       int
	keywordGroups [][]string
	ocrServer     string
	topPercentage float64
}

func (a *AntispamOCR) HandleConfig(config map[string]interface{}) error {
	a.config = config

	a.ocrServer = a.config["ocrServer"].(string)
	a.minSize = int(a.config["minSizeBytes"].(float64))
	a.maxSize = int(a.config["maxSizeBytes"].(float64))
	a.topPercentage = a.config["percentageOfHeight"].(float64)

	ctypes := make([]string, 0)
	for _, t := range a.config["types"].([]interface{}) {
		ctypes = append(ctypes, fmt.Sprintf("%v", t))
	}
	a.contentTypes = ctypes

	kwg := make([][]string, 0)
	for _, c := range a.config["keywordGroups"].([]interface{}) {
		kwg2 := make([]string, 0)
		for _, kw := range c.([]interface{}) {
			kwg2 = append(kwg2, fmt.Sprintf("%v", kw))
		}
		kwg = append(kwg, kwg2)
	}
	a.keywordGroups = kwg

	r, err := regexp.Compile(a.config["userIds"].(string))
	if err != nil {
		return err
	}
	a.userIdRegex = r

	return nil
}

func (a *AntispamOCR) CheckForSpam(b64 string, filename string, contentType string, userId string, origin string, mediaId string) (bool, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return false, err
	}

	if len(b) < a.minSize || len(b) > a.maxSize {
		return false, nil
	}
	if !util.ArrayContains(a.contentTypes, contentType) {
		return false, nil
	}
	if !a.userIdRegex.MatchString(userId) {
		return false, nil
	}

	img, _, err := image.Decode(bytes.NewBuffer(b))
	if err != nil {
		return false, err
	}

	// For certain kinds of spam we don't really need to consider the whole image but just the upper third.
	if a.topPercentage < 1.0 && a.topPercentage > 0 {
		newHeight := int(math.Round(float64(img.Bounds().Max.Y) * a.topPercentage))
		img = imaging.Fill(img, img.Bounds().Max.X, newHeight, imaging.Top, imaging.Linear)
	}

	// Steps:
	// 1. Crush the image to reasonable dimensions (helps with later transforms). Use Lanczos to soften lines on letters.
	// 2. Double the image size, using Lanczos again to do a second round of softening.
	// 3. Try to remove any background noise (usually introduced during upload and by resizing).
	// 4. Adjust contrast to make text more obvious on the background.
	// 5. Convert to grayscale, thus avoiding any colour issues with the OCR.
	img = imaging.Fit(img, 512, 512, imaging.Lanczos)
	img = imaging.Fill(img, img.Bounds().Max.X*2, img.Bounds().Max.Y*2, imaging.Top, imaging.Lanczos)
	img = imaging.Sharpen(img, 50)
	img = imaging.AdjustContrast(img, 2)
	img = imaging.Grayscale(img)

	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, img, imaging.PNG) // dev note: deliberately png (don't use u.Encode())
	if err != nil {
		return false, err
	}
	b64 = base64.StdEncoding.EncodeToString(imgData.Bytes())

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"base64": b64,
		"trim":   "\n",
	})
	if err != nil {
		return false, err
	}

	ocrUrl := util.MakeUrl(a.ocrServer, "/base64")
	req, err := http.NewRequest("POST", ocrUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "matrix-media-repo")
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		a.logger.Error("non-fatal error checking spam: ", err)
		return false, nil
	}
	contents, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	var resp map[string]interface{}
	err = json.Unmarshal(contents, &resp)
	if err != nil {
		return false, err
	}
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}
	ocr := strings.ToLower(resp["result"].(string))

	for _, kwg := range a.keywordGroups {
		hasKeyword := false
		for _, kw := range kwg {
			if strings.Contains(ocr, kw) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			return false, nil
		}
	}

	a.logger.Warn("spam detected")
	return true, nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	antispam := &AntispamOCR{logger: logger}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin_common.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"antispam": &plugin_interfaces.AntispamPlugin{Impl: antispam},
		},
	})
}
