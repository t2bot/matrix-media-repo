package i

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
)

type mp4Generator struct {
}

func (d mp4Generator) supportedContentTypes() []string {
	return []string{"video/mp4"}
}

func (d mp4Generator) supportsAnimation() bool {
	return false
}

func (d mp4Generator) matches(img io.Reader, contentType string) bool {
	return util.ArrayContains(d.supportedContentTypes(), contentType)
}

func (d mp4Generator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d mp4Generator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "mmr-mp4")
	if err != nil {
		return nil, errors.New("mp4: error creating temporary directory: " + err.Error())
	}

	tempFile1 := path.Join(dir, "i.mp4")
	tempFile2 := path.Join(dir, "o.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	defer os.Remove(dir)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return nil, errors.New("mp4: error creating temp video file: " + err.Error())
	}
	if _, err = io.Copy(f, b); err != nil {
		return nil, errors.New("mp4: error writing temp video file: " + err.Error())
	}

	err = exec.Command("ffmpeg", "-i", tempFile1, "-vf", "select=eq(n\\,0)", tempFile2).Run()
	if err != nil {
		return nil, errors.New("mp4: error converting video file: " + err.Error())
	}

	f, err = os.OpenFile(tempFile2, os.O_RDONLY, 0640)
	if err != nil {
		return nil, errors.New("mp4: error reading temp png file: " + err.Error())
	}
	defer f.Close()

	return pngGenerator{}.GenerateThumbnail(f, "image/png", width, height, method, false, ctx)
}

func init() {
	generators = append(generators, mp4Generator{})
}
