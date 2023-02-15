package r0

import (
	"bytes"
	"crypto/md5"
	"image/color"
	"io"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"

	"github.com/cupcake/sigil/gen"
	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func Identicon(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Identicons.Enabled {
		return _responses.NotFoundError()
	}

	seed := _routers.GetParam("seed", r)

	var err error
	width := 96
	height := 96

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	if widthStr != "" {
		width, err = strconv.Atoi(widthStr)
		if err != nil {
			return _responses.InternalServerError("Error parsing width: " + err.Error())
		}
		height = width
	}
	if heightStr != "" {
		height, err = strconv.Atoi(heightStr)
		if err != nil {
			return _responses.InternalServerError("Error parsing height: " + err.Error())
		}
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"identiconWidth":  width,
		"identiconHeight": height,
		"identiconSeed":   seed,
	})

	m := md5.New()
	m.Write([]byte(seed))
	hashed := m.Sum(nil)

	sig := &gen.Sigil{
		Rows:       5,
		Background: rgb(224, 224, 224),
		Foreground: []color.NRGBA{
			rgb(45, 79, 255),
			rgb(254, 180, 44),
			rgb(226, 121, 234),
			rgb(30, 179, 253),
			rgb(232, 77, 65),
			rgb(49, 203, 115),
			rgb(141, 69, 170),
		},
	}

	rctx.Log.Info("Generating identicon")
	img := sig.Make(width, false, []byte(hashed))
	if width != height {
		// Resize to the desired height
		rctx.Log.Info("Resizing image to fit height")
		img = imaging.Resize(img, width, height, imaging.Lanczos)
	}

	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, img, imaging.PNG)
	if err != nil {
		rctx.Log.Error("Error generating image:" + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error generating identicon")
	}

	return &_responses.DownloadResponse{
		ContentType:       "image/png",
		Filename:          string(hashed) + ".png",
		SizeBytes:         0,
		Data:              io.NopCloser(imgData),
		TargetDisposition: "inline",
	}
}

func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}
