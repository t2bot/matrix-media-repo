package r0

import (
	"bytes"
	"crypto/md5"
	"image/color"
	"io"
	"net/http"
	"strconv"

	"github.com/cupcake/sigil/gen"
	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/util"
)

type IdenticonResponse struct {
	Avatar io.Reader
}

func Identicon(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	hs := util.GetHomeserverConfig(r.Host)
	if hs.DownloadRequiresAuth && user.UserId == "" {
		log.Warn("Homeserver requires authenticated downloads - denying request")
		return api.AuthFailed()
	}

	if !config.Get().Identicons.Enabled {
		return api.NotFoundError()
	}

	params := mux.Vars(r)
	seed := params["seed"]

	var err error
	width := 96
	height := 96

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	if widthStr != "" {
		width, err = strconv.Atoi(widthStr)
		if err != nil {
			return api.InternalServerError("Error parsing width: " + err.Error())
		}
		height = width
	}
	if heightStr != "" {
		height, err = strconv.Atoi(heightStr)
		if err != nil {
			return api.InternalServerError("Error parsing height: " + err.Error())
		}
	}

	log = log.WithFields(logrus.Fields{
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

	log.Info("Generating identicon")
	img := sig.Make(width, false, []byte(hashed))
	if width != height {
		// Resize to the desired height
		log.Info("Resizing image to fit height")
		img = imaging.Resize(img, width, height, imaging.Lanczos)
	}

	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, img, imaging.PNG)
	if err != nil {
		log.Error("Error generating image:" + err.Error())
		return api.InternalServerError("error generating identicon")
	}

	return &IdenticonResponse{Avatar: imgData}
}

func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}
