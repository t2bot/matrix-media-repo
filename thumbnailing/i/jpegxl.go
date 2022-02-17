package i

import (
	"errors"
	"github.com/turt2live/matrix-media-repo/util/ids"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type jpegxlGenerator struct {
}

func (d jpegxlGenerator) supportedContentTypes() []string {
	return []string{"image/jxl"}
}

func (d jpegxlGenerator) supportsAnimation() bool {
	return false
}

func (d jpegxlGenerator) matches(img []byte, contentType string) bool {
	return contentType == "image/jxl"
}

func (d jpegxlGenerator) GetOriginDimensions(b []byte, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d jpegxlGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	key, err := ids.NewUniqueId()
	if err != nil {
		return nil, errors.New("jpegxl: error generating temp key: " + err.Error())
	}

	tempFile1 := path.Join(os.TempDir(), "media_repo."+key+".1.jpegxl")
	tempFile2 := path.Join(os.TempDir(), "media_repo."+key+".2.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return nil, errors.New("jpegxl: error writing temp jpegxl file: " + err.Error())
	}
	_, _ = f.Write(b)
	cleanup.DumpAndCloseStream(f)

	err = exec.Command("convert", tempFile1, tempFile2).Run()
	if err != nil {
		return nil, errors.New("jpegxl: error converting jpegxl file: " + err.Error())
	}

	b, err = ioutil.ReadFile(tempFile2)
	if err != nil {
		return nil, errors.New("jpegxl: error reading temp png file: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnail(b, "image/png", width, height, method, false, ctx)
}

func init() {
	generators = append(generators, jpegxlGenerator{})
}
