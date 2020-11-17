package i

import (
	"bytes"
	"errors"
	_ "image/jpeg"
	"io/ioutil"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
	"github.com/turt2live/matrix-media-repo/util"
)

type jpgGenerator struct {
}

func (d jpgGenerator) supportedContentTypes() []string {
	return []string{"image/jpeg", "image/jpg"}
}

func (d jpgGenerator) supportsAnimation() bool {
	return false
}

func (d jpgGenerator) matches(img []byte, contentType string) bool {
	return util.ArrayContains(d.supportedContentTypes(), contentType)
}

func (d jpgGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := imaging.Decode(bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("jpg: error decoding thumbnail: " + err.Error())
	}

	var shouldThumbnail bool
	shouldThumbnail, width, height, animated, method = u.AdjustProperties(src, width, height, animated, false, method)
	if !shouldThumbnail {
		return nil, nil
	}

	thumb, err := u.MakeThumbnail(src, method, width, height)
	if err != nil {
		return nil, errors.New("jpg: error making thumbnail: " + err.Error())
	}

	thumb, err = u.IdentifyAndApplyOrientation(b, thumb)
	if err != nil {
		return nil, errors.New("jpg: error applying orientation: " + err.Error())
	}

	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, thumb, imaging.JPEG)
	if err != nil {
		return nil, errors.New("jpg: error encoding thumbnail: " + err.Error())
	}
	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/jpeg",
		Reader:      ioutil.NopCloser(imgData),
	}, nil
}

func init() {
	generators = append(generators, jpgGenerator{})
}
