package r0

import (
	"crypto/md5"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"strconv"

	"github.com/cupcake/sigil/gen"
	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/api/routers"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func Identicon(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	if !rctx.Config.Identicons.Enabled {
		return responses.NotFoundError()
	}

	seed := routers.GetParam("seed", r)

	var err error
	width := 96
	height := 96

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	if widthStr != "" {
		width, err = strconv.Atoi(widthStr)
		if err != nil {
			return responses.InternalServerError(fmt.Errorf("Error parsing width: %w", err))
		}
		height = width
	}
	if heightStr != "" {
		height, err = strconv.Atoi(heightStr)
		if err != nil {
			return responses.InternalServerError(fmt.Errorf("Error parsing height: %w", err))
		}
	}

	clamp := func(v int) int {
		if v > 512 {
			return 512
		}
		if v < 96 {
			return 96
		}
		return v
	}
	width = clamp(width)
	height = clamp(height)

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
	img := sig.Make(width, false, hashed)
	if width != height {
		// Resize to the desired height
		rctx.Log.Info("Resizing image to fit height")
		img = imaging.Resize(img, width, height, imaging.Lanczos)
	}

	pr, pw := io.Pipe()
	go func() {
		// dev note: we specifically hardcode this to PNG for ease of return type later (don't use u.Encode())
		err = imaging.Encode(pw, img, imaging.PNG)
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	return &responses.DownloadResponse{
		ContentType:       "image/png",
		Filename:          string(hashed) + ".png",
		SizeBytes:         0,
		Data:              pr,
		TargetDisposition: "inline",
	}
}

func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}
