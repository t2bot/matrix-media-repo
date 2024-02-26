package i

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
)

type jpegxlGenerator struct {
}

func (d jpegxlGenerator) supportedContentTypes() []string {
	return []string{"image/jxl"}
}

func (d jpegxlGenerator) supportsAnimation() bool {
	return false
}

func (d jpegxlGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/jxl"
}

func (d jpegxlGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d jpegxlGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "mmr-jpegxl")
	if err != nil {
		return nil, fmt.Errorf("jpegxl: error creating temporary directory: %w", err)
	}

	tempFile1 := path.Join(dir, "i.jpegxl")
	tempFile2 := path.Join(dir, "o.png")

	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	defer os.Remove(dir)

	f, err := os.OpenFile(tempFile1, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return nil, fmt.Errorf("jpegxl: error creating temp jpegxl file: %w", err)
	}
	if _, err = io.Copy(f, b); err != nil {
		return nil, fmt.Errorf("jpegxl: error writing temp jpegxl file: %w", err)
	}

	err = exec.Command("convert", tempFile1, tempFile2).Run()
	if err != nil {
		return nil, fmt.Errorf("jpegxl: error converting jpegxl file: %w", err)
	}

	f, err = os.OpenFile(tempFile2, os.O_RDONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("jpegxl: error reading temp png file: %w", err)
	}
	defer f.Close()

	return pngGenerator{}.GenerateThumbnail(f, "image/png", width, height, method, false, ctx)
}

func init() {
	generators = append(generators, jpegxlGenerator{})
}
